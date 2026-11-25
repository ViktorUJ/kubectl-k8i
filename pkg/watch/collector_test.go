package watch

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/kubectl-k8i/pkg/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func noRetry() retry.RetryConfig { return retry.RetryConfig{} }

func ns(name string) *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func pod(namespace, name string, phase corev1.PodPhase, ready bool) *corev1.Pod {
	status := corev1.ConditionFalse
	if ready {
		status = corev1.ConditionTrue
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, CreationTimestamp: metav1.NewTime(refTime.Add(-time.Hour))},
		Spec:       corev1.PodSpec{NodeName: "node-1", Containers: []corev1.Container{{Name: "app"}}},
		Status: corev1.PodStatus{
			Phase:      phase,
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: status}},
		},
	}
}

func warningEvent(namespace, name, reason string, last time.Time) *corev1.Event {
	return &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: name, Namespace: namespace},
		Type:           corev1.EventTypeWarning,
		Reason:         reason,
		Message:        reason + " happened",
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: name},
		LastTimestamp:  metav1.NewTime(last),
		Count:          3,
	}
}

func clusterObjects() []runtime.Object {
	return []runtime.Object{
		nodeWithReady("node-ok", corev1.ConditionTrue),
		nodeWithReady("node-b-bad", corev1.ConditionFalse),
		nodeWithReady("node-a-unknown", corev1.ConditionUnknown),
		ns("default"), ns("prod"), ns("kube-system"),
		pod("default", "web-ok", corev1.PodRunning, true),
		pod("default", "web-notready", corev1.PodRunning, false),
		pod("default", "job-done", corev1.PodSucceeded, false),
		pod("prod", "api-pending", corev1.PodPending, false),
		pod("prod", "api-failed", corev1.PodFailed, false),
		pod("kube-system", "dns", corev1.PodRunning, true),
	}
}

func podNames(pods []PodProblem) []string {
	out := make([]string, 0, len(pods))
	for _, p := range pods {
		out = append(out, p.Namespace+"/"+p.Name)
	}
	return out
}

func TestCollect_OnlyEnabledSections(t *testing.T) {
	cs := fake.NewClientset(clusterObjects()...)
	c := NewCollector(cs, noRetry())

	snap := c.Collect(context.Background(), Options{Nodes: true}, refTime)

	assert.Len(t, snap.Nodes, 2)
	assert.Nil(t, snap.Pods)
	assert.Nil(t, snap.Events)
	for _, a := range cs.Actions() {
		assert.Equal(t, "nodes", a.GetResource().Resource, "only nodes must be queried when only --nodes is set")
	}
}

func TestCollect_NotReadyNodes(t *testing.T) {
	c := NewCollector(fake.NewClientset(clusterObjects()...), noRetry())

	snap := c.Collect(context.Background(), Options{Nodes: true}, refTime)

	require.Empty(t, snap.Errors)
	assert.Equal(t, 3, snap.TotalNodes)
	require.Len(t, snap.Nodes, 2)
	assert.Equal(t, "node-a-unknown", snap.Nodes[0].Name, "nodes are sorted by name")
	assert.Equal(t, "Unknown", snap.Nodes[0].Status)
	assert.Equal(t, "node-b-bad", snap.Nodes[1].Name)
	assert.Equal(t, "NotReady", snap.Nodes[1].Status)
}

func TestCollect_PodsAllNamespaces_ListsNamespacesThenPodsPerNamespace(t *testing.T) {
	cs := fake.NewClientset(clusterObjects()...)
	c := NewCollector(cs, noRetry())

	snap := c.Collect(context.Background(), Options{Pods: true}, refTime)

	require.Empty(t, snap.Errors)
	assert.Equal(t, []string{"default/web-notready", "prod/api-failed", "prod/api-pending"}, podNames(snap.Pods))
	assert.Equal(t, 6, snap.TotalPods)
	assert.Equal(t, 3, snap.NamespaceCount)

	var nsLists int
	var podNamespaces []string
	for _, a := range cs.Actions() {
		if a.GetVerb() != "list" {
			continue
		}
		switch a.GetResource().Resource {
		case "namespaces":
			nsLists++
		case "pods":
			podNamespaces = append(podNamespaces, a.GetNamespace())
		}
	}
	sort.Strings(podNamespaces)
	assert.Equal(t, 1, nsLists, "namespace list must be fetched once")
	assert.Equal(t, []string{"default", "kube-system", "prod"}, podNamespaces, "pods must be listed per namespace, never cluster-wide")
}

func TestCollect_PodsExplicitNamespaces(t *testing.T) {
	cs := fake.NewClientset(clusterObjects()...)
	c := NewCollector(cs, noRetry())

	snap := c.Collect(context.Background(), Options{Pods: true, Namespaces: []string{"prod", "prod", ""}}, refTime)

	require.Empty(t, snap.Errors)
	assert.Equal(t, []string{"prod/api-failed", "prod/api-pending"}, podNames(snap.Pods))
	assert.Equal(t, 1, snap.NamespaceCount)
	for _, a := range cs.Actions() {
		assert.NotEqual(t, "namespaces", a.GetResource().Resource, "namespaces must not be listed when --namespaces is given")
		if a.GetResource().Resource == "pods" {
			assert.Equal(t, "prod", a.GetNamespace())
		}
	}
}

func TestCollect_PodsNamespaceErrorIsNonFatal(t *testing.T) {
	cs := fake.NewClientset(clusterObjects()...)
	cs.PrependReactor("list", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.GetNamespace() == "prod" {
			return true, nil, errors.New("forbidden")
		}
		return false, nil, nil
	})
	c := NewCollector(cs, noRetry())

	snap := c.Collect(context.Background(), Options{Pods: true}, refTime)

	assert.Equal(t, []string{"default/web-notready"}, podNames(snap.Pods), "other namespaces are still reported")
	assert.Equal(t, 1, snap.FailedNamespaces)
	assert.False(t, snap.PodsFailed, "partial failure is not a full section failure")
	require.Len(t, snap.Errors, 1)
	assert.Contains(t, snap.Errors[0], "namespace prod")
	assert.Contains(t, snap.Errors[0], "forbidden")
}

func TestCollect_NamespaceListErrorIsReported(t *testing.T) {
	cs := fake.NewClientset(clusterObjects()...)
	cs.PrependReactor("list", "namespaces", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("boom")
	})
	c := NewCollector(cs, noRetry())

	snap := c.Collect(context.Background(), Options{Pods: true, Nodes: true}, refTime)

	assert.Len(t, snap.Nodes, 2, "node section is unaffected")
	assert.False(t, snap.NodesFailed)
	assert.True(t, snap.PodsFailed)
	require.Len(t, snap.Errors, 1)
	assert.Contains(t, snap.Errors[0], "pods: boom")
}

func TestCollect_NodeListError(t *testing.T) {
	cs := fake.NewClientset()
	cs.PrependReactor("list", "nodes", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("unreachable")
	})
	snap := NewCollector(cs, noRetry()).Collect(context.Background(), Options{Nodes: true}, refTime)
	assert.Equal(t, []string{"nodes: unreachable"}, snap.Errors)
	assert.True(t, snap.NodesFailed)
}

func TestCollect_AllNamespacesFailMarksPodsFailed(t *testing.T) {
	cs := fake.NewClientset(clusterObjects()...)
	cs.PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("forbidden")
	})
	snap := NewCollector(cs, noRetry()).Collect(context.Background(), Options{Pods: true}, refTime)

	assert.True(t, snap.PodsFailed)
	assert.Equal(t, 3, snap.FailedNamespaces)
	assert.Len(t, snap.Errors, 3)
	assert.Contains(t, snap.Errors[0], "namespace default", "errors are sorted for a stable screen")
}

func TestCollect_EventListError(t *testing.T) {
	cs := fake.NewClientset()
	cs.PrependReactor("list", "events", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("timeout")
	})
	snap := NewCollector(cs, noRetry()).Collect(context.Background(), Options{Events: true, Nodes: true}, refTime)

	assert.True(t, snap.EventsFailed)
	assert.False(t, snap.NodesFailed)
	assert.Equal(t, []string{"events: timeout"}, snap.Errors)
}

func TestCollect_ErrorOrderIsStable(t *testing.T) {
	cs := fake.NewClientset()
	for _, res := range []string{"nodes", "namespaces", "events"} {
		cs.PrependReactor("list", res, func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("down")
		})
	}
	snap := NewCollector(cs, noRetry()).Collect(context.Background(), Options{Nodes: true, Pods: true, Events: true}, refTime)
	assert.Equal(t, []string{"nodes: down", "pods: down", "events: down"}, snap.Errors)
}

func TestCollect_WarningEventsWindowAndOrder(t *testing.T) {
	normal := warningEvent("default", "normal", "Scheduled", refTime.Add(-time.Minute))
	normal.Type = corev1.EventTypeNormal

	objs := []runtime.Object{
		warningEvent("default", "older", "BackOff", refTime.Add(-10*time.Minute)),
		warningEvent("prod", "newest", "FailedMount", refTime.Add(-1*time.Minute)),
		warningEvent("prod", "stale", "FailedScheduling", refTime.Add(-2*time.Hour)),
		normal,
	}
	cs := fake.NewClientset(objs...)
	c := NewCollector(cs, noRetry())

	snap := c.Collect(context.Background(), Options{Events: true, EventsSince: 15 * time.Minute}, refTime)

	require.Empty(t, snap.Errors)
	require.Len(t, snap.Events, 2)
	assert.Equal(t, "Pod/newest", snap.Events[0].Object, "newest first")
	assert.Equal(t, "FailedMount", snap.Events[0].Reason)
	assert.Equal(t, "1m", snap.Events[0].LastSeen)
	assert.Equal(t, int32(3), snap.Events[0].Count)
	assert.Equal(t, "Pod/older", snap.Events[1].Object)

	// Events are requested cluster-wide with a Warning field selector.
	for _, a := range cs.Actions() {
		if la, ok := a.(k8stesting.ListAction); ok && a.GetResource().Resource == "events" {
			assert.Equal(t, "", a.GetNamespace())
			assert.Equal(t, "type=Warning", la.GetListRestrictions().Fields.String())
		}
	}

	// Zero window = no time limit.
	snap = c.Collect(context.Background(), Options{Events: true}, refTime)
	assert.Len(t, snap.Events, 3)
}

func TestEventLastSeen(t *testing.T) {
	t1 := refTime.Add(-time.Minute)
	ev := &corev1.Event{ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.NewTime(refTime.Add(-time.Hour))}}
	assert.Equal(t, refTime.Add(-time.Hour), EventLastSeen(ev), "falls back to creation time")

	ev.FirstTimestamp = metav1.NewTime(t1.Add(-time.Second))
	assert.Equal(t, t1.Add(-time.Second), EventLastSeen(ev))

	ev.EventTime = metav1.NewMicroTime(t1.Add(-500 * time.Millisecond))
	assert.Equal(t, t1.Add(-500*time.Millisecond), EventLastSeen(ev))

	ev.LastTimestamp = metav1.NewTime(t1)
	assert.Equal(t, t1, EventLastSeen(ev))

	ev.Series = &corev1.EventSeries{Count: 9, LastObservedTime: metav1.NewMicroTime(refTime)}
	assert.Equal(t, refTime, EventLastSeen(ev), "series last observed time wins")
}
