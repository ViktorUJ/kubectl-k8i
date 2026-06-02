package analyze

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/kubectl-k8i/pkg/labels"
	"github.com/kubectl-k8i/pkg/model"
	"github.com/kubectl-k8i/pkg/parser"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	metricsv1beta1client "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
)

// ownerKey uniquely identifies a workload owner.
type ownerKey struct {
	namespace string
	kind      string
	name      string
}

// workloadAgg accumulates resource totals for one workload.
type workloadAgg struct {
	podCount        int
	cpuRequestMilli int64
	cpuLimitMilli   int64
	cpuUsageMilli   int64
	memRequestGB    float64
	memLimitGB      float64
	memUsageGB      float64
}

// Collector gathers workload data for the analyze subcommand.
type Collector struct {
	clientset     kubernetes.Interface
	metricsClient metricsv1beta1client.MetricsV1beta1Interface
}

// NewCollector creates an analyze Collector.
func NewCollector(
	clientset kubernetes.Interface,
	metricsClient metricsv1beta1client.MetricsV1beta1Interface,
) *Collector {
	return &Collector{
		clientset:     clientset,
		metricsClient: metricsClient,
	}
}

// CollectForNodes resolves the target node set from cfg, then aggregates
// workload resource data for all running pods on those nodes.
// excludeNS is the set of namespaces to skip.
func (c *Collector) CollectForNodes(
	ctx context.Context,
	cfg model.AnalyzeConfig,
) ([]model.WorkloadInfo, error) {
	// 1. Resolve target node names.
	nodeNames, err := c.resolveNodes(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if len(nodeNames) == 0 {
		return nil, fmt.Errorf("no ready nodes matched the given selector")
	}

	// Build exclude set.
	excludeSet := make(map[string]struct{}, len(cfg.ExcludeNamespaces))
	for _, ns := range cfg.ExcludeNamespaces {
		excludeSet[ns] = struct{}{}
	}

	// 2. List running pods. If --namespace is set, scope the query to it.
	podList, err := c.clientset.CoreV1().Pods(cfg.Namespace).List(ctx, metav1.ListOptions{
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	// 3. Fetch pod metrics (non-fatal if unavailable).
	podMetrics := make(map[string]map[string]int64) // namespace/name → container → cpuMilli
	podMemMetrics := make(map[string]map[string]float64)
	if c.metricsClient != nil {
		ml, err := c.metricsClient.PodMetricses("").List(ctx, metav1.ListOptions{})
		if err == nil && ml != nil {
			for i := range ml.Items {
				pm := &ml.Items[i]
				key := pm.Namespace + "/" + pm.Name
				cpuMap := make(map[string]int64)
				memMap := make(map[string]float64)
				for _, c := range pm.Containers {
					if cpu, ok := c.Usage[corev1.ResourceCPU]; ok {
						cpuMap[c.Name] = parser.ParseCPUMillicores(cpu.String())
					}
					if mem, ok := c.Usage[corev1.ResourceMemory]; ok {
						memMap[c.Name] = parser.ParseMemory(mem.String())
					}
				}
				podMetrics[key] = cpuMap
				podMemMetrics[key] = memMap
			}
		}
	}

	// 4. Build ReplicaSet → Deployment map (one List call, no per-pod API calls).
	// Key: "namespace/rsName" → ownerKey of the top-level owner (Deployment or RS itself).
	rsOwnerMap, err := c.buildRSOwnerMap(ctx)
	if err != nil {
		// Non-fatal: if we can't list RS, pods owned by RS will show as "ReplicaSet".
		rsOwnerMap = make(map[string]ownerKey)
	}

	// 5. Aggregate per workload.
	aggs := make(map[ownerKey]*workloadAgg)

	for i := range podList.Items {
		pod := &podList.Items[i]

		// Only pods on target nodes.
		if _, ok := nodeNames[pod.Spec.NodeName]; !ok {
			continue
		}
		// If --namespace is set, only include pods from that namespace.
		if cfg.Namespace != "" && pod.Namespace != cfg.Namespace {
			continue
		}
		// Skip excluded namespaces.
		if _, excluded := excludeSet[pod.Namespace]; excluded {
			continue
		}

		owner := resolveTopOwner(pod, rsOwnerMap)

		agg, exists := aggs[owner]
		if !exists {
			agg = &workloadAgg{}
			aggs[owner] = agg
		}
		agg.podCount++

		// Sum container requests/limits.
		for j := range pod.Spec.Containers {
			ctr := &pod.Spec.Containers[j]
			if cpu, ok := ctr.Resources.Requests[corev1.ResourceCPU]; ok {
				agg.cpuRequestMilli += parser.ParseCPUMillicores(cpu.String())
			}
			if cpu, ok := ctr.Resources.Limits[corev1.ResourceCPU]; ok {
				agg.cpuLimitMilli += parser.ParseCPUMillicores(cpu.String())
			}
			if mem, ok := ctr.Resources.Requests[corev1.ResourceMemory]; ok {
				agg.memRequestGB += parser.ParseMemory(mem.String())
			}
			if mem, ok := ctr.Resources.Limits[corev1.ResourceMemory]; ok {
				agg.memLimitGB += parser.ParseMemory(mem.String())
			}
		}

		// Sum metrics usage.
		podKey := pod.Namespace + "/" + pod.Name
		if cpuMap, ok := podMetrics[podKey]; ok {
			for _, v := range cpuMap {
				agg.cpuUsageMilli += v
			}
		}
		if memMap, ok := podMemMetrics[podKey]; ok {
			for _, v := range memMap {
				agg.memUsageGB += v
			}
		}
	}

	// 6. Convert to WorkloadInfo slice, compute overcommit, apply thresholds, sort.
	result := make([]model.WorkloadInfo, 0, len(aggs))
	for k, v := range aggs {
		cpuReq := float64(v.cpuRequestMilli) / 1000.0
		cpuLim := float64(v.cpuLimitMilli) / 1000.0

		wl := model.WorkloadInfo{
			Namespace:        k.namespace,
			Kind:             k.kind,
			Name:             k.name,
			PodCount:         v.podCount,
			CPURequestCores:  cpuReq,
			CPULimitCores:    cpuLim,
			CPUUsageCores:    float64(v.cpuUsageMilli) / 1000.0,
			MemRequestGB:     v.memRequestGB,
			MemLimitGB:       v.memLimitGB,
			MemUsageGB:       v.memUsageGB,
			CPUOvercommitPct: overcommitPct(cpuReq, cpuLim),
			MemOvercommitPct: overcommitPct(v.memRequestGB, v.memLimitGB),
		}

		// Apply overcommit thresholds. When both are set, a workload must
		// exceed BOTH thresholds to be included (logical AND).
		if !passesOvercommitThresholds(wl, cfg) {
			continue
		}

		result = append(result, wl)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Namespace != result[j].Namespace {
			return result[i].Namespace < result[j].Namespace
		}
		if result[i].Kind != result[j].Kind {
			return result[i].Kind < result[j].Kind
		}
		return result[i].Name < result[j].Name
	})

	return result, nil
}

// overcommitPct returns the overcommit percentage (limit - request) / request * 100.
// Returns -1 when request is zero (not applicable).
func overcommitPct(request, limit float64) float64 {
	if request <= 0 {
		return -1
	}
	return (limit - request) / request * 100.0
}

// passesOvercommitThresholds reports whether a workload satisfies the overcommit
// thresholds configured in cfg. When both CPU and memory thresholds are set,
// the workload must exceed both (logical AND). When neither is set, all pass.
func passesOvercommitThresholds(wl model.WorkloadInfo, cfg model.AnalyzeConfig) bool {
	if cfg.CPUOvercommitSet {
		if wl.CPUOvercommitPct < 0 || wl.CPUOvercommitPct < cfg.CPUOvercommit {
			return false
		}
	}
	if cfg.MemOvercommitSet {
		if wl.MemOvercommitPct < 0 || wl.MemOvercommitPct < cfg.MemOvercommit {
			return false
		}
	}
	return true
}

// resolveNodes returns the set of node names matching the analyze config.
func (c *Collector) resolveNodes(ctx context.Context, cfg model.AnalyzeConfig) (map[string]struct{}, error) {
	switch {
	case cfg.NodeName != "":
		// Single node by name.
		node, err := c.clientset.CoreV1().Nodes().Get(ctx, cfg.NodeName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("node %q not found: %w", cfg.NodeName, err)
		}
		if !isNodeReady(node) {
			return nil, fmt.Errorf("node %q is not Ready", cfg.NodeName)
		}
		return map[string]struct{}{cfg.NodeName: {}}, nil

	case cfg.Labels != "":
		nodeList, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{
			LabelSelector: cfg.Labels,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list nodes by labels: %w", err)
		}
		result := make(map[string]struct{})
		for i := range nodeList.Items {
			if isNodeReady(&nodeList.Items[i]) {
				result[nodeList.Items[i].Name] = struct{}{}
			}
		}
		return result, nil

	case cfg.Taints != "":
		nodeList, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list nodes: %w", err)
		}
		result := make(map[string]struct{})
		for i := range nodeList.Items {
			node := &nodeList.Items[i]
			if !isNodeReady(node) {
				continue
			}
			if matchTaint(node.Spec.Taints, cfg.Taints) {
				result[node.Name] = struct{}{}
			}
		}
		return result, nil

	case cfg.Autoscaler != "":
		nodeList, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list nodes: %w", err)
		}
		result := make(map[string]struct{})
		for i := range nodeList.Items {
			node := &nodeList.Items[i]
			if !isNodeReady(node) {
				continue
			}
			if strings.EqualFold(labels.DetectAutoscaler(node.Labels), cfg.Autoscaler) {
				result[node.Name] = struct{}{}
			}
		}
		return result, nil

	default:
		return nil, fmt.Errorf("one of --node, --labels, --taints, or --autoscaler is required")
	}
}

// buildRSOwnerMap lists all ReplicaSets across all namespaces and builds
// a map from "namespace/rsName" to the top-level owner (Deployment).
// This replaces per-pod API calls with a single List request.
func (c *Collector) buildRSOwnerMap(ctx context.Context) (map[string]ownerKey, error) {
	rsList, err := c.clientset.AppsV1().ReplicaSets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make(map[string]ownerKey, len(rsList.Items))
	for i := range rsList.Items {
		rs := &rsList.Items[i]
		key := rs.Namespace + "/" + rs.Name

		if len(rs.OwnerReferences) > 0 && rs.OwnerReferences[0].Kind == "Deployment" {
			result[key] = ownerKey{
				namespace: rs.Namespace,
				kind:      "Deployment",
				name:      rs.OwnerReferences[0].Name,
			}
		} else {
			result[key] = ownerKey{
				namespace: rs.Namespace,
				kind:      "ReplicaSet",
				name:      rs.Name,
			}
		}
	}
	return result, nil
}

// resolveTopOwner determines the top-level workload owner for a pod.
// Uses the pre-built rsOwnerMap for ReplicaSet → Deployment resolution (no API calls).
// Returns an ownerKey with kind=Pod if no owner found.
func resolveTopOwner(pod *corev1.Pod, rsOwnerMap map[string]ownerKey) ownerKey {
	if len(pod.OwnerReferences) == 0 {
		return ownerKey{namespace: pod.Namespace, kind: "Pod", name: pod.Name}
	}

	ref := pod.OwnerReferences[0]

	switch ref.Kind {
	case "ReplicaSet":
		key := pod.Namespace + "/" + ref.Name
		if owner, ok := rsOwnerMap[key]; ok {
			return owner
		}
		// RS not in map (shouldn't happen) — fall back to RS as owner.
		return ownerKey{namespace: pod.Namespace, kind: "ReplicaSet", name: ref.Name}

	case "StatefulSet", "DaemonSet", "Job", "CronJob":
		return ownerKey{namespace: pod.Namespace, kind: ref.Kind, name: ref.Name}

	default:
		return ownerKey{namespace: pod.Namespace, kind: ref.Kind, name: ref.Name}
	}
}

// isNodeReady returns true if the node has condition Ready=True.
func isNodeReady(node *corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

// matchTaint checks if any of the node's taints match the filter string.
// Supports "KEY" and "KEY=VALUE" formats.
func matchTaint(taints []corev1.Taint, filter string) bool {
	if idx := strings.Index(filter, "="); idx >= 0 {
		key := filter[:idx]
		val := filter[idx+1:]
		for _, t := range taints {
			if t.Key == key && t.Value == val {
				return true
			}
		}
		return false
	}
	for _, t := range taints {
		if t.Key == filter {
			return true
		}
	}
	return false
}
