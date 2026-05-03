package collector

import (
	"testing"
	"time"

	"github.com/kubectl-k8i/pkg/model"
	"github.com/kubectl-k8i/pkg/retry"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

// --- helpers ---

// makeNode creates a corev1.Node with the given name, Ready condition, capacity, and labels.
func makeNode(name string, ready bool, cpuCap, memCap string, labels map[string]string) corev1.Node {
	condStatus := corev1.ConditionFalse
	if ready {
		condStatus = corev1.ConditionTrue
	}
	n := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Labels:            labels,
			CreationTimestamp: metav1.NewTime(time.Now().Add(-24 * time.Hour)),
		},
		Spec: corev1.NodeSpec{
			ProviderID: "aws:///us-east-1a/i-0abcdef1234567890",
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
	return n
}

// makePod creates a corev1.Pod assigned to the given node with specified resource requests/limits.
func makePod(name, nodeName, cpuReq, cpuLim, memReq, memLim string) corev1.Pod {
	p := corev1.Pod{
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
	return p
}

// makeMetrics creates a metricsv1beta1.NodeMetrics for the given node.
func makeMetrics(nodeName, cpuUsage, memUsage string) metricsv1beta1.NodeMetrics {
	return metricsv1beta1.NodeMetrics{
		ObjectMeta: metav1.ObjectMeta{Name: nodeName},
		Usage: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cpuUsage),
			corev1.ResourceMemory: resource.MustParse(memUsage),
		},
	}
}

// makeNodeclaim creates an unstructured.Unstructured nodeclaim mapping to a node.
func makeNodeclaim(name, nodeName string) unstructured.Unstructured {
	return unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "karpenter.sh/v1",
			"kind":       "NodeClaim",
			"metadata": map[string]interface{}{
				"name": name,
			},
			"status": map[string]interface{}{
				"nodeName": nodeName,
			},
		},
	}
}

// --- tests ---

func TestEnrichNodes_ReadyNodeFiltering(t *testing.T) {
	// Only Ready nodes should appear in the output.
	data := &model.ClusterData{
		Nodes: []corev1.Node{
			makeNode("ready-node", true, "4", "16Gi", nil),
			makeNode("not-ready-node", false, "4", "16Gi", nil),
			makeNode("another-ready", true, "8", "32Gi", nil),
		},
	}

	c := NewCollector(nil, nil, nil, "", nil, testRetryConfig(), nil)
	nodes := c.EnrichNodes(data, time.Now())

	assert.Len(t, nodes, 2)
	names := []string{nodes[0].Name, nodes[1].Name}
	assert.Contains(t, names, "ready-node")
	assert.Contains(t, names, "another-ready")
}

func TestEnrichNodes_PodAggregation(t *testing.T) {
	data := &model.ClusterData{
		Nodes: []corev1.Node{
			makeNode("node-a", true, "4", "16Gi", nil),
		},
		Pods: []corev1.Pod{
			makePod("pod-1", "node-a", "500m", "1", "256Mi", "512Mi"),
			makePod("pod-2", "node-a", "250m", "500m", "128Mi", "256Mi"),
		},
	}

	c := NewCollector(nil, nil, nil, "", nil, testRetryConfig(), nil)
	nodes := c.EnrichNodes(data, time.Now())

	assert.Len(t, nodes, 1)
	n := nodes[0]
	assert.Equal(t, 2, n.PodsUsed)
	// 500m + 250m = 750m = 0.75 cores
	assert.InDelta(t, 0.75, n.CPURequestCores, 0.01)
	// 1 + 500m = 1.5 cores
	assert.InDelta(t, 1.5, n.CPULimitCores, 0.01)
}

func TestEnrichNodes_MetricsAggregation(t *testing.T) {
	data := &model.ClusterData{
		Nodes: []corev1.Node{
			makeNode("node-a", true, "4", "16Gi", nil),
		},
		Metrics: []metricsv1beta1.NodeMetrics{
			makeMetrics("node-a", "2", "8Gi"),
		},
	}

	c := NewCollector(nil, nil, nil, "", nil, testRetryConfig(), nil)
	nodes := c.EnrichNodes(data, time.Now())

	assert.Len(t, nodes, 1)
	n := nodes[0]
	assert.InDelta(t, 2.0, n.CPUUsageCores, 0.01)
	assert.InDelta(t, 8.0, n.MemUsageGB, 0.01)
	assert.Equal(t, 50, n.CPULoadPercent) // 2/4 = 50%
	assert.Equal(t, 50, n.MemLoadPercent) // 8/16 = 50%
}

func TestEnrichNodes_NodeclaimMapping(t *testing.T) {
	data := &model.ClusterData{
		Nodes: []corev1.Node{
			makeNode("node-a", true, "4", "16Gi", nil),
			makeNode("node-b", true, "4", "16Gi", nil),
		},
		Nodeclaims: []unstructured.Unstructured{
			makeNodeclaim("claim-abc", "node-a"),
		},
	}

	c := NewCollector(nil, nil, nil, "", nil, testRetryConfig(), nil)
	nodes := c.EnrichNodes(data, time.Now())

	assert.Len(t, nodes, 2)
	nodeMap := map[string]model.NodeInfo{}
	for _, n := range nodes {
		nodeMap[n.Name] = n
	}
	assert.Equal(t, "claim-abc", nodeMap["node-a"].Nodeclaim)
	assert.Equal(t, "x", nodeMap["node-b"].Nodeclaim) // no nodeclaim → "x"
}

func TestEnrichNodes_NoNodeclaims(t *testing.T) {
	// When nodeclaim CRD is missing, all nodes get "x" for nodeclaim.
	data := &model.ClusterData{
		Nodes: []corev1.Node{
			makeNode("node-a", true, "4", "16Gi", nil),
		},
		Nodeclaims: nil,
	}

	c := NewCollector(nil, nil, nil, "", nil, testRetryConfig(), nil)
	nodes := c.EnrichNodes(data, time.Now())

	assert.Len(t, nodes, 1)
	assert.Equal(t, "x", nodes[0].Nodeclaim)
}

func TestEnrichNodes_NoMetrics(t *testing.T) {
	// When metrics are unavailable, usage values should be zero.
	data := &model.ClusterData{
		Nodes: []corev1.Node{
			makeNode("node-a", true, "4", "16Gi", nil),
		},
		Metrics: nil,
	}

	c := NewCollector(nil, nil, nil, "", nil, testRetryConfig(), nil)
	nodes := c.EnrichNodes(data, time.Now())

	assert.Len(t, nodes, 1)
	n := nodes[0]
	assert.InDelta(t, 0.0, n.CPUUsageCores, 0.001)
	assert.InDelta(t, 0.0, n.MemUsageGB, 0.001)
	assert.Equal(t, 0, n.CPULoadPercent)
	assert.Equal(t, 0, n.MemLoadPercent)
}

func TestEnrichNodes_AllNotReady(t *testing.T) {
	data := &model.ClusterData{
		Nodes: []corev1.Node{
			makeNode("node-a", false, "4", "16Gi", nil),
			makeNode("node-b", false, "4", "16Gi", nil),
		},
	}

	c := NewCollector(nil, nil, nil, "", nil, testRetryConfig(), nil)
	nodes := c.EnrichNodes(data, time.Now())

	assert.Empty(t, nodes)
}

func TestEnrichNodes_PodsMaxFromAllocatable(t *testing.T) {
	data := &model.ClusterData{
		Nodes: []corev1.Node{
			makeNode("node-a", true, "4", "16Gi", nil),
		},
	}

	c := NewCollector(nil, nil, nil, "", nil, testRetryConfig(), nil)
	nodes := c.EnrichNodes(data, time.Now())

	assert.Len(t, nodes, 1)
	assert.Equal(t, 110, nodes[0].PodsMax)
}

func TestBuildPodsByNode(t *testing.T) {
	pods := []corev1.Pod{
		makePod("pod-1", "node-a", "500m", "1", "256Mi", "512Mi"),
		makePod("pod-2", "node-a", "250m", "500m", "128Mi", "256Mi"),
		makePod("pod-3", "node-b", "1", "2", "1Gi", "2Gi"),
	}

	result := buildPodsByNode(pods)

	assert.Len(t, result, 2)
	assert.Equal(t, 2, result["node-a"].PodCount)
	assert.Equal(t, 1, result["node-b"].PodCount)
	// node-a: 500m + 250m = 750m
	assert.Equal(t, int64(750), result["node-a"].CPURequestMilli)
}

func TestBuildMetricsByNode(t *testing.T) {
	metrics := []metricsv1beta1.NodeMetrics{
		makeMetrics("node-a", "2", "8Gi"),
		makeMetrics("node-b", "1", "4Gi"),
	}

	result := buildMetricsByNode(metrics)

	assert.Len(t, result, 2)
	assert.NotNil(t, result["node-a"])
	assert.NotNil(t, result["node-b"])
}

func TestBuildNodeclaimByNode(t *testing.T) {
	nodeclaims := []unstructured.Unstructured{
		makeNodeclaim("claim-1", "node-a"),
		makeNodeclaim("claim-2", "node-b"),
	}

	result := buildNodeclaimByNode(nodeclaims)

	assert.Len(t, result, 2)
	assert.Equal(t, "claim-1", result["node-a"])
	assert.Equal(t, "claim-2", result["node-b"])
}

func TestBuildNodeclaimByNode_MissingStatus(t *testing.T) {
	// Nodeclaim without status.nodeName should be skipped.
	nodeclaims := []unstructured.Unstructured{
		{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{"name": "claim-1"},
			},
		},
	}

	result := buildNodeclaimByNode(nodeclaims)
	assert.Empty(t, result)
}

func TestIsNodeReady(t *testing.T) {
	tests := []struct {
		name     string
		node     corev1.Node
		expected bool
	}{
		{
			name:     "Ready=True",
			node:     makeNode("n1", true, "4", "16Gi", nil),
			expected: true,
		},
		{
			name:     "Ready=False",
			node:     makeNode("n2", false, "4", "16Gi", nil),
			expected: false,
		},
		{
			name: "No Ready condition",
			node: corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse},
					},
				},
			},
			expected: false,
		},
		{
			name: "Empty conditions",
			node: corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: nil,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isNodeReady(&tt.node))
		})
	}
}

// testRetryConfig returns a minimal retry config for testing.
func testRetryConfig() retry.RetryConfig {
	return retry.RetryConfig{
		MaxRetries:     0,
		InitialBackoff: 0,
		MaxBackoff:     0,
		JitterFraction: 0,
	}
}
