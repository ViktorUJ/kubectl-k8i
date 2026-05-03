package color

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestColorizeLoad_BoundaryValues(t *testing.T) {
	cc := ColorConfig{Enabled: true}

	tests := []struct {
		name          string
		loadPercent   int
		expectedColor string
	}{
		{"0 is green", 0, "\033[0;32m"},
		{"60 is green", 60, "\033[0;32m"},
		{"61 is yellow", 61, "\033[1;33m"},
		{"80 is yellow", 80, "\033[1;33m"},
		{"81 is red", 81, "\033[0;31m"},
		{"100 is red", 100, "\033[0;31m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cc.ColorizeLoad(tt.loadPercent)
			assert.Contains(t, result, tt.expectedColor, "Expected ANSI code %q in result %q", tt.expectedColor, result)
			assert.Contains(t, result, "\033[0m", "Expected reset code in result")
		})
	}
}

func TestColorizeLoad_ColorsDisabled(t *testing.T) {
	cc := ColorConfig{Enabled: false}

	tests := []struct {
		name        string
		loadPercent int
		expected    string
	}{
		{"0 no ANSI", 0, "00%"},
		{"60 no ANSI", 60, "60%"},
		{"61 no ANSI", 61, "61%"},
		{"80 no ANSI", 80, "80%"},
		{"81 no ANSI", 81, "81%"},
		{"100 no ANSI", 100, "100%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cc.ColorizeLoad(tt.loadPercent)
			assert.Equal(t, tt.expected, result)
			assert.False(t, strings.Contains(result, "\033["), "Should not contain ANSI codes when disabled")
		})
	}
}

func TestColorizeLoad_OutputFormat(t *testing.T) {
	cc := ColorConfig{Enabled: true}

	// Verify the output contains the formatted number and reset code
	result := cc.ColorizeLoad(42)
	assert.Contains(t, result, "42%")
	assert.Contains(t, result, "\033[0m")

	// Single digit should have leading zero
	result = cc.ColorizeLoad(5)
	assert.Contains(t, result, "05%")
}

func TestNewColorConfig_ForceEnabled(t *testing.T) {
	enabled := true
	cc := NewColorConfig(&enabled)
	assert.True(t, cc.Enabled)
}

func TestNewColorConfig_ForceDisabled(t *testing.T) {
	disabled := false
	cc := NewColorConfig(&disabled)
	assert.False(t, cc.Enabled)
}

func TestNewColorConfig_AutoDetect(t *testing.T) {
	// When nil, auto-detect is used. In test environment, stdout is typically
	// not a terminal, so Enabled should be false.
	cc := NewColorConfig(nil)
	// We just verify it doesn't panic and returns a valid config
	assert.IsType(t, ColorConfig{}, cc)
}
