package taints

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kubectl-k8i/pkg/model"
	corev1 "k8s.io/api/core/v1"
)

// FormatTaints converts a slice of Kubernetes taints to a display string.
// Format: "key=value:effect,key2:effect2" or "none" for empty slice.
// When a taint has an empty value, the format is "key:effect".
func FormatTaints(taints []corev1.Taint) string {
	if len(taints) == 0 {
		return "none"
	}

	parts := make([]string, len(taints))
	for i, t := range taints {
		if t.Value == "" {
			parts[i] = fmt.Sprintf("%s:%s", t.Key, t.Effect)
		} else {
			parts[i] = fmt.Sprintf("%s=%s:%s", t.Key, t.Value, t.Effect)
		}
	}
	return strings.Join(parts, ",")
}

// TaintSetKey returns a canonical string key for a set of taints.
// Taints are sorted by key, then formatted as "key=value:effect|key2:effect2".
// Used for grouping nodes by identical taint sets.
func TaintSetKey(taints []corev1.Taint) string {
	if len(taints) == 0 {
		return ""
	}

	// Make a copy to avoid mutating the input slice
	sorted := make([]corev1.Taint, len(taints))
	copy(sorted, taints)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Key < sorted[j].Key
	})

	parts := make([]string, len(sorted))
	for i, t := range sorted {
		if t.Value == "" {
			parts[i] = fmt.Sprintf("%s:%s", t.Key, t.Effect)
		} else {
			parts[i] = fmt.Sprintf("%s=%s:%s", t.Key, t.Value, t.Effect)
		}
	}
	return strings.Join(parts, "|")
}

// MatchTaintFilter checks if a node's taints match a filter value.
// Supports "KEY" format (matches any taint with that key) and
// "KEY=VALUE" format (matches taint with that key AND value).
func MatchTaintFilter(taints []corev1.Taint, filterValue string) bool {
	if len(taints) == 0 {
		return false
	}

	// Check if filter is KEY=VALUE or just KEY
	if idx := strings.Index(filterValue, "="); idx >= 0 {
		// KEY=VALUE format
		key := filterValue[:idx]
		value := filterValue[idx+1:]
		for _, t := range taints {
			if t.Key == key && t.Value == value {
				return true
			}
		}
		return false
	}

	// KEY-only format: match any taint with that key
	for _, t := range taints {
		if t.Key == filterValue {
			return true
		}
	}
	return false
}

// SortKeyFromTaints returns a string for lexicographic sorting.
// Concatenates taint keys in alphabetical order.
func SortKeyFromTaints(taints []corev1.Taint) string {
	if len(taints) == 0 {
		return ""
	}

	keys := make([]string, len(taints))
	for i, t := range taints {
		keys[i] = t.Key
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

// GroupByTaints groups nodes by their common taint sets.
// Returns groups in stable order (first-seen group order).
// Nodes within each group maintain their original order.
func GroupByTaints(nodes []model.NodeInfo) [][]model.NodeInfo {
	if len(nodes) == 0 {
		return nil
	}

	// Track group order by first occurrence
	var groupOrder []string
	groupMap := make(map[string][]model.NodeInfo)

	for _, node := range nodes {
		key := TaintSetKey(node.Taints)
		if _, exists := groupMap[key]; !exists {
			groupOrder = append(groupOrder, key)
		}
		groupMap[key] = append(groupMap[key], node)
	}

	result := make([][]model.NodeInfo, len(groupOrder))
	for i, key := range groupOrder {
		result[i] = groupMap[key]
	}
	return result
}
