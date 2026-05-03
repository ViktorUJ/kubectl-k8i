package taints

import (
	"testing"

	"github.com/kubectl-k8i/pkg/model"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestFormatTaints_SingleTaint(t *testing.T) {
	taints := []corev1.Taint{
		{Key: "node-role.kubernetes.io/master", Value: "", Effect: corev1.TaintEffectNoSchedule},
	}
	result := FormatTaints(taints)
	assert.Equal(t, "node-role.kubernetes.io/master:NoSchedule", result)
}

func TestFormatTaints_SingleTaintWithValue(t *testing.T) {
	taints := []corev1.Taint{
		{Key: "dedicated", Value: "gpu", Effect: corev1.TaintEffectNoSchedule},
	}
	result := FormatTaints(taints)
	assert.Equal(t, "dedicated=gpu:NoSchedule", result)
}

func TestFormatTaints_MultipleTaints(t *testing.T) {
	taints := []corev1.Taint{
		{Key: "dedicated", Value: "gpu", Effect: corev1.TaintEffectNoSchedule},
		{Key: "special", Value: "true", Effect: corev1.TaintEffectNoExecute},
	}
	result := FormatTaints(taints)
	assert.Equal(t, "dedicated=gpu:NoSchedule,special=true:NoExecute", result)
}

func TestFormatTaints_NoTaints(t *testing.T) {
	result := FormatTaints(nil)
	assert.Equal(t, "none", result)

	result = FormatTaints([]corev1.Taint{})
	assert.Equal(t, "none", result)
}

func TestFormatTaints_EmptyValue(t *testing.T) {
	taints := []corev1.Taint{
		{Key: "node.kubernetes.io/not-ready", Value: "", Effect: corev1.TaintEffectNoExecute},
	}
	result := FormatTaints(taints)
	assert.Equal(t, "node.kubernetes.io/not-ready:NoExecute", result)
}

func TestMatchTaintFilter_ByKey(t *testing.T) {
	taints := []corev1.Taint{
		{Key: "dedicated", Value: "gpu", Effect: corev1.TaintEffectNoSchedule},
		{Key: "special", Value: "true", Effect: corev1.TaintEffectNoExecute},
	}

	assert.True(t, MatchTaintFilter(taints, "dedicated"))
	assert.True(t, MatchTaintFilter(taints, "special"))
	assert.False(t, MatchTaintFilter(taints, "nonexistent"))
}

func TestMatchTaintFilter_ByKeyValue(t *testing.T) {
	taints := []corev1.Taint{
		{Key: "dedicated", Value: "gpu", Effect: corev1.TaintEffectNoSchedule},
		{Key: "special", Value: "true", Effect: corev1.TaintEffectNoExecute},
	}

	assert.True(t, MatchTaintFilter(taints, "dedicated=gpu"))
	assert.True(t, MatchTaintFilter(taints, "special=true"))
	assert.False(t, MatchTaintFilter(taints, "dedicated=cpu"))
	assert.False(t, MatchTaintFilter(taints, "special=false"))
}

func TestMatchTaintFilter_EmptyTaints(t *testing.T) {
	assert.False(t, MatchTaintFilter(nil, "dedicated"))
	assert.False(t, MatchTaintFilter([]corev1.Taint{}, "dedicated"))
}

func TestSortKeyFromTaints(t *testing.T) {
	taints := []corev1.Taint{
		{Key: "zebra", Value: "1", Effect: corev1.TaintEffectNoSchedule},
		{Key: "alpha", Value: "2", Effect: corev1.TaintEffectNoExecute},
		{Key: "middle", Value: "3", Effect: corev1.TaintEffectPreferNoSchedule},
	}
	result := SortKeyFromTaints(taints)
	assert.Equal(t, "alpha,middle,zebra", result)
}

func TestSortKeyFromTaints_Empty(t *testing.T) {
	result := SortKeyFromTaints(nil)
	assert.Equal(t, "", result)

	result = SortKeyFromTaints([]corev1.Taint{})
	assert.Equal(t, "", result)
}

func TestTaintSetKey(t *testing.T) {
	taints := []corev1.Taint{
		{Key: "dedicated", Value: "gpu", Effect: corev1.TaintEffectNoSchedule},
		{Key: "alpha", Value: "", Effect: corev1.TaintEffectNoExecute},
	}
	result := TaintSetKey(taints)
	// Should be sorted by key: alpha first, then dedicated
	assert.Equal(t, "alpha:NoExecute|dedicated=gpu:NoSchedule", result)
}

func TestTaintSetKey_Empty(t *testing.T) {
	result := TaintSetKey(nil)
	assert.Equal(t, "", result)
}

func TestGroupByTaints_Correctness(t *testing.T) {
	taintSetA := []corev1.Taint{
		{Key: "dedicated", Value: "gpu", Effect: corev1.TaintEffectNoSchedule},
	}
	taintSetB := []corev1.Taint{
		{Key: "special", Value: "true", Effect: corev1.TaintEffectNoExecute},
	}

	nodes := []model.NodeInfo{
		{Name: "node1", Taints: taintSetA},
		{Name: "node2", Taints: taintSetB},
		{Name: "node3", Taints: taintSetA},
		{Name: "node4", Taints: taintSetB},
	}

	groups := GroupByTaints(nodes)
	assert.Len(t, groups, 2)

	// First group should be taintSetA (first-seen order)
	assert.Len(t, groups[0], 2)
	assert.Equal(t, "node1", groups[0][0].Name)
	assert.Equal(t, "node3", groups[0][1].Name)

	// Second group should be taintSetB
	assert.Len(t, groups[1], 2)
	assert.Equal(t, "node2", groups[1][0].Name)
	assert.Equal(t, "node4", groups[1][1].Name)
}

func TestGroupByTaints_Empty(t *testing.T) {
	groups := GroupByTaints(nil)
	assert.Nil(t, groups)

	groups = GroupByTaints([]model.NodeInfo{})
	assert.Nil(t, groups)
}

func TestGroupByTaints_NoTaints(t *testing.T) {
	nodes := []model.NodeInfo{
		{Name: "node1", Taints: nil},
		{Name: "node2", Taints: nil},
	}

	groups := GroupByTaints(nodes)
	assert.Len(t, groups, 1)
	assert.Len(t, groups[0], 2)
}
