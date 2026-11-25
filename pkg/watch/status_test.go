package watch

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var refTime = time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)

func nodeWithReady(name string, status corev1.ConditionStatus) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{
			Type:               corev1.NodeReady,
			Status:             status,
			Reason:             "KubeletNotReady",
			Message:            "container runtime is down",
			LastTransitionTime: metav1.NewTime(refTime.Add(-90 * time.Minute)),
		}}},
	}
}

func TestIsNodeReady(t *testing.T) {
	assert.True(t, IsNodeReady(nodeWithReady("a", corev1.ConditionTrue)))
	assert.False(t, IsNodeReady(nodeWithReady("b", corev1.ConditionFalse)))
	assert.False(t, IsNodeReady(nodeWithReady("c", corev1.ConditionUnknown)))
	assert.False(t, IsNodeReady(&corev1.Node{}), "node without Ready condition is not ready")
}

func TestNodeProblemFor(t *testing.T) {
	n := nodeWithReady("node-1", corev1.ConditionFalse)
	n.Spec.Unschedulable = true
	n.Status.Conditions = append(n.Status.Conditions,
		corev1.NodeCondition{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionTrue},
		corev1.NodeCondition{Type: corev1.NodeDiskPressure, Status: corev1.ConditionFalse},
		corev1.NodeCondition{Type: corev1.NodePIDPressure, Status: corev1.ConditionTrue},
	)

	p := NodeProblemFor(n, refTime)
	assert.Equal(t, "node-1", p.Name)
	assert.Equal(t, "NotReady,SchedulingDisabled", p.Status)
	assert.Equal(t, "1h30m", p.Since)
	assert.Equal(t, []string{"MemoryPressure", "PIDPressure"}, p.Conditions)
	assert.Equal(t, "KubeletNotReady", p.Reason)
	assert.Equal(t, "container runtime is down", p.Message)
}

func TestNodeProblemFor_UnknownAndMissing(t *testing.T) {
	assert.Equal(t, "Unknown", NodeProblemFor(nodeWithReady("u", corev1.ConditionUnknown), refTime).Status)

	p := NodeProblemFor(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "m"}}, refTime)
	assert.Equal(t, "Unknown", p.Status)
	assert.Equal(t, "x", p.Since)
	assert.Empty(t, p.Conditions)
}

func podWith(phase corev1.PodPhase, ready *bool) *corev1.Pod {
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns", CreationTimestamp: metav1.NewTime(refTime.Add(-10 * time.Minute))},
		Spec:       corev1.PodSpec{NodeName: "node-1", Containers: []corev1.Container{{Name: "app"}}},
		Status:     corev1.PodStatus{Phase: phase},
	}
	if ready != nil {
		s := corev1.ConditionFalse
		if *ready {
			s = corev1.ConditionTrue
		}
		p.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: s}}
	}
	return p
}

func TestIsPodProblem(t *testing.T) {
	yes, no := true, false
	tests := []struct {
		name string
		pod  *corev1.Pod
		want bool
	}{
		{"running and ready", podWith(corev1.PodRunning, &yes), false},
		{"running not ready", podWith(corev1.PodRunning, &no), true},
		{"running without ready condition", podWith(corev1.PodRunning, nil), true},
		{"succeeded (Completed)", podWith(corev1.PodSucceeded, &no), false},
		{"pending", podWith(corev1.PodPending, nil), true},
		{"failed", podWith(corev1.PodFailed, &no), true},
		{"unknown", podWith(corev1.PodUnknown, nil), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsPodProblem(tt.pod))
		})
	}
}

func TestPodStatusReason(t *testing.T) {
	always := corev1.ContainerRestartPolicyAlways
	started := true

	tests := []struct {
		name   string
		mutate func(p *corev1.Pod)
		want   string
	}{
		{
			name:   "plain pending",
			mutate: func(p *corev1.Pod) { p.Status.Phase = corev1.PodPending },
			want:   "Pending",
		},
		{
			name: "pod-level reason (Evicted)",
			mutate: func(p *corev1.Pod) {
				p.Status.Phase = corev1.PodFailed
				p.Status.Reason = "Evicted"
			},
			want: "Evicted",
		},
		{
			name: "container waiting CrashLoopBackOff",
			mutate: func(p *corev1.Pod) {
				p.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "app",
					State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}}}
			},
			want: "CrashLoopBackOff",
		},
		{
			name: "container terminated with reason",
			mutate: func(p *corev1.Pod) {
				p.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "app",
					State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "OOMKilled", ExitCode: 137}}}}
			},
			want: "OOMKilled",
		},
		{
			name: "container terminated by signal",
			mutate: func(p *corev1.Pod) {
				p.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "app",
					State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Signal: 9}}}}
			},
			want: "Signal:9",
		},
		{
			name: "container terminated with exit code",
			mutate: func(p *corev1.Pod) {
				p.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "app",
					State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 2}}}}
			},
			want: "ExitCode:2",
		},
		{
			name: "init container image pull error",
			mutate: func(p *corev1.Pod) {
				p.Status.Phase = corev1.PodPending
				p.Spec.InitContainers = []corev1.Container{{Name: "init"}}
				p.Status.InitContainerStatuses = []corev1.ContainerStatus{{Name: "init",
					State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"}}}}
			},
			want: "Init:ImagePullBackOff",
		},
		{
			name: "init container failed",
			mutate: func(p *corev1.Pod) {
				p.Spec.InitContainers = []corev1.Container{{Name: "init"}}
				p.Status.InitContainerStatuses = []corev1.ContainerStatus{{Name: "init",
					State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 1}}}}
			},
			want: "Init:ExitCode:1",
		},
		{
			name: "second init container still running",
			mutate: func(p *corev1.Pod) {
				p.Spec.InitContainers = []corev1.Container{{Name: "a"}, {Name: "b"}}
				p.Status.InitContainerStatuses = []corev1.ContainerStatus{
					{Name: "a", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}},
					{Name: "b", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
				}
			},
			want: "Init:1/2",
		},
		{
			name: "started sidecar init container is skipped",
			mutate: func(p *corev1.Pod) {
				p.Status.Phase = corev1.PodRunning
				p.Spec.InitContainers = []corev1.Container{{Name: "sidecar", RestartPolicy: &always}}
				p.Status.InitContainerStatuses = []corev1.ContainerStatus{{Name: "sidecar", Started: &started,
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}
				p.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "app",
					State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CreateContainerConfigError"}}}}
			},
			want: "CreateContainerConfigError",
		},
		{
			name: "completed container with another running one is NotReady",
			mutate: func(p *corev1.Pod) {
				p.Status.Phase = corev1.PodRunning
				p.Status.ContainerStatuses = []corev1.ContainerStatus{
					{Name: "a", Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
					{Name: "b", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "Completed"}}},
				}
			},
			want: "NotReady",
		},
		{
			name: "terminating",
			mutate: func(p *corev1.Pod) {
				ts := metav1.NewTime(refTime)
				p.DeletionTimestamp = &ts
			},
			want: "Terminating",
		},
		{
			name: "node lost",
			mutate: func(p *corev1.Pod) {
				ts := metav1.NewTime(refTime)
				p.DeletionTimestamp = &ts
				p.Status.Reason = "NodeLost"
			},
			want: "Unknown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := podWith(corev1.PodRunning, nil)
			tt.mutate(p)
			assert.Equal(t, tt.want, PodStatusReason(p))
		})
	}
}

func TestPodProblemFor(t *testing.T) {
	p := podWith(corev1.PodRunning, nil)
	p.Spec.Containers = []corev1.Container{{Name: "a"}, {Name: "b"}}
	p.Status.InitContainerStatuses = []corev1.ContainerStatus{{Name: "init", RestartCount: 1,
		State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}}}
	p.Status.ContainerStatuses = []corev1.ContainerStatus{
		{Name: "a", Ready: true, RestartCount: 2, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
		{Name: "b", RestartCount: 5, State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}},
	}

	got := PodProblemFor(p, refTime)
	assert.Equal(t, PodProblem{
		Namespace: "ns", Name: "p", Ready: "1/2", Status: "CrashLoopBackOff",
		Restarts: 8, Node: "node-1", Age: "10m",
	}, got)

	p.Spec.NodeName = ""
	assert.Equal(t, "<none>", PodProblemFor(p, refTime).Node)
}
