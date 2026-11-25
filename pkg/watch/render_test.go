package watch

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/kubectl-k8i/pkg/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func renderString(snap Snapshot, cfg RenderConfig) string {
	var buf bytes.Buffer
	RenderSnapshot(&buf, snap, cfg)
	return buf.String()
}

func fullSnapshot() Snapshot {
	return Snapshot{
		Time:    refTime,
		Options: Options{Nodes: true, Pods: true, Events: true, EventsSince: 15 * time.Minute},
		Nodes: []NodeProblem{{
			Name: "node-1", Status: "NotReady", Since: "5m",
			Conditions: []string{"MemoryPressure"}, Reason: "KubeletNotReady", Message: "PLEG is\nnot healthy",
		}},
		TotalNodes: 10,
		Pods: []PodProblem{
			{Namespace: "default", Name: "web-1", Ready: "0/1", Status: "CrashLoopBackOff", Restarts: 7, Node: "node-2", Age: "1h0m"},
			{Namespace: "prod", Name: "api-1", Ready: "0/1", Status: "Pending", Node: "<none>", Age: "2m"},
		},
		TotalPods:      120,
		NamespaceCount: 8,
		Events: []EventProblem{
			{Namespace: "prod", Object: "Pod/api-1", Reason: "FailedScheduling", Message: "0/10 nodes available", Count: 4, LastSeen: "1m"},
		},
	}
}

func TestRenderSnapshot_AllSections(t *testing.T) {
	out := renderString(fullSnapshot(), RenderConfig{Interval: 5 * time.Second, Context: "prod-cluster", TermWidth: 200})

	assert.Contains(t, out, "k8i watch | context: prod-cluster | every 5s | 2026-11-25 12:00:00 | Ctrl+C to exit")
	assert.Contains(t, out, "NOT READY NODES: 1 of 10")
	assert.Contains(t, out, "PLEG is not healthy", "multi-line messages are flattened")
	assert.Contains(t, out, "PODS NOT READY: 2 (120 pods in 8 namespaces)")
	assert.Contains(t, out, "WARNING EVENTS (last 15m): 1")
	assert.Contains(t, out, "FailedScheduling")
	assert.NotContains(t, out, "ERRORS")
	assert.NotContains(t, out, "\033[", "no ANSI codes when colors are disabled")

	// Pod table header and alignment: STATUS column starts at the same offset in every row.
	lines := strings.Split(out, "\n")
	var header, row1, row2 string
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, "NAMESPACE  NAME"):
			header = l
		case strings.HasPrefix(l, "default"):
			row1 = l
		case strings.HasPrefix(l, "prod ") && strings.Contains(l, "api-1") && strings.Contains(l, "Pending"):
			row2 = l
		}
	}
	require.NotEmpty(t, header)
	require.NotEmpty(t, row1)
	require.NotEmpty(t, row2)
	col := strings.Index(header, "STATUS")
	assert.Equal(t, col, strings.Index(row1, "CrashLoopBackOff"))
	assert.Equal(t, col, strings.Index(row2, "Pending"))
}

func TestRenderSnapshot_OnlyEnabledSections(t *testing.T) {
	snap := fullSnapshot()
	snap.Options = Options{Pods: true}
	out := renderString(snap, RenderConfig{})

	assert.Contains(t, out, "PODS NOT READY")
	assert.NotContains(t, out, "NODES")
	assert.NotContains(t, out, "WARNING EVENTS")
}

func TestRenderSnapshot_HealthyCluster(t *testing.T) {
	snap := Snapshot{
		Time:           refTime,
		Options:        Options{Nodes: true, Pods: true, Events: true},
		TotalNodes:     3,
		TotalPods:      40,
		NamespaceCount: 4,
	}
	out := renderString(snap, RenderConfig{})

	assert.Contains(t, out, "NODES: all 3 nodes are Ready")
	assert.Contains(t, out, "PODS: all pods are Ready or Completed (40 pods in 4 namespaces)")
	assert.Contains(t, out, "WARNING EVENTS: none")
}

func TestRenderSnapshot_Errors(t *testing.T) {
	snap := Snapshot{Time: refTime, Options: Options{Nodes: true}, Errors: []string{"nodes: connection refused"}}
	out := renderString(snap, RenderConfig{})
	assert.Contains(t, out, "ERRORS (1)")
	assert.Contains(t, out, "  nodes: connection refused")
}

func TestRenderSnapshot_FailedSectionsAreNotHealthy(t *testing.T) {
	snap := Snapshot{
		Time:         refTime,
		Options:      Options{Nodes: true, Pods: true, Events: true},
		NodesFailed:  true,
		PodsFailed:   true,
		EventsFailed: true,
		Errors:       []string{"nodes: down", "pods: down", "events: down"},
	}
	out := renderString(snap, RenderConfig{})

	assert.Contains(t, out, "NODES: unavailable (see ERRORS)")
	assert.Contains(t, out, "PODS: unavailable (see ERRORS)")
	assert.Contains(t, out, "WARNING EVENTS: unavailable (see ERRORS)")
	assert.NotContains(t, out, "are Ready")
	assert.NotContains(t, out, "none")
	assert.Contains(t, out, "ERRORS (3)")
}

func TestRenderSnapshot_PartialPodFailure(t *testing.T) {
	snap := Snapshot{
		Time: refTime, Options: Options{Pods: true},
		TotalPods: 10, NamespaceCount: 3, FailedNamespaces: 1,
		Errors: []string{"pods: namespace prod: forbidden"},
	}
	out := renderString(snap, RenderConfig{})
	assert.Contains(t, out, "PODS: no problems found in readable namespaces (10 pods in 3 namespaces, 1 namespaces failed)")
	assert.NotContains(t, out, "all pods are Ready")

	snap.Pods = []PodProblem{{Namespace: "default", Name: "x", Ready: "0/1", Status: "Pending", Node: "<none>", Age: "1m"}}
	out = renderString(snap, RenderConfig{})
	assert.Contains(t, out, "PODS NOT READY: 1 (10 pods in 3 namespaces, 1 namespaces failed)")
}

func TestRenderSnapshot_EventsCapped(t *testing.T) {
	snap := Snapshot{Time: refTime, Options: Options{Events: true}}
	for i := range MaxEvents + 7 {
		snap.Events = append(snap.Events, EventProblem{Object: fmt.Sprintf("Pod/p-%03d", i), Reason: "BackOff", Count: 1, LastSeen: "1m"})
	}
	out := renderString(snap, RenderConfig{})

	assert.Contains(t, out, fmt.Sprintf("WARNING EVENTS: %d", MaxEvents+7))
	assert.Contains(t, out, fmt.Sprintf("Pod/p-%03d", MaxEvents-1))
	assert.NotContains(t, out, fmt.Sprintf("Pod/p-%03d", MaxEvents))
	assert.Contains(t, out, "... and 7 more")
	assert.Contains(t, out, "  -  ", "empty namespace is shown as '-'")
}

func TestRenderSnapshot_FitsTerminalWidth(t *testing.T) {
	snap := fullSnapshot()
	snap.Events[0].Message = strings.Repeat("very long message ", 30)
	out := renderString(snap, RenderConfig{TermWidth: 100})

	for _, l := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		assert.LessOrEqual(t, utf8.RuneCountInString(l), 100, "line too wide: %q", l)
	}
	assert.Contains(t, out, "…")
}

func TestRenderSnapshot_ColorsKeepAlignment(t *testing.T) {
	out := renderString(fullSnapshot(), RenderConfig{Color: color.ColorConfig{Enabled: true}, TermWidth: 200})

	assert.Contains(t, out, "\033[0;31mCrashLoopBackOff\033[0m", "failing status is red")
	assert.Contains(t, out, "\033[1;33mPending\033[0m", "transitional status is yellow")

	strip := func(s string) string {
		for _, code := range []string{"\033[0;31m", "\033[1;33m", "\033[0;32m", "\033[0m"} {
			s = strings.ReplaceAll(s, code, "")
		}
		return s
	}
	plain := renderString(fullSnapshot(), RenderConfig{TermWidth: 200})
	assert.Equal(t, plain, strip(out), "colored output must match plain output once ANSI codes are removed")
}

func TestPodStatusColor(t *testing.T) {
	cc := color.ColorConfig{Enabled: true}
	yellow := cc.ColorizeYellow("x")
	red := cc.ColorizeRed("x")

	for _, s := range []string{"Pending", "ContainerCreating", "Terminating", "Init:0/2", "NotReady"} {
		assert.Equal(t, yellow, podStatusColor(cc, s)("x"), s)
	}
	for _, s := range []string{"CrashLoopBackOff", "Failed", "Init:ImagePullBackOff", "OOMKilled", "Evicted"} {
		assert.Equal(t, red, podStatusColor(cc, s)("x"), s)
	}
}

func TestShortDuration(t *testing.T) {
	tests := map[time.Duration]string{
		5 * time.Second:             "5s",
		10 * time.Second:            "10s",
		15 * time.Minute:            "15m",
		90 * time.Second:            "1m30s",
		time.Hour:                   "1h",
		time.Hour + 30*time.Minute:  "1h30m",
		2*time.Hour + 5*time.Second: "2h0m5s",
		500 * time.Millisecond:      "500ms",
	}
	for d, want := range tests {
		assert.Equal(t, want, shortDuration(d), d.String())
	}
}
