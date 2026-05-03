package filter

import (
	"testing"

	"github.com/kubectl-k8i/pkg/model"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

// helper to build a minimal NodeInfo with the given fields.
func node(name string, opts ...func(*model.NodeInfo)) model.NodeInfo {
	n := model.NodeInfo{Name: name}
	for _, o := range opts {
		o(&n)
	}
	return n
}

func withCapacityType(v string) func(*model.NodeInfo) {
	return func(n *model.NodeInfo) { n.CapacityType = v }
}
func withInstanceType(v string) func(*model.NodeInfo) {
	return func(n *model.NodeInfo) { n.InstanceType = v }
}
func withArch(v string) func(*model.NodeInfo) {
	return func(n *model.NodeInfo) { n.Architecture = v }
}
func withZone(v string) func(*model.NodeInfo) {
	return func(n *model.NodeInfo) { n.Zone = v }
}
func withPool(v string) func(*model.NodeInfo) {
	return func(n *model.NodeInfo) { n.Nodepool = v }
}
func withNodeclaim(v string) func(*model.NodeInfo) {
	return func(n *model.NodeInfo) { n.Nodeclaim = v }
}
func withAutoscaler(v string) func(*model.NodeInfo) {
	return func(n *model.NodeInfo) { n.Autoscaler = v }
}
func withTaints(taints []corev1.Taint) func(*model.NodeInfo) {
	return func(n *model.NodeInfo) { n.Taints = taints }
}

func sampleNodes() []model.NodeInfo {
	return []model.NodeInfo{
		node("node-1",
			withCapacityType("spot"),
			withInstanceType("m5.xlarge"),
			withArch("amd64"),
			withZone("1a"),
			withPool("pool-a"),
			withNodeclaim("claim-1"),
			withAutoscaler("karpenter"),
			withTaints([]corev1.Taint{{Key: "dedicated", Value: "gpu", Effect: corev1.TaintEffectNoSchedule}}),
		),
		node("node-2",
			withCapacityType("od"),
			withInstanceType("c5.2xlarge"),
			withArch("arm64"),
			withZone("1b"),
			withPool("pool-b"),
			withNodeclaim("claim-2"),
			withAutoscaler("cas"),
			withTaints([]corev1.Taint{{Key: "team", Value: "backend", Effect: corev1.TaintEffectNoExecute}}),
		),
		node("node-3",
			withCapacityType("spot"),
			withInstanceType("m5.xlarge"),
			withArch("amd64"),
			withZone("1c"),
			withPool("pool-a"),
			withNodeclaim("claim-3"),
			withAutoscaler("spotio"),
		),
	}
}

// --- ec2_type attribute ---

func TestFilterByEC2Type_Match(t *testing.T) {
	nodes := sampleNodes()
	result, err := FilterNodes(nodes, "ec2_type", "spot")
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "node-1", result[0].Name)
	assert.Equal(t, "node-3", result[1].Name)
}

func TestFilterByEC2Type_NoMatch(t *testing.T) {
	nodes := sampleNodes()
	result, err := FilterNodes(nodes, "ec2_type", "reserved")
	assert.NoError(t, err)
	assert.Empty(t, result)
}

// --- instance_type attribute ---

func TestFilterByInstanceType_Match(t *testing.T) {
	nodes := sampleNodes()
	result, err := FilterNodes(nodes, "instance_type", "c5.2xlarge")
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "node-2", result[0].Name)
}

func TestFilterByInstanceType_NoMatch(t *testing.T) {
	nodes := sampleNodes()
	result, err := FilterNodes(nodes, "instance_type", "r5.large")
	assert.NoError(t, err)
	assert.Empty(t, result)
}

// --- arch attribute ---

func TestFilterByArch_Match(t *testing.T) {
	nodes := sampleNodes()
	result, err := FilterNodes(nodes, "arch", "arm64")
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "node-2", result[0].Name)
}

func TestFilterByArch_NoMatch(t *testing.T) {
	nodes := sampleNodes()
	result, err := FilterNodes(nodes, "arch", "s390x")
	assert.NoError(t, err)
	assert.Empty(t, result)
}

// --- zone attribute ---

func TestFilterByZone_Match(t *testing.T) {
	nodes := sampleNodes()
	result, err := FilterNodes(nodes, "zone", "1b")
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "node-2", result[0].Name)
}

func TestFilterByZone_NoMatch(t *testing.T) {
	nodes := sampleNodes()
	result, err := FilterNodes(nodes, "zone", "2a")
	assert.NoError(t, err)
	assert.Empty(t, result)
}

// --- pool attribute ---

func TestFilterByPool_Match(t *testing.T) {
	nodes := sampleNodes()
	result, err := FilterNodes(nodes, "pool", "pool-a")
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "node-1", result[0].Name)
	assert.Equal(t, "node-3", result[1].Name)
}

func TestFilterByPool_NoMatch(t *testing.T) {
	nodes := sampleNodes()
	result, err := FilterNodes(nodes, "pool", "pool-z")
	assert.NoError(t, err)
	assert.Empty(t, result)
}

// --- nodeclaim attribute ---

func TestFilterByNodeclaim_Match(t *testing.T) {
	nodes := sampleNodes()
	result, err := FilterNodes(nodes, "nodeclaim", "claim-2")
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "node-2", result[0].Name)
}

func TestFilterByNodeclaim_NoMatch(t *testing.T) {
	nodes := sampleNodes()
	result, err := FilterNodes(nodes, "nodeclaim", "claim-99")
	assert.NoError(t, err)
	assert.Empty(t, result)
}

// --- taint attribute ---

func TestFilterByTaint_KeyMatch(t *testing.T) {
	nodes := sampleNodes()
	result, err := FilterNodes(nodes, "taint", "dedicated")
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "node-1", result[0].Name)
}

func TestFilterByTaint_KeyValueMatch(t *testing.T) {
	nodes := sampleNodes()
	result, err := FilterNodes(nodes, "taint", "team=backend")
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "node-2", result[0].Name)
}

func TestFilterByTaint_NoMatch(t *testing.T) {
	nodes := sampleNodes()
	result, err := FilterNodes(nodes, "taint", "nonexistent")
	assert.NoError(t, err)
	assert.Empty(t, result)
}

// --- autoscaler attribute ---

func TestFilterByAutoscaler_Match(t *testing.T) {
	nodes := sampleNodes()
	result, err := FilterNodes(nodes, "autoscaler", "karpenter")
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "node-1", result[0].Name)
}

func TestFilterByAutoscaler_NoMatch(t *testing.T) {
	nodes := sampleNodes()
	result, err := FilterNodes(nodes, "autoscaler", "unknown")
	assert.NoError(t, err)
	assert.Empty(t, result)
}

// --- case-insensitive matching ---

func TestFilterCaseInsensitive(t *testing.T) {
	nodes := sampleNodes()
	result, err := FilterNodes(nodes, "ec2_type", "SPOT")
	assert.NoError(t, err)
	assert.Len(t, result, 2, "filter should be case-insensitive")
}

// --- error cases ---

func TestFilterUnsupportedAttribute(t *testing.T) {
	nodes := sampleNodes()
	_, err := FilterNodes(nodes, "cpu_load", "50")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported filter attribute")
	assert.Contains(t, err.Error(), "cpu_load")
}

func TestFilterEmptyAttribute(t *testing.T) {
	nodes := sampleNodes()
	_, err := FilterNodes(nodes, "", "value")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported filter attribute")
}

// --- Fargate hiding ---

func TestHideFargateNodes(t *testing.T) {
	nodes := []model.NodeInfo{
		node("ip-10-0-1-1.ec2.internal"),
		node("fargate-ip-10-0-2-2.ec2.internal"),
		node("ip-10-0-3-3.ec2.internal"),
		node("fargate-ip-10-0-4-4.ec2.internal"),
	}
	result := HideFargateNodes(nodes)
	assert.Len(t, result, 2)
	for _, n := range result {
		assert.NotContains(t, n.Name, "fargate-")
	}
}

func TestHideFargateNodes_NoFargate(t *testing.T) {
	nodes := []model.NodeInfo{
		node("node-1"),
		node("node-2"),
	}
	result := HideFargateNodes(nodes)
	assert.Len(t, result, 2)
}

func TestHideFargateNodes_AllFargate(t *testing.T) {
	nodes := []model.NodeInfo{
		node("fargate-a"),
		node("fargate-b"),
	}
	result := HideFargateNodes(nodes)
	assert.Empty(t, result)
}

func TestHideFargateNodes_Empty(t *testing.T) {
	result := HideFargateNodes(nil)
	assert.Empty(t, result)
}

// --- empty input ---

func TestFilterEmptyNodeList(t *testing.T) {
	result, err := FilterNodes(nil, "arch", "amd64")
	assert.NoError(t, err)
	assert.Empty(t, result)
}
