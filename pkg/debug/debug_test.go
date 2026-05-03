package debug

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- NewDebugLogger tests ---

func TestNewDebugLogger_Enabled(t *testing.T) {
	logger := NewDebugLogger(true)
	assert.True(t, logger.IsEnabled())
}

func TestNewDebugLogger_Disabled(t *testing.T) {
	logger := NewDebugLogger(false)
	assert.False(t, logger.IsEnabled())
}

func TestIsEnabled_NilLogger(t *testing.T) {
	var logger *DebugLogger
	assert.False(t, logger.IsEnabled())
}

// --- LogAPICall tests ---

func TestLogAPICall_OutputFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := NewDebugLoggerWithWriter(true, &buf)

	logger.LogAPICall("GET", "nodes", "https://api.example.com/v1/nodes", 150*time.Millisecond, 200)

	output := buf.String()
	assert.Contains(t, output, "DEBUG")
	assert.Contains(t, output, "api_call")
	assert.Contains(t, output, "method=GET")
	assert.Contains(t, output, "resource=nodes")
	assert.Contains(t, output, "duration=150ms")
	assert.Contains(t, output, "status=200")
	// Verify timestamp format (RFC3339 starts with a year like 20XX-)
	assert.Regexp(t, `^\d{4}-\d{2}-\d{2}T`, output)
	// Verify ends with newline
	assert.True(t, strings.HasSuffix(output, "\n"))
}

// --- LogRetryAttempt tests ---

func TestLogRetryAttempt_OutputFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := NewDebugLoggerWithWriter(true, &buf)

	logger.LogRetryAttempt("list-nodes", 2, 5, 400*time.Millisecond, errors.New("429 Too Many Requests"))

	output := buf.String()
	assert.Contains(t, output, "DEBUG")
	assert.Contains(t, output, "retry")
	assert.Contains(t, output, "operation=list-nodes")
	assert.Contains(t, output, "attempt=2/5")
	assert.Contains(t, output, "backoff=400ms")
	assert.Contains(t, output, "error=429 Too Many Requests")
}

// --- LogDataProcessing tests ---

func TestLogDataProcessing_OutputFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := NewDebugLoggerWithWriter(true, &buf)

	logger.LogDataProcessing("nodes_found", 42)

	output := buf.String()
	assert.Contains(t, output, "DEBUG")
	assert.Contains(t, output, "data_processing")
	assert.Contains(t, output, "step=nodes_found")
	assert.Contains(t, output, "count=42")
}

// --- LogFilterSort tests ---

func TestLogFilterSort_OutputFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := NewDebugLoggerWithWriter(true, &buf)

	logger.LogFilterSort("filter", "zone=us-east-1a", 42, 12)

	output := buf.String()
	assert.Contains(t, output, "DEBUG")
	assert.Contains(t, output, "filter_sort")
	assert.Contains(t, output, "operation=filter")
	assert.Contains(t, output, "detail=zone=us-east-1a")
	assert.Contains(t, output, "input=42")
	assert.Contains(t, output, "output=12")
}

// --- LogTerminalWidth tests ---

func TestLogTerminalWidth_OutputFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := NewDebugLoggerWithWriter(true, &buf)

	logger.LogTerminalWidth(120, true)

	output := buf.String()
	assert.Contains(t, output, "DEBUG")
	assert.Contains(t, output, "terminal")
	assert.Contains(t, output, "width=120")
	assert.Contains(t, output, "detected=true")
}

// --- LogOutputFormat tests ---

func TestLogOutputFormat_OutputFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := NewDebugLoggerWithWriter(true, &buf)

	logger.LogOutputFormat("table")

	output := buf.String()
	assert.Contains(t, output, "DEBUG")
	assert.Contains(t, output, "output")
	assert.Contains(t, output, "format=table")
}

// --- Disabled logger produces no output ---

func TestDisabledLogger_NoOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := NewDebugLoggerWithWriter(false, &buf)

	logger.LogAPICall("GET", "nodes", "https://api.example.com", 100*time.Millisecond, 200)
	logger.LogRetryAttempt("op", 1, 5, 100*time.Millisecond, errors.New("err"))
	logger.LogDataProcessing("step", 10)
	logger.LogFilterSort("filter", "detail", 10, 5)
	logger.LogTerminalWidth(120, true)
	logger.LogOutputFormat("json")

	assert.Empty(t, buf.String(), "disabled logger should produce no output")
}

// --- Nil logger produces no output (no panic) ---

func TestNilLogger_NoPanic(t *testing.T) {
	var logger *DebugLogger

	// These should not panic.
	assert.NotPanics(t, func() {
		logger.LogAPICall("GET", "nodes", "url", 100*time.Millisecond, 200)
		logger.LogRetryAttempt("op", 1, 5, 100*time.Millisecond, errors.New("err"))
		logger.LogDataProcessing("step", 10)
		logger.LogFilterSort("filter", "detail", 10, 5)
		logger.LogTerminalWidth(120, true)
		logger.LogOutputFormat("json")
	})
}

// --- All output goes to configured writer ---

func TestAllOutputGoesToConfiguredWriter(t *testing.T) {
	var buf bytes.Buffer
	logger := NewDebugLoggerWithWriter(true, &buf)

	logger.LogAPICall("POST", "pods", "url", 50*time.Millisecond, 201)
	logger.LogDataProcessing("pods_counted", 100)

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Equal(t, 2, len(lines), "expected 2 log lines written to the configured writer")
	assert.Contains(t, lines[0], "api_call")
	assert.Contains(t, lines[1], "data_processing")
}
