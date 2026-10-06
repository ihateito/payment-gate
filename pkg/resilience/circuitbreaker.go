package resilience

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrCircuitOpen = errors.New("circuit breaker is OPEN")
)

type State string

const (
	StateClosed   State = "CLOSED"
	StateHalfOpen State = "HALF_OPEN"
	StateOpen     State = "OPEN"
)

// CircuitBreakerConfig configures the failure thresholds and reset intervals.
type CircuitBreakerConfig struct {
	MaxFailures  int           // Number of consecutive failures to trip into OPEN
	ResetTimeout time.Duration // Duration to wait before switching from OPEN to HALF_OPEN
	HalfOpenMax  int           // Successful requests needed in HALF_OPEN to close the breaker
}

// CircuitBreaker prevents cascading failures when downstream services are degraded.
type CircuitBreaker struct {
	mu           sync.Mutex
	cfg          CircuitBreakerConfig
	state        State
	failures     int
	successes    int
	lastOpenedAt time.Time
}

// NewCircuitBreaker creates a new CircuitBreaker.
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	if cfg.MaxFailures <= 0 {
		cfg.MaxFailures = 5
	}
	if cfg.ResetTimeout <= 0 {
		cfg.ResetTimeout = 30 * time.Second
	}
	if cfg.HalfOpenMax <= 0 {
		cfg.HalfOpenMax = 2
	}
	return &CircuitBreaker{
		cfg:   cfg,
		state: StateClosed,
	}
}

// State returns current breaker state.
func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.checkStateChange()
	return cb.state
}

func (cb *CircuitBreaker) checkStateChange() {
	if cb.state == StateOpen && time.Since(cb.lastOpenedAt) > cb.cfg.ResetTimeout {
		cb.state = StateHalfOpen
		cb.failures = 0
		cb.successes = 0
	}
}

// Execute runs the operation if allowed by the circuit breaker.
func (cb *CircuitBreaker) Execute(ctx context.Context, op func(ctx context.Context) error) error {
	cb.mu.Lock()
	cb.checkStateChange()

	if cb.state == StateOpen {
		cb.mu.Unlock()
		return ErrCircuitOpen
	}
	cb.mu.Unlock()

	// Execute operation
	err := op(ctx)

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.onFailure()
		return err
	}

	cb.onSuccess()
	return nil
}

func (cb *CircuitBreaker) onFailure() {
	if cb.state == StateHalfOpen {
		cb.state = StateOpen
		cb.lastOpenedAt = time.Now()
		return
	}

	cb.failures++
	if cb.failures >= cb.cfg.MaxFailures {
		cb.state = StateOpen
		cb.lastOpenedAt = time.Now()
	}
}

func (cb *CircuitBreaker) onSuccess() {
	if cb.state == StateHalfOpen {
		cb.successes++
		if cb.successes >= cb.cfg.HalfOpenMax {
			cb.state = StateClosed
			cb.failures = 0
			cb.successes = 0
		}
		return
	}
	cb.failures = 0
}
