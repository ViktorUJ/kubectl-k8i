package analyze

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/kubectl-k8i/pkg/color"
	"github.com/kubectl-k8i/pkg/model"
)

// RenderConfig holds rendering options for the analyze table.
type RenderConfig struct {
	Color     color.ColorConfig
	NodeDesc  string // description of the node selector used
	NoHeaders bool
	TermWidth int
}

// RenderTable writes the workload analysis table to w.
func RenderTable(w io.Writer, workloads []model.WorkloadInfo, cfg RenderConfig) {
	if len(workloads) == 0 {
		_, _ = fmt.Fprintf(w, "no workloads found on the selected nodes\n")
		return
	}

	if !cfg.NoHeaders {
		renderAnalyzeHeader(w)
		renderSeparator(w, cfg.TermWidth)
	}

	for _, wl := range workloads {
		renderWorkloadRow(w, wl, cfg.Color)
	}
}

// renderAnalyzeHeader writes the 2-line header for the analyze table.
func renderAnalyzeHeader(w io.Writer) {
	_, _ = fmt.Fprintf(w, "%-20s %-12s %-35s %5s  %-17s  %-17s  %-7s %-7s\n",
		"NAMESPACE", "KIND", "NAME", "PODS",
		"CPU req/lim/use", "MEM req/lim/use GB", "CPU OC%", "MEM OC%")
	_, _ = fmt.Fprintf(w, "%-20s %-12s %-35s %5s  %-17s  %-17s  %-7s %-7s\n",
		"", "", "", "", "(cores)", "", "(lim/req)", "(lim/req)")
}

// renderSeparator writes an equals-sign separator.
func renderSeparator(w io.Writer, termWidth int) {
	width := termWidth
	if width <= 0 {
		width = 140
	}
	_, _ = fmt.Fprintf(w, "%s\n", strings.Repeat("=", width))
}

// renderWorkloadRow writes a single workload data row.
func renderWorkloadRow(w io.Writer, wl model.WorkloadInfo, cc color.ColorConfig) {
	ns := truncate(wl.Namespace, 20)
	kind := truncate(wl.Kind, 12)
	name := truncate(wl.Name, 35)

	cpuCombo := fmt.Sprintf("%.2f/%.2f/%.2f", wl.CPURequestCores, wl.CPULimitCores, wl.CPUUsageCores)
	memCombo := fmt.Sprintf("%.2f/%.2f/%.2f", wl.MemRequestGB, wl.MemLimitGB, wl.MemUsageGB)

	// Colorize CPU by overcommit ratio (limits vs requests).
	if wl.CPURequestCores > 0 {
		ratio := wl.CPULimitCores / wl.CPURequestCores
		cpuCombo = cc.ColorizeRatio(cpuCombo, ratio, 1.0)
	}

	// Colorize MEM: green if limits ≤ requests*2, else by overcommit.
	if wl.MemLimitGB <= wl.MemRequestGB*2 {
		memCombo = cc.ColorizeGreen(memCombo)
	} else {
		memCombo = cc.ColorizeOvercommitPct(memCombo, wl.MemLimitGB, wl.MemRequestGB)
	}

	cpuPad := cpuCombo + strings.Repeat(" ", max(0, 17-displayWidth(cpuCombo)))
	memPad := memCombo + strings.Repeat(" ", max(0, 17-displayWidth(memCombo)))

	_, _ = fmt.Fprintf(w, "%-20s %-12s %-35s %5d  %s  %s  %-7s %-7s\n",
		ns, kind, name, wl.PodCount, cpuPad, memPad,
		formatOvercommit(wl.CPUOvercommitPct), formatOvercommit(wl.MemOvercommitPct))
}

// formatOvercommit formats an overcommit percentage for display.
// A negative value (request is zero) renders as "n/a".
func formatOvercommit(pct float64) string {
	if pct < 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.0f%%", pct)
}

// truncate truncates s to maxLen runes, appending "…" if needed.
func truncate(s string, maxLen int) string {
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

// displayWidth returns visible width of s, excluding ANSI escape sequences.
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
