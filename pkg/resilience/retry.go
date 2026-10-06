package resilience

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// RetryConfig configures the retry policy.
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Jitter      bool
}

// DefaultRetryConfig returns standard retry settings.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    2 * time.Second,
		Jitter:      true,
	}
}

// RetryableFunc is an operation that can be retried.
type RetryableFunc func(ctx context.Context, attempt int) error

// Retry executes an operation with exponential backoff and jitter.
func Retry(ctx context.Context, cfg RetryConfig, op RetryableFunc) error {
	var err error
	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err = op(ctx, attempt)
		if err == nil {
			return nil
		}

		if attempt == cfg.MaxAttempts {
			break
		}

		// Calculate backoff
		delay := float64(cfg.BaseDelay) * math.Pow(2, float64(attempt-1))
		if delay > float64(cfg.MaxDelay) {
			delay = float64(cfg.MaxDelay)
		}

		if cfg.Jitter {
			// Full jitter: uniformly random between 0 and calculated delay
			delay = rand.Float64() * delay
		}

		sleepDuration := time.Duration(delay)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sleepDuration):
		}
	}
	return err
}
