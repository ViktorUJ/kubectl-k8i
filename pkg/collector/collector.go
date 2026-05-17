package collector

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/kubectl-k8i/pkg/age"
	"github.com/kubectl-k8i/pkg/debug"
	"github.com/kubectl-k8i/pkg/labels"
	"github.com/kubectl-k8i/pkg/model"
	"github.com/kubectl-k8i/pkg/parser"
	"github.com/kubectl-k8i/pkg/retry"
	"github.com/kubectl-k8i/pkg/taints"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsv1beta1client "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
)

// nodeclaimGVR is the GroupVersionResource for Karpenter nodeclaims.
var nodeclaimGVR = schema.GroupVersionResource{
	Group:    "karpenter.sh",
	Version:  "v1",
	Resource: "nodeclaims",
}

// Collector fetches data from the Kubernetes API concurrently.
type Collector struct {
	clientset        kubernetes.Interface
	metricsClient    metricsv1beta1client.MetricsV1beta1Interface
	dynamicClient    dynamic.Interface
	labelSelector    string
	progressReporter func(step, total int, message string)
	retryConfig      retry.RetryConfig
	debugLogger      *debug.DebugLogger
}

// NewCollector creates a new Collector with the given dependencies.
func NewCollector(
	clientset kubernetes.Interface,
	metricsClient metricsv1beta1client.MetricsV1beta1Interface,
	dynamicClient dynamic.Interface,
	labelSelector string,
	progressReporter func(step, total int, message string),
	retryConfig retry.RetryConfig,
	debugLogger *debug.DebugLogger,
) *Collector {
	return &Collector{
		clientset:        clientset,
		metricsClient:    metricsClient,
		dynamicClient:    dynamicClient,
		labelSelector:    labelSelector,
		progressReporter: progressReporter,
		retryConfig:      retryConfig,
		debugLogger:      debugLogger,
	}
}

// Collect fetches all data concurrently using errgroup.
// Each API call is wrapped with retry.WithRetry for transient error handling.
// Nodeclaim failures are non-fatal (404 is OK).
func (c *Collector) Collect(ctx context.Context) (*model.ClusterData, error) {
	const totalSteps = 4

	var (
		nodeList      *corev1.NodeList
		metricsList   *metricsv1beta1.NodeMetricsList
		podList       *corev1.PodList
		nodeclaimList []unstructured.Unstructured
	)

	g, gctx := errgroup.WithContext(ctx)

	// Goroutine 1: List nodes (with optional label selector).
	g.Go(func() error {
		c.reportProgress(1, totalSteps, "Fetching nodes...")
		var err error
		retryErr := retry.WithRetry(gctx, c.retryConfig, "list-nodes", func() error {
			start := time.Now()
			nodeList, err = c.clientset.CoreV1().Nodes().List(gctx, metav1.ListOptions{
				LabelSelector: c.labelSelector,
			})
			if c.debugLogger != nil {
				c.debugLogger.LogAPICall("GET", "nodes", "", time.Since(start), 200)
			}
			return err
		})
		if retryErr != nil {
			return fmt.Errorf("failed to list nodes: %w", retryErr)
		}
		if c.debugLogger != nil {
			c.debugLogger.LogDataProcessing("nodes_fetched", len(nodeList.Items))
		}
		return nil
	})

	// Goroutine 2: List node metrics from metrics API.
	g.Go(func() error {
		c.reportProgress(2, totalSteps, "Fetching metrics...")
		var err error
		retryErr := retry.WithRetry(gctx, c.retryConfig, "list-metrics", func() error {
			start := time.Now()
			metricsList, err = c.metricsClient.NodeMetricses().List(gctx, metav1.ListOptions{})
			if c.debugLogger != nil {
				c.debugLogger.LogAPICall("GET", "metrics", "", time.Since(start), 200)
			}
			return err
		})
		if retryErr != nil {
			// Metrics unavailable is non-fatal — we continue with zero usage values.
			metricsList = nil
			if c.debugLogger != nil {
				c.debugLogger.LogDataProcessing("metrics_unavailable", 0)
			}
			return nil
		}
		if c.debugLogger != nil && metricsList != nil {
			c.debugLogger.LogDataProcessing("metrics_fetched", len(metricsList.Items))
		}
		return nil
	})

	// Goroutine 3: List running pods (field selector status.phase=Running).
	g.Go(func() error {
		c.reportProgress(3, totalSteps, "Fetching pods...")
		var err error
		retryErr := retry.WithRetry(gctx, c.retryConfig, "list-pods", func() error {
			start := time.Now()
			podList, err = c.clientset.CoreV1().Pods("").List(gctx, metav1.ListOptions{
				FieldSelector: "status.phase=Running",
			})
			if c.debugLogger != nil {
				c.debugLogger.LogAPICall("GET", "pods", "", time.Since(start), 200)
			}
			return err
		})
		if retryErr != nil {
			return fmt.Errorf("failed to list pods: %w", retryErr)
		}
		if c.debugLogger != nil {
			c.debugLogger.LogDataProcessing("pods_fetched", len(podList.Items))
		}
		return nil
	})

	// Goroutine 4: List nodeclaims via dynamic client (non-fatal if CRD missing).
	g.Go(func() error {
		c.reportProgress(4, totalSteps, "Fetching nodeclaims...")
		if c.dynamicClient == nil {
			return nil
		}
		var result *unstructured.UnstructuredList
		retryErr := retry.WithRetry(gctx, c.retryConfig, "list-nodeclaims", func() error {
			start := time.Now()
			var err error
			result, err = c.dynamicClient.Resource(nodeclaimGVR).List(gctx, metav1.ListOptions{})
			if c.debugLogger != nil {
				c.debugLogger.LogAPICall("GET", "nodeclaims", "", time.Since(start), 200)
			}
			return err
		})
		if retryErr != nil {
			// Nodeclaim CRD missing (404) or other error — non-fatal.
			if c.debugLogger != nil {
				c.debugLogger.LogDataProcessing("nodeclaims_unavailable", 0)
			}
			return nil
		}
		if result != nil {
			nodeclaimList = result.Items
		}
		if c.debugLogger != nil {
			c.debugLogger.LogDataProcessing("nodeclaims_fetched", len(nodeclaimList))
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	data := &model.ClusterData{
		Nodes:      nodeList.Items,
		Nodeclaims: nodeclaimList,
	}
	if metricsList != nil {
		data.Metrics = metricsList.Items
	}
	if podList != nil {
		data.Pods = podList.Items
	}

	return data, nil
}

// EnrichNodes processes raw ClusterData into a slice of enriched NodeInfo.
// It builds lookup maps for O(1) access, filters only Ready nodes,
// and populates all fields on each NodeInfo.
func (c *Collector) EnrichNodes(data *model.ClusterData, now time.Time) []model.NodeInfo {
	// Build podsByNode map: node name → aggregated pod resources.
	podsByNode := buildPodsByNode(data.Pods)

	// Build metricsByNode map: node name → NodeMetrics.
	metricsByNode := buildMetricsByNode(data.Metrics)

	// Build nodeclaimByNode map: node name → nodeclaim name.
	nodeclaimByNode := buildNodeclaimByNode(data.Nodeclaims)

	if c.debugLogger != nil {
		c.debugLogger.LogDataProcessing("pods_by_node_map", len(podsByNode))
		c.debugLogger.LogDataProcessing("metrics_by_node_map", len(metricsByNode))
		c.debugLogger.LogDataProcessing("nodeclaim_by_node_map", len(nodeclaimByNode))
	}

	// Filter only Ready nodes and enrich.
	var nodes []model.NodeInfo
	for i := range data.Nodes {
		node := &data.Nodes[i]
		if !isNodeReady(node) {
			continue
		}

		info := enrichNode(node, podsByNode, metricsByNode, nodeclaimByNode, now)
		nodes = append(nodes, info)
	}

	if c.debugLogger != nil {
		c.debugLogger.LogDataProcessing("ready_nodes", len(nodes))
	}

	return nodes
}

// reportProgress calls the progress reporter if set.
func (c *Collector) reportProgress(step, total int, message string) {
	if c.progressReporter != nil {
		c.progressReporter(step, total, message)
	}
}

// NodeNamesForDeployment returns the set of node names that have at least one
// running pod belonging to the given deployment (namespace/name).
// It resolves the deployment → ReplicaSet → Pod chain using label selectors.
func (c *Collector) NodeNamesForDeployment(ctx context.Context, namespace, name string) (map[string]struct{}, error) {
	// Fetch the deployment to get its pod template label selector.
	deployment, err := c.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment %s/%s: %w", namespace, name, err)
	}

	// Build a label selector string from the deployment's selector.
	selector := deployment.Spec.Selector
	if selector == nil || len(selector.MatchLabels) == 0 {
		return nil, fmt.Errorf("deployment %s/%s has no label selector", namespace, name)
	}

	parts := make([]string, 0, len(selector.MatchLabels))
	for k, v := range selector.MatchLabels {
		parts = append(parts, k+"="+v)
	}
	labelSel := strings.Join(parts, ",")

	// List running pods matching the deployment's selector in its namespace.
	podList, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSel,
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods for deployment %s/%s: %w", namespace, name, err)
	}

	nodeNames := make(map[string]struct{}, len(podList.Items))
	for i := range podList.Items {
		if n := podList.Items[i].Spec.NodeName; n != "" {
			nodeNames[n] = struct{}{}
		}
	}
	return nodeNames, nil
}

// NodeNamesForStatefulSet returns the set of node names that have at least one
// running pod belonging to the given statefulset (namespace/name).
func (c *Collector) NodeNamesForStatefulSet(ctx context.Context, namespace, name string) (map[string]struct{}, error) {
	// Fetch the statefulset to get its pod template label selector.
	sts, err := c.clientset.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get statefulset %s/%s: %w", namespace, name, err)
	}

	selector := sts.Spec.Selector
	if selector == nil || len(selector.MatchLabels) == 0 {
		return nil, fmt.Errorf("statefulset %s/%s has no label selector", namespace, name)
	}

	parts := make([]string, 0, len(selector.MatchLabels))
	for k, v := range selector.MatchLabels {
		parts = append(parts, k+"="+v)
	}
	labelSel := strings.Join(parts, ",")

	podList, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSel,
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods for statefulset %s/%s: %w", namespace, name, err)
	}

	nodeNames := make(map[string]struct{}, len(podList.Items))
	for i := range podList.Items {
		if n := podList.Items[i].Spec.NodeName; n != "" {
			nodeNames[n] = struct{}{}
		}
	}
	return nodeNames, nil
}

// NodeNamesForNamespace returns the set of node names that have at least one
// running pod in the given namespace.
func (c *Collector) NodeNamesForNamespace(ctx context.Context, namespace string) (map[string]struct{}, error) {
	podList, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods in namespace %s: %w", namespace, err)
	}

	nodeNames := make(map[string]struct{}, len(podList.Items))
	for i := range podList.Items {
		if n := podList.Items[i].Spec.NodeName; n != "" {
			nodeNames[n] = struct{}{}
		}
	}
	return nodeNames, nil
}

func buildPodsByNode(pods []corev1.Pod) map[string]*model.PodAggregation {
	result := make(map[string]*model.PodAggregation)
	for i := range pods {
		pod := &pods[i]
		nodeName := pod.Spec.NodeName
		if nodeName == "" {
			continue
		}

		agg, ok := result[nodeName]
		if !ok {
			agg = &model.PodAggregation{}
			result[nodeName] = agg
		}
		agg.PodCount++

		for j := range pod.Spec.Containers {
			container := &pod.Spec.Containers[j]
			requests := container.Resources.Requests
			limits := container.Resources.Limits

			if cpu, ok := requests[corev1.ResourceCPU]; ok {
				agg.CPURequestMilli += parser.ParseCPUMillicores(cpu.String())
			}
			if cpu, ok := limits[corev1.ResourceCPU]; ok {
				agg.CPULimitMilli += parser.ParseCPUMillicores(cpu.String())
			}
			if mem, ok := requests[corev1.ResourceMemory]; ok {
				agg.MemRequestGB += parser.ParseMemory(mem.String())
			}
			if mem, ok := limits[corev1.ResourceMemory]; ok {
				agg.MemLimitGB += parser.ParseMemory(mem.String())
			}
		}
	}
	return result
}

// buildMetricsByNode creates a map from node name to NodeMetrics.
func buildMetricsByNode(metrics []metricsv1beta1.NodeMetrics) map[string]*metricsv1beta1.NodeMetrics {
	result := make(map[string]*metricsv1beta1.NodeMetrics, len(metrics))
	for i := range metrics {
		result[metrics[i].Name] = &metrics[i]
	}
	return result
}

// buildNodeclaimByNode creates a map from node name to nodeclaim name.
// It matches nodeclaims to nodes by checking the nodeclaim's status.nodeName field.
func buildNodeclaimByNode(nodeclaims []unstructured.Unstructured) map[string]string {
	result := make(map[string]string, len(nodeclaims))
	for i := range nodeclaims {
		nc := &nodeclaims[i]
		// Get status.nodeName from the unstructured object.
		status, found, err := unstructured.NestedMap(nc.Object, "status")
		if err != nil || !found {
			continue
		}
		nodeName, ok := status["nodeName"].(string)
		if !ok || nodeName == "" {
			continue
		}
		result[nodeName] = nc.GetName()
	}
	return result
}

// isNodeReady returns true if the node has condition type=Ready with status=True.
func isNodeReady(node *corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

// enrichNode creates a fully populated NodeInfo from a node and lookup maps.
func enrichNode(
	node *corev1.Node,
	podsByNode map[string]*model.PodAggregation,
	metricsByNode map[string]*metricsv1beta1.NodeMetrics,
	nodeclaimByNode map[string]string,
	now time.Time,
) model.NodeInfo {
	name := node.Name

	// Pod usage from podsByNode.
	var podsUsed int
	var cpuReqMilli, cpuLimMilli int64
	var memReqGB, memLimGB float64
	if agg, ok := podsByNode[name]; ok {
		podsUsed = agg.PodCount
		cpuReqMilli = agg.CPURequestMilli
		cpuLimMilli = agg.CPULimitMilli
		memReqGB = agg.MemRequestGB
		memLimGB = agg.MemLimitGB
	}

	// CPU/Memory capacity from node.Status.Capacity.
	cpuCapStr := ""
	memCapStr := ""
	if cpu, ok := node.Status.Capacity[corev1.ResourceCPU]; ok {
		cpuCapStr = cpu.String()
	}
	if mem, ok := node.Status.Capacity[corev1.ResourceMemory]; ok {
		memCapStr = mem.String()
	}
	cpuCapCores := parser.ParseCPU(cpuCapStr)
	cpuCapMilli := parser.ParseCPUMillicores(cpuCapStr)
	memCapGB := parser.ParseMemory(memCapStr)

	// CPU/Memory usage from metrics.
	var cpuUseCores float64
	var cpuUseMilli int64
	var memUseGB float64
	if m, ok := metricsByNode[name]; ok {
		if cpu, ok := m.Usage[corev1.ResourceCPU]; ok {
			cpuUseCores = parser.ParseCPU(cpu.String())
			cpuUseMilli = parser.ParseCPUMillicores(cpu.String())
		}
		if mem, ok := m.Usage[corev1.ResourceMemory]; ok {
			memUseGB = parser.ParseMemory(mem.String())
		}
	}

	// Load percentages.
	cpuLoadPercent := calcLoadPercent(cpuUseMilli, cpuCapMilli)
	memLoadPercent := calcLoadPercentFloat(memUseGB, memCapGB)

	// Metadata from labels.
	metadata := labels.ExtractMetadata(node.Labels, node.Spec.ProviderID)

	// Age.
	nodeAge := age.FormatAge(node.CreationTimestamp.Time, now)

	// Taints.
	nodeTaints := node.Spec.Taints
	taintStr := taints.FormatTaints(nodeTaints)
	taintSortKey := taints.SortKeyFromTaints(nodeTaints)

	// Nodeclaim from map or "x".
	nodeclaim := "x"
	if nc, ok := nodeclaimByNode[name]; ok {
		nodeclaim = nc
	}

	// PodsMax from node.Status.Allocatable "pods" resource.
	var podsMax int
	if pods, ok := node.Status.Allocatable[corev1.ResourcePods]; ok {
		podsMax = int(pods.Value())
	}

	return model.NodeInfo{
		Name:     name,
		PodsUsed: podsUsed,
		PodsMax:  podsMax,

		CPURequestCores:  float64(cpuReqMilli) / 1000.0,
		CPULimitCores:    float64(cpuLimMilli) / 1000.0,
		CPUUsageCores:    cpuUseCores,
		CPUCapacityCores: cpuCapCores,
		CPURequestMilli:  cpuReqMilli,
		CPULimitMilli:    cpuLimMilli,
		CPUUsageMilli:    cpuUseMilli,
		CPUCapacityMilli: cpuCapMilli,
		CPULoadPercent:   cpuLoadPercent,

		MemRequestGB:   memReqGB,
		MemLimitGB:     memLimGB,
		MemUsageGB:     memUseGB,
		MemCapacityGB:  memCapGB,
		MemLoadPercent: memLoadPercent,

		EC2InstanceID: metadata.EC2InstanceID,
		InstanceType:  metadata.InstanceType,
		CapacityType:  metadata.CapacityType,
		Architecture:  metadata.Architecture,
		Zone:          metadata.Zone,
		Nodepool:      metadata.Nodepool,
		Nodeclaim:     nodeclaim,
		Autoscaler:    metadata.Autoscaler,

		CreationTime: node.CreationTimestamp.Time,
		Age:          nodeAge,

		Taints:       nodeTaints,
		TaintStr:     taintStr,
		TaintSortKey: taintSortKey,
	}
}

// calcLoadPercent computes (usage * 100) / capacity for integer millicores.
// Returns 0 if capacity is zero.
func calcLoadPercent(usage, capacity int64) int {
	if capacity == 0 {
		return 0
	}
	return int(math.Round(float64(usage) * 100.0 / float64(capacity)))
}

// calcLoadPercentFloat computes (usage * 100) / capacity for float64 GB values.
// Returns 0 if capacity is zero.
func calcLoadPercentFloat(usage, capacity float64) int {
	if capacity == 0 {
		return 0
	}
	return int(math.Round(usage * 100.0 / capacity))
}
