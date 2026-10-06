package resilience

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		MaxFailures:  2,
		ResetTimeout: 50 * time.Millisecond,
		HalfOpenMax:  1,
	})

	dummyErr := errors.New("provider down")

	// First failure
	_ = cb.Execute(context.Background(), func(ctx context.Context) error {
		return dummyErr
	})
	if cb.State() != StateClosed {
		t.Fatalf("expected CLOSED, got %s", cb.State())
	}

	// Second failure -> Trips to OPEN
	_ = cb.Execute(context.Background(), func(ctx context.Context) error {
		return dummyErr
	})
	if cb.State() != StateOpen {
		t.Fatalf("expected OPEN, got %s", cb.State())
	}

	// Fast-fail while OPEN
	err := cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})
	if err != ErrCircuitOpen {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}

	// Wait for reset timeout
	time.Sleep(60 * time.Millisecond)

	// In Half-Open, successful execution closes breaker
	err = cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	if cb.State() != StateClosed {
		t.Fatalf("expected CLOSED after recovery, got %s", cb.State())
	}
}

func TestRetry_SuccessAfterFailure(t *testing.T) {
	var attempts int32
	cfg := RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    50 * time.Millisecond,
		Jitter:      false,
	}

	err := Retry(context.Background(), cfg, func(ctx context.Context, attempt int) error {
		count := atomic.AddInt32(&attempts, 1)
		if count < 3 {
			return errors.New("temporary error")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("expected retry to succeed, got: %v", err)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}
