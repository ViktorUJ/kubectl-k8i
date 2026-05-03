package debug

import (
	"fmt"
	"io"
	"os"
	"time"
)

// DebugLogger provides structured debug logging to stderr.
type DebugLogger struct {
	enabled bool
	writer  io.Writer // defaults to os.Stderr
}

// NewDebugLogger creates a new DebugLogger.
// When enabled=false, all log methods are no-ops.
func NewDebugLogger(enabled bool) *DebugLogger {
	return &DebugLogger{
		enabled: enabled,
		writer:  os.Stderr,
	}
}

// NewDebugLoggerWithWriter creates a new DebugLogger with a custom writer (for testing).
func NewDebugLoggerWithWriter(enabled bool, w io.Writer) *DebugLogger {
	return &DebugLogger{
		enabled: enabled,
		writer:  w,
	}
}

// IsEnabled returns whether the logger is enabled.
func (d *DebugLogger) IsEnabled() bool {
	if d == nil {
		return false
	}
	return d.enabled
}

func (d *DebugLogger) log(category string, kvPairs ...string) {
	if d == nil || !d.enabled {
		return
	}
	ts := time.Now().UTC().Format(time.RFC3339)
	msg := fmt.Sprintf("%s DEBUG %s", ts, category)
	for i := 0; i+1 < len(kvPairs); i += 2 {
		msg += fmt.Sprintf(" %s=%s", kvPairs[i], kvPairs[i+1])
	}
	_, _ = fmt.Fprintln(d.writer, msg)
}

// LogAPICall logs an API call with method, resource type, duration, and status code.
func (d *DebugLogger) LogAPICall(method, resourceType, url string, duration time.Duration, statusCode int) {
	d.log("api_call",
		"method", method,
		"resource", resourceType,
		"duration", duration.String(),
		"status", fmt.Sprintf("%d", statusCode),
	)
}

// LogRetryAttempt logs a retry attempt with attempt number, backoff duration, and error reason.
func (d *DebugLogger) LogRetryAttempt(operation string, attempt, maxRetries int, backoff time.Duration, err error) {
	d.log("retry",
		"operation", operation,
		"attempt", fmt.Sprintf("%d/%d", attempt, maxRetries),
		"backoff", backoff.String(),
		"error", err.Error(),
	)
}

// LogDataProcessing logs data processing step with counts.
func (d *DebugLogger) LogDataProcessing(step string, count int) {
	d.log("data_processing",
		"step", step,
		"count", fmt.Sprintf("%d", count),
	)
}

// LogFilterSort logs filter/sort operations with input/output counts.
func (d *DebugLogger) LogFilterSort(operation, detail string, inputCount, outputCount int) {
	d.log("filter_sort",
		"operation", operation,
		"detail", detail,
		"input", fmt.Sprintf("%d", inputCount),
		"output", fmt.Sprintf("%d", outputCount),
	)
}

// LogTerminalWidth logs the detected terminal width.
func (d *DebugLogger) LogTerminalWidth(width int, detected bool) {
	d.log("terminal",
		"width", fmt.Sprintf("%d", width),
		"detected", fmt.Sprintf("%t", detected),
	)
}

// LogOutputFormat logs the output format being used.
func (d *DebugLogger) LogOutputFormat(format string) {
	d.log("output",
		"format", format,
	)
}
