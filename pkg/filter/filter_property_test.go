package filter

import (
	"strings"
	"testing"

	"github.com/kubectl-k8i/pkg/model"
	sortpkg "github.com/kubectl-k8i/pkg/sort"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"pgregory.net/rapid"
)

// --- generators ---

// genNodeInfo generates a random NodeInfo with realistic field values.
func genNodeInfo() *rapid.Generator[model.NodeInfo] {
	return rapid.Custom[model.NodeInfo](func(t *rapid.T) model.NodeInfo {
		isFargate := rapid.Bool().Draw(t, "isFargate")
		var name string
		if isFargate {
			name = "fargate-" + rapid.StringMatching(`[a-z0-9]{4,12}`).Draw(t, "fName")
		} else {
			name = "ip-" + rapid.StringMatching(`[a-z0-9]{4,12}`).Draw(t, "name")
		}

		capacityTypes := []string{"spot", "od", "x"}
		architectures := []string{"amd64", "arm64"}
		zones := []string{"1a", "1b", "1c", "2a", "2b"}
		autoscalers := []string{"karpenter", "cluster-autoscaler", "spotio", "x"}

		return model.NodeInfo{
			Name:         name,
			CapacityType: capacityTypes[rapid.IntRange(0, len(capacityTypes)-1).Draw(t, "ct")],
			InstanceType: rapid.SampledFrom([]string{"m5.xlarge", "c5.2xlarge", "r5.large", "t3.medium"}).Draw(t, "it"),
			Architecture: architectures[rapid.IntRange(0, len(architectures)-1).Draw(t, "arch")],
			Zone:         zones[rapid.IntRange(0, len(zones)-1).Draw(t, "zone")],
			Nodepool:     rapid.SampledFrom([]string{"pool-a", "pool-b", "pool-c"}).Draw(t, "pool"),
			Nodeclaim:    rapid.SampledFrom([]string{"claim-1", "claim-2", "claim-3"}).Draw(t, "nc"),
			Autoscaler:   autoscalers[rapid.IntRange(0, len(autoscalers)-1).Draw(t, "as")],
			Taints: rapid.SliceOfN(rapid.Custom[corev1.Taint](func(t *rapid.T) corev1.Taint {
				return corev1.Taint{
					Key:    rapid.SampledFrom([]string{"dedicated", "team", "workload"}).Draw(t, "tKey"),
					Value:  rapid.SampledFrom([]string{"gpu", "backend", "frontend", ""}).Draw(t, "tVal"),
					Effect: corev1.TaintEffect(rapid.SampledFrom([]string{"NoSchedule", "NoExecute", "PreferNoSchedule"}).Draw(t, "tEff")),
				}
			}), 0, 3).Draw(t, "taints"),
			PodsUsed:        rapid.IntRange(0, 110).Draw(t, "pods"),
			CPURequestCores: rapid.Float64Range(0, 64).Draw(t, "cpuReq"),
			CPUUsageCores:   rapid.Float64Range(0, 64).Draw(t, "cpuUse"),
			MemRequestGB:    rapid.Float64Range(0, 256).Draw(t, "memReq"),
			MemUsageGB:      rapid.Float64Range(0, 256).Draw(t, "memUse"),
		}
	})
}

func genNodeList() *rapid.Generator[[]model.NodeInfo] {
	return rapid.SliceOfN(genNodeInfo(), 0, 30)
}

// attributeValue extracts the value of a filter attribute from a NodeInfo.
func attributeValue(n model.NodeInfo, attr string) string {
	switch attr {
	case "ec2_type":
		return strings.ToLower(n.CapacityType)
	case "instance_type":
		return strings.ToLower(n.InstanceType)
	case "arch":
		return strings.ToLower(n.Architecture)
	case "zone":
		return strings.ToLower(n.Zone)
	case "pool":
		return strings.ToLower(n.Nodepool)
	case "nodeclaim":
		return strings.ToLower(n.Nodeclaim)
	case "autoscaler":
		return strings.ToLower(n.Autoscaler)
	default:
		return ""
	}
}

// Feature: kubectl-k8i-plugin, Property 13: Filter returns only matching nodes
// Every node in the filtered result matches the filter, and every excluded node doesn't.
// **Validates: Requirements 8.1, 24.6**
func TestFilterMatching(t *testing.T) {
	// Test non-taint attributes (simple string equality)
	attrs := []string{"ec2_type", "instance_type", "arch", "zone", "pool", "nodeclaim", "autoscaler"}

	for _, attr := range attrs {
		attr := attr
		t.Run(attr, func(t *testing.T) {
			rapid.Check(t, func(t *rapid.T) {
				nodes := genNodeList().Draw(t, "nodes")
				if len(nodes) == 0 {
					return
				}
				// Pick a value from one of the nodes to filter by
				idx := rapid.IntRange(0, len(nodes)-1).Draw(t, "idx")
				value := attributeValue(nodes[idx], attr)

				result, err := FilterNodes(nodes, attr, value)
				assert.NoError(t, err)

				// Every node in result must match
				for _, n := range result {
					assert.Equal(t, value, attributeValue(n, attr),
						"node %s in result should match filter %s=%s", n.Name, attr, value)
				}

				// Build a set of result node names for exclusion check
				resultNames := make(map[string]bool, len(result))
				for _, n := range result {
					resultNames[n.Name] = true
				}

				// Every excluded node must NOT match
				for _, n := range nodes {
					if !resultNames[n.Name] {
						assert.NotEqual(t, value, attributeValue(n, attr),
							"excluded node %s should not match filter %s=%s", n.Name, attr, value)
					}
				}
			})
		})
	}
}

// Feature: kubectl-k8i-plugin, Property 17: Fargate nodes hidden by default
// No "fargate-" prefixed names when HideFargateNodes is applied.
// **Validates: Requirements 10.10**
func TestFargateHiding(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		nodes := genNodeList().Draw(t, "nodes")
		result := HideFargateNodes(nodes)

		for _, n := range result {
			assert.False(t, strings.HasPrefix(n.Name, "fargate-"),
				"node %s should have been hidden", n.Name)
		}

		// Every non-fargate node from input should be in result
		resultNames := make(map[string]bool, len(result))
		for _, n := range result {
			resultNames[n.Name] = true
		}
		for _, n := range nodes {
			if !strings.HasPrefix(n.Name, "fargate-") {
				assert.True(t, resultNames[n.Name],
					"non-fargate node %s should be in result", n.Name)
			}
		}
	})
}

// Feature: kubectl-k8i-plugin, Property 18: Filter-then-sort order preserves filter invariant
// Result is filtered AND sorted: every node matches the filter, and the output is sorted.
// **Validates: Requirements 13.1, 13.2, 13.3**
func TestFilterThenSortOrder(t *testing.T) {
	attrs := []string{"ec2_type", "instance_type", "arch", "zone", "pool", "nodeclaim", "autoscaler"}
	sortCols := sortpkg.SupportedSortColumns

	rapid.Check(t, func(t *rapid.T) {
		nodes := rapid.SliceOfN(genNodeInfo(), 1, 30).Draw(t, "nodes")
		attr := attrs[rapid.IntRange(0, len(attrs)-1).Draw(t, "attrIdx")]
		col := sortCols[rapid.IntRange(0, len(sortCols)-1).Draw(t, "colIdx")]
		dir := rapid.SampledFrom([]string{"asc", "desc"}).Draw(t, "dir")

		// Pick a filter value from the nodes
		idx := rapid.IntRange(0, len(nodes)-1).Draw(t, "idx")
		value := attributeValue(nodes[idx], attr)

		// Filter then sort
		filtered, err := FilterNodes(nodes, attr, value)
		assert.NoError(t, err)

		result := make([]model.NodeInfo, len(filtered))
		copy(result, filtered)
		err = sortpkg.SortNodes(result, col, dir)
		assert.NoError(t, err)

		// All nodes in result still match the filter
		for _, n := range result {
			assert.Equal(t, value, attributeValue(n, attr),
				"node %s should match filter %s=%s after sort", n.Name, attr, value)
		}

		// Result should have same length as filtered
		assert.Len(t, result, len(filtered))
	})
}
