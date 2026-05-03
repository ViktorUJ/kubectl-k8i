package parser

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ParseCPU converts a Kubernetes CPU string to cores (float64).
// Examples: "500m" → 0.5, "2" → 2.0, "" → 0.0
func ParseCPU(value string) float64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0.0
	}

	if strings.HasSuffix(value, "n") {
		nanos := strings.TrimSuffix(value, "n")
		v, err := strconv.ParseFloat(nanos, 64)
		if err != nil {
			return 0.0
		}
		return v / 1e9
	}

	if strings.HasSuffix(value, "m") {
		millis := strings.TrimSuffix(value, "m")
		v, err := strconv.ParseFloat(millis, 64)
		if err != nil {
			return 0.0
		}
		return v / 1000.0
	}

	v, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0.0
	}
	return v
}

// ParseMemory converts a Kubernetes memory string to gigabytes (float64).
// Examples: "1Gi" → 1.0, "512Mi" → 0.5, "1048576Ki" → 1.0, "" → 0.0
func ParseMemory(value string) float64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0.0
	}

	if strings.HasSuffix(value, "Ki") {
		numStr := strings.TrimSuffix(value, "Ki")
		v, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0.0
		}
		return v / 1048576.0
	}

	if strings.HasSuffix(value, "Mi") {
		numStr := strings.TrimSuffix(value, "Mi")
		v, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0.0
		}
		return v / 1024.0
	}

	if strings.HasSuffix(value, "Gi") {
		numStr := strings.TrimSuffix(value, "Gi")
		v, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0.0
		}
		return v
	}

	// Plain bytes (no suffix) — convert to GB
	v, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0.0
	}
	return v / (1024.0 * 1024.0 * 1024.0)
}

// FormatCPU converts cores (float64) to a Kubernetes CPU string.
// Used for round-trip testing.
// Values < 1 core are expressed in millicores (e.g., 0.5 → "500m").
// Values ≥ 1 core are expressed as whole or fractional cores (e.g., 2.0 → "2").
func FormatCPU(cores float64) string {
	if cores == 0 {
		return "0"
	}

	millis := math.Round(cores * 1000)
	if millis < 1000 {
		return fmt.Sprintf("%dm", int64(millis))
	}

	// If it's a whole number of cores, format without decimal
	if millis == math.Trunc(millis) && math.Mod(millis, 1000) == 0 {
		return fmt.Sprintf("%d", int64(millis/1000))
	}

	// Otherwise express in millicores for precision
	return fmt.Sprintf("%dm", int64(millis))
}

// FormatMemory converts gigabytes (float64) to a Kubernetes memory string.
// Used for round-trip testing.
// Chooses the most appropriate unit (Gi, Mi, Ki) to avoid fractional values.
func FormatMemory(gb float64) string {
	if gb == 0 {
		return "0"
	}

	// Try Gi first — if it's a whole number of Gi
	if gb == math.Trunc(gb) && gb >= 1 {
		return fmt.Sprintf("%dGi", int64(gb))
	}

	// Try Mi — convert GB to Mi
	mi := gb * 1024.0
	miRounded := math.Round(mi)
	if math.Abs(mi-miRounded) < 1e-9 && miRounded >= 1 {
		return fmt.Sprintf("%dMi", int64(miRounded))
	}

	// Fall back to Ki — convert GB to Ki
	ki := gb * 1048576.0
	kiRounded := math.Round(ki)
	if kiRounded >= 1 {
		return fmt.Sprintf("%dKi", int64(kiRounded))
	}

	// Very small values — express in Gi with decimal
	return fmt.Sprintf("%gGi", gb)
}

// ParseCPUMillicores converts a Kubernetes CPU string to millicores (int64).
// Examples: "500m" → 500, "2" → 2000, "" → 0
func ParseCPUMillicores(value string) int64 {
	cores := ParseCPU(value)
	return int64(math.Round(cores * 1000))
}

// CalculateLoadPercent computes (usage * 100) / capacity for integer values,
// rounded to the nearest integer. Returns 0 if capacity is 0 or usage is 0.
func CalculateLoadPercent(usage, capacity int64) int {
	if capacity == 0 || usage == 0 {
		return 0
	}
	return int(math.Round(float64(usage) * 100.0 / float64(capacity)))
}
