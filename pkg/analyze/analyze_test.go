package analyze

import (
	"testing"

	"github.com/kubectl-k8i/pkg/model"
)

func TestOvercommitPct(t *testing.T) {
	tests := []struct {
		name    string
		request float64
		limit   float64
		want    float64
	}{
		{"limit double request", 1.0, 2.0, 100},
		{"limit equals request", 1.0, 1.0, 0},
		{"limit 50% over", 2.0, 3.0, 50},
		{"limit below request", 2.0, 1.0, -50},
		{"zero request", 0, 2.0, -1},
		{"both zero", 0, 0, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := overcommitPct(tt.request, tt.limit)
			if got != tt.want {
				t.Errorf("overcommitPct(%v, %v) = %v, want %v", tt.request, tt.limit, got, tt.want)
			}
		})
	}
}

func TestPassesOvercommitThresholds(t *testing.T) {
	tests := []struct {
		name string
		wl   model.WorkloadInfo
		cfg  model.AnalyzeConfig
		want bool
	}{
		{
			name: "no thresholds set passes",
			wl:   model.WorkloadInfo{CPUOvercommitPct: 10, MemOvercommitPct: 10},
			cfg:  model.AnalyzeConfig{},
			want: true,
		},
		{
			name: "cpu above threshold passes",
			wl:   model.WorkloadInfo{CPUOvercommitPct: 120},
			cfg:  model.AnalyzeConfig{CPUOvercommit: 100, CPUOvercommitSet: true},
			want: true,
		},
		{
			name: "cpu below threshold fails",
			wl:   model.WorkloadInfo{CPUOvercommitPct: 80},
			cfg:  model.AnalyzeConfig{CPUOvercommit: 100, CPUOvercommitSet: true},
			want: false,
		},
		{
			name: "cpu n/a fails when threshold set",
			wl:   model.WorkloadInfo{CPUOvercommitPct: -1},
			cfg:  model.AnalyzeConfig{CPUOvercommit: 0, CPUOvercommitSet: true},
			want: false,
		},
		{
			name: "both thresholds: only one met fails",
			wl:   model.WorkloadInfo{CPUOvercommitPct: 120, MemOvercommitPct: 10},
			cfg:  model.AnalyzeConfig{CPUOvercommit: 100, CPUOvercommitSet: true, MemOvercommit: 50, MemOvercommitSet: true},
			want: false,
		},
		{
			name: "both thresholds: both met passes",
			wl:   model.WorkloadInfo{CPUOvercommitPct: 120, MemOvercommitPct: 60},
			cfg:  model.AnalyzeConfig{CPUOvercommit: 100, CPUOvercommitSet: true, MemOvercommit: 50, MemOvercommitSet: true},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := passesOvercommitThresholds(tt.wl, tt.cfg)
			if got != tt.want {
				t.Errorf("passesOvercommitThresholds() = %v, want %v", got, tt.want)
			}
		})
	}
}
