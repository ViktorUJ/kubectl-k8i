package watch

import (
	"fmt"
	"strings"
	"time"

	"github.com/kubectl-k8i/pkg/age"
	corev1 "k8s.io/api/core/v1"
)

// NodeProblem describes a node whose Ready condition is not True.
type NodeProblem struct {
	Name       string   `json:"name"`
	Status     string   `json:"status"`     // NotReady, Unknown; ",SchedulingDisabled" appended when cordoned
	Since      string   `json:"since"`      // time since the Ready condition last changed
	Conditions []string `json:"conditions"` // active pressure conditions (MemoryPressure, DiskPressure, ...)
	Reason     string   `json:"reason"`
	Message    string   `json:"message"`
}

// PodProblem describes a pod that is neither Ready nor Completed.
type PodProblem struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Ready     string `json:"ready"`  // ready/total containers
	Status    string `json:"status"` // kubectl-style status (CrashLoopBackOff, Pending, Init:0/1, ...)
	Restarts  int32  `json:"restarts"`
	Node      string `json:"node"`
	Age       string `json:"age"`
}

// pressureConditions are node conditions that signal a problem when True.
var pressureConditions = []corev1.NodeConditionType{
	corev1.NodeMemoryPressure,
	corev1.NodeDiskPressure,
	corev1.NodePIDPressure,
	corev1.NodeNetworkUnavailable,
}

// IsNodeReady reports whether the node's Ready condition is True.
// A node without a Ready condition is treated as not ready.
func IsNodeReady(node *corev1.Node) bool {
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

// NodeProblemFor builds a NodeProblem for a not-ready node.
func NodeProblemFor(node *corev1.Node, now time.Time) NodeProblem {
	p := NodeProblem{Name: node.Name, Status: "Unknown", Since: "x"}

	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			if c.Status == corev1.ConditionFalse {
				p.Status = "NotReady"
			}
			p.Reason = c.Reason
			p.Message = c.Message
			if !c.LastTransitionTime.IsZero() {
				p.Since = age.FormatAge(c.LastTransitionTime.Time, now)
			}
		}
	}
	if node.Spec.Unschedulable {
		p.Status += ",SchedulingDisabled"
	}

	for _, want := range pressureConditions {
		for _, c := range node.Status.Conditions {
			if c.Type == want && c.Status == corev1.ConditionTrue {
				p.Conditions = append(p.Conditions, string(c.Type))
			}
		}
	}
	return p
}

// IsPodProblem reports whether the pod is neither Ready nor Completed.
// Succeeded pods (Completed) are never a problem. Running pods are a problem
// only when their Ready condition is not True. Pending, Failed and Unknown
// pods are always a problem.
func IsPodProblem(pod *corev1.Pod) bool {
	switch pod.Status.Phase {
	case corev1.PodSucceeded:
		return false
	case corev1.PodRunning:
		return !isPodReady(pod)
	default:
		return true
	}
}

func isPodReady(pod *corev1.Pod) bool {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

// PodProblemFor builds a PodProblem for a pod.
func PodProblemFor(pod *corev1.Pod, now time.Time) PodProblem {
	ready := 0
	var restarts int32
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Ready {
			ready++
		}
		restarts += cs.RestartCount
	}
	for _, cs := range pod.Status.InitContainerStatuses {
		restarts += cs.RestartCount
	}

	node := pod.Spec.NodeName
	if node == "" {
		node = "<none>"
	}

	return PodProblem{
		Namespace: pod.Namespace,
		Name:      pod.Name,
		Ready:     fmt.Sprintf("%d/%d", ready, len(pod.Spec.Containers)),
		Status:    PodStatusReason(pod),
		Restarts:  restarts,
		Node:      node,
		Age:       age.FormatAge(pod.CreationTimestamp.Time, now),
	}
}

// PodStatusReason returns a kubectl-style status string for a pod
// (e.g. "CrashLoopBackOff", "Init:ImagePullBackOff", "Init:0/2", "Terminating").
func PodStatusReason(pod *corev1.Pod) string {
	reason := string(pod.Status.Phase)
	if pod.Status.Reason != "" {
		reason = pod.Status.Reason
	}

	sidecars := make(map[string]bool, len(pod.Spec.InitContainers))
	for _, c := range pod.Spec.InitContainers {
		if c.RestartPolicy != nil && *c.RestartPolicy == corev1.ContainerRestartPolicyAlways {
			sidecars[c.Name] = true
		}
	}

	initializing := false
	for i, cs := range pod.Status.InitContainerStatuses {
		if r, done := initContainerReason(cs, sidecars[cs.Name], i, len(pod.Spec.InitContainers)); !done {
			reason = r
			initializing = true
			break
		}
	}

	if !initializing {
		hasRunning := false
		for i := len(pod.Status.ContainerStatuses) - 1; i >= 0; i-- {
			cs := pod.Status.ContainerStatuses[i]
			switch {
			case cs.State.Waiting != nil && cs.State.Waiting.Reason != "":
				reason = cs.State.Waiting.Reason
			case cs.State.Terminated != nil && cs.State.Terminated.Reason != "":
				reason = cs.State.Terminated.Reason
			case cs.State.Terminated != nil && cs.State.Terminated.Signal != 0:
				reason = fmt.Sprintf("Signal:%d", cs.State.Terminated.Signal)
			case cs.State.Terminated != nil:
				reason = fmt.Sprintf("ExitCode:%d", cs.State.Terminated.ExitCode)
			case cs.Ready && cs.State.Running != nil:
				hasRunning = true
			}
		}
		if reason == "Completed" && hasRunning {
			reason = "NotReady"
		}
	}

	if pod.DeletionTimestamp != nil {
		if pod.Status.Reason == "NodeLost" {
			return "Unknown"
		}
		return "Terminating"
	}
	return reason
}

// initContainerReason returns (reason, done). done=true means the init
// container finished successfully (or is a started sidecar) and the next one
// should be inspected.
func initContainerReason(cs corev1.ContainerStatus, sidecar bool, idx, total int) (string, bool) {
	switch {
	case cs.State.Terminated != nil && cs.State.Terminated.ExitCode == 0:
		return "", true
	case sidecar && cs.Started != nil && *cs.Started:
		return "", true
	case cs.State.Terminated != nil:
		if cs.State.Terminated.Reason != "" {
			return "Init:" + cs.State.Terminated.Reason, false
		}
		if cs.State.Terminated.Signal != 0 {
			return fmt.Sprintf("Init:Signal:%d", cs.State.Terminated.Signal), false
		}
		return fmt.Sprintf("Init:ExitCode:%d", cs.State.Terminated.ExitCode), false
	case cs.State.Waiting != nil && cs.State.Waiting.Reason != "" && cs.State.Waiting.Reason != "PodInitializing":
		return "Init:" + cs.State.Waiting.Reason, false
	default:
		return fmt.Sprintf("Init:%d/%d", idx, total), false
	}
}

// joinOrDash joins values with commas, returning "-" for an empty slice.
func joinOrDash(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ",")
}
