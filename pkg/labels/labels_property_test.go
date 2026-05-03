package labels

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

// capacityTypeLabelKeys in priority order (highest first).
var capacityTypeLabelKeys = []string{
	"karpenter.sh/capacity-type",
	"karpenter.k8s.aws/capacity-type",
	"spotinst.io/node-lifecycle",
	"eks.amazonaws.com/capacityType",
}

// nodepoolLabelKeys in priority order (highest first).
var nodepoolLabelKeys = []string{
	"karpenter.sh/nodepool",
	"karpenter.k8s.aws/nodepool",
	"spotinst.io/ocean-vng-id",
	"eks.amazonaws.com/nodegroup",
}

// autoscalerLabelKeys used for autoscaler detection.
var autoscalerLabelKeys = []string{
	"karpenter.sh/nodepool",
	"karpenter.k8s.aws/nodepool",
	"spotinst.io/ocean-vng-id",
	"spotinst.io/node-lifecycle",
	"eks.amazonaws.com/nodegroup",
}

// TestProperty3_CapacityTypePriority verifies that for any label map containing
// one or more capacity type label keys, the extracted capacity type equals the
// value of the highest-priority key present.
//
// **Validates: Requirements 3.1**
func TestProperty3_CapacityTypePriority(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a label map with at least one capacity type key
		labels := make(map[string]string)
		values := []string{"spot", "preemptible", "reserved"}

		// Decide which keys to include (at least one)
		included := false
		for _, key := range capacityTypeLabelKeys {
			if rapid.Bool().Draw(t, fmt.Sprintf("include_%s", key)) {
				val := rapid.SampledFrom(values).Draw(t, fmt.Sprintf("val_%s", key))
				labels[key] = val
				included = true
			}
		}
		if !included {
			// Ensure at least one key is present
			idx := rapid.IntRange(0, len(capacityTypeLabelKeys)-1).Draw(t, "forced_key_idx")
			val := rapid.SampledFrom(values).Draw(t, "forced_val")
			labels[capacityTypeLabelKeys[idx]] = val
		}

		result := ExtractCapacityType(labels)

		// Find the expected value: the value of the highest-priority key present
		var expected string
		for _, key := range capacityTypeLabelKeys {
			if val, ok := labels[key]; ok && val != "" {
				expected = val
				break
			}
		}

		// The result should match the expected value (after normalization)
		expectedNormalized := normalizeForTest(expected)
		assert.Equal(t, expectedNormalized, result,
			"Priority chain failed: labels=%v, expected=%q (from %q), got=%q",
			labels, expectedNormalized, expected, result)
	})
}

// TestProperty4_NodepoolPriority verifies that for any label map containing
// one or more nodepool label keys, the extracted nodepool equals the value of
// the highest-priority key (with EKS truncation to 15 chars).
//
// **Validates: Requirements 3.2**
func TestProperty4_NodepoolPriority(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		labels := make(map[string]string)

		// Generate pool name values of varying lengths
		poolNames := make([]string, len(nodepoolLabelKeys))
		for i, key := range nodepoolLabelKeys {
			nameLen := rapid.IntRange(1, 30).Draw(t, fmt.Sprintf("len_%d", i))
			name := generateAlphaString(t, nameLen, fmt.Sprintf("name_%s", key))
			poolNames[i] = name
		}

		// Include at least one key
		included := false
		for i, key := range nodepoolLabelKeys {
			if rapid.Bool().Draw(t, fmt.Sprintf("include_%s", key)) {
				labels[key] = poolNames[i]
				included = true
			}
		}
		if !included {
			idx := rapid.IntRange(0, len(nodepoolLabelKeys)-1).Draw(t, "forced_idx")
			labels[nodepoolLabelKeys[idx]] = poolNames[idx]
		}

		result := ExtractNodepool(labels)

		// Find expected: highest-priority key's value
		var expectedKey string
		var expectedVal string
		for _, key := range nodepoolLabelKeys {
			if val, ok := labels[key]; ok && val != "" {
				expectedKey = key
				expectedVal = val
				break
			}
		}

		// Apply EKS truncation if the winning key is eks.amazonaws.com/nodegroup
		if expectedKey == "eks.amazonaws.com/nodegroup" && len(expectedVal) > 15 {
			expectedVal = expectedVal[:15]
		}

		assert.Equal(t, expectedVal, result,
			"Nodepool priority failed: labels=%v, expected=%q, got=%q", labels, expectedVal, result)
	})
}

// TestProperty5_CapacityTypeNormalization verifies that all on-demand variants
// are normalized to "od".
//
// **Validates: Requirements 3.3**
func TestProperty5_CapacityTypeNormalization(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate on-demand variants
		variants := []string{
			"on-demand", "On-Demand", "ON-DEMAND", "ON_DEMAND",
			"ondemand", "OnDemand", "ONDEMAND", "on_demand",
			"On_Demand", "oN-dEmAnD",
		}
		variant := rapid.SampledFrom(variants).Draw(t, "variant")

		labels := map[string]string{
			"karpenter.sh/capacity-type": variant,
		}

		result := ExtractCapacityType(labels)
		assert.Equal(t, "od", result,
			"On-demand variant %q should normalize to 'od', got %q", variant, result)
	})
}

// TestProperty6_LabelValueTruncation verifies that EKS nodegroup values are
// truncated to ≤15 chars and nodeclaim values to ≤20 chars, and shorter
// strings are unchanged.
//
// **Validates: Requirements 3.4, 3.10**
func TestProperty6_LabelValueTruncation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a string of random length
		strLen := rapid.IntRange(1, 50).Draw(t, "strLen")
		val := generateAlphaString(t, strLen, "value")

		// Test EKS nodegroup truncation (15 chars)
		eksLabels := map[string]string{
			"eks.amazonaws.com/nodegroup": val,
		}
		eksResult := ExtractNodepool(eksLabels)
		assert.LessOrEqual(t, len(eksResult), 15,
			"EKS nodegroup should be ≤15 chars, got %d for input len %d", len(eksResult), len(val))
		if len(val) <= 15 {
			assert.Equal(t, val, eksResult, "Short EKS value should be unchanged")
		} else {
			assert.Equal(t, val[:15], eksResult, "Long EKS value should be prefix")
		}

		// Test nodeclaim truncation (20 chars)
		claimLabels := map[string]string{
			"karpenter.sh/nodeclaim": val,
		}
		meta := ExtractMetadata(claimLabels, "")
		assert.LessOrEqual(t, len(meta.Nodeclaim), 20,
			"Nodeclaim should be ≤20 chars, got %d for input len %d", len(meta.Nodeclaim), len(val))
		if len(val) <= 20 {
			assert.Equal(t, val, meta.Nodeclaim, "Short nodeclaim should be unchanged")
		} else {
			assert.Equal(t, val[:20], meta.Nodeclaim, "Long nodeclaim should be prefix")
		}
	})
}

// TestProperty7_ZoneSuffixExtraction verifies that for any zone string of
// length ≥2, the extracted zone equals the last 2 characters.
//
// **Validates: Requirements 3.6**
func TestProperty7_ZoneSuffixExtraction(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a zone string of length ≥ 2
		zoneLen := rapid.IntRange(2, 30).Draw(t, "zoneLen")
		zone := generateAlphaNumString(t, zoneLen, "zone")

		labels := map[string]string{
			"topology.kubernetes.io/zone": zone,
		}
		meta := ExtractMetadata(labels, "")

		expected := zone[len(zone)-2:]
		assert.Equal(t, expected, meta.Zone,
			"Zone suffix should be last 2 chars of %q, got %q", zone, meta.Zone)
	})
}

// TestProperty8_EC2IDExtraction verifies that EC2 instance IDs matching
// i-[A-Za-z0-9-]+ are correctly extracted, and non-matching strings return "x".
//
// **Validates: Requirements 3.9**
func TestProperty8_EC2IDExtraction(t *testing.T) {
	ec2Pattern := regexp.MustCompile(`i-[A-Za-z0-9-]+`)

	rapid.Check(t, func(t *rapid.T) {
		hasEC2 := rapid.Bool().Draw(t, "hasEC2")

		if hasEC2 {
			// Generate a valid EC2 ID
			idLen := rapid.IntRange(1, 20).Draw(t, "idLen")
			idChars := generateAlphaNumString(t, idLen, "ec2id")
			ec2ID := "i-" + idChars
			prefix := generateAlphaString(t, rapid.IntRange(0, 20).Draw(t, "prefixLen"), "prefix")
			providerID := prefix + "/" + ec2ID

			result := ExtractEC2ID(providerID)
			// Result should contain the EC2 ID pattern
			assert.True(t, ec2Pattern.MatchString(result),
				"Expected EC2 ID pattern in result %q for providerID %q", result, providerID)
			assert.Contains(t, result, "i-",
				"Result should contain 'i-' prefix")
		} else {
			// Generate a string without EC2 ID pattern
			noEC2Strings := []string{
				"gce:///zone/instance",
				"azure:///subscriptions/sub/resourceGroups/rg",
				"no-match-here",
				"",
			}
			providerID := rapid.SampledFrom(noEC2Strings).Draw(t, "noEC2")

			result := ExtractEC2ID(providerID)
			assert.Equal(t, "x", result,
				"Non-EC2 providerID %q should return 'x', got %q", providerID, result)
		}
	})
}

// TestProperty34_AutoscalerDetectionPriority verifies that the autoscaler
// detection follows priority: karpenter > spotio > cas > x across all
// label combinations.
//
// **Validates: Requirements 3.11, 24.1, 24.2, 24.3, 24.4, 24.5**
func TestProperty34_AutoscalerDetectionPriority(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		labels := make(map[string]string)

		// Randomly include each autoscaler label
		for _, key := range autoscalerLabelKeys {
			if rapid.Bool().Draw(t, fmt.Sprintf("include_%s", key)) {
				labels[key] = "some-value"
			}
		}

		result := DetectAutoscaler(labels)

		// Determine expected based on priority
		hasKarpenter := hasLabelInMap(labels, "karpenter.sh/nodepool") ||
			hasLabelInMap(labels, "karpenter.k8s.aws/nodepool")
		hasSpotio := hasLabelInMap(labels, "spotinst.io/ocean-vng-id") ||
			hasLabelInMap(labels, "spotinst.io/node-lifecycle")
		hasCAS := hasLabelInMap(labels, "eks.amazonaws.com/nodegroup")

		var expected string
		switch {
		case hasKarpenter:
			expected = "karpenter"
		case hasSpotio:
			expected = "spotio"
		case hasCAS:
			expected = "cas"
		default:
			expected = "x"
		}

		assert.Equal(t, expected, result,
			"Autoscaler priority failed: labels=%v, expected=%q, got=%q",
			labels, expected, result)
	})
}

// --- Helper functions ---

// normalizeForTest applies the same normalization as the production code.
func normalizeForTest(val string) string {
	lower := strings.ToLower(val)
	normalized := strings.NewReplacer("-", "", "_", "").Replace(lower)
	if normalized == "ondemand" {
		return "od"
	}
	return val
}

// hasLabelInMap checks if a label key exists with a non-empty value.
func hasLabelInMap(labels map[string]string, key string) bool {
	val, ok := labels[key]
	return ok && val != ""
}

// generateAlphaString generates a string of the given length using lowercase letters.
func generateAlphaString(t *rapid.T, length int, label string) string {
	if length <= 0 {
		return ""
	}
	chars := make([]byte, length)
	for i := 0; i < length; i++ {
		chars[i] = byte(rapid.IntRange(97, 122).Draw(t, fmt.Sprintf("%s_char_%d", label, i)))
	}
	return string(chars)
}

// generateAlphaNumString generates a string of the given length using lowercase
// letters and digits.
func generateAlphaNumString(t *rapid.T, length int, label string) string {
	if length <= 0 {
		return ""
	}
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	chars := make([]byte, length)
	for i := 0; i < length; i++ {
		idx := rapid.IntRange(0, len(charset)-1).Draw(t, fmt.Sprintf("%s_char_%d", label, i))
		chars[i] = charset[idx]
	}
	return string(chars)
}
