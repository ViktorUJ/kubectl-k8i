package render

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kubectl-k8i/pkg/color"
	"github.com/kubectl-k8i/pkg/model"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func genNodeInfoForRender() *rapid.Generator[model.NodeInfo] {
	return rapid.Custom[model.NodeInfo](func(t *rapid.T) model.NodeInfo {
		return model.NodeInfo{
			Name:             rapid.StringMatching(`[a-z0-9\-]{1,40}`).Draw(t, "name"),
			PodsUsed:         rapid.IntRange(0, 110).Draw(t, "podsUsed"),
			PodsMax:          rapid.IntRange(0, 200).Draw(t, "podsMax"),
			CPURequestCores:  rapid.Float64Range(0, 64).Draw(t, "cpuReq"),
			CPULimitCores:    rapid.Float64Range(0, 64).Draw(t, "cpuLim"),
			CPUUsageCores:    rapid.Float64Range(0, 64).Draw(t, "cpuUse"),
			CPUCapacityCores: rapid.Float64Range(0, 64).Draw(t, "cpuCap"),
			CPULoadPercent:   rapid.IntRange(0, 100).Draw(t, "cpuLoad"),
			MemRequestGB:     rapid.Float64Range(0, 512).Draw(t, "memReq"),
			MemLimitGB:       rapid.Float64Range(0, 512).Draw(t, "memLim"),
			MemUsageGB:       rapid.Float64Range(0, 512).Draw(t, "memUse"),
			MemCapacityGB:    rapid.Float64Range(0, 512).Draw(t, "memCap"),
			MemLoadPercent:   rapid.IntRange(0, 100).Draw(t, "memLoad"),
			EC2InstanceID:    rapid.StringMatching(`i-[a-f0-9]{17}`).Draw(t, "ec2"),
			InstanceType:     rapid.SampledFrom([]string{"m5.xlarge", "c5.2xlarge", "r5.large"}).Draw(t, "type"),
			CapacityType:     rapid.SampledFrom([]string{"spot", "od", "x"}).Draw(t, "cap"),
			Architecture:     rapid.SampledFrom([]string{"amd64", "arm64"}).Draw(t, "arch"),
			Zone:             rapid.SampledFrom([]string{"1a", "1b", "2a"}).Draw(t, "zone"),
			Nodepool:         rapid.StringMatching(`[a-z]{3,12}`).Draw(t, "pool"),
			Nodeclaim:        rapid.StringMatching(`[a-z]{3,18}`).Draw(t, "claim"),
			Autoscaler:       rapid.SampledFrom([]string{"karpenter", "cluster-autoscaler", "spotio", "x"}).Draw(t, "as"),
			Age:              rapid.SampledFrom([]string{"5d12h", "3h45m", "12m", "0m"}).Draw(t, "age"),
			TaintStr:         rapid.SampledFrom([]string{"none", "dedicated=gpu:NoSchedule", "a:NoSchedule,b:NoExecute"}).Draw(t, "taints"),
		}
	})
}

func TestTruncationEllipsis(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.StringMatching(`[a-zA-Z0-9\-]{0,50}`).Draw(t, "str")
		maxLen := rapid.IntRange(1, 50).Draw(t, "maxLen")
		result := TruncateToFit(s, maxLen)
		runeLen := len([]rune(result))
		if len([]rune(s)) > maxLen {
			assert.LessOrEqual(t, runeLen, maxLen)
			assert.True(t, strings.HasSuffix(result, "…"))
		} else {
			assert.Equal(t, s, result)
		}
	})
}

func TestNumericColumnsNotTruncated(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		node := genNodeInfoForRender().Draw(t, "node")
		var buf bytes.Buffer
		cfg := RenderConfig{Color: color.ColorConfig{Enabled: false}, NoHeaders: true}
		RenderTable(&buf, []model.NodeInfo{node}, cfg)
		output := buf.String()
		assert.Contains(t, output, fmt.Sprintf("%d/%d", node.PodsUsed, node.PodsMax))
		assert.Contains(t, output, fmt.Sprintf("%.1f", node.CPURequestCores))
		assert.Contains(t, output, fmt.Sprintf("%.1f", node.CPULimitCores))
		assert.Contains(t, output, fmt.Sprintf("%.1f", node.CPUUsageCores))
		assert.Contains(t, output, fmt.Sprintf("%.1f", node.MemRequestGB))
		assert.Contains(t, output, fmt.Sprintf("%.1f", node.MemLimitGB))
		assert.Contains(t, output, fmt.Sprintf("%.1f", node.MemUsageGB))
	})
}

func TestNoHeadersMode(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		numNodes := rapid.IntRange(1, 10).Draw(t, "numNodes")
		nodes := make([]model.NodeInfo, numNodes)
		for i := range nodes {
			nodes[i] = genNodeInfoForRender().Draw(t, "node")
		}
		var buf bytes.Buffer
		cfg := RenderConfig{
			Color: color.ColorConfig{Enabled: false}, Filter: "arch=amd64", Sort: "pool=asc",
			Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC), NoHeaders: true,
		}
		RenderTable(&buf, nodes, cfg)
		lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
		assert.Equal(t, numNodes, len(lines))
		assert.NotContains(t, buf.String(), "NODE")
		assert.NotContains(t, buf.String(), "===")
	})
}

func TestDataRowContainsNodeInfoCombo(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		node := genNodeInfoForRender().Draw(t, "node")
		var buf bytes.Buffer
		cfg := RenderConfig{Color: color.ColorConfig{Enabled: false}, NoHeaders: true}
		RenderTable(&buf, []model.NodeInfo{node}, cfg)
		// Node info should contain the taint (possibly truncated).
		taintDisplay := node.TaintStr
		if len([]rune(taintDisplay)) > 20 {
			taintDisplay = string([]rune(taintDisplay)[:19]) + "…"
		}
		expected := fmt.Sprintf("%s/%s/%s/%s/%s/%s/%s/%s/%s",
			node.EC2InstanceID, node.InstanceType, node.CapacityType,
			node.Architecture, node.Zone, node.Nodepool, node.Nodeclaim, node.Age, taintDisplay)
		assert.Contains(t, buf.String(), expected)
	})
}
