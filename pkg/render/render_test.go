package render

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/kubectl-k8i/pkg/color"
	"github.com/kubectl-k8i/pkg/model"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func sampleNode(name string) model.NodeInfo {
	return model.NodeInfo{
		Name:             name,
		PodsUsed:         5,
		PodsMax:          110,
		CPURequestCores:  1.5,
		CPULimitCores:    3.0,
		CPUUsageCores:    2.0,
		CPUCapacityCores: 4.0,
		CPULoadPercent:   50,
		MemRequestGB:     4.0,
		MemLimitGB:       8.0,
		MemUsageGB:       6.0,
		MemCapacityGB:    16.0,
		MemLoadPercent:   38,
		EC2InstanceID:    "i-0abcdef1234567890",
		InstanceType:     "m5.xlarge",
		CapacityType:     "spot",
		Architecture:     "amd64",
		Zone:             "1a",
		Nodepool:         "pool-a",
		Nodeclaim:        "claim-1",
		Autoscaler:       "karpenter",
		Age:              "5d12h",
		TaintStr:         "none",
	}
}

func noColorConfig() color.ColorConfig {
	return color.ColorConfig{Enabled: false}
}

func defaultRenderConfig() RenderConfig {
	return RenderConfig{
		Color:     noColorConfig(),
		Sort:      "pool=asc",
		Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		TermWidth: 0,
	}
}

func TestRenderTable_FullWidth(t *testing.T) {
	nodes := []model.NodeInfo{sampleNode("node-1"), sampleNode("node-2")}
	var buf bytes.Buffer
	cfg := defaultRenderConfig()
	RenderTable(&buf, nodes, cfg)

	output := buf.String()
	assert.Contains(t, output, "NODE")
	assert.Contains(t, output, "PODS")
	assert.Contains(t, output, "CPU cores")
	assert.Contains(t, output, "MEMORY GB")
	assert.Contains(t, output, "Node info")
	assert.Contains(t, output, "node-1")
	assert.Contains(t, output, "node-2")
	assert.Contains(t, output, "===")
}

func TestRenderTable_ThreeLineHeader(t *testing.T) {
	nodes := []model.NodeInfo{sampleNode("node-1")}
	var buf bytes.Buffer
	cfg := defaultRenderConfig()
	RenderTable(&buf, nodes, cfg)

	lines := strings.Split(buf.String(), "\n")
	assert.GreaterOrEqual(t, len(lines), 5)
	assert.Contains(t, lines[0], "NODE")
	assert.Contains(t, lines[0], "CPU cores")
	assert.Contains(t, lines[0], "MEMORY GB")
	assert.Contains(t, lines[1], "used/")
	assert.Contains(t, lines[1], "req/lim/")
	assert.Contains(t, lines[1], "LOAD")
	assert.Contains(t, lines[2], "max")
	assert.Contains(t, lines[2], "use/total")
	assert.True(t, strings.HasPrefix(lines[3], "==="))
}

func TestRenderTable_DataRowFormat(t *testing.T) {
	node := sampleNode("node-1")
	var buf bytes.Buffer
	cfg := defaultRenderConfig()
	cfg.NoHeaders = true
	RenderTable(&buf, []model.NodeInfo{node}, cfg)

	output := buf.String()
	assert.Contains(t, output, "1.5/3.0/2.0/4")
	assert.Contains(t, output, "4.0/8.0/6.0/16")
	assert.Contains(t, output, "i-0abcdef1234567890/m5.xlarge/spot/amd64/1a/pool-a/claim-1/5d12h/none")
	assert.Contains(t, output, "5/110")
	assert.Contains(t, output, "50%")
	assert.Contains(t, output, "38%")
}

func TestRenderTable_CapacityFormatting(t *testing.T) {
	node := sampleNode("node-1")
	node.CPUCapacityCores = 8.0
	node.MemCapacityGB = 15.3

	var buf bytes.Buffer
	cfg := defaultRenderConfig()
	cfg.NoHeaders = true
	RenderTable(&buf, []model.NodeInfo{node}, cfg)

	output := buf.String()
	assert.Contains(t, output, "/8")
	assert.Contains(t, output, "/15.3")
}

func TestTruncateToFit_Ellipsis(t *testing.T) {
	result := TruncateToFit("abcdefghij", 5)
	assert.Equal(t, "abcd…", result)
	assert.Len(t, []rune(result), 5)

	result = TruncateToFit("abc", 10)
	assert.Equal(t, "abc", result)

	result = TruncateToFit("abcde", 5)
	assert.Equal(t, "abcde", result)

	result = TruncateToFit("abc", 0)
	assert.Equal(t, "", result)

	result = TruncateToFit("abc", 1)
	assert.Equal(t, "…", result)
}

func TestRenderTable_NumericColumnsNeverTruncated(t *testing.T) {
	node := sampleNode("node-1")
	node.CPURequestCores = 99.9
	node.MemCapacityGB = 512.0
	node.PodsUsed = 100
	node.PodsMax = 110

	var buf bytes.Buffer
	cfg := defaultRenderConfig()
	RenderTable(&buf, []model.NodeInfo{node}, cfg)

	output := buf.String()
	assert.Contains(t, output, "99.9")
	assert.Contains(t, output, "512")
	assert.Contains(t, output, "100/110")
}

func TestRenderTable_NoHeadersMode(t *testing.T) {
	nodes := []model.NodeInfo{sampleNode("node-1"), sampleNode("node-2")}
	var buf bytes.Buffer
	cfg := defaultRenderConfig()
	cfg.NoHeaders = true
	RenderTable(&buf, nodes, cfg)

	output := buf.String()
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")

	assert.NotContains(t, output, "NODE")
	assert.NotContains(t, output, "PODS")
	assert.NotContains(t, output, "===")
	assert.Equal(t, 2, len(lines))
	assert.Contains(t, lines[0], "node-1")
	assert.Contains(t, lines[1], "node-2")
}

func TestRenderTable_GroupSeparators(t *testing.T) {
	node1 := sampleNode("node-1")
	node1.Taints = []corev1.Taint{{Key: "dedicated", Value: "gpu", Effect: corev1.TaintEffectNoSchedule}}
	node1.TaintStr = "dedicated=gpu:NoSchedule"

	node2 := sampleNode("node-2")
	node2.Taints = []corev1.Taint{{Key: "dedicated", Value: "gpu", Effect: corev1.TaintEffectNoSchedule}}
	node2.TaintStr = "dedicated=gpu:NoSchedule"

	node3 := sampleNode("node-3")
	node3.Taints = []corev1.Taint{{Key: "team", Value: "backend", Effect: corev1.TaintEffectNoExecute}}
	node3.TaintStr = "team=backend:NoExecute"

	var buf bytes.Buffer
	cfg := defaultRenderConfig()
	cfg.GroupByTaint = true
	RenderTable(&buf, []model.NodeInfo{node1, node2, node3}, cfg)

	output := buf.String()
	assert.Contains(t, output, "~~~")
}

func TestRenderTable_EmptyNodeList(t *testing.T) {
	var buf bytes.Buffer
	cfg := defaultRenderConfig()
	RenderTable(&buf, []model.NodeInfo{}, cfg)
	assert.Contains(t, buf.String(), "no nodes match filter")
}

func TestRenderTable_EmptyNodeList_NoHeaders(t *testing.T) {
	var buf bytes.Buffer
	cfg := defaultRenderConfig()
	cfg.NoHeaders = true
	RenderTable(&buf, []model.NodeInfo{}, cfg)
	output := buf.String()
	assert.Contains(t, output, "no nodes match filter")
}

func TestRenderTable_NoAnnotations(t *testing.T) {
	nodes := []model.NodeInfo{sampleNode("node-1")}
	var buf bytes.Buffer
	cfg := defaultRenderConfig()
	cfg.Filter = "arch=amd64"
	RenderTable(&buf, nodes, cfg)
	output := buf.String()
	assert.NotContains(t, output, "Filter applied:")
	assert.NotContains(t, output, "Data collected at:")
}

func TestFormatCapacity(t *testing.T) {
	assert.Equal(t, "4", formatCapacity(4.0))
	assert.Equal(t, "16", formatCapacity(16.0))
	assert.Equal(t, "0", formatCapacity(0.0))
	assert.Equal(t, "15.3", formatCapacity(15.3))
	assert.Equal(t, "7.5", formatCapacity(7.5))
}
