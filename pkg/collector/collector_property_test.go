package collector

import (
	"testing"
	"time"

	"github.com/kubectl-k8i/pkg/model"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"pgregory.net/rapid"
)

// Feature: kubectl-k8i-plugin, Property 12: Only Ready nodes processed
// **Validates: Requirements 6.6**
func TestOnlyReadyNodes(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random list of nodes with mixed Ready/NotReady conditions.
		numNodes := rapid.IntRange(0, 20).Draw(t, "numNodes")
		var nodes []corev1.Node
		expectedReadyCount := 0
		usedNames := make(map[string]bool)

		for i := 0; i < numNodes; i++ {
			isReady := rapid.Bool().Draw(t, "ready")
			if isReady {
				expectedReadyCount++
			}

			condStatus := corev1.ConditionFalse
			if isReady {
				condStatus = corev1.ConditionTrue
			}

			// Generate unique name to avoid duplicate node name collisions
			var name string
			for {
				name = rapid.StringMatching(`node-[a-z0-9]{3,8}`).Draw(t, "name")
				if !usedNames[name] {
					usedNames[name] = true
					break
				}
			}
			node := corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:              name,
					CreationTimestamp: metav1.NewTime(time.Now().Add(-1 * time.Hour)),
				},
				Spec: corev1.NodeSpec{},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: condStatus},
					},
					Capacity: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("4"),
						corev1.ResourceMemory: resource.MustParse("16Gi"),
					},
					Allocatable: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("4"),
						corev1.ResourceMemory: resource.MustParse("16Gi"),
						corev1.ResourcePods:   resource.MustParse("110"),
					},
				},
			}
			nodes = append(nodes, node)
		}

		data := &model.ClusterData{Nodes: nodes}
		c := NewCollector(nil, nil, nil, "", nil, testRetryConfig(), nil)
		result := c.EnrichNodes(data, time.Now())

		// Verify: output count matches expected Ready count.
		assert.Equal(t, expectedReadyCount, len(result),
			"output should contain exactly the Ready nodes")

		// Verify: every node in the output corresponds to a Ready node in the input.
		for _, enriched := range result {
			found := false
			for _, orig := range nodes {
				if orig.Name == enriched.Name {
					for _, cond := range orig.Status.Conditions {
						if cond.Type == corev1.NodeReady {
							assert.Equal(t, corev1.ConditionTrue, cond.Status,
								"node %s in output must have Ready=True", enriched.Name)
							found = true
						}
					}
				}
			}
			assert.True(t, found, "node %s in output must exist in input", enriched.Name)
		}
	})
}
