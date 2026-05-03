package retry

import (
	"math"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"pgregory.net/rapid"
)

// Property 32: Transient vs permanent error classification
// For any HTTP status code, IsTransientError returns true for 429, 500-599
// and false for 401, 403, 404.
//
// **Validates: Requirements 23.1, 23.2**
func TestProperty32_TransientVsPermanentErrorClassification(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		statusCode := rapid.Int32Range(100, 599).Draw(t, "statusCode")

		err := &apierrors.StatusError{
			ErrStatus: metav1.Status{
				Code:   statusCode,
				Reason: metav1.StatusReasonUnknown,
			},
		}

		result := IsTransientError(err)

		switch {
		case statusCode == 429:
			if !result {
				t.Fatalf("expected IsTransientError=true for status %d (429 Too Many Requests)", statusCode)
			}
		case statusCode >= 500 && statusCode <= 599:
			if !result {
				t.Fatalf("expected IsTransientError=true for status %d (5xx server error)", statusCode)
			}
		case statusCode == 401:
			if result {
				t.Fatalf("expected IsTransientError=false for status %d (401 Unauthorized)", statusCode)
			}
		case statusCode == 403:
			if result {
				t.Fatalf("expected IsTransientError=false for status %d (403 Forbidden)", statusCode)
			}
		case statusCode == 404:
			if result {
				t.Fatalf("expected IsTransientError=false for status %d (404 Not Found)", statusCode)
			}
		default:
			// For other status codes, the function may return true or false
			// depending on apierrors helpers. We only assert the specified codes.
		}
	})
}

// Property 33: Exponential backoff with jitter bounds
// For any attempt number n, the calculated backoff SHALL be within
// [base, base*(1+jitterFraction)] where base = min(initialBackoff * 2^n, maxBackoff).
//
// **Validates: Requirements 23.3, 23.8**
func TestProperty33_ExponentialBackoffWithJitterBounds(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a valid retry config.
		initialMs := rapid.Int64Range(10, 500).Draw(t, "initialMs")
		maxMs := rapid.Int64Range(initialMs, initialMs*64).Draw(t, "maxMs")
		jitterFraction := rapid.Float64Range(0.0, 1.0).Draw(t, "jitterFraction")
		attempt := rapid.IntRange(0, 10).Draw(t, "attempt")

		config := RetryConfig{
			InitialBackoff: time.Duration(initialMs) * time.Millisecond,
			MaxBackoff:     time.Duration(maxMs) * time.Millisecond,
			JitterFraction: jitterFraction,
		}

		backoff := CalculateBackoff(config, attempt)

		// Calculate expected base: initialBackoff * 2^attempt, capped at maxBackoff.
		base := float64(config.InitialBackoff) * math.Pow(2, float64(attempt))
		if base > float64(config.MaxBackoff) {
			base = float64(config.MaxBackoff)
		}

		minBackoff := time.Duration(base)
		maxBackoff := time.Duration(base * (1 + jitterFraction))

		if backoff < minBackoff {
			t.Fatalf("backoff %v is below minimum %v (attempt=%d, initial=%dms, max=%dms, jitter=%.2f)",
				backoff, minBackoff, attempt, initialMs, maxMs, jitterFraction)
		}
		if backoff > maxBackoff {
			t.Fatalf("backoff %v exceeds maximum %v (attempt=%d, initial=%dms, max=%dms, jitter=%.2f)",
				backoff, maxBackoff, attempt, initialMs, maxMs, jitterFraction)
		}
	})
}
