package analyze

import (
	"encoding/json"
	"io"

	"github.com/kubectl-k8i/pkg/model"
	"gopkg.in/yaml.v3"
)

// WorkloadOutput is the JSON/YAML-serializable representation of a workload.
type WorkloadOutput struct {
	Namespace       string  `json:"namespace"         yaml:"namespace"`
	Kind            string  `json:"kind"              yaml:"kind"`
	Name            string  `json:"name"              yaml:"name"`
	PodCount        int     `json:"pod_count"         yaml:"pod_count"`
	CPURequestCores float64 `json:"cpu_request_cores" yaml:"cpu_request_cores"`
	CPULimitCores   float64 `json:"cpu_limit_cores"   yaml:"cpu_limit_cores"`
	CPUUsageCores   float64 `json:"cpu_usage_cores"   yaml:"cpu_usage_cores"`
	MemRequestGB    float64 `json:"mem_request_gb"    yaml:"mem_request_gb"`
	MemLimitGB      float64 `json:"mem_limit_gb"      yaml:"mem_limit_gb"`
	MemUsageGB      float64 `json:"mem_usage_gb"      yaml:"mem_usage_gb"`
}

// toOutput converts a WorkloadInfo to WorkloadOutput.
func toOutput(wl model.WorkloadInfo) WorkloadOutput {
	return WorkloadOutput{
		Namespace:       wl.Namespace,
		Kind:            wl.Kind,
		Name:            wl.Name,
		PodCount:        wl.PodCount,
		CPURequestCores: wl.CPURequestCores,
		CPULimitCores:   wl.CPULimitCores,
		CPUUsageCores:   wl.CPUUsageCores,
		MemRequestGB:    wl.MemRequestGB,
		MemLimitGB:      wl.MemLimitGB,
		MemUsageGB:      wl.MemUsageGB,
	}
}

// toOutputList converts a slice of WorkloadInfo to a slice of WorkloadOutput.
func toOutputList(workloads []model.WorkloadInfo) []WorkloadOutput {
	out := make([]WorkloadOutput, len(workloads))
	for i, wl := range workloads {
		out[i] = toOutput(wl)
	}
	return out
}

// WriteJSON writes workloads as an indented JSON array.
func WriteJSON(w io.Writer, workloads []model.WorkloadInfo) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(toOutputList(workloads))
}

// WriteYAML writes workloads as a YAML list.
func WriteYAML(w io.Writer, workloads []model.WorkloadInfo) error {
	enc := yaml.NewEncoder(w)
	if err := enc.Encode(toOutputList(workloads)); err != nil {
		_ = enc.Close()
		return err
	}
	return enc.Close()
}
