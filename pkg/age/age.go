package age

import (
	"fmt"
	"time"
)

// FormatAge converts a creation timestamp to human-readable age
// relative to the given reference time.
// ≥24h: "{days}d{hours}h", ≥1h: "{hours}h{minutes}m", <1h: "{minutes}m"
// Zero time returns "x".
func FormatAge(creationTime time.Time, now time.Time) string {
	if creationTime.IsZero() {
		return "x"
	}

	d := now.Sub(creationTime)
	if d < 0 {
		d = 0
	}

	totalMinutes := int(d.Minutes())
	totalHours := totalMinutes / 60
	days := totalHours / 24
	hours := totalHours % 24
	minutes := totalMinutes % 60

	if days >= 1 {
		return fmt.Sprintf("%dd%dh", days, hours)
	}
	if totalHours >= 1 {
		return fmt.Sprintf("%dh%dm", totalHours, minutes)
	}
	return fmt.Sprintf("%dm", totalMinutes)
}

// FormatAgeFromString parses an ISO timestamp string and formats the age
// relative to the current time. Returns "x" for empty or unparseable timestamps.
func FormatAgeFromString(timestamp string) string {
	if timestamp == "" {
		return "x"
	}

	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return "x"
	}

	if t.IsZero() {
		return "x"
	}

	return FormatAge(t, time.Now())
}
