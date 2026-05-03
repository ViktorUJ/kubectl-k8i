package parser

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCPU(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{"millicores 500m", "500m", 0.5},
		{"whole cores 2", "2", 2.0},
		{"empty string", "", 0.0},
		{"zero millicores", "0m", 0.0},
		{"one core", "1", 1.0},
		{"100 millicores", "100m", 0.1},
		{"1000 millicores equals 1 core", "1000m", 1.0},
		{"fractional cores", "1.5", 1.5},
		{"250 millicores", "250m", 0.25},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseCPU(tt.input)
			assert.InDelta(t, tt.expected, result, 1e-9, "ParseCPU(%q)", tt.input)
		})
	}
}

func TestParseCPU_LargeValues(t *testing.T) {
	result := ParseCPU("128")
	assert.InDelta(t, 128.0, result, 1e-9)

	result = ParseCPU("64000m")
	assert.InDelta(t, 64.0, result, 1e-9)
}

func TestParseCPU_InvalidInput(t *testing.T) {
	result := ParseCPU("abc")
	assert.Equal(t, 0.0, result)

	result = ParseCPU("m")
	assert.Equal(t, 0.0, result)
}

func TestParseMemory(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{"1 Gi", "1Gi", 1.0},
		{"512 Mi", "512Mi", 0.5},
		{"1048576 Ki", "1048576Ki", 1.0},
		{"empty string", "", 0.0},
		{"0 Gi", "0Gi", 0.0},
		{"0 Mi", "0Mi", 0.0},
		{"0 Ki", "0Ki", 0.0},
		{"256 Mi", "256Mi", 0.25},
		{"2 Gi", "2Gi", 2.0},
		{"1024 Mi equals 1 Gi", "1024Mi", 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseMemory(tt.input)
			assert.InDelta(t, tt.expected, result, 1e-9, "ParseMemory(%q)", tt.input)
		})
	}
}

func TestParseMemory_LargeValues(t *testing.T) {
	// 128 Gi
	result := ParseMemory("128Gi")
	assert.InDelta(t, 128.0, result, 1e-9)

	// Large Ki value: 1073741824 Ki = 1024 Gi
	result = ParseMemory("1073741824Ki")
	assert.InDelta(t, 1024.0, result, 1e-6)
}

func TestParseMemory_PlainBytes(t *testing.T) {
	// 1073741824 bytes = 1 Gi
	result := ParseMemory("1073741824")
	assert.InDelta(t, 1.0, result, 1e-9)
}

func TestParseMemory_InvalidInput(t *testing.T) {
	result := ParseMemory("abc")
	assert.Equal(t, 0.0, result)

	result = ParseMemory("Gi")
	assert.Equal(t, 0.0, result)
}

func TestParseCPUMillicores(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{"500m to 500 millicores", "500m", 500},
		{"2 cores to 2000 millicores", "2", 2000},
		{"empty to 0", "", 0},
		{"0m to 0", "0m", 0},
		{"1 core to 1000", "1", 1000},
		{"250m to 250", "250m", 250},
		{"100m to 100", "100m", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseCPUMillicores(tt.input)
			assert.Equal(t, tt.expected, result, "ParseCPUMillicores(%q)", tt.input)
		})
	}
}

func TestFormatCPU(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected string
	}{
		{"zero", 0.0, "0"},
		{"half core", 0.5, "500m"},
		{"one core", 1.0, "1"},
		{"two cores", 2.0, "2"},
		{"quarter core", 0.25, "250m"},
		{"100 millicores", 0.1, "100m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatCPU(tt.input)
			assert.Equal(t, tt.expected, result, "FormatCPU(%f)", tt.input)
		})
	}
}

func TestFormatMemory(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected string
	}{
		{"zero", 0.0, "0"},
		{"1 Gi", 1.0, "1Gi"},
		{"2 Gi", 2.0, "2Gi"},
		{"0.5 Gi is 512 Mi", 0.5, "512Mi"},
		{"0.25 Gi is 256 Mi", 0.25, "256Mi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatMemory(tt.input)
			assert.Equal(t, tt.expected, result, "FormatMemory(%f)", tt.input)
		})
	}
}

func TestFormatCPU_RoundTrip(t *testing.T) {
	// Parse → Format → Parse should produce equivalent value
	inputs := []string{"500m", "2", "1", "100m", "250m", "1000m", "0m"}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			parsed := ParseCPU(input)
			formatted := FormatCPU(parsed)
			reparsed := ParseCPU(formatted)
			assert.InDelta(t, parsed, reparsed, 1e-9,
				"Round-trip failed: %q → %f → %q → %f", input, parsed, formatted, reparsed)
		})
	}
}

func TestFormatMemory_RoundTrip(t *testing.T) {
	// Parse → Format → Parse should produce equivalent value
	inputs := []string{"1Gi", "512Mi", "1048576Ki", "256Mi", "2Gi", "1024Mi"}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			parsed := ParseMemory(input)
			formatted := FormatMemory(parsed)
			reparsed := ParseMemory(formatted)
			assert.InDelta(t, parsed, reparsed, 1e-9,
				"Round-trip failed: %q → %f → %q → %f", input, parsed, formatted, reparsed)
		})
	}
}

func TestParseCPU_WhitespaceHandling(t *testing.T) {
	result := ParseCPU("  500m  ")
	assert.InDelta(t, 0.5, result, 1e-9)

	result = ParseCPU("  2  ")
	assert.InDelta(t, 2.0, result, 1e-9)
}

func TestParseMemory_WhitespaceHandling(t *testing.T) {
	result := ParseMemory("  1Gi  ")
	assert.InDelta(t, 1.0, result, 1e-9)

	result = ParseMemory("  512Mi  ")
	assert.InDelta(t, 0.5, result, 1e-9)
}

func TestParseCPU_ZeroValues(t *testing.T) {
	assert.Equal(t, 0.0, ParseCPU("0"))
	assert.Equal(t, 0.0, ParseCPU("0m"))
	assert.Equal(t, 0.0, ParseCPU(""))
}

func TestParseMemory_ZeroValues(t *testing.T) {
	assert.Equal(t, 0.0, ParseMemory("0Gi"))
	assert.Equal(t, 0.0, ParseMemory("0Mi"))
	assert.Equal(t, 0.0, ParseMemory("0Ki"))
	assert.Equal(t, 0.0, ParseMemory("0"))
	assert.Equal(t, 0.0, ParseMemory(""))
}

func TestParseCPUMillicores_LargeValues(t *testing.T) {
	result := ParseCPUMillicores("128")
	assert.Equal(t, int64(128000), result)

	result = ParseCPUMillicores("64000m")
	assert.Equal(t, int64(64000), result)
}

func TestFormatCPU_LargeValues(t *testing.T) {
	result := FormatCPU(128.0)
	assert.Equal(t, "128", result)
}

func TestFormatMemory_LargeValues(t *testing.T) {
	result := FormatMemory(128.0)
	assert.Equal(t, "128Gi", result)
}

func TestFormatMemory_SmallValues(t *testing.T) {
	// Very small value that doesn't fit neatly into Ki
	smallGB := 1.0 / 1048576.0 // 1 Ki in GB
	result := FormatMemory(smallGB)
	reparsed := ParseMemory(result)
	assert.InDelta(t, smallGB, reparsed, 1e-9,
		"Small value round-trip: %g → %q → %g", smallGB, result, reparsed)
}

func TestParseCPU_Consistency(t *testing.T) {
	// 1000m should equal 1 core
	assert.InDelta(t, ParseCPU("1"), ParseCPU("1000m"), 1e-9)
	// 2000m should equal 2 cores
	assert.InDelta(t, ParseCPU("2"), ParseCPU("2000m"), 1e-9)
}

func TestParseMemory_Consistency(t *testing.T) {
	// 1024 Mi should equal 1 Gi
	assert.InDelta(t, ParseMemory("1Gi"), ParseMemory("1024Mi"), 1e-9)
	// 1048576 Ki should equal 1 Gi
	assert.InDelta(t, ParseMemory("1Gi"), ParseMemory("1048576Ki"), 1e-9)
	// 1073741824 bytes should equal 1 Gi
	assert.InDelta(t, ParseMemory("1Gi"), ParseMemory("1073741824"), 1e-9)
}

func TestParseCPU_NaN_Inf(t *testing.T) {
	// Ensure NaN and Inf strings don't produce unexpected results
	result := ParseCPU("NaN")
	assert.True(t, math.IsNaN(result) || result == 0.0, "NaN input should return NaN or 0")

	result = ParseCPU("Inf")
	assert.True(t, math.IsInf(result, 0) || result == 0.0, "Inf input should return Inf or 0")
}
