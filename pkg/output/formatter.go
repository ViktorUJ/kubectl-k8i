package output

import (
	"fmt"
	"io"

	"github.com/kubectl-k8i/pkg/model"
)

// OutputFormat represents the output format type.
type OutputFormat string

const (
	FormatTable OutputFormat = "table"
	FormatJSON  OutputFormat = "json"
	FormatYAML  OutputFormat = "yaml"
)

// Formatter is the interface for output formatters.
type Formatter interface {
	// Format writes the node data to the given writer.
	Format(w io.Writer, nodes []model.NodeInfo) error
}

// NewFormatter creates a Formatter for the given output format.
// Returns an error for unsupported formats.
func NewFormatter(format OutputFormat) (Formatter, error) {
	switch format {
	case FormatJSON:
		return &JSONFormatter{}, nil
	case FormatYAML:
		return &YAMLFormatter{}, nil
	default:
		return nil, fmt.Errorf("unsupported output format: %q (supported: json, yaml)", format)
	}
}

// NodeOutput is the JSON/YAML-serializable representation of a node.
// Uses json and yaml struct tags for field naming.
type NodeOutput struct {
	Name             string  `json:"name" yaml:"name"`
	PodsUsed         int     `json:"pods_used" yaml:"pods_used"`
	PodsMax          int     `json:"pods_max" yaml:"pods_max"`
	CPURequestCores  float64 `json:"cpu_request_cores" yaml:"cpu_request_cores"`
	CPULimitCores    float64 `json:"cpu_limit_cores" yaml:"cpu_limit_cores"`
	CPUUsageCores    float64 `json:"cpu_usage_cores" yaml:"cpu_usage_cores"`
	CPUCapacityCores float64 `json:"cpu_capacity_cores" yaml:"cpu_capacity_cores"`
	CPULoadPercent   int     `json:"cpu_load_percent" yaml:"cpu_load_percent"`
	MemRequestGB     float64 `json:"mem_request_gb" yaml:"mem_request_gb"`
	MemLimitGB       float64 `json:"mem_limit_gb" yaml:"mem_limit_gb"`
	MemUsageGB       float64 `json:"mem_usage_gb" yaml:"mem_usage_gb"`
	MemCapacityGB    float64 `json:"mem_capacity_gb" yaml:"mem_capacity_gb"`
	MemLoadPercent   int     `json:"mem_load_percent" yaml:"mem_load_percent"`
	EC2InstanceID    string  `json:"ec2_instance_id" yaml:"ec2_instance_id"`
	InstanceType     string  `json:"instance_type" yaml:"instance_type"`
	CapacityType     string  `json:"capacity_type" yaml:"capacity_type"`
	Architecture     string  `json:"architecture" yaml:"architecture"`
	Zone             string  `json:"zone" yaml:"zone"`
	Nodepool         string  `json:"nodepool" yaml:"nodepool"`
	Nodeclaim        string  `json:"nodeclaim" yaml:"nodeclaim"`
	Autoscaler       string  `json:"autoscaler" yaml:"autoscaler"`
	Age              string  `json:"age" yaml:"age"`
	Taints           string  `json:"taints" yaml:"taints"`
}

// ToNodeOutput converts a model.NodeInfo to a NodeOutput for serialization.
func ToNodeOutput(n model.NodeInfo) NodeOutput {
	return NodeOutput{
		Name:             n.Name,
		PodsUsed:         n.PodsUsed,
		PodsMax:          n.PodsMax,
		CPURequestCores:  n.CPURequestCores,
		CPULimitCores:    n.CPULimitCores,
		CPUUsageCores:    n.CPUUsageCores,
		CPUCapacityCores: n.CPUCapacityCores,
		CPULoadPercent:   n.CPULoadPercent,
		MemRequestGB:     n.MemRequestGB,
		MemLimitGB:       n.MemLimitGB,
		MemUsageGB:       n.MemUsageGB,
		MemCapacityGB:    n.MemCapacityGB,
		MemLoadPercent:   n.MemLoadPercent,
		EC2InstanceID:    n.EC2InstanceID,
		InstanceType:     n.InstanceType,
		CapacityType:     n.CapacityType,
		Architecture:     n.Architecture,
		Zone:             n.Zone,
		Nodepool:         n.Nodepool,
		Nodeclaim:        n.Nodeclaim,
		Autoscaler:       n.Autoscaler,
		Age:              n.Age,
		Taints:           n.TaintStr,
	}
}

// ToNodeOutputList converts a slice of model.NodeInfo to a slice of NodeOutput.
func ToNodeOutputList(nodes []model.NodeInfo) []NodeOutput {
	out := make([]NodeOutput, len(nodes))
	for i, n := range nodes {
		out[i] = ToNodeOutput(n)
	}
	return out
}
