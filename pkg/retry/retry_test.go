package retry

import (
	"context"
	"errors"
	"fmt"
	"net"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// --- DefaultRetryConfig tests ---

func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()
	assert.Equal(t, 5, cfg.MaxRetries)
	assert.Equal(t, 100*time.Millisecond, cfg.InitialBackoff)
	assert.Equal(t, 1600*time.Millisecond, cfg.MaxBackoff)
	assert.Equal(t, 0.5, cfg.JitterFraction)
	assert.Nil(t, cfg.DebugLogger)
}

// --- IsTransientError tests ---

func TestIsTransientError_Nil(t *testing.T) {
	assert.False(t, IsTransientError(nil))
}

func TestIsTransientError_NetworkTimeout(t *testing.T) {
	err := &net.DNSError{IsTimeout: true}
	assert.True(t, IsTransientError(err))
}

func TestIsTransientError_ConnectionRefused(t *testing.T) {
	err := &net.OpError{
		Op:   "dial",
		Net:  "tcp",
		Addr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 6443},
		Err: &net.OpError{
			Op:  "dial",
			Net: "tcp",
			Err: fmt.Errorf("connect: %w", syscall.ECONNREFUSED),
		},
	}
	assert.True(t, IsTransientError(err))
}

func TestIsTransientError_ConnectionRefusedDirect(t *testing.T) {
	err := fmt.Errorf("connection error: %w", syscall.ECONNREFUSED)
	assert.True(t, IsTransientError(err))
}

func TestIsTransientError_429TooManyRequests(t *testing.T) {
	err := apierrors.NewTooManyRequests("rate limited", 5)
	assert.True(t, IsTransientError(err))
}

func TestIsTransientError_500InternalServerError(t *testing.T) {
	err := apierrors.NewInternalError(errors.New("internal"))
	assert.True(t, IsTransientError(err))
}

func TestIsTransientError_502BadGateway(t *testing.T) {
	err := &apierrors.StatusError{
		ErrStatus: metav1.Status{
			Code:   502,
			Reason: metav1.StatusReasonUnknown,
		},
	}
	assert.True(t, IsTransientError(err))
}

func TestIsTransientError_503ServiceUnavailable(t *testing.T) {
	err := apierrors.NewServiceUnavailable("unavailable")
	assert.True(t, IsTransientError(err))
}

func TestIsTransientError_504GatewayTimeout(t *testing.T) {
	err := &apierrors.StatusError{
		ErrStatus: metav1.Status{
			Code:   504,
			Reason: metav1.StatusReasonTimeout,
		},
	}
	assert.True(t, IsTransientError(err))
}

func TestIsTransientError_401Unauthorized(t *testing.T) {
	err := apierrors.NewUnauthorized("unauthorized")
	assert.False(t, IsTransientError(err))
}

func TestIsTransientError_403Forbidden(t *testing.T) {
	err := apierrors.NewForbidden(schema.GroupResource{Group: "", Resource: "nodes"}, "test", errors.New("forbidden"))
	assert.False(t, IsTransientError(err))
}

func TestIsTransientError_404NotFound(t *testing.T) {
	err := apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "nodes"}, "test")
	assert.False(t, IsTransientError(err))
}

func TestIsTransientError_GenericError(t *testing.T) {
	err := errors.New("some random error")
	assert.False(t, IsTransientError(err))
}

// --- CalculateBackoff tests ---

func TestCalculateBackoff_ExponentialGrowth(t *testing.T) {
	cfg := RetryConfig{
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     1600 * time.Millisecond,
		JitterFraction: 0.0, // No jitter for deterministic testing.
	}

	// Without jitter, backoff should be exactly: 100, 200, 400, 800, 1600.
	expected := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		400 * time.Millisecond,
		800 * time.Millisecond,
		1600 * time.Millisecond,
	}

	for i, exp := range expected {
		got := CalculateBackoff(cfg, i)
		assert.Equal(t, exp, got, "attempt %d", i)
	}
}

func TestCalculateBackoff_CappedAtMax(t *testing.T) {
	cfg := RetryConfig{
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     1600 * time.Millisecond,
		JitterFraction: 0.0,
	}

	// Attempt 5 would be 3200ms without cap, should be capped at 1600ms.
	got := CalculateBackoff(cfg, 5)
	assert.Equal(t, 1600*time.Millisecond, got)
}

func TestCalculateBackoff_JitterWithinBounds(t *testing.T) {
	cfg := RetryConfig{
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     1600 * time.Millisecond,
		JitterFraction: 0.5,
	}

	// Run multiple times to check jitter bounds.
	for attempt := 0; attempt < 5; attempt++ {
		base := float64(cfg.InitialBackoff) * float64(int(1)<<attempt) //nolint:gosec // attempt is always 0-4 in this test
		if base > float64(cfg.MaxBackoff) {
			base = float64(cfg.MaxBackoff)
		}
		maxWithJitter := time.Duration(base * (1 + cfg.JitterFraction))

		for i := 0; i < 50; i++ {
			got := CalculateBackoff(cfg, attempt)
			assert.GreaterOrEqual(t, got, time.Duration(base), "attempt %d, iteration %d", attempt, i)
			assert.LessOrEqual(t, got, maxWithJitter, "attempt %d, iteration %d", attempt, i)
		}
	}
}

// --- WithRetry tests ---

func TestWithRetry_SuccessOnFirstAttempt(t *testing.T) {
	cfg := DefaultRetryConfig()
	cfg.InitialBackoff = 1 * time.Millisecond // Fast for testing.

	callCount := 0
	err := WithRetry(context.Background(), cfg, "test-op", func() error {
		callCount++
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

func TestWithRetry_SuccessAfterTransientErrors(t *testing.T) {
	cfg := DefaultRetryConfig()
	cfg.InitialBackoff = 1 * time.Millisecond

	callCount := 0
	err := WithRetry(context.Background(), cfg, "test-op", func() error {
		callCount++
		if callCount < 3 {
			return apierrors.NewTooManyRequests("rate limited", 1)
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 3, callCount)
}

func TestWithRetry_PermanentErrorNoRetry(t *testing.T) {
	cfg := DefaultRetryConfig()
	cfg.InitialBackoff = 1 * time.Millisecond

	callCount := 0
	err := WithRetry(context.Background(), cfg, "test-op", func() error {
		callCount++
		return apierrors.NewUnauthorized("unauthorized")
	})

	assert.Error(t, err)
	assert.Equal(t, 1, callCount) // No retries for permanent errors.
	assert.True(t, apierrors.IsUnauthorized(err))
}

func TestWithRetry_ExhaustsRetries(t *testing.T) {
	cfg := DefaultRetryConfig()
	cfg.InitialBackoff = 1 * time.Millisecond
	cfg.MaxRetries = 3

	callCount := 0
	err := WithRetry(context.Background(), cfg, "test-op", func() error {
		callCount++
		return apierrors.NewInternalError(errors.New("server error"))
	})

	require.Error(t, err)
	// 1 initial + 3 retries = 4 total calls.
	assert.Equal(t, 4, callCount)
	assert.Contains(t, err.Error(), "failed after 3 retries")
}

func TestWithRetry_ContextCancellation(t *testing.T) {
	cfg := DefaultRetryConfig()
	cfg.InitialBackoff = 500 * time.Millisecond // Slow enough to cancel during backoff.

	ctx, cancel := context.WithCancel(context.Background())

	callCount := 0
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := WithRetry(ctx, cfg, "test-op", func() error {
		callCount++
		return apierrors.NewTooManyRequests("rate limited", 1)
	})

	assert.Error(t, err)
	assert.True(t, callCount >= 1 && callCount <= 2, "expected 1-2 calls, got %d", callCount)
}

func TestWithRetry_ContextAlreadyCancelled(t *testing.T) {
	cfg := DefaultRetryConfig()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	err := WithRetry(ctx, cfg, "test-op", func() error {
		t.Fatal("should not be called")
		return nil
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context cancelled")
}
