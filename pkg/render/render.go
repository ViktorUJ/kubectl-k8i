package render

import (
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kubectl-k8i/pkg/color"
	"github.com/kubectl-k8i/pkg/model"
	"github.com/kubectl-k8i/pkg/taints"
)

// RenderConfig holds rendering options for the table output.
type RenderConfig struct {
	Color        color.ColorConfig
	Filter       string // display active filter
	Sort         string // display active sort
	GroupByTaint bool
	Timestamp    time.Time
	TermWidth    int  // detected terminal width (0 = no limit)
	NoHeaders    bool // suppress header, separator, timestamp, annotations
}

// Column widths matching the original k8i bash script layout.
const (
	nameWidth = 45 // node name
)

// TruncateToFit truncates a string to maxLen, appending "…" (Unicode ellipsis) if truncated.
func TruncateToFit(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	if maxLen <= 1 {
		return "…"
	}
	return string(runes[:maxLen-1]) + "…"
}

// RenderTable renders the final table to the given writer.
func RenderTable(w io.Writer, nodes []model.NodeInfo, config RenderConfig) {
	if len(nodes) == 0 {
		_, _ = fmt.Fprintf(w, "no nodes match filter\n")
		return
	}

	if !config.NoHeaders {
		renderHeader(w)
		renderSeparator(w, config.TermWidth)
	}

	if config.GroupByTaint {
		groups := taints.GroupByTaints(nodes)
		for i, group := range groups {
			for _, node := range group {
				renderDataRow(w, node, config.Color)
			}
			if i < len(groups)-1 && !config.NoHeaders {
				renderGroupSeparator(w, config.TermWidth)
			}
		}
	} else {
		for _, node := range nodes {
			renderDataRow(w, node, config.Color)
		}
	}
}

// renderHeader writes a 3-line header.
// Uses the same spacing as data rows — all spaces, no tabs.
func renderHeader(w io.Writer) {
	// Line 1: group names.
	_, _ = fmt.Fprintf(w, "%-45s %-7s %-17s %-4s  %-19s %-4s  %s\n",
		"                    NODE", "PODS", "CPU cores", "CPU", "MEMORY GB", "MEM", "        Node info")
	// Line 2: sub-column detail part 1.
	_, _ = fmt.Fprintf(w, "%-45s %-7s %-17s %-4s  %-19s %-4s  %s\n",
		"", "used/", "req/lim/", "LOAD", "req/lim/", "LOAD", "     ec2/type/spot/arch/")
	// Line 3: sub-column detail part 2.
	_, _ = fmt.Fprintf(w, "%-45s %-7s %-17s %-4s  %-19s %-4s  %s\n",
		"", "max", "use/total", "", "use/total", "", "     zone/pool/nodeclaim/age/taints")
}

// renderSeparator writes an equals-sign separator line.
func renderSeparator(w io.Writer, termWidth int) {
	width := termWidth
	if width <= 0 {
		width = 152
	}
	_, _ = fmt.Fprintf(w, "%s\n", strings.Repeat("=", width))
}

// renderGroupSeparator writes a tilde separator between taint groups.
func renderGroupSeparator(w io.Writer, termWidth int) {
	width := termWidth
	if width <= 0 {
		width = 152
	}
	_, _ = fmt.Fprintf(w, "%s\n", strings.Repeat("~", width))
}

// renderDataRow writes a single node data row.
// Format: %-45s\t%-7s\t%-19s %s\t  %-19s %s  %s
func renderDataRow(w io.Writer, node model.NodeInfo, cc color.ColorConfig) {
	name := TruncateToFit(node.Name, nameWidth)
	pods := fmt.Sprintf("%d/%d", node.PodsUsed, node.PodsMax)

	cpuCap := formatCapacity(node.CPUCapacityCores)
	cpuCombo := fmt.Sprintf("%.1f/%.1f/%.1f/%s",
		node.CPURequestCores, node.CPULimitCores, node.CPUUsageCores, cpuCap)

	// Colorize CPU combo based on overcommit ratio (limits vs requests, limits vs capacity).
	// Green (≤5x), Yellow (5-10x), Red (>10x).
	maxRatio := maxOvercommitRatio(node.CPULimitCores, node.CPURequestCores, node.CPUCapacityCores)
	cpuCombo = cc.ColorizeRatio(cpuCombo, maxRatio, 1.0)

	cpuLoad := cc.ColorizeLoad(node.CPULoadPercent)

	memCap := formatCapacity(node.MemCapacityGB)
	memCombo := fmt.Sprintf("%.1f/%.1f/%.1f/%s",
		node.MemRequestGB, node.MemLimitGB, node.MemUsageGB, memCap)

	// Colorize MEM combo:
	// If limits ≤ capacity — green (node has enough resources).
	// If limits > capacity — color by overcommit %: green ≤30%, yellow 30–50%, red >50%.
	if node.MemLimitGB <= node.MemCapacityGB {
		memCombo = cc.ColorizeGreen(memCombo)
	} else {
		memCombo = cc.ColorizeOvercommitPct(memCombo, node.MemLimitGB, node.MemRequestGB)
	}

	memLoad := cc.ColorizeLoad(node.MemLoadPercent)

	// Truncate taints to 20 visible characters.
	taintDisplay := TruncateToFit(node.TaintStr, 20)

	nodeInfo := fmt.Sprintf("%s/%s/%s/%s/%s/%s/%s/%s/%s",
		node.EC2InstanceID, node.InstanceType, node.CapacityType,
		node.Architecture, node.Zone, node.Nodepool,
		node.Nodeclaim, node.Age, taintDisplay)

	// Pad load to 4 visible chars to match header "LOAD".
	cpuLoadPad := cpuLoad + strings.Repeat(" ", max(0, 4-displayWidth(cpuLoad)))
	memLoadPad := memLoad + strings.Repeat(" ", max(0, 4-displayWidth(memLoad)))

	// Pad CPU combo to 17 visible chars (may contain ANSI codes).
	cpuComboPad := cpuCombo + strings.Repeat(" ", max(0, 17-displayWidth(cpuCombo)))
	// Pad MEM combo to 19 visible chars (may contain ANSI codes).
	memComboPad := memCombo + strings.Repeat(" ", max(0, 19-displayWidth(memCombo)))

	// All spaces, no tabs.
	_, _ = fmt.Fprintf(w, "%-45s %-7s %s %s  %s %s  %s\n",
		name, pods, cpuComboPad, cpuLoadPad, memComboPad, memLoadPad, nodeInfo)
}

// maxOvercommitRatio returns the maximum overcommit ratio from limits/requests and limits/capacity.
// Returns 0 if limits are zero.
func maxOvercommitRatio(limits, requests, capacity float64) float64 {
	if limits <= 0 {
		return 0
	}
	var ratio float64
	if requests > 0 {
		r := limits / requests
		if r > ratio {
			ratio = r
		}
	}
	if capacity > 0 {
		r := limits / capacity
		if r > ratio {
			ratio = r
		}
	}
	return ratio
}

// formatCapacity formats a capacity value: integer if whole number, one decimal otherwise.
func formatCapacity(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d", int64(v))
	}
	return fmt.Sprintf("%.1f", v)
}

// displayWidth returns the visible display width of a string,
// excluding ANSI escape sequences.
func displayWidth(s string) int {
	inEscape := false
	width := 0
	for _, r := range s {
		if r == '\033' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		width++
	}
	return width
}
