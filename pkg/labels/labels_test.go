package labels

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- Capacity Type Priority Chain ---

func TestExtractCapacityType_KarpenterSH(t *testing.T) {
	labels := map[string]string{
		"karpenter.sh/capacity-type": "spot",
	}
	assert.Equal(t, "spot", ExtractCapacityType(labels))
}

func TestExtractCapacityType_KarpenterAWS(t *testing.T) {
	labels := map[string]string{
		"karpenter.k8s.aws/capacity-type": "on-demand",
	}
	assert.Equal(t, "od", ExtractCapacityType(labels))
}

func TestExtractCapacityType_Spotinst(t *testing.T) {
	labels := map[string]string{
		"spotinst.io/node-lifecycle": "spot",
	}
	assert.Equal(t, "spot", ExtractCapacityType(labels))
}

func TestExtractCapacityType_EKS(t *testing.T) {
	labels := map[string]string{
		"eks.amazonaws.com/capacityType": "ON_DEMAND",
	}
	assert.Equal(t, "od", ExtractCapacityType(labels))
}

func TestExtractCapacityType_PriorityKarpenterSHOverAWS(t *testing.T) {
	labels := map[string]string{
		"karpenter.sh/capacity-type":      "spot",
		"karpenter.k8s.aws/capacity-type": "on-demand",
	}
	assert.Equal(t, "spot", ExtractCapacityType(labels))
}

func TestExtractCapacityType_PriorityKarpenterOverSpotinst(t *testing.T) {
	labels := map[string]string{
		"karpenter.sh/capacity-type": "spot",
		"spotinst.io/node-lifecycle": "on-demand",
	}
	assert.Equal(t, "spot", ExtractCapacityType(labels))
}

func TestExtractCapacityType_PrioritySpotinstOverEKS(t *testing.T) {
	labels := map[string]string{
		"spotinst.io/node-lifecycle":     "spot",
		"eks.amazonaws.com/capacityType": "ON_DEMAND",
	}
	assert.Equal(t, "spot", ExtractCapacityType(labels))
}

func TestExtractCapacityType_AllPresent(t *testing.T) {
	labels := map[string]string{
		"karpenter.sh/capacity-type":      "spot",
		"karpenter.k8s.aws/capacity-type": "on-demand",
		"spotinst.io/node-lifecycle":      "on-demand",
		"eks.amazonaws.com/capacityType":  "ON_DEMAND",
	}
	// Highest priority: karpenter.sh
	assert.Equal(t, "spot", ExtractCapacityType(labels))
}

func TestExtractCapacityType_Missing(t *testing.T) {
	labels := map[string]string{}
	assert.Equal(t, "x", ExtractCapacityType(labels))
}

// --- On-Demand Normalization ---

func TestNormalizeCapacityType_OnDemandVariants(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"lowercase hyphen", "on-demand"},
		{"title case hyphen", "On-Demand"},
		{"uppercase underscore", "ON_DEMAND"},
		{"lowercase no separator", "ondemand"},
		{"uppercase no separator", "ONDEMAND"},
		{"mixed case hyphen", "ON-DEMAND"},
		{"mixed case no separator", "OnDemand"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels := map[string]string{
				"karpenter.sh/capacity-type": tt.input,
			}
			assert.Equal(t, "od", ExtractCapacityType(labels))
		})
	}
}

func TestNormalizeCapacityType_SpotUnchanged(t *testing.T) {
	labels := map[string]string{
		"karpenter.sh/capacity-type": "spot",
	}
	assert.Equal(t, "spot", ExtractCapacityType(labels))
}

// --- Nodepool Priority Chain ---

func TestExtractNodepool_KarpenterSH(t *testing.T) {
	labels := map[string]string{
		"karpenter.sh/nodepool": "my-pool",
	}
	assert.Equal(t, "my-pool", ExtractNodepool(labels))
}

func TestExtractNodepool_KarpenterAWS(t *testing.T) {
	labels := map[string]string{
		"karpenter.k8s.aws/nodepool": "aws-pool",
	}
	assert.Equal(t, "aws-pool", ExtractNodepool(labels))
}

func TestExtractNodepool_Spotinst(t *testing.T) {
	labels := map[string]string{
		"spotinst.io/ocean-vng-id": "vng-123",
	}
	assert.Equal(t, "vng-123", ExtractNodepool(labels))
}

func TestExtractNodepool_EKS(t *testing.T) {
	labels := map[string]string{
		"eks.amazonaws.com/nodegroup": "my-nodegroup",
	}
	assert.Equal(t, "my-nodegroup", ExtractNodepool(labels))
}

func TestExtractNodepool_PriorityKarpenterSHOverAWS(t *testing.T) {
	labels := map[string]string{
		"karpenter.sh/nodepool":      "sh-pool",
		"karpenter.k8s.aws/nodepool": "aws-pool",
	}
	assert.Equal(t, "sh-pool", ExtractNodepool(labels))
}

func TestExtractNodepool_PriorityKarpenterOverSpotinst(t *testing.T) {
	labels := map[string]string{
		"karpenter.sh/nodepool":    "karp-pool",
		"spotinst.io/ocean-vng-id": "vng-456",
	}
	assert.Equal(t, "karp-pool", ExtractNodepool(labels))
}

func TestExtractNodepool_PrioritySpotinstOverEKS(t *testing.T) {
	labels := map[string]string{
		"spotinst.io/ocean-vng-id":    "vng-789",
		"eks.amazonaws.com/nodegroup": "eks-group",
	}
	assert.Equal(t, "vng-789", ExtractNodepool(labels))
}

func TestExtractNodepool_Missing(t *testing.T) {
	labels := map[string]string{}
	assert.Equal(t, "x", ExtractNodepool(labels))
}

// --- EKS Nodegroup Truncation ---

func TestExtractNodepool_EKSTruncation14Chars(t *testing.T) {
	labels := map[string]string{
		"eks.amazonaws.com/nodegroup": "14-chars-value", // 14 chars
	}
	assert.Equal(t, "14-chars-value", ExtractNodepool(labels))
	assert.Len(t, ExtractNodepool(labels), 14)
}

func TestExtractNodepool_EKSTruncation15Chars(t *testing.T) {
	labels := map[string]string{
		"eks.amazonaws.com/nodegroup": "15-chars-valuee", // 15 chars
	}
	assert.Equal(t, "15-chars-valuee", ExtractNodepool(labels))
	assert.Len(t, ExtractNodepool(labels), 15)
}

func TestExtractNodepool_EKSTruncation16Chars(t *testing.T) {
	labels := map[string]string{
		"eks.amazonaws.com/nodegroup": "16-chars-valueee", // 16 chars
	}
	result := ExtractNodepool(labels)
	assert.Len(t, result, 15)
	assert.Equal(t, "16-chars-valuee", result)
}

func TestExtractNodepool_EKSTruncationLong(t *testing.T) {
	labels := map[string]string{
		"eks.amazonaws.com/nodegroup": "very-long-nodegroup-name-that-exceeds-limit",
	}
	result := ExtractNodepool(labels)
	assert.Len(t, result, 15)
	assert.Equal(t, "very-long-nodeg", result)
}

func TestExtractNodepool_NonEKSNotTruncated(t *testing.T) {
	// Karpenter nodepool values should NOT be truncated to 15
	labels := map[string]string{
		"karpenter.sh/nodepool": "very-long-karpenter-pool-name",
	}
	assert.Equal(t, "very-long-karpenter-pool-name", ExtractNodepool(labels))
}

// --- Nodeclaim Truncation ---

func TestExtractMetadata_NodeclaimTruncation19Chars(t *testing.T) {
	labels := map[string]string{
		"karpenter.sh/nodeclaim": "19-chars-nodeclaim!", // 19 chars
	}
	meta := ExtractMetadata(labels, "")
	assert.Equal(t, "19-chars-nodeclaim!", meta.Nodeclaim)
	assert.Len(t, meta.Nodeclaim, 19)
}

func TestExtractMetadata_NodeclaimTruncation20Chars(t *testing.T) {
	labels := map[string]string{
		"karpenter.sh/nodeclaim": "20-chars-nodeclaim!!", // 20 chars
	}
	meta := ExtractMetadata(labels, "")
	assert.Equal(t, "20-chars-nodeclaim!!", meta.Nodeclaim)
	assert.Len(t, meta.Nodeclaim, 20)
}

func TestExtractMetadata_NodeclaimTruncation21Chars(t *testing.T) {
	labels := map[string]string{
		"karpenter.sh/nodeclaim": "21-chars-nodeclaim!!!", // 21 chars
	}
	meta := ExtractMetadata(labels, "")
	assert.Len(t, meta.Nodeclaim, 20)
	assert.Equal(t, "21-chars-nodeclaim!!", meta.Nodeclaim)
}

func TestExtractMetadata_NodeclaimMissing(t *testing.T) {
	labels := map[string]string{}
	meta := ExtractMetadata(labels, "")
	assert.Equal(t, "x", meta.Nodeclaim)
}

// --- EC2 ID Extraction ---

func TestExtractEC2ID_Standard(t *testing.T) {
	providerID := "aws:///us-east-1a/i-0abcdef1234567890"
	assert.Equal(t, "i-0abcdef1234567890", ExtractEC2ID(providerID))
}

func TestExtractEC2ID_ShortID(t *testing.T) {
	providerID := "aws:///us-west-2b/i-abc123"
	assert.Equal(t, "i-abc123", ExtractEC2ID(providerID))
}

func TestExtractEC2ID_NoMatch(t *testing.T) {
	assert.Equal(t, "x", ExtractEC2ID(""))
	assert.Equal(t, "x", ExtractEC2ID("gce:///zone/instance-name"))
	assert.Equal(t, "x", ExtractEC2ID("no-ec2-id-here"))
}

func TestExtractEC2ID_EmbeddedInLongString(t *testing.T) {
	providerID := "aws:///us-east-1a/i-0123456789abcdef0/extra"
	result := ExtractEC2ID(providerID)
	assert.Contains(t, result, "i-0123456789abcdef0")
}

func TestExtractEC2ID_WithHyphens(t *testing.T) {
	providerID := "aws:///us-east-1a/i-abc-def-123"
	result := ExtractEC2ID(providerID)
	assert.Equal(t, "i-abc-def-123", result)
}

// --- Missing Labels Default to "x" ---

func TestExtractMetadata_AllMissing(t *testing.T) {
	meta := ExtractMetadata(map[string]string{}, "")
	assert.Equal(t, "x", meta.EC2InstanceID)
	assert.Equal(t, "x", meta.InstanceType)
	assert.Equal(t, "x", meta.CapacityType)
	assert.Equal(t, "x", meta.Architecture)
	assert.Equal(t, "x", meta.Zone)
	assert.Equal(t, "x", meta.Nodepool)
	assert.Equal(t, "x", meta.Nodeclaim)
	assert.Equal(t, "x", meta.Autoscaler)
}

func TestExtractMetadata_FullLabels(t *testing.T) {
	labels := map[string]string{
		"karpenter.sh/capacity-type":       "spot",
		"karpenter.sh/nodepool":            "default",
		"karpenter.sh/nodeclaim":           "claim-abc",
		"kubernetes.io/arch":               "amd64",
		"topology.kubernetes.io/zone":      "us-east-1a",
		"node.kubernetes.io/instance-type": "m5.xlarge",
	}
	providerID := "aws:///us-east-1a/i-0abcdef1234567890"

	meta := ExtractMetadata(labels, providerID)
	assert.Equal(t, "i-0abcdef1234567890", meta.EC2InstanceID)
	assert.Equal(t, "m5.xlarge", meta.InstanceType)
	assert.Equal(t, "spot", meta.CapacityType)
	assert.Equal(t, "amd64", meta.Architecture)
	assert.Equal(t, "1a", meta.Zone)
	assert.Equal(t, "default", meta.Nodepool)
	assert.Equal(t, "claim-abc", meta.Nodeclaim)
	assert.Equal(t, "karpenter", meta.Autoscaler)
}

// --- Zone Extraction ---

func TestExtractMetadata_ZoneLastTwoChars(t *testing.T) {
	tests := []struct {
		name     string
		zone     string
		expected string
	}{
		{"us-east-1a", "us-east-1a", "1a"},
		{"us-west-2b", "us-west-2b", "2b"},
		{"eu-west-1c", "eu-west-1c", "1c"},
		{"ap-southeast-1a", "ap-southeast-1a", "1a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels := map[string]string{
				"topology.kubernetes.io/zone": tt.zone,
			}
			meta := ExtractMetadata(labels, "")
			assert.Equal(t, tt.expected, meta.Zone)
		})
	}
}

func TestExtractMetadata_ZoneMissing(t *testing.T) {
	meta := ExtractMetadata(map[string]string{}, "")
	assert.Equal(t, "x", meta.Zone)
}

func TestExtractMetadata_ZoneSingleChar(t *testing.T) {
	labels := map[string]string{
		"topology.kubernetes.io/zone": "a",
	}
	meta := ExtractMetadata(labels, "")
	// Single char zone — less than 2 chars, returned as-is by extractZone
	assert.Equal(t, "a", meta.Zone)
}

// --- Architecture ---

func TestExtractMetadata_Architecture(t *testing.T) {
	labels := map[string]string{
		"kubernetes.io/arch": "arm64",
	}
	meta := ExtractMetadata(labels, "")
	assert.Equal(t, "arm64", meta.Architecture)
}

// --- Instance Type ---

func TestExtractMetadata_InstanceType(t *testing.T) {
	labels := map[string]string{
		"node.kubernetes.io/instance-type": "c5.2xlarge",
	}
	meta := ExtractMetadata(labels, "")
	assert.Equal(t, "c5.2xlarge", meta.InstanceType)
}

// --- Autoscaler Detection ---

func TestDetectAutoscaler_KarpenterSH(t *testing.T) {
	labels := map[string]string{
		"karpenter.sh/nodepool": "default",
	}
	assert.Equal(t, "karpenter", DetectAutoscaler(labels))
}

func TestDetectAutoscaler_KarpenterAWS(t *testing.T) {
	labels := map[string]string{
		"karpenter.k8s.aws/nodepool": "default",
	}
	assert.Equal(t, "karpenter", DetectAutoscaler(labels))
}

func TestDetectAutoscaler_Spotio_OceanVNG(t *testing.T) {
	labels := map[string]string{
		"spotinst.io/ocean-vng-id": "vng-123",
	}
	assert.Equal(t, "spotio", DetectAutoscaler(labels))
}

func TestDetectAutoscaler_Spotio_NodeLifecycle(t *testing.T) {
	labels := map[string]string{
		"spotinst.io/node-lifecycle": "spot",
	}
	assert.Equal(t, "spotio", DetectAutoscaler(labels))
}

func TestDetectAutoscaler_CAS(t *testing.T) {
	labels := map[string]string{
		"eks.amazonaws.com/nodegroup": "my-group",
	}
	assert.Equal(t, "cluster-autoscaler", DetectAutoscaler(labels))
}

func TestDetectAutoscaler_Unknown(t *testing.T) {
	labels := map[string]string{
		"some.other/label": "value",
	}
	assert.Equal(t, "x", DetectAutoscaler(labels))
}

func TestDetectAutoscaler_Empty(t *testing.T) {
	assert.Equal(t, "x", DetectAutoscaler(map[string]string{}))
}

func TestDetectAutoscaler_PriorityKarpenterOverSpotio(t *testing.T) {
	labels := map[string]string{
		"karpenter.sh/nodepool":      "default",
		"spotinst.io/ocean-vng-id":   "vng-123",
		"spotinst.io/node-lifecycle": "spot",
	}
	assert.Equal(t, "karpenter", DetectAutoscaler(labels))
}

func TestDetectAutoscaler_PriorityKarpenterOverCAS(t *testing.T) {
	labels := map[string]string{
		"karpenter.sh/nodepool":       "default",
		"eks.amazonaws.com/nodegroup": "my-group",
	}
	assert.Equal(t, "karpenter", DetectAutoscaler(labels))
}

func TestDetectAutoscaler_PrioritySpotioOverCAS(t *testing.T) {
	labels := map[string]string{
		"spotinst.io/ocean-vng-id":    "vng-123",
		"eks.amazonaws.com/nodegroup": "my-group",
	}
	assert.Equal(t, "spotio", DetectAutoscaler(labels))
}

func TestDetectAutoscaler_AllPresent(t *testing.T) {
	labels := map[string]string{
		"karpenter.sh/nodepool":       "default",
		"karpenter.k8s.aws/nodepool":  "aws-pool",
		"spotinst.io/ocean-vng-id":    "vng-123",
		"spotinst.io/node-lifecycle":  "spot",
		"eks.amazonaws.com/nodegroup": "my-group",
	}
	// Highest priority: karpenter
	assert.Equal(t, "karpenter", DetectAutoscaler(labels))
}

func TestDetectAutoscaler_EmptyValueIgnored(t *testing.T) {
	labels := map[string]string{
		"karpenter.sh/nodepool":       "",
		"eks.amazonaws.com/nodegroup": "my-group",
	}
	// Empty karpenter label should be ignored, fall through to CAS
	assert.Equal(t, "cluster-autoscaler", DetectAutoscaler(labels))
}
