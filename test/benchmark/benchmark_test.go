package benchmark

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/kubectl-k8i/pkg/collector"
	"github.com/kubectl-k8i/pkg/color"
	"github.com/kubectl-k8i/pkg/filter"
	"github.com/kubectl-k8i/pkg/model"
	"github.com/kubectl-k8i/pkg/render"
	"github.com/kubectl-k8i/pkg/retry"
	sortpkg "github.com/kubectl-k8i/pkg/sort"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

var refTime = time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

func testRetryConfig() retry.RetryConfig {
	return retry.RetryConfig{MaxRetries: 0, InitialBackoff: 0, MaxBackoff: 0, JitterFraction: 0}
}

// generateFakeClusterData creates a ClusterData with the specified number of
// nodes and pods per node. Each node gets realistic labels, capacity, and
// metrics. Pods have varied resource requests/limits.
func generateFakeClusterData(numNodes, podsPerNode int) *model.ClusterData {
	zones := []string{"us-east-1a", "us-east-1b", "us-east-1c", "us-west-2a"}
	instanceTypes := []string{"m5.xlarge", "c6g.2xlarge", "r5.large", "t3.medium", "m6i.4xlarge"}
	capacityTypes := []string{"spot", "on-demand"}
	archs := []string{"amd64", "arm64"}
	pools := []string{"default", "gpu-pool", "compute", "memory-opt"}

	nodes := make([]corev1.Node, numNodes)
	var pods []corev1.Pod
	metrics := make([]metricsv1beta1.NodeMetrics, numNodes)

	for i := 0; i < numNodes; i++ {
		nodeName := fmt.Sprintf("node-%04d", i)
		zone := zones[i%len(zones)]
		instType := instanceTypes[i%len(instanceTypes)]
		capType := capacityTypes[i%len(capacityTypes)]
		arch := archs[i%len(archs)]
		pool := pools[i%len(pools)]

		labels := map[string]string{
			"karpenter.sh/capacity-type":       capType,
			"karpenter.sh/nodepool":            pool,
			"kubernetes.io/arch":               arch,
			"node.kubernetes.io/instance-type": instType,
			"topology.kubernetes.io/zone":      zone,
		}

		var taints []corev1.Taint
		if i%5 == 0 {
			taints = []corev1.Taint{
				{Key: "dedicated", Value: "gpu", Effect: corev1.TaintEffectNoSchedule},
			}
		}

		nodes[i] = corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:              nodeName,
				Labels:            labels,
				CreationTimestamp: metav1.NewTime(refTime.Add(-time.Duration(i) * time.Hour)),
			},
			Spec: corev1.NodeSpec{
				ProviderID: fmt.Sprintf("aws:///%s/i-%016x", zone, i),
				Taints:     taints,
			},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
				},
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("16"),
					corev1.ResourceMemory: resource.MustParse("64Gi"),
				},
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("16"),
					corev1.ResourceMemory: resource.MustParse("64Gi"),
					corev1.ResourcePods:   resource.MustParse("110"),
				},
			},
		}

		// Create pods for this node
		for j := 0; j < podsPerNode; j++ {
			podName := fmt.Sprintf("pod-%04d-%02d", i, j)
			cpuReq := fmt.Sprintf("%dm", 100+j*50)
			cpuLim := fmt.Sprintf("%dm", 200+j*50)
			memReq := fmt.Sprintf("%dMi", 128+j*64)
			memLim := fmt.Sprintf("%dMi", 256+j*64)

			pods = append(pods, corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: "default"},
				Spec: corev1.PodSpec{
					NodeName: nodeName,
					Containers: []corev1.Container{
						{
							Name: "main",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(cpuReq),
									corev1.ResourceMemory: resource.MustParse(memReq),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(cpuLim),
									corev1.ResourceMemory: resource.MustParse(memLim),
								},
							},
						},
					},
				},
			})
		}

		// Metrics for this node
		cpuUsage := fmt.Sprintf("%d", 4+i%12)
		memUsage := fmt.Sprintf("%dGi", 16+i%48)
		metrics[i] = metricsv1beta1.NodeMetrics{
			ObjectMeta: metav1.ObjectMeta{Name: nodeName},
			Usage: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(cpuUsage),
				corev1.ResourceMemory: resource.MustParse(memUsage),
			},
		}
	}

	return &model.ClusterData{
		Nodes:   nodes,
		Metrics: metrics,
		Pods:    pods,
	}
}

// enrichNodes enriches cluster data into NodeInfo slice.
func enrichNodes(data *model.ClusterData) []model.NodeInfo {
	c := collector.NewCollector(nil, nil, nil, "", nil, testRetryConfig(), nil)
	return c.EnrichNodes(data, refTime)
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

// BenchmarkProcess1000Nodes measures the full pipeline: enrich + filter + sort + render
// for 1000 nodes with 10 pods each (10000 pods total).
func BenchmarkProcess1000Nodes(b *testing.B) {
	data := generateFakeClusterData(1000, 10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Enrich
		nodes := enrichNodes(data)

		// Filter (hide fargate — none in this dataset, but exercises the path)
		nodes = filter.HideFargateNodes(nodes)

		// Sort by pool ascending (default)
		_ = sortpkg.SortNodes(nodes, "pool", "asc")

		// Render to buffer
		var buf bytes.Buffer
		render.RenderTable(&buf, nodes, render.RenderConfig{
			Color:     color.ColorConfig{Enabled: false},
			Timestamp: refTime,
			TermWidth: 200,
		})
	}
}

// BenchmarkResourceAggregation measures per-node resource aggregation with map lookups.
// This isolates the enrichment step which builds podsByNode, metricsByNode, and
// nodeclaimByNode maps.
func BenchmarkResourceAggregation(b *testing.B) {
	data := generateFakeClusterData(1000, 10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = enrichNodes(data)
	}
}

// BenchmarkFilterAndSort measures filter + sort on 1000 enriched nodes.
func BenchmarkFilterAndSort(b *testing.B) {
	data := generateFakeClusterData(1000, 10)
	nodes := enrichNodes(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Make a copy to avoid sorting an already-sorted slice
		nodesCopy := make([]model.NodeInfo, len(nodes))
		copy(nodesCopy, nodes)

		// Filter by architecture
		filtered, _ := filter.FilterNodes(nodesCopy, "arch", "amd64")

		// Sort by CPU load descending
		_ = sortpkg.SortNodes(filtered, "cpu_load", "desc")
	}
}

// BenchmarkTableRender measures table rendering for 1000 rows.
func BenchmarkTableRender(b *testing.B) {
	data := generateFakeClusterData(1000, 10)
	nodes := enrichNodes(data)
	_ = sortpkg.SortNodes(nodes, "pool", "asc")

	cfg := render.RenderConfig{
		Color:     color.ColorConfig{Enabled: false},
		Timestamp: refTime,
		TermWidth: 200,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		render.RenderTable(&buf, nodes, cfg)
	}
}
