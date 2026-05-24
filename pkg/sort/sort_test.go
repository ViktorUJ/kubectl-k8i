package sort

import (
	"testing"
	"time"

	"github.com/kubectl-k8i/pkg/model"
	"github.com/stretchr/testify/assert"
)

// helper to build a NodeInfo with common fields set.
func mkNode(name string, opts ...func(*model.NodeInfo)) model.NodeInfo {
	n := model.NodeInfo{Name: name}
	for _, o := range opts {
		o(&n)
	}
	return n
}

func names(nodes []model.NodeInfo) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.Name
	}
	return out
}

// --- sort by name (text, lexicographic) ---

func TestSortByName_Asc(t *testing.T) {
	nodes := []model.NodeInfo{mkNode("charlie"), mkNode("alpha"), mkNode("bravo")}
	err := SortNodes(nodes, "name", "asc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"alpha", "bravo", "charlie"}, names(nodes))
}

func TestSortByName_Desc(t *testing.T) {
	nodes := []model.NodeInfo{mkNode("charlie"), mkNode("alpha"), mkNode("bravo")}
	err := SortNodes(nodes, "name", "desc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"charlie", "bravo", "alpha"}, names(nodes))
}

// --- sort by pods (numeric) ---

func TestSortByPods_Asc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.PodsUsed = 30 }),
		mkNode("b", func(n *model.NodeInfo) { n.PodsUsed = 10 }),
		mkNode("c", func(n *model.NodeInfo) { n.PodsUsed = 20 }),
	}
	err := SortNodes(nodes, "pods", "asc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"b", "c", "a"}, names(nodes))
}

func TestSortByPods_Desc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.PodsUsed = 30 }),
		mkNode("b", func(n *model.NodeInfo) { n.PodsUsed = 10 }),
		mkNode("c", func(n *model.NodeInfo) { n.PodsUsed = 20 }),
	}
	err := SortNodes(nodes, "pods", "desc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"a", "c", "b"}, names(nodes))
}

// --- sort by cpu_req (numeric) ---

func TestSortByCPUReq_Asc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.CPURequestCores = 4.0 }),
		mkNode("b", func(n *model.NodeInfo) { n.CPURequestCores = 1.0 }),
		mkNode("c", func(n *model.NodeInfo) { n.CPURequestCores = 2.5 }),
	}
	err := SortNodes(nodes, "cpu_req", "asc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"b", "c", "a"}, names(nodes))
}

func TestSortByCPUReq_Desc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.CPURequestCores = 4.0 }),
		mkNode("b", func(n *model.NodeInfo) { n.CPURequestCores = 1.0 }),
	}
	err := SortNodes(nodes, "cpu_req", "desc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, names(nodes))
}

// --- sort by cpu_lim (numeric) ---

func TestSortByCPULim_Asc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.CPULimitCores = 8.0 }),
		mkNode("b", func(n *model.NodeInfo) { n.CPULimitCores = 2.0 }),
	}
	err := SortNodes(nodes, "cpu_lim", "asc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"b", "a"}, names(nodes))
}

func TestSortByCPULim_Desc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.CPULimitCores = 8.0 }),
		mkNode("b", func(n *model.NodeInfo) { n.CPULimitCores = 2.0 }),
	}
	err := SortNodes(nodes, "cpu_lim", "desc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, names(nodes))
}

// --- sort by cpu_use (numeric) ---

func TestSortByCPUUse_Asc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.CPUUsageCores = 3.5 }),
		mkNode("b", func(n *model.NodeInfo) { n.CPUUsageCores = 0.5 }),
	}
	err := SortNodes(nodes, "cpu_use", "asc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"b", "a"}, names(nodes))
}

func TestSortByCPUUse_Desc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.CPUUsageCores = 3.5 }),
		mkNode("b", func(n *model.NodeInfo) { n.CPUUsageCores = 0.5 }),
	}
	err := SortNodes(nodes, "cpu_use", "desc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, names(nodes))
}

// --- sort by cpu_cap (numeric) ---

func TestSortByCPUCap_Asc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.CPUCapacityCores = 16.0 }),
		mkNode("b", func(n *model.NodeInfo) { n.CPUCapacityCores = 4.0 }),
	}
	err := SortNodes(nodes, "cpu_cap", "asc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"b", "a"}, names(nodes))
}

func TestSortByCPUCap_Desc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.CPUCapacityCores = 16.0 }),
		mkNode("b", func(n *model.NodeInfo) { n.CPUCapacityCores = 4.0 }),
	}
	err := SortNodes(nodes, "cpu_cap", "desc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, names(nodes))
}

// --- sort by cpu_load (numeric) ---

func TestSortByCPULoad_Asc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.CPULoadPercent = 90 }),
		mkNode("b", func(n *model.NodeInfo) { n.CPULoadPercent = 30 }),
		mkNode("c", func(n *model.NodeInfo) { n.CPULoadPercent = 60 }),
	}
	err := SortNodes(nodes, "cpu_load", "asc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"b", "c", "a"}, names(nodes))
}

func TestSortByCPULoad_Desc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.CPULoadPercent = 90 }),
		mkNode("b", func(n *model.NodeInfo) { n.CPULoadPercent = 30 }),
	}
	err := SortNodes(nodes, "cpu_load", "desc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, names(nodes))
}

// --- sort by mem_req (numeric) ---

func TestSortByMemReq_Asc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.MemRequestGB = 32.0 }),
		mkNode("b", func(n *model.NodeInfo) { n.MemRequestGB = 8.0 }),
	}
	err := SortNodes(nodes, "mem_req", "asc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"b", "a"}, names(nodes))
}

func TestSortByMemReq_Desc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.MemRequestGB = 32.0 }),
		mkNode("b", func(n *model.NodeInfo) { n.MemRequestGB = 8.0 }),
	}
	err := SortNodes(nodes, "mem_req", "desc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, names(nodes))
}

// --- sort by mem_lim (numeric) ---

func TestSortByMemLim_Asc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.MemLimitGB = 64.0 }),
		mkNode("b", func(n *model.NodeInfo) { n.MemLimitGB = 16.0 }),
	}
	err := SortNodes(nodes, "mem_lim", "asc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"b", "a"}, names(nodes))
}

func TestSortByMemLim_Desc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.MemLimitGB = 64.0 }),
		mkNode("b", func(n *model.NodeInfo) { n.MemLimitGB = 16.0 }),
	}
	err := SortNodes(nodes, "mem_lim", "desc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, names(nodes))
}

// --- sort by mem_use (numeric) ---

func TestSortByMemUse_Asc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.MemUsageGB = 50.0 }),
		mkNode("b", func(n *model.NodeInfo) { n.MemUsageGB = 10.0 }),
	}
	err := SortNodes(nodes, "mem_use", "asc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"b", "a"}, names(nodes))
}

func TestSortByMemUse_Desc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.MemUsageGB = 50.0 }),
		mkNode("b", func(n *model.NodeInfo) { n.MemUsageGB = 10.0 }),
	}
	err := SortNodes(nodes, "mem_use", "desc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, names(nodes))
}

// --- sort by mem_cap (numeric) ---

func TestSortByMemCap_Asc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.MemCapacityGB = 128.0 }),
		mkNode("b", func(n *model.NodeInfo) { n.MemCapacityGB = 32.0 }),
	}
	err := SortNodes(nodes, "mem_cap", "asc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"b", "a"}, names(nodes))
}

func TestSortByMemCap_Desc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.MemCapacityGB = 128.0 }),
		mkNode("b", func(n *model.NodeInfo) { n.MemCapacityGB = 32.0 }),
	}
	err := SortNodes(nodes, "mem_cap", "desc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, names(nodes))
}

// --- sort by mem_load (numeric) ---

func TestSortByMemLoad_Asc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.MemLoadPercent = 85 }),
		mkNode("b", func(n *model.NodeInfo) { n.MemLoadPercent = 20 }),
	}
	err := SortNodes(nodes, "mem_load", "asc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"b", "a"}, names(nodes))
}

func TestSortByMemLoad_Desc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.MemLoadPercent = 85 }),
		mkNode("b", func(n *model.NodeInfo) { n.MemLoadPercent = 20 }),
	}
	err := SortNodes(nodes, "mem_load", "desc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, names(nodes))
}

// --- sort by ec2_type (text) ---

func TestSortByEC2Type_Asc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.CapacityType = "spot" }),
		mkNode("b", func(n *model.NodeInfo) { n.CapacityType = "od" }),
	}
	err := SortNodes(nodes, "ec2_type", "asc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"b", "a"}, names(nodes))
}

func TestSortByEC2Type_Desc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.CapacityType = "spot" }),
		mkNode("b", func(n *model.NodeInfo) { n.CapacityType = "od" }),
	}
	err := SortNodes(nodes, "ec2_type", "desc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, names(nodes))
}

// --- sort by instance_type (text) ---

func TestSortByInstanceType_Asc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.InstanceType = "r5.large" }),
		mkNode("b", func(n *model.NodeInfo) { n.InstanceType = "c5.xlarge" }),
	}
	err := SortNodes(nodes, "instance_type", "asc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"b", "a"}, names(nodes))
}

func TestSortByInstanceType_Desc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.InstanceType = "r5.large" }),
		mkNode("b", func(n *model.NodeInfo) { n.InstanceType = "c5.xlarge" }),
	}
	err := SortNodes(nodes, "instance_type", "desc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, names(nodes))
}

// --- sort by arch (text) ---

func TestSortByArch_Asc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.Architecture = "arm64" }),
		mkNode("b", func(n *model.NodeInfo) { n.Architecture = "amd64" }),
	}
	err := SortNodes(nodes, "arch", "asc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"b", "a"}, names(nodes))
}

func TestSortByArch_Desc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.Architecture = "arm64" }),
		mkNode("b", func(n *model.NodeInfo) { n.Architecture = "amd64" }),
	}
	err := SortNodes(nodes, "arch", "desc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, names(nodes))
}

// --- sort by zone (text) ---

func TestSortByZone_Asc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.Zone = "1c" }),
		mkNode("b", func(n *model.NodeInfo) { n.Zone = "1a" }),
	}
	err := SortNodes(nodes, "zone", "asc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"b", "a"}, names(nodes))
}

func TestSortByZone_Desc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.Zone = "1c" }),
		mkNode("b", func(n *model.NodeInfo) { n.Zone = "1a" }),
	}
	err := SortNodes(nodes, "zone", "desc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, names(nodes))
}

// --- sort by pool (text) ---

func TestSortByPool_Asc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.Nodepool = "pool-b" }),
		mkNode("b", func(n *model.NodeInfo) { n.Nodepool = "pool-a" }),
	}
	err := SortNodes(nodes, "pool", "asc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"b", "a"}, names(nodes))
}

func TestSortByPool_Desc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.Nodepool = "pool-b" }),
		mkNode("b", func(n *model.NodeInfo) { n.Nodepool = "pool-a" }),
	}
	err := SortNodes(nodes, "pool", "desc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, names(nodes))
}

// --- sort by age (numeric — uses CreationTime) ---

func TestSortByAge_Asc(t *testing.T) {
	now := time.Now()
	nodes := []model.NodeInfo{
		mkNode("new", func(n *model.NodeInfo) { n.CreationTime = now.Add(-1 * time.Hour) }),
		mkNode("old", func(n *model.NodeInfo) { n.CreationTime = now.Add(-48 * time.Hour) }),
	}
	err := SortNodes(nodes, "age", "asc")
	assert.NoError(t, err)
	// Ascending by CreationTime Unix → oldest first (smallest timestamp)
	assert.Equal(t, []string{"old", "new"}, names(nodes))
}

func TestSortByAge_Desc(t *testing.T) {
	now := time.Now()
	nodes := []model.NodeInfo{
		mkNode("new", func(n *model.NodeInfo) { n.CreationTime = now.Add(-1 * time.Hour) }),
		mkNode("old", func(n *model.NodeInfo) { n.CreationTime = now.Add(-48 * time.Hour) }),
	}
	err := SortNodes(nodes, "age", "desc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"new", "old"}, names(nodes))
}

// --- sort by taint (text — uses TaintSortKey) ---

func TestSortByTaint_Asc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.TaintSortKey = "team,workload" }),
		mkNode("b", func(n *model.NodeInfo) { n.TaintSortKey = "dedicated" }),
	}
	err := SortNodes(nodes, "taint", "asc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"b", "a"}, names(nodes))
}

func TestSortByTaint_Desc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.TaintSortKey = "team,workload" }),
		mkNode("b", func(n *model.NodeInfo) { n.TaintSortKey = "dedicated" }),
	}
	err := SortNodes(nodes, "taint", "desc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, names(nodes))
}

// --- sort by autoscaler (text) ---

func TestSortByAutoscaler_Asc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.Autoscaler = "spotio" }),
		mkNode("b", func(n *model.NodeInfo) { n.Autoscaler = "cluster-autoscaler" }),
		mkNode("c", func(n *model.NodeInfo) { n.Autoscaler = "karpenter" }),
	}
	err := SortNodes(nodes, "autoscaler", "asc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"b", "c", "a"}, names(nodes))
}

func TestSortByAutoscaler_Desc(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.Autoscaler = "spotio" }),
		mkNode("b", func(n *model.NodeInfo) { n.Autoscaler = "cluster-autoscaler" }),
		mkNode("c", func(n *model.NodeInfo) { n.Autoscaler = "karpenter" }),
	}
	err := SortNodes(nodes, "autoscaler", "desc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"a", "c", "b"}, names(nodes))
}

// --- numeric vs lexicographic: 9 < 10 numerically, "9" > "10" lexicographically ---

func TestNumericVsLexicographic(t *testing.T) {
	// Numeric: pods 9 < 10
	numNodes := []model.NodeInfo{
		mkNode("ten", func(n *model.NodeInfo) { n.PodsUsed = 10 }),
		mkNode("nine", func(n *model.NodeInfo) { n.PodsUsed = 9 }),
	}
	err := SortNodes(numNodes, "pods", "asc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"nine", "ten"}, names(numNodes), "numeric: 9 < 10")

	// Lexicographic: "9" > "10" (string comparison)
	lexNodes := []model.NodeInfo{
		mkNode("ten", func(n *model.NodeInfo) { n.Name = "10" }),
		mkNode("nine", func(n *model.NodeInfo) { n.Name = "9" }),
	}
	err = SortNodes(lexNodes, "name", "asc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"10", "9"}, names(lexNodes), "lexicographic: \"10\" < \"9\"")
}

// --- default sort (pool=asc) ---

func TestDefaultSort(t *testing.T) {
	nodes := []model.NodeInfo{
		mkNode("a", func(n *model.NodeInfo) { n.Nodepool = "pool-c" }),
		mkNode("b", func(n *model.NodeInfo) { n.Nodepool = "pool-a" }),
		mkNode("c", func(n *model.NodeInfo) { n.Nodepool = "pool-b" }),
	}
	// Default sort is pool=asc
	err := SortNodes(nodes, "pool", "asc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"b", "c", "a"}, names(nodes))
}

// --- error cases ---

func TestSortUnsupportedColumn(t *testing.T) {
	nodes := []model.NodeInfo{mkNode("a")}
	err := SortNodes(nodes, "nonexistent", "asc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported sort column")
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestSortInvalidDirection(t *testing.T) {
	nodes := []model.NodeInfo{mkNode("a")}
	err := SortNodes(nodes, "name", "up")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid sort direction")
	assert.Contains(t, err.Error(), "up")
}

// --- empty and single-element ---

func TestSortEmptySlice(t *testing.T) {
	var nodes []model.NodeInfo
	err := SortNodes(nodes, "name", "asc")
	assert.NoError(t, err)
	assert.Empty(t, nodes)
}

func TestSortSingleElement(t *testing.T) {
	nodes := []model.NodeInfo{mkNode("only")}
	err := SortNodes(nodes, "name", "asc")
	assert.NoError(t, err)
	assert.Equal(t, []string{"only"}, names(nodes))
}
