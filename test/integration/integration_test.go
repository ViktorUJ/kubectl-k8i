//go:build integration

package integration

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/kubectl-k8i/pkg/collector"
	"github.com/kubectl-k8i/pkg/color"
	"github.com/kubectl-k8i/pkg/filter"
	"github.com/kubectl-k8i/pkg/model"
	"github.com/kubectl-k8i/pkg/render"
	"github.com/kubectl-k8i/pkg/retry"
	sortpkg "github.com/kubectl-k8i/pkg/sort"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func testRetryConfig() retry.RetryConfig {
	return retry.RetryConfig{MaxRetries: 0, InitialBackoff: 0, MaxBackoff: 0, JitterFraction: 0}
}

var refTime = time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

func makeNode(name string, ready bool, cpuCap, memCap string, labels map[string]string, taints []corev1.Taint, createdAt time.Time, providerID string) corev1.Node {
	condStatus := corev1.ConditionFalse
	if ready {
		condStatus = corev1.ConditionTrue
	}
	if providerID == "" {
		providerID = "aws:///us-east-1a/i-0abcdef1234567890"
	}
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Labels:            labels,
			CreationTimestamp: metav1.NewTime(createdAt),
		},
		Spec: corev1.NodeSpec{
			ProviderID: providerID,
			Taints:     taints,
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: condStatus},
			},
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(cpuCap),
				corev1.ResourceMemory: resource.MustParse(memCap),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(cpuCap),
				corev1.ResourceMemory: resource.MustParse(memCap),
				corev1.ResourcePods:   resource.MustParse("110"),
			},
		},
	}
}

func makePod(name, nodeName, cpuReq, cpuLim, memReq, memLim string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Containers: []corev1.Container{
				{
					Name: "main",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse(cpuReq),
							corev1.ResourceMemory: resource.MustParse(memReq),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse(cpuLim),
							corev1.ResourceMemory: resource.MustParse(memLim),
						},
					},
				},
			},
		},
	}
}

func makeMetrics(nodeName, cpuUsage, memUsage string) metricsv1beta1.NodeMetrics {
	return metricsv1beta1.NodeMetrics{
		ObjectMeta: metav1.ObjectMeta{Name: nodeName},
		Usage: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cpuUsage),
			corev1.ResourceMemory: resource.MustParse(memUsage),
		},
	}
}

func makeNodeclaim(name, nodeName string) unstructured.Unstructured {
	return unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "karpenter.sh/v1",
			"kind":       "NodeClaim",
			"metadata":   map[string]interface{}{"name": name},
			"status":     map[string]interface{}{"nodeName": nodeName},
		},
	}
}

// enrichCluster is a convenience that creates a Collector and enriches data.
func enrichCluster(data *model.ClusterData) []model.NodeInfo {
	c := collector.NewCollector(nil, nil, nil, "", nil, testRetryConfig(), nil)
	return c.EnrichNodes(data, refTime)
}

// renderToString renders nodes to a string using the table renderer.
func renderToString(nodes []model.NodeInfo, cfg render.RenderConfig) string {
	var buf bytes.Buffer
	render.RenderTable(&buf, nodes, cfg)
	return buf.String()
}

func defaultRenderConfig() render.RenderConfig {
	return render.RenderConfig{
		Color:     color.ColorConfig{Enabled: false},
		Timestamp: refTime,
		TermWidth: 0, // unlimited
	}
}

// ---------------------------------------------------------------------------
// Realistic cluster data
// ---------------------------------------------------------------------------

// buildRealisticCluster creates a mixed cluster with Karpenter, Spotinst, EKS,
// and Fargate nodes, each with different labels, taints, and resources.
func buildRealisticCluster() *model.ClusterData {
	karpenterLabels := map[string]string{
		"karpenter.sh/capacity-type":       "spot",
		"karpenter.sh/nodepool":            "default",
		"karpenter.sh/nodeclaim":           "claim-karp-1",
		"kubernetes.io/arch":               "amd64",
		"node.kubernetes.io/instance-type": "m5.xlarge",
		"topology.kubernetes.io/zone":      "us-east-1a",
	}
	spotinstLabels := map[string]string{
		"spotinst.io/node-lifecycle":       "on-demand",
		"spotinst.io/ocean-vng-id":         "vng-abc123",
		"kubernetes.io/arch":               "arm64",
		"node.kubernetes.io/instance-type": "c6g.2xlarge",
		"topology.kubernetes.io/zone":      "us-east-1b",
	}
	eksLabels := map[string]string{
		"eks.amazonaws.com/capacityType":   "ON_DEMAND",
		"eks.amazonaws.com/nodegroup":      "my-eks-nodegroup-prod",
		"kubernetes.io/arch":               "amd64",
		"node.kubernetes.io/instance-type": "r5.large",
		"topology.kubernetes.io/zone":      "us-east-1c",
	}
	plainLabels := map[string]string{
		"kubernetes.io/arch":               "amd64",
		"node.kubernetes.io/instance-type": "t3.medium",
		"topology.kubernetes.io/zone":      "us-west-2a",
	}

	gpuTaint := corev1.Taint{Key: "nvidia.com/gpu", Value: "true", Effect: corev1.TaintEffectNoSchedule}
	spotTaint := corev1.Taint{Key: "spot", Effect: corev1.TaintEffectNoSchedule}

	return &model.ClusterData{
		Nodes: []corev1.Node{
			makeNode("karpenter-node-1", true, "4", "16Gi", karpenterLabels, []corev1.Taint{spotTaint}, refTime.Add(-48*time.Hour), "aws:///us-east-1a/i-0karp111111111111"),
			makeNode("spotinst-node-1", true, "8", "32Gi", spotinstLabels, nil, refTime.Add(-2*time.Hour), "aws:///us-east-1b/i-0spot222222222222"),
			makeNode("eks-node-1", true, "2", "8Gi", eksLabels, []corev1.Taint{gpuTaint}, refTime.Add(-72*time.Hour), "aws:///us-east-1c/i-0eks3333333333333"),
			makeNode("plain-node-1", true, "4", "16Gi", plainLabels, nil, refTime.Add(-30*time.Minute), "aws:///us-west-2a/i-0plain44444444444"),
			makeNode("fargate-ip-10-0-1-1.ec2.internal", true, "2", "4Gi", plainLabels, nil, refTime.Add(-1*time.Hour), "aws:///us-west-2a/i-0farg555555555555"),
			makeNode("not-ready-node", false, "4", "16Gi", karpenterLabels, nil, refTime.Add(-24*time.Hour), ""),
		},
		Pods: []corev1.Pod{
			makePod("pod-k1", "karpenter-node-1", "500m", "1", "1Gi", "2Gi"),
			makePod("pod-k2", "karpenter-node-1", "250m", "500m", "512Mi", "1Gi"),
			makePod("pod-s1", "spotinst-node-1", "2", "4", "4Gi", "8Gi"),
			makePod("pod-e1", "eks-node-1", "100m", "200m", "256Mi", "512Mi"),
			makePod("pod-p1", "plain-node-1", "1", "2", "2Gi", "4Gi"),
			makePod("pod-f1", "fargate-ip-10-0-1-1.ec2.internal", "250m", "500m", "512Mi", "1Gi"),
		},
		Metrics: []metricsv1beta1.NodeMetrics{
			makeMetrics("karpenter-node-1", "3", "12Gi"),
			makeMetrics("spotinst-node-1", "6", "24Gi"),
			makeMetrics("eks-node-1", "1", "4Gi"),
			makeMetrics("plain-node-1", "2", "8Gi"),
			makeMetrics("fargate-ip-10-0-1-1.ec2.internal", "1", "2Gi"),
		},
		Nodeclaims: []unstructured.Unstructured{
			makeNodeclaim("claim-karp-1", "karpenter-node-1"),
		},
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestFullPipeline_RealisticCluster(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)

	// Not-ready node should be excluded
	for _, n := range nodes {
		assert.NotEqual(t, "not-ready-node", n.Name)
	}
	// 5 ready nodes total (including fargate)
	assert.Len(t, nodes, 5)

	// Hide fargate by default
	filtered := filter.HideFargateNodes(nodes)
	assert.Len(t, filtered, 4)
	for _, n := range filtered {
		assert.False(t, strings.HasPrefix(n.Name, "fargate-"), "fargate node should be hidden")
	}

	// Default sort: pool=asc
	err := sortpkg.SortNodes(filtered, "pool", "asc")
	require.NoError(t, err)

	// Render table
	out := renderToString(filtered, defaultRenderConfig())
	assert.Contains(t, out, "NODE")
	assert.Contains(t, out, "PODS")
	assert.Contains(t, out, "karpenter-node-1")
	assert.Contains(t, out, "spotinst-node-1")
	assert.Contains(t, out, "eks-node-1")
	assert.Contains(t, out, "plain-node-1")
	assert.NotContains(t, out, "fargate-")
	assert.NotContains(t, out, "not-ready-node")
}

func TestLabelPriorityChains_FullPipeline(t *testing.T) {
	// Node with both karpenter.sh and eks labels — karpenter should win
	labels := map[string]string{
		"karpenter.sh/capacity-type":       "spot",
		"karpenter.sh/nodepool":            "karp-pool",
		"eks.amazonaws.com/capacityType":   "ON_DEMAND",
		"eks.amazonaws.com/nodegroup":      "eks-group",
		"kubernetes.io/arch":               "arm64",
		"node.kubernetes.io/instance-type": "m6g.xlarge",
		"topology.kubernetes.io/zone":      "eu-west-1a",
	}
	data := &model.ClusterData{
		Nodes: []corev1.Node{
			makeNode("priority-node", true, "4", "16Gi", labels, nil, refTime.Add(-1*time.Hour), "aws:///eu-west-1a/i-0priority1111111"),
		},
	}
	nodes := enrichCluster(data)
	require.Len(t, nodes, 1)
	n := nodes[0]

	assert.Equal(t, "spot", n.CapacityType, "karpenter.sh/capacity-type should win over eks")
	assert.Equal(t, "karp-pool", n.Nodepool, "karpenter.sh/nodepool should win over eks")
	assert.Equal(t, "karpenter", n.Autoscaler, "karpenter labels should detect karpenter autoscaler")
	assert.Equal(t, "arm64", n.Architecture)
	assert.Equal(t, "1a", n.Zone)
	assert.Equal(t, "m6g.xlarge", n.InstanceType)
}

func TestFilterAttribute_EC2Type(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	// karpenter-node-1 has spot, spotinst-node-1 has od, eks-node-1 has od
	filtered, err := filter.FilterNodes(nodes, "ec2_type", "spot")
	require.NoError(t, err)
	assert.Len(t, filtered, 1)
	assert.Equal(t, "karpenter-node-1", filtered[0].Name)
}

func TestFilterAttribute_InstanceType(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	filtered, err := filter.FilterNodes(nodes, "instance_type", "m5.xlarge")
	require.NoError(t, err)
	assert.Len(t, filtered, 1)
	assert.Equal(t, "karpenter-node-1", filtered[0].Name)
}

func TestFilterAttribute_Arch(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	filtered, err := filter.FilterNodes(nodes, "arch", "arm64")
	require.NoError(t, err)
	assert.Len(t, filtered, 1)
	assert.Equal(t, "spotinst-node-1", filtered[0].Name)
}

func TestFilterAttribute_Zone(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	filtered, err := filter.FilterNodes(nodes, "zone", "1b")
	require.NoError(t, err)
	assert.Len(t, filtered, 1)
	assert.Equal(t, "spotinst-node-1", filtered[0].Name)
}

func TestFilterAttribute_Pool(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	filtered, err := filter.FilterNodes(nodes, "pool", "default")
	require.NoError(t, err)
	assert.Len(t, filtered, 1)
	assert.Equal(t, "karpenter-node-1", filtered[0].Name)
}

func TestFilterAttribute_Nodeclaim(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	filtered, err := filter.FilterNodes(nodes, "nodeclaim", "claim-karp-1")
	require.NoError(t, err)
	assert.Len(t, filtered, 1)
	assert.Equal(t, "karpenter-node-1", filtered[0].Name)
}

func TestFilterAttribute_Taint(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	// Filter by taint key
	filtered, err := filter.FilterNodes(nodes, "taint", "nvidia.com/gpu")
	require.NoError(t, err)
	assert.Len(t, filtered, 1)
	assert.Equal(t, "eks-node-1", filtered[0].Name)

	// Filter by taint key=value
	filtered, err = filter.FilterNodes(nodes, "taint", "nvidia.com/gpu=true")
	require.NoError(t, err)
	assert.Len(t, filtered, 1)
	assert.Equal(t, "eks-node-1", filtered[0].Name)
}

func TestFilterAttribute_Autoscaler(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	filtered, err := filter.FilterNodes(nodes, "autoscaler", "karpenter")
	require.NoError(t, err)
	assert.Len(t, filtered, 1)
	assert.Equal(t, "karpenter-node-1", filtered[0].Name)

	filtered, err = filter.FilterNodes(nodes, "autoscaler", "spotio")
	require.NoError(t, err)
	assert.Len(t, filtered, 1)
	assert.Equal(t, "spotinst-node-1", filtered[0].Name)

	filtered, err = filter.FilterNodes(nodes, "autoscaler", "cas")
	require.NoError(t, err)
	assert.Len(t, filtered, 1)
	assert.Equal(t, "eks-node-1", filtered[0].Name)

	filtered, err = filter.FilterNodes(nodes, "autoscaler", "x")
	require.NoError(t, err)
	assert.Len(t, filtered, 1)
	assert.Equal(t, "plain-node-1", filtered[0].Name)
}

func TestSortColumn_Name(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	err := sortpkg.SortNodes(nodes, "name", "asc")
	require.NoError(t, err)
	assert.Equal(t, "eks-node-1", nodes[0].Name)
	assert.Equal(t, "spotinst-node-1", nodes[len(nodes)-1].Name)
}

func TestSortColumn_Pods(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	err := sortpkg.SortNodes(nodes, "pods", "desc")
	require.NoError(t, err)
	// karpenter-node-1 has 2 pods, others have 1
	assert.Equal(t, "karpenter-node-1", nodes[0].Name)
}

func TestSortColumn_CPULoad(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	err := sortpkg.SortNodes(nodes, "cpu_load", "desc")
	require.NoError(t, err)
	// Verify descending order
	for i := 0; i < len(nodes)-1; i++ {
		assert.GreaterOrEqual(t, nodes[i].CPULoadPercent, nodes[i+1].CPULoadPercent)
	}
}

func TestSortColumn_MemLoad(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	err := sortpkg.SortNodes(nodes, "mem_load", "asc")
	require.NoError(t, err)
	for i := 0; i < len(nodes)-1; i++ {
		assert.LessOrEqual(t, nodes[i].MemLoadPercent, nodes[i+1].MemLoadPercent)
	}
}

func TestSortColumn_Age(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	err := sortpkg.SortNodes(nodes, "age", "asc")
	require.NoError(t, err)
	// Ascending by creation time = oldest first
	for i := 0; i < len(nodes)-1; i++ {
		assert.True(t, !nodes[i].CreationTime.After(nodes[i+1].CreationTime),
			"node %s (%v) should be before %s (%v)", nodes[i].Name, nodes[i].CreationTime, nodes[i+1].Name, nodes[i+1].CreationTime)
	}
}

func TestSortColumn_Autoscaler(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	err := sortpkg.SortNodes(nodes, "autoscaler", "asc")
	require.NoError(t, err)
	for i := 0; i < len(nodes)-1; i++ {
		assert.LessOrEqual(t, nodes[i].Autoscaler, nodes[i+1].Autoscaler)
	}
}

func TestSortColumn_InstanceType(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	err := sortpkg.SortNodes(nodes, "instance_type", "asc")
	require.NoError(t, err)
	for i := 0; i < len(nodes)-1; i++ {
		assert.LessOrEqual(t, nodes[i].InstanceType, nodes[i+1].InstanceType)
	}
}

func TestSortColumn_Zone(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	err := sortpkg.SortNodes(nodes, "zone", "asc")
	require.NoError(t, err)
	for i := 0; i < len(nodes)-1; i++ {
		assert.LessOrEqual(t, nodes[i].Zone, nodes[i+1].Zone)
	}
}

func TestSortColumn_Pool(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	err := sortpkg.SortNodes(nodes, "pool", "asc")
	require.NoError(t, err)
	for i := 0; i < len(nodes)-1; i++ {
		assert.LessOrEqual(t, nodes[i].Nodepool, nodes[i+1].Nodepool)
	}
}

func TestSortColumn_Taint(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	err := sortpkg.SortNodes(nodes, "taint", "asc")
	require.NoError(t, err)
	for i := 0; i < len(nodes)-1; i++ {
		assert.LessOrEqual(t, nodes[i].TaintSortKey, nodes[i+1].TaintSortKey)
	}
}

func TestTaintGrouping_WithSeparators(t *testing.T) {
	gpuTaint := corev1.Taint{Key: "nvidia.com/gpu", Value: "true", Effect: corev1.TaintEffectNoSchedule}
	spotTaint := corev1.Taint{Key: "spot", Effect: corev1.TaintEffectNoSchedule}

	data := &model.ClusterData{
		Nodes: []corev1.Node{
			makeNode("gpu-node-1", true, "4", "16Gi", nil, []corev1.Taint{gpuTaint}, refTime.Add(-1*time.Hour), ""),
			makeNode("gpu-node-2", true, "4", "16Gi", nil, []corev1.Taint{gpuTaint}, refTime.Add(-2*time.Hour), ""),
			makeNode("spot-node-1", true, "4", "16Gi", nil, []corev1.Taint{spotTaint}, refTime.Add(-3*time.Hour), ""),
			makeNode("clean-node-1", true, "4", "16Gi", nil, nil, refTime.Add(-4*time.Hour), ""),
		},
	}
	nodes := enrichCluster(data)

	cfg := defaultRenderConfig()
	cfg.GroupByTaint = true
	out := renderToString(nodes, cfg)

	// Should contain group separators (tilde lines)
	assert.Contains(t, out, "~")
	assert.Contains(t, out, "gpu-node-1")
	assert.Contains(t, out, "gpu-node-2")
	assert.Contains(t, out, "spot-node-1")
	assert.Contains(t, out, "clean-node-1")
}

func TestFargateHiding(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)

	// Before hiding: fargate node present
	hasFargate := false
	for _, n := range nodes {
		if strings.HasPrefix(n.Name, "fargate-") {
			hasFargate = true
			break
		}
	}
	assert.True(t, hasFargate, "fargate node should be present before hiding")

	// After hiding
	filtered := filter.HideFargateNodes(nodes)
	for _, n := range filtered {
		assert.False(t, strings.HasPrefix(n.Name, "fargate-"), "fargate node should be hidden")
	}
}

func TestFargateShowing(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)

	// When --fargate is set, don't call HideFargateNodes
	hasFargate := false
	for _, n := range nodes {
		if strings.HasPrefix(n.Name, "fargate-") {
			hasFargate = true
			break
		}
	}
	assert.True(t, hasFargate, "fargate node should be visible when --fargate is set")

	out := renderToString(nodes, defaultRenderConfig())
	assert.Contains(t, out, "fargate-")
}

func TestMetricsUnavailable_ZeroUsage(t *testing.T) {
	data := &model.ClusterData{
		Nodes: []corev1.Node{
			makeNode("node-a", true, "4", "16Gi", nil, nil, refTime.Add(-1*time.Hour), ""),
		},
		Pods: []corev1.Pod{
			makePod("pod-1", "node-a", "500m", "1", "1Gi", "2Gi"),
		},
		Metrics: nil, // metrics unavailable
	}
	nodes := enrichCluster(data)
	require.Len(t, nodes, 1)

	n := nodes[0]
	assert.InDelta(t, 0.0, n.CPUUsageCores, 0.001)
	assert.InDelta(t, 0.0, n.MemUsageGB, 0.001)
	assert.Equal(t, 0, n.CPULoadPercent)
	assert.Equal(t, 0, n.MemLoadPercent)

	// Render should still work
	out := renderToString(nodes, defaultRenderConfig())
	assert.Contains(t, out, "node-a")
	assert.Contains(t, out, "0.0") // zero usage
}

func TestNodeclaimCRDMissing_XValues(t *testing.T) {
	data := &model.ClusterData{
		Nodes: []corev1.Node{
			makeNode("node-a", true, "4", "16Gi", nil, nil, refTime.Add(-1*time.Hour), ""),
			makeNode("node-b", true, "4", "16Gi", nil, nil, refTime.Add(-2*time.Hour), ""),
		},
		Nodeclaims: nil, // CRD missing
	}
	nodes := enrichCluster(data)
	require.Len(t, nodes, 2)

	for _, n := range nodes {
		assert.Equal(t, "x", n.Nodeclaim, "nodeclaim should be 'x' when CRD is missing")
	}

	out := renderToString(nodes, defaultRenderConfig())
	assert.Contains(t, out, "x")
}

func TestEmptyCluster_Message(t *testing.T) {
	data := &model.ClusterData{
		Nodes: []corev1.Node{},
	}
	nodes := enrichCluster(data)
	assert.Empty(t, nodes)

	out := renderToString(nodes, defaultRenderConfig())
	assert.Contains(t, out, "no nodes match filter")
}

func TestAutoscalerDetection_FullPipeline(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	nodeMap := map[string]model.NodeInfo{}
	for _, n := range nodes {
		nodeMap[n.Name] = n
	}

	assert.Equal(t, "karpenter", nodeMap["karpenter-node-1"].Autoscaler)
	assert.Equal(t, "spotio", nodeMap["spotinst-node-1"].Autoscaler)
	assert.Equal(t, "cas", nodeMap["eks-node-1"].Autoscaler)
	assert.Equal(t, "x", nodeMap["plain-node-1"].Autoscaler)
}

func TestAutoscalerFiltering_FullPipeline(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	// Filter for karpenter
	filtered, err := filter.FilterNodes(nodes, "autoscaler", "karpenter")
	require.NoError(t, err)
	for _, n := range filtered {
		assert.Equal(t, "karpenter", n.Autoscaler)
	}

	// Filter for spotio
	filtered, err = filter.FilterNodes(nodes, "autoscaler", "spotio")
	require.NoError(t, err)
	for _, n := range filtered {
		assert.Equal(t, "spotio", n.Autoscaler)
	}
}

func TestAutoscalerSorting_FullPipeline(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	err := sortpkg.SortNodes(nodes, "autoscaler", "asc")
	require.NoError(t, err)

	// Verify ascending lexicographic order
	for i := 0; i < len(nodes)-1; i++ {
		assert.LessOrEqual(t, nodes[i].Autoscaler, nodes[i+1].Autoscaler)
	}

	// Render and verify autoscaler column is present
	out := renderToString(nodes, defaultRenderConfig())
	assert.Contains(t, out, "AUTOSCALER")
	assert.Contains(t, out, "karpenter")
	assert.Contains(t, out, "spotio")
	assert.Contains(t, out, "cas")
}

func TestCompleteOutputFormat(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)
	_ = sortpkg.SortNodes(nodes, "pool", "asc")

	cfg := defaultRenderConfig()
	cfg.Filter = "ec2_type=spot"
	cfg.Sort = "pool=asc"
	out := renderToString(nodes, cfg)

	// Verify timestamp
	assert.Contains(t, out, "Data collected at:")
	// Verify filter annotation
	assert.Contains(t, out, "Filter applied: ec2_type=spot")
	// Verify sort annotation
	assert.Contains(t, out, "Sort applied: pool=asc")
	// Verify header
	assert.Contains(t, out, "NODE")
	assert.Contains(t, out, "PODS")
	assert.Contains(t, out, "CREQ")
	assert.Contains(t, out, "CLIM")
	assert.Contains(t, out, "CUSE")
	assert.Contains(t, out, "CCAP")
	assert.Contains(t, out, "C%")
	assert.Contains(t, out, "MREQ")
	assert.Contains(t, out, "MLIM")
	assert.Contains(t, out, "MUSE")
	assert.Contains(t, out, "MCAP")
	assert.Contains(t, out, "M%")
	assert.Contains(t, out, "EC2")
	assert.Contains(t, out, "TYPE")
	assert.Contains(t, out, "SPOT")
	assert.Contains(t, out, "ARCH")
	assert.Contains(t, out, "ZN")
	assert.Contains(t, out, "POOL")
	assert.Contains(t, out, "NODECLAIM")
	assert.Contains(t, out, "AUTOSCALER")
	assert.Contains(t, out, "AGE")
	assert.Contains(t, out, "TAINTS")
	// Verify separator (dashes)
	assert.Contains(t, out, "---")
	// Verify data rows
	lines := strings.Split(strings.TrimSpace(out), "\n")
	// At least: timestamp + filter + sort + header + separator + 4 data rows = 9 lines
	assert.GreaterOrEqual(t, len(lines), 9)
}

func TestResourceAggregation_FullPipeline(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)

	nodeMap := map[string]model.NodeInfo{}
	for _, n := range nodes {
		nodeMap[n.Name] = n
	}

	// karpenter-node-1: 2 pods, 500m+250m=750m req, 1+500m=1.5 lim
	kn := nodeMap["karpenter-node-1"]
	assert.Equal(t, 2, kn.PodsUsed)
	assert.InDelta(t, 0.75, kn.CPURequestCores, 0.01)
	assert.InDelta(t, 1.5, kn.CPULimitCores, 0.01)
	// CPU usage 3 cores, capacity 4 cores → 75%
	assert.Equal(t, 75, kn.CPULoadPercent)
	// Mem usage 12Gi, capacity 16Gi → 75%
	assert.Equal(t, 75, kn.MemLoadPercent)

	// spotinst-node-1: 1 pod, 2 req, 4 lim
	sn := nodeMap["spotinst-node-1"]
	assert.Equal(t, 1, sn.PodsUsed)
	assert.InDelta(t, 2.0, sn.CPURequestCores, 0.01)
	assert.InDelta(t, 4.0, sn.CPULimitCores, 0.01)
	// CPU usage 6 cores, capacity 8 cores → 75%
	assert.Equal(t, 75, sn.CPULoadPercent)
	// Mem usage 24Gi, capacity 32Gi → 75%
	assert.Equal(t, 75, sn.MemLoadPercent)
}

func TestFilterThenSort_FullPipeline(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	// Filter by arch=amd64
	filtered, err := filter.FilterNodes(nodes, "arch", "amd64")
	require.NoError(t, err)
	assert.Greater(t, len(filtered), 1)

	// Sort by cpu_load desc
	err = sortpkg.SortNodes(filtered, "cpu_load", "desc")
	require.NoError(t, err)

	// Verify all results are amd64 AND sorted
	for i, n := range filtered {
		assert.Equal(t, "amd64", n.Architecture)
		if i > 0 {
			assert.GreaterOrEqual(t, filtered[i-1].CPULoadPercent, n.CPULoadPercent)
		}
	}
}

func TestOnDemandNormalization_FullPipeline(t *testing.T) {
	// Spotinst uses "on-demand" → should normalize to "od"
	data := buildRealisticCluster()
	nodes := enrichCluster(data)

	nodeMap := map[string]model.NodeInfo{}
	for _, n := range nodes {
		nodeMap[n.Name] = n
	}

	assert.Equal(t, "od", nodeMap["spotinst-node-1"].CapacityType, "on-demand should normalize to od")
	assert.Equal(t, "od", nodeMap["eks-node-1"].CapacityType, "ON_DEMAND should normalize to od")
}

func TestEKSNodegroupTruncation_FullPipeline(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)

	nodeMap := map[string]model.NodeInfo{}
	for _, n := range nodes {
		nodeMap[n.Name] = n
	}

	// "my-eks-nodegroup-prod" is 21 chars, should be truncated to 15
	eksPool := nodeMap["eks-node-1"].Nodepool
	assert.LessOrEqual(t, len(eksPool), 15)
	assert.Equal(t, "my-eks-nodegrou", eksPool)
}

func TestNoHeadersMode(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	cfg := defaultRenderConfig()
	cfg.NoHeaders = true
	out := renderToString(nodes, cfg)

	assert.NotContains(t, out, "NODE")
	assert.NotContains(t, out, "PODS")
	assert.NotContains(t, out, "Data collected at:")
	assert.NotContains(t, out, "---")

	// Should have exactly as many lines as nodes
	lines := strings.Split(strings.TrimSpace(out), "\n")
	assert.Equal(t, len(nodes), len(lines))
}

func TestAllNotReady_EmptyResult(t *testing.T) {
	data := &model.ClusterData{
		Nodes: []corev1.Node{
			makeNode("node-a", false, "4", "16Gi", nil, nil, refTime.Add(-1*time.Hour), ""),
			makeNode("node-b", false, "4", "16Gi", nil, nil, refTime.Add(-2*time.Hour), ""),
		},
	}
	nodes := enrichCluster(data)
	assert.Empty(t, nodes)

	out := renderToString(nodes, defaultRenderConfig())
	assert.Contains(t, out, "no nodes match filter")
}

func TestEC2IDExtraction_FullPipeline(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)

	nodeMap := map[string]model.NodeInfo{}
	for _, n := range nodes {
		nodeMap[n.Name] = n
	}

	assert.Equal(t, "i-0karp111111111111", nodeMap["karpenter-node-1"].EC2InstanceID)
	assert.Equal(t, "i-0spot222222222222", nodeMap["spotinst-node-1"].EC2InstanceID)
	assert.Equal(t, "i-0eks3333333333333", nodeMap["eks-node-1"].EC2InstanceID)
}

func TestAgeFormatting_FullPipeline(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)

	nodeMap := map[string]model.NodeInfo{}
	for _, n := range nodes {
		nodeMap[n.Name] = n
	}

	// karpenter-node-1: 48h ago → 2d0h
	assert.Equal(t, "2d0h", nodeMap["karpenter-node-1"].Age)
	// spotinst-node-1: 2h ago → 2h0m
	assert.Equal(t, "2h0m", nodeMap["spotinst-node-1"].Age)
	// plain-node-1: 30m ago → 30m
	assert.Equal(t, "30m", nodeMap["plain-node-1"].Age)
	// eks-node-1: 72h ago → 3d0h
	assert.Equal(t, "3d0h", nodeMap["eks-node-1"].Age)
}

func TestSortColumn_CPUReq(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	err := sortpkg.SortNodes(nodes, "cpu_req", "asc")
	require.NoError(t, err)
	for i := 0; i < len(nodes)-1; i++ {
		assert.LessOrEqual(t, nodes[i].CPURequestCores, nodes[i+1].CPURequestCores)
	}
}

func TestSortColumn_MemCap(t *testing.T) {
	data := buildRealisticCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	err := sortpkg.SortNodes(nodes, "mem_cap", "desc")
	require.NoError(t, err)
	for i := 0; i < len(nodes)-1; i++ {
		assert.GreaterOrEqual(t, nodes[i].MemCapacityGB, nodes[i+1].MemCapacityGB)
	}
}
