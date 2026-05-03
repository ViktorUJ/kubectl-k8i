package output

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/kubectl-k8i/pkg/model"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	"pgregory.net/rapid"
)

// genNodeInfo generates a random NodeInfo for property testing.
func genNodeInfo() *rapid.Generator[model.NodeInfo] {
	return rapid.Custom[model.NodeInfo](func(t *rapid.T) model.NodeInfo {
		return model.NodeInfo{
			Name:             rapid.StringMatching(`[a-z0-9\-]{1,30}`).Draw(t, "name"),
			PodsUsed:         rapid.IntRange(0, 200).Draw(t, "podsUsed"),
			PodsMax:          rapid.IntRange(0, 200).Draw(t, "podsMax"),
			CPURequestCores:  rapid.Float64Range(0, 128).Draw(t, "cpuReq"),
			CPULimitCores:    rapid.Float64Range(0, 128).Draw(t, "cpuLim"),
			CPUUsageCores:    rapid.Float64Range(0, 128).Draw(t, "cpuUse"),
			CPUCapacityCores: rapid.Float64Range(0, 128).Draw(t, "cpuCap"),
			CPULoadPercent:   rapid.IntRange(0, 100).Draw(t, "cpuLoad"),
			MemRequestGB:     rapid.Float64Range(0, 512).Draw(t, "memReq"),
			MemLimitGB:       rapid.Float64Range(0, 512).Draw(t, "memLim"),
			MemUsageGB:       rapid.Float64Range(0, 512).Draw(t, "memUse"),
			MemCapacityGB:    rapid.Float64Range(0, 512).Draw(t, "memCap"),
			MemLoadPercent:   rapid.IntRange(0, 100).Draw(t, "memLoad"),
			EC2InstanceID:    rapid.StringMatching(`i-[a-f0-9]{17}`).Draw(t, "ec2"),
			InstanceType:     rapid.SampledFrom([]string{"m5.xlarge", "c5.2xlarge", "r5.large", "t3.medium"}).Draw(t, "type"),
			CapacityType:     rapid.SampledFrom([]string{"spot", "od", "x"}).Draw(t, "cap"),
			Architecture:     rapid.SampledFrom([]string{"amd64", "arm64"}).Draw(t, "arch"),
			Zone:             rapid.SampledFrom([]string{"1a", "1b", "2a"}).Draw(t, "zone"),
			Nodepool:         rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "pool"),
			Nodeclaim:        rapid.StringMatching(`[a-z]{3,15}`).Draw(t, "claim"),
			Autoscaler:       rapid.SampledFrom([]string{"karpenter", "cas", "spotio", "x"}).Draw(t, "as"),
			Age:              rapid.SampledFrom([]string{"5d12h", "3h45m", "12m"}).Draw(t, "age"),
			TaintStr:         rapid.SampledFrom([]string{"none", "dedicated=gpu:NoSchedule", "team=backend:NoExecute"}).Draw(t, "taints"),
		}
	})
}

// Feature: kubectl-k8i-plugin, Property 25: JSON output round-trip
// **Validates: Requirements 21.2, 21.4**
func TestJSONRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		numNodes := rapid.IntRange(0, 10).Draw(t, "numNodes")
		nodes := make([]model.NodeInfo, numNodes)
		for i := range nodes {
			nodes[i] = genNodeInfo().Draw(t, "node")
		}

		// Serialize to JSON.
		var buf bytes.Buffer
		f := &JSONFormatter{}
		err := f.Format(&buf, nodes)
		assert.NoError(t, err)

		// Deserialize back.
		var parsed []NodeOutput
		err = json.Unmarshal(buf.Bytes(), &parsed)
		assert.NoError(t, err)

		// Verify same count.
		assert.Equal(t, len(nodes), len(parsed))

		// Verify field equivalence.
		for i, n := range nodes {
			p := parsed[i]
			assert.Equal(t, n.Name, p.Name)
			assert.Equal(t, n.PodsUsed, p.PodsUsed)
			assert.Equal(t, n.PodsMax, p.PodsMax)
			assert.InDelta(t, n.CPURequestCores, p.CPURequestCores, 0.001)
			assert.InDelta(t, n.CPULimitCores, p.CPULimitCores, 0.001)
			assert.InDelta(t, n.CPUUsageCores, p.CPUUsageCores, 0.001)
			assert.InDelta(t, n.CPUCapacityCores, p.CPUCapacityCores, 0.001)
			assert.Equal(t, n.CPULoadPercent, p.CPULoadPercent)
			assert.InDelta(t, n.MemRequestGB, p.MemRequestGB, 0.001)
			assert.InDelta(t, n.MemLimitGB, p.MemLimitGB, 0.001)
			assert.InDelta(t, n.MemUsageGB, p.MemUsageGB, 0.001)
			assert.InDelta(t, n.MemCapacityGB, p.MemCapacityGB, 0.001)
			assert.Equal(t, n.MemLoadPercent, p.MemLoadPercent)
			assert.Equal(t, n.EC2InstanceID, p.EC2InstanceID)
			assert.Equal(t, n.InstanceType, p.InstanceType)
			assert.Equal(t, n.CapacityType, p.CapacityType)
			assert.Equal(t, n.Architecture, p.Architecture)
			assert.Equal(t, n.Zone, p.Zone)
			assert.Equal(t, n.Nodepool, p.Nodepool)
			assert.Equal(t, n.Nodeclaim, p.Nodeclaim)
			assert.Equal(t, n.Autoscaler, p.Autoscaler)
			assert.Equal(t, n.Age, p.Age)
			assert.Equal(t, n.TaintStr, p.Taints)
		}
	})
}

// Feature: kubectl-k8i-plugin, Property 26: YAML output round-trip
// **Validates: Requirements 21.3, 21.5**
func TestYAMLRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		numNodes := rapid.IntRange(0, 10).Draw(t, "numNodes")
		nodes := make([]model.NodeInfo, numNodes)
		for i := range nodes {
			nodes[i] = genNodeInfo().Draw(t, "node")
		}

		// Serialize to YAML.
		var buf bytes.Buffer
		f := &YAMLFormatter{}
		err := f.Format(&buf, nodes)
		assert.NoError(t, err)

		// Deserialize back.
		var parsed []NodeOutput
		err = yaml.Unmarshal(buf.Bytes(), &parsed)
		assert.NoError(t, err)

		// Verify same count.
		assert.Equal(t, len(nodes), len(parsed))

		// Verify field equivalence.
		for i, n := range nodes {
			p := parsed[i]
			assert.Equal(t, n.Name, p.Name)
			assert.Equal(t, n.PodsUsed, p.PodsUsed)
			assert.Equal(t, n.PodsMax, p.PodsMax)
			assert.InDelta(t, n.CPURequestCores, p.CPURequestCores, 0.001)
			assert.InDelta(t, n.CPULimitCores, p.CPULimitCores, 0.001)
			assert.InDelta(t, n.CPUUsageCores, p.CPUUsageCores, 0.001)
			assert.InDelta(t, n.CPUCapacityCores, p.CPUCapacityCores, 0.001)
			assert.Equal(t, n.CPULoadPercent, p.CPULoadPercent)
			assert.InDelta(t, n.MemRequestGB, p.MemRequestGB, 0.001)
			assert.InDelta(t, n.MemLimitGB, p.MemLimitGB, 0.001)
			assert.InDelta(t, n.MemUsageGB, p.MemUsageGB, 0.001)
			assert.InDelta(t, n.MemCapacityGB, p.MemCapacityGB, 0.001)
			assert.Equal(t, n.MemLoadPercent, p.MemLoadPercent)
			assert.Equal(t, n.EC2InstanceID, p.EC2InstanceID)
			assert.Equal(t, n.InstanceType, p.InstanceType)
			assert.Equal(t, n.CapacityType, p.CapacityType)
			assert.Equal(t, n.Architecture, p.Architecture)
			assert.Equal(t, n.Zone, p.Zone)
			assert.Equal(t, n.Nodepool, p.Nodepool)
			assert.Equal(t, n.Nodeclaim, p.Nodeclaim)
			assert.Equal(t, n.Autoscaler, p.Autoscaler)
			assert.Equal(t, n.Age, p.Age)
			assert.Equal(t, n.TaintStr, p.Taints)
		}
	})
}

// Feature: kubectl-k8i-plugin, Property 27: Output format data equivalence
// **Validates: Requirements 21.6**
func TestOutputFormatEquivalence(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		numNodes := rapid.IntRange(0, 10).Draw(t, "numNodes")
		nodes := make([]model.NodeInfo, numNodes)
		for i := range nodes {
			nodes[i] = genNodeInfo().Draw(t, "node")
		}

		// Serialize to JSON.
		var jsonBuf bytes.Buffer
		jf := &JSONFormatter{}
		err := jf.Format(&jsonBuf, nodes)
		assert.NoError(t, err)

		// Serialize to YAML.
		var yamlBuf bytes.Buffer
		yf := &YAMLFormatter{}
		err = yf.Format(&yamlBuf, nodes)
		assert.NoError(t, err)

		// Deserialize both.
		var jsonParsed []NodeOutput
		err = json.Unmarshal(jsonBuf.Bytes(), &jsonParsed)
		assert.NoError(t, err)

		var yamlParsed []NodeOutput
		err = yaml.Unmarshal(yamlBuf.Bytes(), &yamlParsed)
		assert.NoError(t, err)

		// Same count.
		assert.Equal(t, len(jsonParsed), len(yamlParsed))

		// Same field values.
		for i := range jsonParsed {
			j := jsonParsed[i]
			y := yamlParsed[i]
			assert.Equal(t, j.Name, y.Name)
			assert.Equal(t, j.PodsUsed, y.PodsUsed)
			assert.Equal(t, j.PodsMax, y.PodsMax)
			assert.InDelta(t, j.CPURequestCores, y.CPURequestCores, 0.001)
			assert.InDelta(t, j.CPULimitCores, y.CPULimitCores, 0.001)
			assert.InDelta(t, j.CPUUsageCores, y.CPUUsageCores, 0.001)
			assert.InDelta(t, j.CPUCapacityCores, y.CPUCapacityCores, 0.001)
			assert.Equal(t, j.CPULoadPercent, y.CPULoadPercent)
			assert.InDelta(t, j.MemRequestGB, y.MemRequestGB, 0.001)
			assert.InDelta(t, j.MemLimitGB, y.MemLimitGB, 0.001)
			assert.InDelta(t, j.MemUsageGB, y.MemUsageGB, 0.001)
			assert.InDelta(t, j.MemCapacityGB, y.MemCapacityGB, 0.001)
			assert.Equal(t, j.MemLoadPercent, y.MemLoadPercent)
			assert.Equal(t, j.EC2InstanceID, y.EC2InstanceID)
			assert.Equal(t, j.InstanceType, y.InstanceType)
			assert.Equal(t, j.CapacityType, y.CapacityType)
			assert.Equal(t, j.Architecture, y.Architecture)
			assert.Equal(t, j.Zone, y.Zone)
			assert.Equal(t, j.Nodepool, y.Nodepool)
			assert.Equal(t, j.Nodeclaim, y.Nodeclaim)
			assert.Equal(t, j.Autoscaler, y.Autoscaler)
			assert.Equal(t, j.Age, y.Age)
			assert.Equal(t, j.Taints, y.Taints)
		}
	})
}

// Feature: kubectl-k8i-plugin, Property 28: Structured output integrity
// **Validates: Requirements 21.7, 22.8**
func TestStructuredOutputIntegrity(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		numNodes := rapid.IntRange(1, 10).Draw(t, "numNodes")
		nodes := make([]model.NodeInfo, numNodes)
		for i := range nodes {
			nodes[i] = genNodeInfo().Draw(t, "node")
		}

		// Test JSON output.
		var jsonBuf bytes.Buffer
		jf := &JSONFormatter{}
		err := jf.Format(&jsonBuf, nodes)
		assert.NoError(t, err)
		jsonStr := jsonBuf.String()

		// No ANSI codes in JSON.
		assert.False(t, strings.Contains(jsonStr, "\033["),
			"JSON output must not contain ANSI escape codes")

		// Deserialize and verify untruncated field values.
		var jsonParsed []NodeOutput
		err = json.Unmarshal(jsonBuf.Bytes(), &jsonParsed)
		assert.NoError(t, err)
		for i, n := range nodes {
			p := jsonParsed[i]
			assert.Equal(t, n.Name, p.Name, "name must not be truncated in JSON")
			assert.Equal(t, n.Nodepool, p.Nodepool, "nodepool must not be truncated in JSON")
			assert.Equal(t, n.Nodeclaim, p.Nodeclaim, "nodeclaim must not be truncated in JSON")
			assert.Equal(t, n.TaintStr, p.Taints, "taints must not be truncated in JSON")
			// Verify float fields are not NaN or Inf.
			assert.False(t, math.IsNaN(p.CPURequestCores), "CPU request must not be NaN")
			assert.False(t, math.IsInf(p.CPURequestCores, 0), "CPU request must not be Inf")
		}

		// Test YAML output.
		var yamlBuf bytes.Buffer
		yf := &YAMLFormatter{}
		err = yf.Format(&yamlBuf, nodes)
		assert.NoError(t, err)
		yamlStr := yamlBuf.String()

		// No ANSI codes in YAML.
		assert.False(t, strings.Contains(yamlStr, "\033["),
			"YAML output must not contain ANSI escape codes")
	})
}
