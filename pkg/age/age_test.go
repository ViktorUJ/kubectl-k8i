package age

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatAge(t *testing.T) {
	now := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		created  time.Time
		expected string
	}{
		{
			name:     "zero time returns x",
			created:  time.Time{},
			expected: "x",
		},
		{
			name:     "0 minutes",
			created:  now,
			expected: "0m",
		},
		{
			name:     "30 minutes",
			created:  now.Add(-30 * time.Minute),
			expected: "30m",
		},
		{
			name:     "59 minutes",
			created:  now.Add(-59 * time.Minute),
			expected: "59m",
		},
		{
			name:     "exactly 1 hour",
			created:  now.Add(-1 * time.Hour),
			expected: "1h0m",
		},
		{
			name:     "3 hours 45 minutes",
			created:  now.Add(-3*time.Hour - 45*time.Minute),
			expected: "3h45m",
		},
		{
			name:     "23 hours 59 minutes",
			created:  now.Add(-23*time.Hour - 59*time.Minute),
			expected: "23h59m",
		},
		{
			name:     "exactly 1 day",
			created:  now.Add(-24 * time.Hour),
			expected: "1d0h",
		},
		{
			name:     "5 days 12 hours",
			created:  now.Add(-5*24*time.Hour - 12*time.Hour),
			expected: "5d12h",
		},
		{
			name:     "365 days 0 hours",
			created:  now.Add(-365 * 24 * time.Hour),
			expected: "365d0h",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatAge(tt.created, now)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatAgeFromString(t *testing.T) {
	tests := []struct {
		name      string
		timestamp string
		wantX     bool // if true, expect "x"
	}{
		{
			name:      "empty string returns x",
			timestamp: "",
			wantX:     true,
		},
		{
			name:      "invalid timestamp returns x",
			timestamp: "not-a-timestamp",
			wantX:     true,
		},
		{
			name:      "zero time returns x",
			timestamp: "0001-01-01T00:00:00Z",
			wantX:     true,
		},
		{
			name:      "valid recent timestamp returns age",
			timestamp: time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
			wantX:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatAgeFromString(tt.timestamp)
			if tt.wantX {
				assert.Equal(t, "x", result)
			} else {
				assert.NotEqual(t, "x", result)
			}
		})
	}
}
