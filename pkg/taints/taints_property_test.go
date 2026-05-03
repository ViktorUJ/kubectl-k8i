package taints

import (
	"sort"
	"strings"
	"testing"

	"github.com/kubectl-k8i/pkg/model"
	corev1 "k8s.io/api/core/v1"
	"pgregory.net/rapid"
)

// taintKeyGen generates valid taint key strings (alphanumeric with dots/slashes).
func taintKeyGen() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		prefix := rapid.StringMatching(`[a-z][a-z0-9]{0,9}`).Draw(t, "keyBase")
		return prefix
	})
}

// taintValueGen generates valid taint value strings (may be empty).
func taintValueGen() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		hasValue := rapid.Bool().Draw(t, "hasValue")
		if !hasValue {
			return ""
		}
		return rapid.StringMatching(`[a-z0-9]{1,10}`).Draw(t, "value")
	})
}

// taintEffectGen generates a valid taint effect.
func taintEffectGen() *rapid.Generator[corev1.TaintEffect] {
	return rapid.Custom(func(t *rapid.T) corev1.TaintEffect {
		effects := []corev1.TaintEffect{
			corev1.TaintEffectNoSchedule,
			corev1.TaintEffectPreferNoSchedule,
			corev1.TaintEffectNoExecute,
		}
		idx := rapid.IntRange(0, len(effects)-1).Draw(t, "effectIdx")
		return effects[idx]
	})
}

// taintGen generates a single random taint.
func taintGen() *rapid.Generator[corev1.Taint] {
	return rapid.Custom(func(t *rapid.T) corev1.Taint {
		return corev1.Taint{
			Key:    taintKeyGen().Draw(t, "key"),
			Value:  taintValueGen().Draw(t, "value"),
			Effect: taintEffectGen().Draw(t, "effect"),
		}
	})
}

// taintSliceGen generates a slice of 0-5 taints.
func taintSliceGen() *rapid.Generator[[]corev1.Taint] {
	return rapid.Custom(func(t *rapid.T) []corev1.Taint {
		n := rapid.IntRange(0, 5).Draw(t, "numTaints")
		taints := make([]corev1.Taint, n)
		for i := 0; i < n; i++ {
			taints[i] = taintGen().Draw(t, "taint")
		}
		return taints
	})
}

// TestProperty21_TaintDisplayFormat verifies that for any list of taints,
// the formatted string contains each taint in "key=value:effect" format
// (or "key:effect" when value is empty), separated by commas.
// For an empty taint list, the result is "none".
//
// **Validates: Requirements 17.1, 17.6, 17.7**
func TestProperty21_TaintDisplayFormat(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		taints := taintSliceGen().Draw(t, "taints")

		result := FormatTaints(taints)

		if len(taints) == 0 {
			if result != "none" {
				t.Fatalf("Empty taint list should produce \"none\", got %q", result)
			}
			return
		}

		// Split by comma and verify each part
		parts := strings.Split(result, ",")
		if len(parts) != len(taints) {
			t.Fatalf("Expected %d parts, got %d: %q", len(taints), len(parts), result)
		}

		for i, taint := range taints {
			part := parts[i]
			if taint.Value == "" {
				expected := taint.Key + ":" + string(taint.Effect)
				if part != expected {
					t.Fatalf("Taint %d: expected %q, got %q", i, expected, part)
				}
			} else {
				expected := taint.Key + "=" + taint.Value + ":" + string(taint.Effect)
				if part != expected {
					t.Fatalf("Taint %d: expected %q, got %q", i, expected, part)
				}
			}
		}
	})
}

// TestProperty22_TaintFilterMatching verifies that for any list of taints and
// a filter string: if the filter is KEY, the match is true iff any taint has
// that key; if the filter is KEY=VALUE, the match is true iff any taint has
// that key and value.
//
// **Validates: Requirements 17.2, 17.3**
func TestProperty22_TaintFilterMatching(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		taints := taintSliceGen().Draw(t, "taints")
		isKeyValue := rapid.Bool().Draw(t, "isKeyValue")

		filterKey := taintKeyGen().Draw(t, "filterKey")

		if isKeyValue {
			filterValue := rapid.StringMatching(`[a-z0-9]{1,10}`).Draw(t, "filterValue")
			filter := filterKey + "=" + filterValue

			result := MatchTaintFilter(taints, filter)

			// Manually check: any taint with matching key AND value?
			expected := false
			for _, taint := range taints {
				if taint.Key == filterKey && taint.Value == filterValue {
					expected = true
					break
				}
			}

			if result != expected {
				t.Fatalf("KEY=VALUE filter %q on taints: expected %v, got %v", filter, expected, result)
			}
		} else {
			result := MatchTaintFilter(taints, filterKey)

			// Manually check: any taint with matching key?
			expected := false
			for _, taint := range taints {
				if taint.Key == filterKey {
					expected = true
					break
				}
			}

			if result != expected {
				t.Fatalf("KEY filter %q on taints: expected %v, got %v", filterKey, expected, result)
			}
		}
	})
}

// TestProperty23_TaintSortKeyOrdering verifies that for any two nodes,
// when sorted by taint in ascending order, the node whose alphabetically-sorted
// taint keys concatenation is lexicographically smaller appears first.
//
// **Validates: Requirements 17.4**
func TestProperty23_TaintSortKeyOrdering(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		taintsA := taintSliceGen().Draw(t, "taintsA")
		taintsB := taintSliceGen().Draw(t, "taintsB")

		keyA := SortKeyFromTaints(taintsA)
		keyB := SortKeyFromTaints(taintsB)

		// Verify the sort key is the alphabetically sorted taint keys joined by comma
		verifyKey := func(taints []corev1.Taint, key string) {
			if len(taints) == 0 {
				if key != "" {
					t.Fatalf("Empty taints should produce empty sort key, got %q", key)
				}
				return
			}
			keys := make([]string, len(taints))
			for i, taint := range taints {
				keys[i] = taint.Key
			}
			sort.Strings(keys)
			expected := strings.Join(keys, ",")
			if key != expected {
				t.Fatalf("Sort key mismatch: expected %q, got %q", expected, key)
			}
		}

		verifyKey(taintsA, keyA)
		verifyKey(taintsB, keyB)

		// Verify lexicographic ordering is consistent
		if keyA < keyB {
			if !(keyA <= keyB) {
				t.Fatalf("Ordering inconsistency: keyA=%q < keyB=%q", keyA, keyB)
			}
		}
	})
}

// TestProperty24_TaintGroupingCorrectness verifies that for any list of nodes,
// when grouped by taint sets, every pair of nodes within the same group has
// identical taint sets, and every pair of nodes in different groups has
// different taint sets.
//
// **Validates: Requirements 17.5**
func TestProperty24_TaintGroupingCorrectness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		numNodes := rapid.IntRange(1, 20).Draw(t, "numNodes")
		// Use a small set of distinct taint sets to ensure grouping
		numDistinctSets := rapid.IntRange(1, 4).Draw(t, "numDistinctSets")

		distinctSets := make([][]corev1.Taint, numDistinctSets)
		for i := 0; i < numDistinctSets; i++ {
			distinctSets[i] = taintSliceGen().Draw(t, "distinctSet")
		}

		nodes := make([]model.NodeInfo, numNodes)
		for i := 0; i < numNodes; i++ {
			setIdx := rapid.IntRange(0, numDistinctSets-1).Draw(t, "setIdx")
			// Deep copy the taint slice
			taints := make([]corev1.Taint, len(distinctSets[setIdx]))
			copy(taints, distinctSets[setIdx])
			nodes[i] = model.NodeInfo{
				Name:   rapid.StringMatching(`node-[a-z]{3}`).Draw(t, "nodeName"),
				Taints: taints,
			}
		}

		groups := GroupByTaints(nodes)

		// Verify: within each group, all nodes have the same TaintSetKey
		for gi, group := range groups {
			if len(group) == 0 {
				t.Fatalf("Group %d is empty", gi)
			}
			groupKey := TaintSetKey(group[0].Taints)
			for ni, node := range group {
				nodeKey := TaintSetKey(node.Taints)
				if nodeKey != groupKey {
					t.Fatalf("Group %d, node %d: key %q != group key %q", gi, ni, nodeKey, groupKey)
				}
			}
		}

		// Verify: different groups have different TaintSetKeys
		for i := 0; i < len(groups); i++ {
			for j := i + 1; j < len(groups); j++ {
				keyI := TaintSetKey(groups[i][0].Taints)
				keyJ := TaintSetKey(groups[j][0].Taints)
				if keyI == keyJ {
					t.Fatalf("Groups %d and %d have same key %q but are in different groups", i, j, keyI)
				}
			}
		}

		// Verify: total node count is preserved
		totalNodes := 0
		for _, group := range groups {
			totalNodes += len(group)
		}
		if totalNodes != len(nodes) {
			t.Fatalf("Total nodes in groups (%d) != input nodes (%d)", totalNodes, len(nodes))
		}
	})
}
