package parser

import (
	"fmt"
	"math"
	"testing"

	"pgregory.net/rapid"
)

// TestProperty1_CPURoundTrip verifies that for any valid CPU string (integer
// with "m" suffix or whole number), parse→format→parse produces an equivalent
// value within floating-point tolerance.
//
// **Validates: Requirements 1.1, 1.2, 1.3, 1.4**
func TestProperty1_CPURoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate either a millicores string ("Nm") or a whole-core string ("N")
		isMillicores := rapid.Bool().Draw(t, "isMillicores")

		var cpuStr string
		if isMillicores {
			// Generate millicores in range [0, 128000] (up to 128 cores)
			millis := rapid.IntRange(0, 128000).Draw(t, "millis")
			cpuStr = fmt.Sprintf("%dm", millis)
		} else {
			// Generate whole cores in range [0, 128]
			cores := rapid.IntRange(0, 128).Draw(t, "cores")
			cpuStr = fmt.Sprintf("%d", cores)
		}

		// Parse → Format → Parse
		parsed1 := ParseCPU(cpuStr)
		formatted := FormatCPU(parsed1)
		parsed2 := ParseCPU(formatted)

		// The two parsed values must be equivalent within floating-point tolerance
		if math.Abs(parsed1-parsed2) > 1e-9 {
			t.Fatalf("CPU round-trip failed: %q → %f → %q → %f (diff=%e)",
				cpuStr, parsed1, formatted, parsed2, math.Abs(parsed1-parsed2))
		}
	})
}

// TestProperty2_MemoryRoundTrip verifies that for any valid memory string
// (with "Ki", "Mi", or "Gi" suffix), parse→format→parse produces an equivalent
// value within floating-point tolerance.
//
// **Validates: Requirements 2.1, 2.2, 2.3, 2.4, 2.5**
func TestProperty2_MemoryRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Choose a suffix
		suffixIdx := rapid.IntRange(0, 2).Draw(t, "suffixIdx")
		suffixes := []string{"Ki", "Mi", "Gi"}
		suffix := suffixes[suffixIdx]

		var memStr string
		switch suffix {
		case "Ki":
			// Generate Ki values in range [0, 134217728] (up to 128 Gi)
			ki := rapid.IntRange(0, 134217728).Draw(t, "ki")
			memStr = fmt.Sprintf("%dKi", ki)
		case "Mi":
			// Generate Mi values in range [0, 131072] (up to 128 Gi)
			mi := rapid.IntRange(0, 131072).Draw(t, "mi")
			memStr = fmt.Sprintf("%dMi", mi)
		case "Gi":
			// Generate Gi values in range [0, 128]
			gi := rapid.IntRange(0, 128).Draw(t, "gi")
			memStr = fmt.Sprintf("%dGi", gi)
		}

		// Parse → Format → Parse
		parsed1 := ParseMemory(memStr)
		formatted := FormatMemory(parsed1)
		parsed2 := ParseMemory(formatted)

		// The two parsed values must be equivalent within floating-point tolerance
		if math.Abs(parsed1-parsed2) > 1e-6 {
			t.Fatalf("Memory round-trip failed: %q → %f → %q → %f (diff=%e)",
				memStr, parsed1, formatted, parsed2, math.Abs(parsed1-parsed2))
		}
	})
}

// TestProperty19_LoadPercentageCalculation verifies that for any (usage, capacity)
// pair, load = (usage*100)/capacity rounded to int; capacity=0 or usage=0 → 0.
//
// **Validates: Requirements 16.1, 16.2, 16.3, 16.4**
func TestProperty19_LoadPercentageCalculation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		usage := rapid.Int64Range(0, 1000000).Draw(t, "usage")
		capacity := rapid.Int64Range(0, 1000000).Draw(t, "capacity")

		result := CalculateLoadPercent(usage, capacity)

		if capacity == 0 || usage == 0 {
			// When capacity is 0 or usage is 0, load must be 0
			if result != 0 {
				t.Fatalf("Expected 0 for usage=%d, capacity=%d, got %d",
					usage, capacity, result)
			}
		} else {
			// load = round((usage * 100) / capacity)
			expected := int(math.Round(float64(usage) * 100.0 / float64(capacity)))
			if result != expected {
				t.Fatalf("Load mismatch for usage=%d, capacity=%d: expected %d, got %d",
					usage, capacity, expected, result)
			}
		}

		// Result must be non-negative
		if result < 0 {
			t.Fatalf("Load percentage must be non-negative, got %d for usage=%d, capacity=%d",
				result, usage, capacity)
		}
	})
}
