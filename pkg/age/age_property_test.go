package age

import (
	"regexp"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// TestProperty9_AgeFormattingByDurationRange verifies that for any non-negative
// duration: if ≥ 24 hours, the formatted age matches "{days}d{hours}h";
// if ≥ 1 hour and < 24 hours, it matches "{hours}h{minutes}m";
// if < 1 hour, it matches "{minutes}m". Numeric components are consistent
// with the input duration.
//
// **Validates: Requirements 4.1, 4.2, 4.3**
func TestProperty9_AgeFormattingByDurationRange(t *testing.T) {
	// Patterns for each range
	patDaysHours := regexp.MustCompile(`^(\d+)d(\d+)h$`)
	patHoursMinutes := regexp.MustCompile(`^(\d+)h(\d+)m$`)
	patMinutes := regexp.MustCompile(`^(\d+)m$`)

	rapid.Check(t, func(t *rapid.T) {
		// Generate a random non-negative duration in minutes [0, 525960] (up to ~365 days)
		totalMinutes := rapid.IntRange(0, 525960).Draw(t, "totalMinutes")
		dur := time.Duration(totalMinutes) * time.Minute

		now := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
		created := now.Add(-dur)

		result := FormatAge(created, now)

		totalHours := totalMinutes / 60
		days := totalHours / 24
		hours := totalHours % 24
		minutes := totalMinutes % 60

		if days >= 1 {
			// ≥ 24 hours: must match {days}d{hours}h
			matches := patDaysHours.FindStringSubmatch(result)
			if matches == nil {
				t.Fatalf("Duration %d minutes (≥24h): expected pattern {d}d{h}h, got %q", totalMinutes, result)
			}
			// Verify numeric components
			gotDays := matches[1]
			gotHours := matches[2]
			expectedDays := days
			expectedHours := hours
			if gotDays != itoa(expectedDays) || gotHours != itoa(expectedHours) {
				t.Fatalf("Duration %d minutes: expected %dd%dh, got %s", totalMinutes, expectedDays, expectedHours, result)
			}
		} else if totalHours >= 1 {
			// ≥ 1 hour and < 24 hours: must match {hours}h{minutes}m
			matches := patHoursMinutes.FindStringSubmatch(result)
			if matches == nil {
				t.Fatalf("Duration %d minutes (≥1h, <24h): expected pattern {h}h{m}m, got %q", totalMinutes, result)
			}
			gotHours := matches[1]
			gotMinutes := matches[2]
			if gotHours != itoa(totalHours) || gotMinutes != itoa(minutes) {
				t.Fatalf("Duration %d minutes: expected %dh%dm, got %s", totalMinutes, totalHours, minutes, result)
			}
		} else {
			// < 1 hour: must match {minutes}m
			matches := patMinutes.FindStringSubmatch(result)
			if matches == nil {
				t.Fatalf("Duration %d minutes (<1h): expected pattern {m}m, got %q", totalMinutes, result)
			}
			gotMinutes := matches[1]
			if gotMinutes != itoa(totalMinutes) {
				t.Fatalf("Duration %d minutes: expected %dm, got %s", totalMinutes, totalMinutes, result)
			}
		}
	})
}

// itoa converts an int to its string representation without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}
