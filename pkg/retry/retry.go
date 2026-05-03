package retry

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"net"
	"syscall"
	"time"

	"github.com/kubectl-k8i/pkg/debug"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// RetryConfig holds retry configuration.
type RetryConfig struct {
	MaxRetries     int                // Maximum number of retry attempts (default: 5)
	InitialBackoff time.Duration      // Initial backoff duration (default: 100ms)
	MaxBackoff     time.Duration      // Maximum backoff duration (default: 1600ms)
	JitterFraction float64            // Jitter as fraction of backoff (default: 0.5)
	DebugLogger    *debug.DebugLogger // Optional debug logger for retry attempts
}

// DefaultRetryConfig returns the default retry configuration.
// MaxRetries=5, InitialBackoff=100ms, MaxBackoff=1600ms, JitterFraction=0.5
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     5,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     1600 * time.Millisecond,
		JitterFraction: 0.5,
	}
}

// IsTransientError returns true if the error is transient and should be retried.
// Transient: network timeouts, connection refused, 429 Too Many Requests, 5xx server errors.
// Permanent (not retried): 401 Unauthorized, 403 Forbidden, 404 Not Found.
func IsTransientError(err error) bool {
	if err == nil {
		return false
	}

	// Check for network timeout errors.
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	// Check for connection refused (syscall.ECONNREFUSED).
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		var sysErr *syscall.Errno
		if errors.As(opErr.Err, &sysErr) && *sysErr == syscall.ECONNREFUSED {
			return true
		}
		// Also check if the inner error wraps ECONNREFUSED directly.
		if errors.Is(opErr.Err, syscall.ECONNREFUSED) {
			return true
		}
	}
	// Also check directly for ECONNREFUSED in case it's not wrapped in OpError.
	if errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}

	// Check for Kubernetes API errors using k8s.io/apimachinery.
	var statusErr *apierrors.StatusError
	if errors.As(err, &statusErr) {
		code := statusErr.Status().Code
		// 429 Too Many Requests is transient.
		if code == 429 {
			return true
		}
		// 5xx server errors are transient.
		if code >= 500 && code < 600 {
			return true
		}
		// 401, 403, 404 are permanent — not transient.
		if code == 401 || code == 403 || code == 404 {
			return false
		}
	}

	// Also check using apierrors helpers for broader coverage.
	if apierrors.IsTooManyRequests(err) {
		return true
	}
	if apierrors.IsServerTimeout(err) {
		return true
	}
	if apierrors.IsInternalError(err) {
		return true
	}
	if apierrors.IsServiceUnavailable(err) {
		return true
	}

	return false
}

// CalculateBackoff returns the backoff duration for the given attempt number.
// Uses exponential backoff: initialBackoff * 2^attempt, capped at maxBackoff.
// Adds random jitter up to jitterFraction of the calculated backoff.
func CalculateBackoff(config RetryConfig, attempt int) time.Duration {
	// Calculate base backoff: initialBackoff * 2^attempt.
	base := float64(config.InitialBackoff) * math.Pow(2, float64(attempt))

	// Cap at maxBackoff.
	if base > float64(config.MaxBackoff) {
		base = float64(config.MaxBackoff)
	}

	// Add random jitter: up to jitterFraction of the base backoff.
	jitter := base * config.JitterFraction * rand.Float64() // #nosec G404 -- jitter does not require crypto-grade randomness

	return time.Duration(base + jitter)
}

// WithRetry wraps a function with retry logic using exponential backoff with jitter.
// Retries only on transient errors; returns immediately on permanent errors.
// On exhaustion, returns error with retry count info.
func WithRetry(ctx context.Context, config RetryConfig, operation string, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Check context before each attempt.
		if err := ctx.Err(); err != nil {
			if lastErr != nil {
				return fmt.Errorf("%s: context cancelled after %d retries: %w (last error: %v)", operation, attempt, err, lastErr)
			}
			return fmt.Errorf("%s: context cancelled: %w", operation, err)
		}

		lastErr = fn()
		if lastErr == nil {
			return nil // Success.
		}

		// If the error is not transient, return immediately.
		if !IsTransientError(lastErr) {
			return lastErr
		}

		// If we've exhausted all retries, break out.
		if attempt == config.MaxRetries {
			break
		}

		// Calculate backoff and sleep.
		backoff := CalculateBackoff(config, attempt)

		// Log retry attempt if debug logger is available.
		if config.DebugLogger != nil && config.DebugLogger.IsEnabled() {
			config.DebugLogger.LogRetryAttempt(operation, attempt+1, config.MaxRetries, backoff, lastErr)
		}

		// Wait for backoff duration or context cancellation.
		select {
		case <-ctx.Done():
			return fmt.Errorf("%s: context cancelled during backoff after %d retries: %w (last error: %v)", operation, attempt+1, ctx.Err(), lastErr)
		case <-time.After(backoff):
			// Continue to next attempt.
		}
	}

	return fmt.Errorf("%s: failed after %d retries: %w", operation, config.MaxRetries, lastErr)
}
