package watch

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kubectl-k8i/pkg/color"
	"github.com/kubectl-k8i/pkg/render"
)

// MaxEvents caps the number of events rendered per refresh so a warning storm
// does not push the other sections off the screen.
const MaxEvents = 50

// defaultWidth is used when the terminal width is unknown.
const defaultWidth = 160

// RenderConfig holds rendering options for a watch snapshot.
type RenderConfig struct {
	Color     color.ColorConfig
	TermWidth int           // 0 = defaultWidth
	Interval  time.Duration // shown in the title line
	Context   string        // kube context name shown in the title line
}

// RenderSnapshot writes one full watch screen to w.
func RenderSnapshot(w io.Writer, snap Snapshot, cfg RenderConfig) {
	width := cfg.TermWidth
	if width <= 0 {
		width = defaultWidth
	}
	cc := cfg.Color

	title := "k8i watch"
	if cfg.Context != "" {
		title += " | context: " + cfg.Context
	}
	if cfg.Interval > 0 {
		title += " | every " + shortDuration(cfg.Interval)
	}
	title += " | " + snap.Time.Format("2006-01-02 15:04:05") + " | Ctrl+C to exit"
	_, _ = fmt.Fprintln(w, render.TruncateToFit(title, width))
	_, _ = fmt.Fprintln(w, strings.Repeat("=", width))

	if snap.Options.Nodes {
		renderNodes(w, snap, cc, width)
	}
	if snap.Options.Pods {
		renderPods(w, snap, cc, width)
	}
	if snap.Options.Events {
		renderEvents(w, snap, cc, width)
	}

	if len(snap.Errors) > 0 {
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, cc.ColorizeRed(fmt.Sprintf("ERRORS (%d)", len(snap.Errors))))
		for _, e := range snap.Errors {
			_, _ = fmt.Fprintln(w, "  "+render.TruncateToFit(e, width-2))
		}
	}
}

func renderNodes(w io.Writer, snap Snapshot, cc color.ColorConfig, width int) {
	_, _ = fmt.Fprintln(w)
	if snap.NodesFailed {
		_, _ = fmt.Fprintln(w, cc.ColorizeRed("NODES: unavailable (see ERRORS)"))
		return
	}
	if len(snap.Nodes) == 0 {
		_, _ = fmt.Fprintln(w, cc.ColorizeGreen(fmt.Sprintf("NODES: all %d nodes are Ready", snap.TotalNodes)))
		return
	}
	_, _ = fmt.Fprintln(w, cc.ColorizeRed(fmt.Sprintf("NOT READY NODES: %d of %d", len(snap.Nodes), snap.TotalNodes)))

	rows := make([][]string, 0, len(snap.Nodes))
	for _, n := range snap.Nodes {
		rows = append(rows, []string{n.Name, n.Status, n.Since, joinOrDash(n.Conditions), orDash(n.Reason), orDash(flattenMessage(n.Message))})
	}
	writeTable(w, []string{"NAME", "STATUS", "SINCE", "CONDITIONS", "REASON", "MESSAGE"}, rows, 1,
		func(string) func(string) string { return cc.ColorizeRed }, width)
}

func renderPods(w io.Writer, snap Snapshot, cc color.ColorConfig, width int) {
	_, _ = fmt.Fprintln(w)
	if snap.PodsFailed {
		_, _ = fmt.Fprintln(w, cc.ColorizeRed("PODS: unavailable (see ERRORS)"))
		return
	}
	scope := fmt.Sprintf("%d pods in %d namespaces", snap.TotalPods, snap.NamespaceCount)
	if snap.FailedNamespaces > 0 {
		scope += fmt.Sprintf(", %d namespaces failed", snap.FailedNamespaces)
	}
	if len(snap.Pods) == 0 && snap.FailedNamespaces > 0 {
		_, _ = fmt.Fprintln(w, cc.ColorizeYellow("PODS: no problems found in readable namespaces ("+scope+")"))
		return
	}
	if len(snap.Pods) == 0 {
		_, _ = fmt.Fprintln(w, cc.ColorizeGreen("PODS: all pods are Ready or Completed ("+scope+")"))
		return
	}
	_, _ = fmt.Fprintln(w, cc.ColorizeRed(fmt.Sprintf("PODS NOT READY: %d (%s)", len(snap.Pods), scope)))

	rows := make([][]string, 0, len(snap.Pods))
	for _, p := range snap.Pods {
		rows = append(rows, []string{p.Namespace, p.Name, p.Ready, p.Status, strconv.Itoa(int(p.Restarts)), p.Node, p.Age})
	}
	writeTable(w, []string{"NAMESPACE", "NAME", "READY", "STATUS", "RESTARTS", "NODE", "AGE"}, rows, 3,
		func(status string) func(string) string { return podStatusColor(cc, status) }, width)
}

func renderEvents(w io.Writer, snap Snapshot, cc color.ColorConfig, width int) {
	_, _ = fmt.Fprintln(w)
	window := ""
	if snap.Options.EventsSince > 0 {
		window = " (last " + shortDuration(snap.Options.EventsSince) + ")"
	}
	if snap.EventsFailed {
		_, _ = fmt.Fprintln(w, cc.ColorizeRed("WARNING EVENTS"+window+": unavailable (see ERRORS)"))
		return
	}
	if len(snap.Events) == 0 {
		_, _ = fmt.Fprintln(w, cc.ColorizeGreen("WARNING EVENTS"+window+": none"))
		return
	}
	_, _ = fmt.Fprintln(w, cc.ColorizeYellow(fmt.Sprintf("WARNING EVENTS%s: %d", window, len(snap.Events))))

	shown := snap.Events
	if len(shown) > MaxEvents {
		shown = shown[:MaxEvents]
	}
	rows := make([][]string, 0, len(shown))
	for _, e := range shown {
		rows = append(rows, []string{e.LastSeen, strconv.Itoa(int(e.Count)), orDash(e.Namespace), e.Object, e.Reason, flattenMessage(e.Message)})
	}
	writeTable(w, []string{"LAST SEEN", "COUNT", "NAMESPACE", "OBJECT", "REASON", "MESSAGE"}, rows, 4,
		func(string) func(string) string { return cc.ColorizeYellow }, width)

	if hidden := len(snap.Events) - len(shown); hidden > 0 {
		_, _ = fmt.Fprintf(w, "... and %d more\n", hidden)
	}
}

// podStatusColor picks a color for a pod status: yellow for transitional
// states, red for everything else.
func podStatusColor(cc color.ColorConfig, status string) func(string) string {
	switch status {
	case "Pending", "ContainerCreating", "PodInitializing", "Terminating", "NotReady", "Running":
		return cc.ColorizeYellow
	}
	if strings.HasPrefix(status, "Init:") && strings.Contains(status, "/") {
		return cc.ColorizeYellow
	}
	return cc.ColorizeRed
}

// writeTable writes an aligned table. The last column is truncated so lines
// fit into width. colorCol's cells are colorized after padding so ANSI codes
// do not break alignment.
func writeTable(w io.Writer, headers []string, rows [][]string, colorCol int, colorFor func(cell string) func(string) string, width int) {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = utf8.RuneCountInString(h)
	}
	for _, r := range rows {
		for i := 0; i < len(r)-1; i++ {
			if l := utf8.RuneCountInString(r[i]); l > widths[i] {
				widths[i] = l
			}
		}
	}

	// Space left for the last column (2-space gaps between columns).
	used := 0
	for i := 0; i < len(headers)-1; i++ {
		used += widths[i] + 2
	}
	lastWidth := max(width-used, 10)

	writeRow := func(cells []string, colorize bool) {
		var b strings.Builder
		for i, cell := range cells {
			if i == len(cells)-1 {
				b.WriteString(render.TruncateToFit(cell, lastWidth))
				break
			}
			padded := cell + strings.Repeat(" ", widths[i]-utf8.RuneCountInString(cell))
			if colorize && i == colorCol {
				padded = colorFor(cell)(cell) + strings.Repeat(" ", widths[i]-utf8.RuneCountInString(cell))
			}
			b.WriteString(padded)
			b.WriteString("  ")
		}
		_, _ = fmt.Fprintln(w, strings.TrimRight(b.String(), " "))
	}

	writeRow(headers, false)
	for _, r := range rows {
		writeRow(r, true)
	}
}

// shortDuration formats d without trailing zero units: 15m0s → 15m, 1h0m0s → 1h.
func shortDuration(d time.Duration) string {
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = strings.TrimSuffix(s, "0s")
	}
	if strings.HasSuffix(s, "h0m") {
		s = strings.TrimSuffix(s, "0m")
	}
	return s
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// flattenMessage collapses newlines so multi-line event messages stay on one row.
func flattenMessage(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
