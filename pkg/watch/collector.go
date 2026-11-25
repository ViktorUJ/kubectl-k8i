// Package watch implements the "watch" subcommand: a continuously refreshing
// view of cluster problems (not-ready nodes, not-ready pods, warning events).
package watch

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/kubectl-k8i/pkg/age"
	"github.com/kubectl-k8i/pkg/retry"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// DefaultConcurrency is the maximum number of parallel per-namespace pod list calls.
const DefaultConcurrency = 10

// Options selects what the watch collector gathers.
type Options struct {
	Nodes       bool          // collect not-ready nodes
	Pods        bool          // collect not-ready / not-completed pods
	Events      bool          // collect Warning events
	Namespaces  []string      // namespaces to inspect for pods; empty = all namespaces
	EventsSince time.Duration // only events last seen within this window are shown
	Concurrency int           // parallel namespace queries; <=0 uses DefaultConcurrency
}

// EventProblem describes a Warning event.
type EventProblem struct {
	Namespace string `json:"namespace"`
	Object    string `json:"object"` // Kind/Name of the involved object
	Reason    string `json:"reason"`
	Message   string `json:"message"`
	Count     int32  `json:"count"`
	LastSeen  string `json:"lastSeen"` // age of the last occurrence

	lastSeenTime time.Time
}

// Snapshot is one refresh of the watch view.
type Snapshot struct {
	Time    time.Time
	Options Options

	Nodes      []NodeProblem
	TotalNodes int

	Pods           []PodProblem
	TotalPods      int
	NamespaceCount int

	Events []EventProblem

	// Section availability: true when the section could not be collected at
	// all, so the view must not claim the cluster is healthy.
	NodesFailed  bool
	PodsFailed   bool
	EventsFailed bool
	// FailedNamespaces counts namespaces whose pods could not be listed
	// (pod results are partial when > 0).
	FailedNamespaces int

	// Errors are non-fatal per-section errors shown in the view; the watch
	// loop keeps running when the API is temporarily unavailable.
	Errors []string
}

// Collector gathers watch snapshots from the Kubernetes API.
type Collector struct {
	clientset   kubernetes.Interface
	retryConfig retry.RetryConfig
}

// NewCollector creates a watch Collector.
func NewCollector(clientset kubernetes.Interface, retryConfig retry.RetryConfig) *Collector {
	return &Collector{clientset: clientset, retryConfig: retryConfig}
}

// Collect gathers a single snapshot. Section failures are recorded in
// Snapshot.Errors instead of aborting the whole refresh.
func (c *Collector) Collect(ctx context.Context, opts Options, now time.Time) Snapshot {
	snap := Snapshot{Time: now, Options: opts}

	// Sections write to disjoint Snapshot fields, so they run in parallel;
	// errors are kept per section and appended in a fixed order.
	var (
		wg                sync.WaitGroup
		nodeErr, eventErr error
		podErrs           []error
	)
	if opts.Nodes {
		wg.Go(func() { nodeErr = c.collectNodes(ctx, &snap, now) })
	}
	if opts.Pods {
		wg.Go(func() { podErrs = c.collectPods(ctx, &snap, now) })
	}
	if opts.Events {
		wg.Go(func() { eventErr = c.collectEvents(ctx, &snap, now) })
	}
	wg.Wait()

	if nodeErr != nil {
		snap.NodesFailed = true
		snap.Errors = append(snap.Errors, fmt.Sprintf("nodes: %v", nodeErr))
	}
	for _, err := range podErrs {
		snap.Errors = append(snap.Errors, fmt.Sprintf("pods: %v", err))
	}
	if eventErr != nil {
		snap.EventsFailed = true
		snap.Errors = append(snap.Errors, fmt.Sprintf("events: %v", eventErr))
	}
	return snap
}

// WatchRetryConfig is a short retry policy for the watch loop: a failed
// refresh is retried on the next tick anyway, so long backoffs only delay
// the screen.
func WatchRetryConfig() retry.RetryConfig {
	return retry.RetryConfig{
		MaxRetries:     2,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     400 * time.Millisecond,
		JitterFraction: 0.5,
	}
}

func (c *Collector) collectNodes(ctx context.Context, snap *Snapshot, now time.Time) error {
	var list *corev1.NodeList
	err := retry.WithRetry(ctx, c.retryConfig, "list nodes", func() error {
		var err error
		list, err = c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		return err
	})
	if err != nil {
		return err
	}

	snap.TotalNodes = len(list.Items)
	for i := range list.Items {
		if !IsNodeReady(&list.Items[i]) {
			snap.Nodes = append(snap.Nodes, NodeProblemFor(&list.Items[i], now))
		}
	}
	sort.Slice(snap.Nodes, func(i, j int) bool { return snap.Nodes[i].Name < snap.Nodes[j].Name })
	return nil
}

// resolveNamespaces returns the namespaces to inspect: the explicit list
// (deduplicated) or every namespace in the cluster.
func (c *Collector) resolveNamespaces(ctx context.Context, explicit []string) ([]string, error) {
	if len(explicit) > 0 {
		seen := make(map[string]struct{}, len(explicit))
		out := make([]string, 0, len(explicit))
		for _, ns := range explicit {
			if _, ok := seen[ns]; ok || ns == "" {
				continue
			}
			seen[ns] = struct{}{}
			out = append(out, ns)
		}
		sort.Strings(out)
		return out, nil
	}

	var list *corev1.NamespaceList
	err := retry.WithRetry(ctx, c.retryConfig, "list namespaces", func() error {
		var err error
		list, err = c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		return err
	})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(list.Items))
	for _, ns := range list.Items {
		out = append(out, ns.Name)
	}
	sort.Strings(out)
	return out, nil
}

// collectPods lists namespaces, then lists pods in each namespace in parallel.
// A failure in one namespace does not prevent results from the others.
func (c *Collector) collectPods(ctx context.Context, snap *Snapshot, now time.Time) []error {
	namespaces, err := c.resolveNamespaces(ctx, snap.Options.Namespaces)
	if err != nil {
		snap.PodsFailed = true
		return []error{err}
	}
	snap.NamespaceCount = len(namespaces)

	limit := snap.Options.Concurrency
	if limit <= 0 {
		limit = DefaultConcurrency
	}

	var (
		mu       sync.Mutex
		errs     []error
		problems []PodProblem
		total    int
	)
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(limit)

	for _, ns := range namespaces {
		g.Go(func() error {
			var list *corev1.PodList
			err := retry.WithRetry(gctx, c.retryConfig, "list pods in "+ns, func() error {
				var err error
				list, err = c.clientset.CoreV1().Pods(ns).List(gctx, metav1.ListOptions{})
				return err
			})

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, fmt.Errorf("namespace %s: %w", ns, err))
				return nil // keep collecting other namespaces
			}
			total += len(list.Items)
			for i := range list.Items {
				if IsPodProblem(&list.Items[i]) {
					problems = append(problems, PodProblemFor(&list.Items[i], now))
				}
			}
			return nil
		})
	}
	_ = g.Wait()

	sort.Slice(problems, func(i, j int) bool {
		if problems[i].Namespace != problems[j].Namespace {
			return problems[i].Namespace < problems[j].Namespace
		}
		return problems[i].Name < problems[j].Name
	})
	sort.Slice(errs, func(i, j int) bool { return errs[i].Error() < errs[j].Error() })

	snap.Pods = problems
	snap.TotalPods = total
	snap.FailedNamespaces = len(errs)
	if len(namespaces) > 0 && len(errs) == len(namespaces) {
		snap.PodsFailed = true
	}
	return errs
}

// collectEvents lists Warning events cluster-wide that were last seen within
// Options.EventsSince, newest first.
func (c *Collector) collectEvents(ctx context.Context, snap *Snapshot, now time.Time) error {
	var list *corev1.EventList
	err := retry.WithRetry(ctx, c.retryConfig, "list events", func() error {
		var err error
		list, err = c.clientset.CoreV1().Events("").List(ctx, metav1.ListOptions{
			FieldSelector: "type=" + corev1.EventTypeWarning,
		})
		return err
	})
	if err != nil {
		return err
	}

	var cutoff time.Time
	if snap.Options.EventsSince > 0 {
		cutoff = now.Add(-snap.Options.EventsSince)
	}

	for i := range list.Items {
		ev := &list.Items[i]
		if ev.Type != corev1.EventTypeWarning {
			continue
		}
		last := EventLastSeen(ev)
		if !cutoff.IsZero() && last.Before(cutoff) {
			continue
		}
		count := ev.Count
		if ev.Series != nil && ev.Series.Count > count {
			count = ev.Series.Count
		}
		if count == 0 {
			count = 1
		}
		snap.Events = append(snap.Events, EventProblem{
			Namespace:    ev.Namespace,
			Object:       ev.InvolvedObject.Kind + "/" + ev.InvolvedObject.Name,
			Reason:       ev.Reason,
			Message:      ev.Message,
			Count:        count,
			LastSeen:     age.FormatAge(last, now),
			lastSeenTime: last,
		})
	}

	sort.SliceStable(snap.Events, func(i, j int) bool {
		return snap.Events[i].lastSeenTime.After(snap.Events[j].lastSeenTime)
	})
	return nil
}

// EventLastSeen returns the most recent timestamp recorded on an event,
// falling back through the various fields used by different event producers.
func EventLastSeen(ev *corev1.Event) time.Time {
	if ev.Series != nil && !ev.Series.LastObservedTime.IsZero() {
		return ev.Series.LastObservedTime.Time
	}
	if !ev.LastTimestamp.IsZero() {
		return ev.LastTimestamp.Time
	}
	if !ev.EventTime.IsZero() {
		return ev.EventTime.Time
	}
	if !ev.FirstTimestamp.IsZero() {
		return ev.FirstTimestamp.Time
	}
	return ev.CreationTimestamp.Time
}
