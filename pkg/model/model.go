package model

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

// NodeInfo holds all processed information for a single node.
type NodeInfo struct {
	// Identity
	Name string

	// Pod usage
	PodsUsed int
	PodsMax  int

	// CPU (all in cores as float64, millicores as int64 for load calc)
	CPURequestCores  float64
	CPULimitCores    float64
	CPUUsageCores    float64
	CPUCapacityCores float64
	CPURequestMilli  int64
	CPULimitMilli    int64
	CPUUsageMilli    int64
	CPUCapacityMilli int64
	CPULoadPercent   int // (usage_milli * 100) / capacity_milli

	// Memory (all in GB as float64)
	MemRequestGB   float64
	MemLimitGB     float64
	MemUsageGB     float64
	MemCapacityGB  float64
	MemLoadPercent int // (usage_gb * 100) / capacity_gb

	// Metadata from labels
	EC2InstanceID string
	InstanceType  string
	CapacityType  string // "spot", "od", "x"
	Architecture  string // "amd64", "arm64"
	Zone          string // last 2 chars, e.g. "1a"
	Nodepool      string
	Nodeclaim     string

	// Autoscaler type
	Autoscaler string // "karpenter", "cluster-autoscaler", "spotio", "x"

	// Age
	CreationTime time.Time
	Age          string // formatted: "5d12h", "3h45m", "12m"

	// Taints
	Taints       []corev1.Taint
	TaintStr     string // formatted for display
	TaintSortKey string // for sorting
}

// RunConfig holds all parsed CLI options.
type RunConfig struct {
	Context     string // --context
	Labels      string // --labels
	Taints      string // --taints (taint key or key=value filter)
	Filter      string // --filter (attribute=value)
	Sort        string // --sort (column=direction)
	Fargate     bool   // --fargate
	Color       *bool  // --color (nil = auto-detect)
	Debug       bool   // --debug
	GroupBy     string // --group-by (currently only "taint")
	Output      string // --output / -o (table, json, yaml; default: table)
	NoHeaders   bool   // --no-headers: suppress header, separator, timestamp, annotations
	Deployment  string // --deployment namespace/name: show only nodes running pods of this deployment
	StatefulSet string // --statefulset namespace/name: show only nodes running pods of this statefulset
	Namespace   string // --namespace name: show only nodes running pods from this namespace
	DaemonSet   string // --daemonset namespace/name: show only nodes running pods of this daemonset
	Autoscaler  string // --autoscaler value: show only nodes managed by this autoscaler (karpenter, cas, spotio, x)
}

// WorkloadInfo holds aggregated resource information for a single workload owner.
type WorkloadInfo struct {
	Namespace string // kubernetes namespace
	Kind      string // Deployment, StatefulSet, DaemonSet, Pod
	Name      string // workload name

	PodCount int // number of running pods on the selected nodes

	CPURequestCores float64
	CPULimitCores   float64
	CPUUsageCores   float64

	MemRequestGB float64
	MemLimitGB   float64
	MemUsageGB   float64
}

// AnalyzeConfig holds all parsed CLI options for the analyze subcommand.
type AnalyzeConfig struct {
	Context           string   // --context
	NodeName          string   // --node
	Labels            string   // --labels
	Taints            string   // --taints
	Autoscaler        string   // --autoscaler (karpenter, cas, spotio, x)
	ExcludeNamespaces []string // --exclude-namespace (repeatable)
	Output            string   // --output / -o
	Color             *bool    // --color
	Debug             bool     // --debug
}

// PodAggregation holds per-node pod resource totals.
type PodAggregation struct {
	PodCount        int
	CPURequestMilli int64
	CPULimitMilli   int64
	MemRequestGB    float64
	MemLimitGB      float64
}

// ClusterData holds all raw data collected from the API.
type ClusterData struct {
	Nodes      []corev1.Node
	Metrics    []metricsv1beta1.NodeMetrics
	Pods       []corev1.Pod
	Nodeclaims []unstructured.Unstructured
}
