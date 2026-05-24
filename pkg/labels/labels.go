package labels

import (
	"regexp"
	"strings"
)

// NodeMetadata holds extracted label values for a single node.
type NodeMetadata struct {
	EC2InstanceID string
	InstanceType  string
	CapacityType  string // normalized: "spot", "od", "x"
	Architecture  string
	Zone          string // last 2 chars only
	Nodepool      string // truncated to 15 chars for EKS nodegroups
	Nodeclaim     string // truncated to 20 chars
	Autoscaler    string // "karpenter", "cluster-autoscaler", "spotio", "x"
}

// Label keys used for capacity type detection, in priority order.
var capacityTypeKeys = []string{
	"karpenter.sh/capacity-type",
	"karpenter.k8s.aws/capacity-type",
	"spotinst.io/node-lifecycle",
	"eks.amazonaws.com/capacityType",
}

// Label keys used for nodepool detection, in priority order.
var nodepoolKeys = []string{
	"karpenter.sh/nodepool",
	"karpenter.k8s.aws/nodepool",
	"spotinst.io/ocean-vng-id",
	"eks.amazonaws.com/nodegroup",
}

// ec2IDRegex matches EC2 instance IDs in providerID strings.
var ec2IDRegex = regexp.MustCompile(`i-[A-Za-z0-9-]+`)

// ExtractMetadata extracts all metadata from a node's labels and providerID.
func ExtractMetadata(labels map[string]string, providerID string) NodeMetadata {
	return NodeMetadata{
		EC2InstanceID: ExtractEC2ID(providerID),
		InstanceType:  extractLabel(labels, "node.kubernetes.io/instance-type"),
		CapacityType:  ExtractCapacityType(labels),
		Architecture:  extractLabel(labels, "kubernetes.io/arch"),
		Zone:          extractZone(labels),
		Nodepool:      ExtractNodepool(labels),
		Nodeclaim:     extractNodeclaim(labels),
		Autoscaler:    DetectAutoscaler(labels),
	}
}

// ExtractCapacityType checks the priority chain and normalizes the value.
// Priority: karpenter.sh/capacity-type > karpenter.k8s.aws/capacity-type >
// spotinst.io/node-lifecycle > eks.amazonaws.com/capacityType
func ExtractCapacityType(labels map[string]string) string {
	for _, key := range capacityTypeKeys {
		if val, ok := labels[key]; ok && val != "" {
			return normalizeCapacityType(val)
		}
	}
	return "x"
}

// ExtractNodepool checks the priority chain with EKS truncation.
// Priority: karpenter.sh/nodepool > karpenter.k8s.aws/nodepool >
// spotinst.io/ocean-vng-id > eks.amazonaws.com/nodegroup
func ExtractNodepool(labels map[string]string) string {
	for _, key := range nodepoolKeys {
		if val, ok := labels[key]; ok && val != "" {
			// EKS nodegroup values get truncated to 15 chars
			if key == "eks.amazonaws.com/nodegroup" {
				return truncate(val, 15)
			}
			return val
		}
	}
	return "x"
}

// ExtractEC2ID extracts the EC2 instance ID from a providerID string
// using the regex pattern i-[A-Za-z0-9-]+.
func ExtractEC2ID(providerID string) string {
	match := ec2IDRegex.FindString(providerID)
	if match == "" {
		return "x"
	}
	return match
}

// DetectAutoscaler determines the autoscaler type from node labels.
// Priority: karpenter > spotio > cas > x
func DetectAutoscaler(labels map[string]string) string {
	// Karpenter: karpenter.sh/nodepool OR karpenter.k8s.aws/nodepool
	if hasLabel(labels, "karpenter.sh/nodepool") || hasLabel(labels, "karpenter.k8s.aws/nodepool") {
		return "karpenter"
	}
	// Spot.io: spotinst.io/ocean-vng-id OR spotinst.io/node-lifecycle
	if hasLabel(labels, "spotinst.io/ocean-vng-id") || hasLabel(labels, "spotinst.io/node-lifecycle") {
		return "spotio"
	}
	// CAS: eks.amazonaws.com/nodegroup
	if hasLabel(labels, "eks.amazonaws.com/nodegroup") {
		return "cluster-autoscaler"
	}
	return "x"
}

// normalizeCapacityType normalizes on-demand variants to "od".
// Matches case-insensitive: "on-demand", "ondemand", "ON_DEMAND", "On-Demand", etc.
func normalizeCapacityType(val string) string {
	lower := strings.ToLower(val)
	// Remove hyphens and underscores for comparison
	normalized := strings.NewReplacer("-", "", "_", "").Replace(lower)
	if normalized == "ondemand" {
		return "od"
	}
	return val
}

// extractZone extracts the last 2 characters from the zone label.
func extractZone(labels map[string]string) string {
	zone := extractLabel(labels, "topology.kubernetes.io/zone")
	if zone == "x" || len(zone) < 2 {
		return zone
	}
	return zone[len(zone)-2:]
}

// extractNodeclaim extracts the nodeclaim name from Karpenter labels,
// truncated to 20 characters.
func extractNodeclaim(labels map[string]string) string {
	val := extractLabel(labels, "karpenter.sh/nodeclaim")
	if val == "x" {
		return val
	}
	return truncate(val, 20)
}

// extractLabel returns the value for a single label key, or "x" if missing/empty.
func extractLabel(labels map[string]string, key string) string {
	if val, ok := labels[key]; ok && val != "" {
		return val
	}
	return "x"
}

// hasLabel returns true if the label key exists and has a non-empty value.
func hasLabel(labels map[string]string, key string) bool {
	val, ok := labels[key]
	return ok && val != ""
}

// truncate returns s truncated to maxLen characters. If s is already
// within the limit, it is returned unchanged.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
