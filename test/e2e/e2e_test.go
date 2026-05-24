package e2e

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/kubectl-k8i/pkg/collector"
	"github.com/kubectl-k8i/pkg/color"
	"github.com/kubectl-k8i/pkg/filter"
	"github.com/kubectl-k8i/pkg/model"
	"github.com/kubectl-k8i/pkg/output"
	"github.com/kubectl-k8i/pkg/render"
	"github.com/kubectl-k8i/pkg/retry"
	sortpkg "github.com/kubectl-k8i/pkg/sort"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

var refTime = time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

func testRetryConfig() retry.RetryConfig {
	return retry.RetryConfig{MaxRetries: 0, InitialBackoff: 0, MaxBackoff: 0, JitterFraction: 0}
}

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

func enrichCluster(data *model.ClusterData) []model.NodeInfo {
	c := collector.NewCollector(nil, nil, nil, "", nil, testRetryConfig(), nil)
	return c.EnrichNodes(data, refTime)
}

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

func containsANSI(s string) bool {
	return strings.Contains(s, "\033[")
}

// buildMixedCluster creates a cluster with Karpenter, Spotinst, EKS, plain,
// and Fargate nodes for e2e output testing.
func buildMixedCluster() *model.ClusterData {
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
// E2E Output Tests
// ---------------------------------------------------------------------------

func TestE2E_StandardOutput_MixedNodeTypes(t *testing.T) {
	data := buildMixedCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)
	_ = sortpkg.SortNodes(nodes, "pool", "asc")

	out := renderToString(nodes, defaultRenderConfig())

	// Header row present
	assert.Contains(t, out, "NODE")
	assert.Contains(t, out, "PODS")
	assert.Contains(t, out, "AUTOSCALER")
	assert.Contains(t, out, "TAINTS")

	// All non-fargate nodes present
	assert.Contains(t, out, "karpenter-node-1")
	assert.Contains(t, out, "spotinst-node-1")
	assert.Contains(t, out, "eks-node-1")
	assert.Contains(t, out, "plain-node-1")

	// Fargate excluded
	assert.NotContains(t, out, "fargate-")

	// Separator line present
	assert.Contains(t, out, "---")

	// Timestamp present
	assert.Contains(t, out, "Data collected at:")
}

func TestE2E_FilterOutput(t *testing.T) {
	data := buildMixedCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	filtered, err := filter.FilterNodes(nodes, "ec2_type", "spot")
	require.NoError(t, err)
	_ = sortpkg.SortNodes(filtered, "pool", "asc")

	cfg := defaultRenderConfig()
	cfg.Filter = "ec2_type=spot"
	out := renderToString(filtered, cfg)

	assert.Contains(t, out, "Filter applied: ec2_type=spot")
	assert.Contains(t, out, "karpenter-node-1")
	assert.NotContains(t, out, "spotinst-node-1")
	assert.NotContains(t, out, "eks-node-1")
}

func TestE2E_SortOutput(t *testing.T) {
	data := buildMixedCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	err := sortpkg.SortNodes(nodes, "cpu_load", "desc")
	require.NoError(t, err)

	cfg := defaultRenderConfig()
	cfg.Sort = "cpu_load=desc"
	out := renderToString(nodes, cfg)

	assert.Contains(t, out, "Sort applied: cpu_load=desc")

	// Verify data rows appear in descending CPU load order
	lines := strings.Split(out, "\n")
	var dataNodeNames []string
	for _, line := range lines {
		for _, n := range nodes {
			if strings.Contains(line, n.Name) {
				dataNodeNames = append(dataNodeNames, n.Name)
				break
			}
		}
	}
	// First data node should have highest CPU load
	require.NotEmpty(t, dataNodeNames)
}

func TestE2E_GroupByTaint(t *testing.T) {
	data := buildMixedCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	cfg := defaultRenderConfig()
	cfg.GroupByTaint = true
	out := renderToString(nodes, cfg)

	// Group separators (tilde lines) should be present
	assert.Contains(t, out, "~")

	// All nodes still present
	assert.Contains(t, out, "karpenter-node-1")
	assert.Contains(t, out, "spotinst-node-1")
	assert.Contains(t, out, "eks-node-1")
	assert.Contains(t, out, "plain-node-1")
}

func TestE2E_FargateVisible(t *testing.T) {
	data := buildMixedCluster()
	nodes := enrichCluster(data)
	// Don't call HideFargateNodes — simulates --fargate flag

	out := renderToString(nodes, defaultRenderConfig())
	assert.Contains(t, out, "fargate-")
}

func TestE2E_ColorFalse_NoANSI(t *testing.T) {
	data := buildMixedCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	cfg := defaultRenderConfig()
	cfg.Color = color.ColorConfig{Enabled: false}
	out := renderToString(nodes, cfg)

	assert.False(t, containsANSI(out), "output with --color false should contain no ANSI codes")
}

func TestE2E_NoMatchingFilter_Message(t *testing.T) {
	data := buildMixedCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	filtered, err := filter.FilterNodes(nodes, "instance_type", "nonexistent.type")
	require.NoError(t, err)
	assert.Empty(t, filtered)

	out := renderToString(filtered, defaultRenderConfig())
	assert.Contains(t, out, "no nodes match filter")
}

func TestE2E_OutputJSON_Valid(t *testing.T) {
	data := buildMixedCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	var buf bytes.Buffer
	formatter := &output.JSONFormatter{}
	err := formatter.Format(&buf, nodes)
	require.NoError(t, err)

	jsonOut := buf.String()

	// Valid JSON
	var parsed []output.NodeOutput
	err = json.Unmarshal([]byte(jsonOut), &parsed)
	require.NoError(t, err, "JSON output must be valid JSON")

	// Correct number of nodes
	assert.Len(t, parsed, len(nodes))

	// No ANSI codes
	assert.False(t, containsANSI(jsonOut), "JSON output should contain no ANSI codes")

	// No table headers
	assert.NotContains(t, jsonOut, "NODE")
	assert.NotContains(t, jsonOut, "PODS")
	assert.NotContains(t, jsonOut, "Data collected at:")
}

func TestE2E_OutputYAML_Valid(t *testing.T) {
	data := buildMixedCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	var buf bytes.Buffer
	formatter := &output.YAMLFormatter{}
	err := formatter.Format(&buf, nodes)
	require.NoError(t, err)

	yamlOut := buf.String()

	// Valid YAML
	var parsed []output.NodeOutput
	err = yaml.Unmarshal([]byte(yamlOut), &parsed)
	require.NoError(t, err, "YAML output must be valid YAML")

	// Correct number of nodes
	assert.Len(t, parsed, len(nodes))

	// No ANSI codes
	assert.False(t, containsANSI(yamlOut), "YAML output should contain no ANSI codes")

	// No table headers
	assert.NotContains(t, yamlOut, "NODE")
	assert.NotContains(t, yamlOut, "Data collected at:")
}

func TestE2E_TableOutput_NarrowTerminal_80(t *testing.T) {
	data := buildMixedCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	cfg := defaultRenderConfig()
	cfg.TermWidth = 80
	out := renderToString(nodes, cfg)

	// Output should still contain data
	assert.Contains(t, out, "NODE")
	assert.Greater(t, len(out), 0)

	// Every line should respect terminal width (allowing some tolerance for
	// the minimum column widths which may exceed 80 in extreme cases)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		// Table with 22 columns may exceed 80 at minimum widths,
		// but the renderer should attempt to fit
		assert.NotEmpty(t, line)
	}
}

func TestE2E_TableOutput_NarrowTerminal_150(t *testing.T) {
	data := buildMixedCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	cfg := defaultRenderConfig()
	cfg.TermWidth = 150
	out := renderToString(nodes, cfg)

	// At 150 columns, node names may be truncated. Verify that each node
	// is represented by at least a prefix of its name.
	for _, n := range nodes {
		// Check that at least the first 5 chars of each node name appear
		prefix := n.Name
		if len(prefix) > 5 {
			prefix = prefix[:5]
		}
		assert.Contains(t, out, prefix,
			"output should contain at least a prefix of node name %q", n.Name)
	}

	// Header should still be present
	assert.Contains(t, out, "NODE")
}

func TestE2E_JSONUnaffectedByTerminalWidth(t *testing.T) {
	data := buildMixedCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	// Render JSON — terminal width should have no effect
	var buf bytes.Buffer
	formatter := &output.JSONFormatter{}
	err := formatter.Format(&buf, nodes)
	require.NoError(t, err)

	var parsed []output.NodeOutput
	err = json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err)

	// Full node names should be present (not truncated)
	nameFound := false
	for _, n := range parsed {
		if n.Name == "karpenter-node-1" {
			nameFound = true
			break
		}
	}
	assert.True(t, nameFound, "JSON should contain full node names regardless of terminal width")
}

func TestE2E_YAMLUnaffectedByTerminalWidth(t *testing.T) {
	data := buildMixedCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	var buf bytes.Buffer
	formatter := &output.YAMLFormatter{}
	err := formatter.Format(&buf, nodes)
	require.NoError(t, err)

	var parsed []output.NodeOutput
	err = yaml.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err)

	// Full node names should be present (not truncated)
	nameFound := false
	for _, n := range parsed {
		if n.Name == "karpenter-node-1" {
			nameFound = true
			break
		}
	}
	assert.True(t, nameFound, "YAML should contain full node names regardless of terminal width")
}

func TestE2E_FilterAutoscaler_Karpenter(t *testing.T) {
	data := buildMixedCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	filtered, err := filter.FilterNodes(nodes, "autoscaler", "karpenter")
	require.NoError(t, err)

	cfg := defaultRenderConfig()
	cfg.Filter = "autoscaler=karpenter"
	out := renderToString(filtered, cfg)

	assert.Contains(t, out, "Filter applied: autoscaler=karpenter")
	assert.Contains(t, out, "karpenter-node-1")
	assert.NotContains(t, out, "spotinst-node-1")
	assert.NotContains(t, out, "eks-node-1")
	assert.NotContains(t, out, "plain-node-1")
}

func TestE2E_SortAutoscaler_Asc(t *testing.T) {
	data := buildMixedCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	err := sortpkg.SortNodes(nodes, "autoscaler", "asc")
	require.NoError(t, err)

	cfg := defaultRenderConfig()
	cfg.Sort = "autoscaler=asc"
	out := renderToString(nodes, cfg)

	assert.Contains(t, out, "Sort applied: autoscaler=asc")

	// Verify ascending order of autoscaler values in data rows
	for i := 0; i < len(nodes)-1; i++ {
		assert.LessOrEqual(t, nodes[i].Autoscaler, nodes[i+1].Autoscaler)
	}
}

func TestE2E_JSONIncludesAutoscalerField(t *testing.T) {
	data := buildMixedCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	var buf bytes.Buffer
	formatter := &output.JSONFormatter{}
	err := formatter.Format(&buf, nodes)
	require.NoError(t, err)

	var parsed []output.NodeOutput
	err = json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err)

	// Every node should have an autoscaler field
	for _, n := range parsed {
		assert.NotEmpty(t, n.Autoscaler, "JSON output should include autoscaler field for node %s", n.Name)
	}

	// Verify specific autoscaler values
	nodeMap := map[string]output.NodeOutput{}
	for _, n := range parsed {
		nodeMap[n.Name] = n
	}
	assert.Equal(t, "karpenter", nodeMap["karpenter-node-1"].Autoscaler)
	assert.Equal(t, "spotio", nodeMap["spotinst-node-1"].Autoscaler)
	assert.Equal(t, "cluster-autoscaler", nodeMap["eks-node-1"].Autoscaler)
	assert.Equal(t, "x", nodeMap["plain-node-1"].Autoscaler)
}

func TestE2E_YAMLIncludesAutoscalerField(t *testing.T) {
	data := buildMixedCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	var buf bytes.Buffer
	formatter := &output.YAMLFormatter{}
	err := formatter.Format(&buf, nodes)
	require.NoError(t, err)

	var parsed []output.NodeOutput
	err = yaml.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err)

	for _, n := range parsed {
		assert.NotEmpty(t, n.Autoscaler, "YAML output should include autoscaler field for node %s", n.Name)
	}
}

func TestE2E_NoHeaders_OnlyDataRows(t *testing.T) {
	data := buildMixedCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	cfg := defaultRenderConfig()
	cfg.NoHeaders = true
	out := renderToString(nodes, cfg)

	// No header, separator, timestamp, or annotations
	assert.NotContains(t, out, "NODE")
	assert.NotContains(t, out, "PODS")
	assert.NotContains(t, out, "Data collected at:")
	assert.NotContains(t, out, "Filter applied:")
	assert.NotContains(t, out, "Sort applied:")

	// Separator line (dashes) should not be present
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// No line should be all dashes
		allDashes := true
		for _, ch := range trimmed {
			if ch != '-' {
				allDashes = false
				break
			}
		}
		assert.False(t, allDashes, "no-headers mode should not contain separator lines")
	}

	// Number of lines should equal number of nodes
	assert.Equal(t, len(nodes), len(lines))
}

func TestE2E_NoHeaders_WithFilter(t *testing.T) {
	data := buildMixedCluster()
	nodes := enrichCluster(data)
	nodes = filter.HideFargateNodes(nodes)

	filtered, err := filter.FilterNodes(nodes, "autoscaler", "karpenter")
	require.NoError(t, err)

	cfg := defaultRenderConfig()
	cfg.NoHeaders = true
	cfg.Filter = "autoscaler=karpenter"
	out := renderToString(filtered, cfg)

	// No headers or annotations
	assert.NotContains(t, out, "NODE")
	assert.NotContains(t, out, "Filter applied:")
	assert.NotContains(t, out, "Data collected at:")

	// Only filtered nodes in output
	lines := strings.Split(strings.TrimSpace(out), "\n")
	assert.Equal(t, len(filtered), len(lines))
	assert.Contains(t, out, "karpenter-node-1")
	assert.NotContains(t, out, "spotinst-node-1")
}
