package sort

import (
	"testing"
	"time"

	"github.com/kubectl-k8i/pkg/model"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

// --- generators ---

func genNodeInfo() *rapid.Generator[model.NodeInfo] {
	return rapid.Custom[model.NodeInfo](func(t *rapid.T) model.NodeInfo {
		return model.NodeInfo{
			Name:             rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "name"),
			PodsUsed:         rapid.IntRange(0, 110).Draw(t, "pods"),
			CPURequestCores:  rapid.Float64Range(0, 64).Draw(t, "cpuReq"),
			CPULimitCores:    rapid.Float64Range(0, 64).Draw(t, "cpuLim"),
			CPUUsageCores:    rapid.Float64Range(0, 64).Draw(t, "cpuUse"),
			CPUCapacityCores: rapid.Float64Range(0, 64).Draw(t, "cpuCap"),
			CPULoadPercent:   rapid.IntRange(0, 100).Draw(t, "cpuLoad"),
			MemRequestGB:     rapid.Float64Range(0, 256).Draw(t, "memReq"),
			MemLimitGB:       rapid.Float64Range(0, 256).Draw(t, "memLim"),
			MemUsageGB:       rapid.Float64Range(0, 256).Draw(t, "memUse"),
			MemCapacityGB:    rapid.Float64Range(0, 256).Draw(t, "memCap"),
			MemLoadPercent:   rapid.IntRange(0, 100).Draw(t, "memLoad"),
			CapacityType:     rapid.SampledFrom([]string{"spot", "od", "x"}).Draw(t, "ct"),
			InstanceType:     rapid.SampledFrom([]string{"m5.xlarge", "c5.2xlarge", "r5.large", "t3.medium"}).Draw(t, "it"),
			Architecture:     rapid.SampledFrom([]string{"amd64", "arm64"}).Draw(t, "arch"),
			Zone:             rapid.SampledFrom([]string{"1a", "1b", "1c", "2a"}).Draw(t, "zone"),
			Nodepool:         rapid.SampledFrom([]string{"pool-a", "pool-b", "pool-c"}).Draw(t, "pool"),
			TaintSortKey:     rapid.SampledFrom([]string{"", "dedicated", "team", "dedicated,team"}).Draw(t, "tsk"),
			Autoscaler:       rapid.SampledFrom([]string{"karpenter", "cluster-autoscaler", "spotio", "x"}).Draw(t, "as"),
			CreationTime:     time.Unix(rapid.Int64Range(1600000000, 1750000000).Draw(t, "ctime"), 0),
		}
	})
}

func genNodeList() *rapid.Generator[[]model.NodeInfo] {
	return rapid.SliceOfN(genNodeInfo(), 0, 30)
}

// copyNodes makes a deep copy of a node slice.
func copyNodes(nodes []model.NodeInfo) []model.NodeInfo {
	c := make([]model.NodeInfo, len(nodes))
	copy(c, nodes)
	return c
}

// reverse reverses a slice in place.
func reverse(nodes []model.NodeInfo) {
	for i, j := 0, len(nodes)-1; i < j; i, j = i+1, j-1 {
		nodes[i], nodes[j] = nodes[j], nodes[i]
	}
}

// Feature: kubectl-k8i-plugin, Property 14: Sort asc is reverse of sort desc
// Ascending then reversing = descending.
// **Validates: Requirements 9.1, 9.3**
func TestSortAscDescSymmetry(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		nodes := genNodeList().Draw(t, "nodes")
		col := rapid.SampledFrom(SupportedSortColumns).Draw(t, "col")

		asc := copyNodes(nodes)
		desc := copyNodes(nodes)

		err := SortNodes(asc, col, "asc")
		assert.NoError(t, err)

		err = SortNodes(desc, col, "desc")
		assert.NoError(t, err)

		// Reverse the ascending result
		reverse(asc)

		// Compare names (since nodes may have duplicate sort keys, we compare
		// the full sequence of names after reversal)
		ascNames := make([]string, len(asc))
		descNames := make([]string, len(desc))
		for i := range asc {
			ascNames[i] = asc[i].Name
			descNames[i] = desc[i].Name
		}
		assert.Equal(t, descNames, ascNames,
			"ascending reversed should equal descending for column %s", col)
	})
}

// Feature: kubectl-k8i-plugin, Property 15: Numeric sort uses numeric comparison
// Each element ≤ next for numeric columns when sorted ascending.
// **Validates: Requirements 9.7**
func TestNumericSort(t *testing.T) {
	numCols := NumericColumns

	rapid.Check(t, func(t *rapid.T) {
		nodes := rapid.SliceOfN(genNodeInfo(), 2, 30).Draw(t, "nodes")
		col := rapid.SampledFrom(numCols).Draw(t, "col")

		sorted := copyNodes(nodes)
		err := SortNodes(sorted, col, "asc")
		assert.NoError(t, err)

		for i := 0; i < len(sorted)-1; i++ {
			vi := numericValue(sorted[i], col)
			vj := numericValue(sorted[i+1], col)
			assert.LessOrEqual(t, vi, vj,
				"numeric column %s: element %d (%.4f) should be ≤ element %d (%.4f)",
				col, i, vi, i+1, vj)
		}
	})
}

// Feature: kubectl-k8i-plugin, Property 16: Lexicographic sort uses string comparison
// Each element ≤ next for text columns when sorted ascending.
// **Validates: Requirements 9.8**
func TestLexicographicSort(t *testing.T) {
	textCols := []string{"name", "ec2_type", "instance_type", "arch", "zone", "pool", "taint", "autoscaler"}

	rapid.Check(t, func(t *rapid.T) {
		nodes := rapid.SliceOfN(genNodeInfo(), 2, 30).Draw(t, "nodes")
		col := rapid.SampledFrom(textCols).Draw(t, "col")

		sorted := copyNodes(nodes)
		err := SortNodes(sorted, col, "asc")
		assert.NoError(t, err)

		for i := 0; i < len(sorted)-1; i++ {
			si := textValue(sorted[i], col)
			sj := textValue(sorted[i+1], col)
			assert.LessOrEqual(t, si, sj,
				"text column %s: element %d (%q) should be ≤ element %d (%q)",
				col, i, si, i+1, sj)
		}
	})
}
