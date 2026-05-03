package color

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

// ANSI color codes
const (
	colorGreen  = "\033[0;32m"
	colorYellow = "\033[1;33m"
	colorRed    = "\033[0;31m"
	colorReset  = "\033[0m"
)

// ColorConfig holds color rendering settings.
type ColorConfig struct {
	Enabled bool
}

// NewColorConfig creates a ColorConfig, auto-detecting terminal support
// if forceColor is nil. If forceColor is non-nil, the pointed-to value
// is used directly.
func NewColorConfig(forceColor *bool) ColorConfig {
	if forceColor != nil {
		return ColorConfig{Enabled: *forceColor}
	}
	return ColorConfig{Enabled: DetectColorSupport()}
}

// ColorizeLoad applies ANSI color to a load percentage value.
// Green (≤60), Yellow (61–80), Red (>80).
// When Enabled is false, returns the formatted percentage without ANSI codes.
// The load percentage is formatted with %02d (leading zero for single digits).
func (c ColorConfig) ColorizeLoad(loadPercent int) string {
	formatted := fmt.Sprintf("%02d%%", loadPercent)

	if !c.Enabled {
		return formatted
	}

	var code string
	switch {
	case loadPercent <= 60:
		code = colorGreen
	case loadPercent <= 80:
		code = colorYellow
	default:
		code = colorRed
	}

	return code + formatted + colorReset
}

// DetectColorSupport checks if stdout is a terminal that supports ANSI colors.
// Uses golang.org/x/term for cross-platform terminal detection.
func DetectColorSupport() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// ColorizeGreen wraps a string in green ANSI codes. No-op when colors are disabled.
func (c ColorConfig) ColorizeGreen(s string) string {
	if !c.Enabled {
		return s
	}
	return colorGreen + s + colorReset
}

// ColorizeRatio applies ANSI color to a string based on the ratio between two values.
// Used to highlight CPU overcommit (limits/requests or limits/capacity).
// Green (ratio ≤ 5), Yellow (5 < ratio ≤ 10), Red (ratio > 10).
// If denominator is zero or ratio ≤ 1, no color is applied.
func (c ColorConfig) ColorizeRatio(s string, numerator, denominator float64) string {
	if !c.Enabled || denominator <= 0 || numerator <= denominator {
		return s
	}

	ratio := numerator / denominator

	var code string
	switch {
	case ratio <= 5:
		code = colorGreen
	case ratio <= 10:
		code = colorYellow
	default:
		code = colorRed
	}

	return code + s + colorReset
}

// ColorizeOvercommitPct applies ANSI color based on the percentage by which
// numerator exceeds denominator: pct = (num - denom) / denom * 100.
// Green (≤30%), Yellow (30–50%), Red (>50%).
// No color if denominator is zero or numerator ≤ denominator.
func (c ColorConfig) ColorizeOvercommitPct(s string, numerator, denominator float64) string {
	if !c.Enabled || denominator <= 0 || numerator <= denominator {
		return s
	}

	pct := (numerator - denominator) / denominator * 100

	var code string
	switch {
	case pct <= 30:
		code = colorGreen
	case pct <= 50:
		code = colorYellow
	default:
		code = colorRed
	}

	return code + s + colorReset
}
