package sort

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kubectl-k8i/pkg/model"
)

// SupportedSortColumns lists valid sort columns.
var SupportedSortColumns = []string{
	"name", "pods", "cpu_req", "cpu_lim", "cpu_use", "cpu_cap", "cpu_load",
	"mem_req", "mem_lim", "mem_use", "mem_cap", "mem_load",
	"ec2_type", "instance_type", "arch", "zone", "pool", "age", "taint", "autoscaler",
}

// NumericColumns lists columns that use numeric comparison.
var NumericColumns = []string{
	"pods", "cpu_req", "cpu_lim", "cpu_use", "cpu_cap", "cpu_load",
	"mem_req", "mem_lim", "mem_use", "mem_cap", "mem_load", "age",
}

// numericColumnSet provides O(1) lookup for numeric columns.
var numericColumnSet map[string]bool

// supportedColumnSet provides O(1) lookup for supported columns.
var supportedColumnSet map[string]bool

func init() {
	numericColumnSet = make(map[string]bool, len(NumericColumns))
	for _, col := range NumericColumns {
		numericColumnSet[col] = true
	}
	supportedColumnSet = make(map[string]bool, len(SupportedSortColumns))
	for _, col := range SupportedSortColumns {
		supportedColumnSet[col] = true
	}
}

// SortNodes sorts a slice of NodeInfo by the specified column and direction.
// column must be one of SupportedSortColumns.
// direction must be "asc" or "desc".
// Returns an error for unsupported column or invalid direction.
func SortNodes(nodes []model.NodeInfo, column, direction string) error {
	if !supportedColumnSet[column] {
		return fmt.Errorf("unsupported sort column %q, supported columns: %s", column, strings.Join(SupportedSortColumns, ", "))
	}
	if direction != "asc" && direction != "desc" {
		return fmt.Errorf("invalid sort direction %q, supported directions: asc, desc", direction)
	}

	isNumeric := numericColumnSet[column]
	desc := direction == "desc"

	sort.SliceStable(nodes, func(i, j int) bool {
		var less bool
		if isNumeric {
			vi := numericValue(nodes[i], column)
			vj := numericValue(nodes[j], column)
			less = vi < vj
		} else {
			si := textValue(nodes[i], column)
			sj := textValue(nodes[j], column)
			less = si < sj
		}
		if desc {
			return !less
		}
		return less
	})

	return nil
}

// numericValue extracts a float64 value from a NodeInfo for numeric sorting.
// For the "age" column, it returns the Unix timestamp (negated so older nodes
// sort as "larger" values in ascending order — oldest first).
func numericValue(n model.NodeInfo, column string) float64 {
	switch column {
	case "pods":
		return float64(n.PodsUsed)
	case "cpu_req":
		return n.CPURequestCores
	case "cpu_lim":
		return n.CPULimitCores
	case "cpu_use":
		return n.CPUUsageCores
	case "cpu_cap":
		return n.CPUCapacityCores
	case "cpu_load":
		return float64(n.CPULoadPercent)
	case "mem_req":
		return n.MemRequestGB
	case "mem_lim":
		return n.MemLimitGB
	case "mem_use":
		return n.MemUsageGB
	case "mem_cap":
		return n.MemCapacityGB
	case "mem_load":
		return float64(n.MemLoadPercent)
	case "age":
		// Sort by CreationTime: ascending = oldest first (smallest Unix time first).
		return float64(n.CreationTime.Unix())
	default:
		return 0
	}
}

// textValue extracts a string value from a NodeInfo for lexicographic sorting.
func textValue(n model.NodeInfo, column string) string {
	switch column {
	case "name":
		return n.Name
	case "ec2_type":
		return n.CapacityType
	case "instance_type":
		return n.InstanceType
	case "arch":
		return n.Architecture
	case "zone":
		return n.Zone
	case "pool":
		return n.Nodepool
	case "taint":
		return n.TaintSortKey
	case "autoscaler":
		return n.Autoscaler
	default:
		return ""
	}
}
