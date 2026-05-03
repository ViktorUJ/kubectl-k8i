package filter

import (
	"fmt"
	"strings"

	"github.com/kubectl-k8i/pkg/model"
	"github.com/kubectl-k8i/pkg/taints"
)

// SupportedFilterAttributes lists valid filter attributes.
var SupportedFilterAttributes = []string{
	"ec2_type", "instance_type", "arch", "zone", "pool", "nodeclaim", "taint", "autoscaler",
}

// FilterNodes applies a filter to a slice of NodeInfo.
// It returns only nodes where the specified attribute matches the given value.
// Returns an error for unsupported attributes.
func FilterNodes(nodes []model.NodeInfo, attribute, value string) ([]model.NodeInfo, error) {
	matcher, err := matcherForAttribute(attribute)
	if err != nil {
		return nil, err
	}

	var result []model.NodeInfo
	for _, node := range nodes {
		if matcher(node, value) {
			result = append(result, node)
		}
	}
	return result, nil
}

// HideFargateNodes filters out nodes whose Name starts with "fargate-".
func HideFargateNodes(nodes []model.NodeInfo) []model.NodeInfo {
	var result []model.NodeInfo
	for _, node := range nodes {
		if !strings.HasPrefix(node.Name, "fargate-") {
			result = append(result, node)
		}
	}
	return result
}

// matcherForAttribute returns a function that checks whether a node matches
// the given value for the specified attribute. Returns an error if the
// attribute is not supported.
func matcherForAttribute(attribute string) (func(model.NodeInfo, string) bool, error) {
	switch attribute {
	case "ec2_type":
		return func(n model.NodeInfo, v string) bool {
			return strings.EqualFold(n.CapacityType, v)
		}, nil
	case "instance_type":
		return func(n model.NodeInfo, v string) bool {
			return strings.EqualFold(n.InstanceType, v)
		}, nil
	case "arch":
		return func(n model.NodeInfo, v string) bool {
			return strings.EqualFold(n.Architecture, v)
		}, nil
	case "zone":
		return func(n model.NodeInfo, v string) bool {
			return strings.EqualFold(n.Zone, v)
		}, nil
	case "pool":
		return func(n model.NodeInfo, v string) bool {
			return strings.EqualFold(n.Nodepool, v)
		}, nil
	case "nodeclaim":
		return func(n model.NodeInfo, v string) bool {
			return strings.EqualFold(n.Nodeclaim, v)
		}, nil
	case "taint":
		return func(n model.NodeInfo, v string) bool {
			return taints.MatchTaintFilter(n.Taints, v)
		}, nil
	case "autoscaler":
		return func(n model.NodeInfo, v string) bool {
			return strings.EqualFold(n.Autoscaler, v)
		}, nil
	default:
		return nil, fmt.Errorf("unsupported filter attribute %q; supported attributes: %s",
			attribute, strings.Join(SupportedFilterAttributes, ", "))
	}
}

// FilterByTaints returns only nodes that have a taint matching the given filter.
// The filter format is the same as MatchTaintFilter: "KEY" or "KEY=VALUE".
func FilterByTaints(nodes []model.NodeInfo, taintFilter string) []model.NodeInfo {
	var result []model.NodeInfo
	for _, node := range nodes {
		if taints.MatchTaintFilter(node.Taints, taintFilter) {
			result = append(result, node)
		}
	}
	return result
}
