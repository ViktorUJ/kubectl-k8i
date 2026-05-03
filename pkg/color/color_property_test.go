package color

import (
	"fmt"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// TestProperty10_ColorThresholdCorrectness verifies that for any integer load
// percentage in [0, 100] with colors enabled: ≤60 → green, 61-80 → yellow, >80 → red.
//
// **Validates: Requirements 5.1, 5.2, 5.3**
func TestProperty10_ColorThresholdCorrectness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		loadPercent := rapid.IntRange(0, 100).Draw(t, "loadPercent")
		cc := ColorConfig{Enabled: true}

		result := cc.ColorizeLoad(loadPercent)

		switch {
		case loadPercent <= 60:
			if !strings.Contains(result, "\033[0;32m") {
				t.Fatalf("Load %d (≤60) should contain green ANSI code, got %q", loadPercent, result)
			}
		case loadPercent <= 80:
			if !strings.Contains(result, "\033[1;33m") {
				t.Fatalf("Load %d (61-80) should contain yellow ANSI code, got %q", loadPercent, result)
			}
		default:
			if !strings.Contains(result, "\033[0;31m") {
				t.Fatalf("Load %d (>80) should contain red ANSI code, got %q", loadPercent, result)
			}
		}

		// All colored outputs must contain the reset code
		if !strings.Contains(result, "\033[0m") {
			t.Fatalf("Load %d: output should contain reset ANSI code, got %q", loadPercent, result)
		}
	})
}

// TestProperty11_NoANSIWhenDisabled verifies that for any load percentage value,
// when colors are disabled, the output contains no ANSI escape sequences.
//
// **Validates: Requirements 5.4**
func TestProperty11_NoANSIWhenDisabled(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		loadPercent := rapid.IntRange(0, 200).Draw(t, "loadPercent")
		cc := ColorConfig{Enabled: false}

		result := cc.ColorizeLoad(loadPercent)

		if strings.Contains(result, "\033[") {
			t.Fatalf("Load %d with colors disabled should not contain ANSI codes, got %q", loadPercent, result)
		}
	})
}

// TestProperty20_LoadPercentagePadding verifies that single-digit percentages
// have a leading zero (%02d%% format), producing a 3-character string for values 0-9.
//
// **Validates: Requirements 16.5**
func TestProperty20_LoadPercentagePadding(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		loadPercent := rapid.IntRange(0, 9).Draw(t, "loadPercent")
		cc := ColorConfig{Enabled: false}

		result := cc.ColorizeLoad(loadPercent)

		// For single-digit values, the result should be 3 characters: leading zero + digit + %
		expected := fmt.Sprintf("%02d%%", loadPercent)
		if result != expected {
			t.Fatalf("Load %d: expected %q (padded with %%), got %q", loadPercent, expected, result)
		}
		if len(result) != 3 {
			t.Fatalf("Load %d: expected 3-char string, got %d chars: %q", loadPercent, len(result), result)
		}
	})
}
