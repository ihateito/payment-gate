package outbox

import (
	"context"
	"log/slog"
	"math"
	"math/rand"
	"time"
)

// WorkerConfig holds tuning parameters for outbox processor.
type WorkerConfig struct {
	BatchSize      int
	PollInterval   time.Duration
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

// DefaultWorkerConfig provides safe production defaults.
func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{
		BatchSize:      50,
		PollInterval:   200 * time.Millisecond,
		MaxRetries:     5,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     1 * time.Minute,
	}
}

// Worker continuously polls and relays outbox messages to the message broker.
type Worker struct {
	cfg       WorkerConfig
	repo      Repository
	publisher BrokerPublisher
	logger    *slog.Logger
}

// NewWorker initializes an outbox background worker.
func NewWorker(cfg WorkerConfig, repo Repository, pub BrokerPublisher, l *slog.Logger) *Worker {
	if l == nil {
		l = slog.Default()
	}
	return &Worker{
		cfg:       cfg,
		repo:      repo,
		publisher: pub,
		logger:    l.With("component", "outbox_worker"),
	}
}

// Run starts the processing loop until context cancellation.
func (w *Worker) Run(ctx context.Context) error {
	w.logger.Info("starting outbox worker", "poll_interval", w.cfg.PollInterval, "batch_size", w.cfg.BatchSize)
	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("shutting down outbox worker")
			return ctx.Err()
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

func (w *Worker) processBatch(ctx context.Context) {
	messages, err := w.repo.FetchBatch(ctx, w.cfg.BatchSize)
	if err != nil {
		w.logger.Error("failed to fetch outbox batch", "error", err)
		return
	}

	if len(messages) == 0 {
		return
	}

	for _, msg := range messages {
		// Stop processing batch if context is cancelled
		if ctx.Err() != nil {
			return
		}

		pubErr := w.publisher.Publish(ctx, msg)
		if pubErr == nil {
			if err := w.repo.MarkPublished(ctx, msg.ID, time.Now().UTC()); err != nil {
				w.logger.Error("failed to mark message published", "id", msg.ID, "error", err)
			}
			continue
		}

		// Handle failure with exponential backoff & jitter
		msg.Attempts++
		backoff := w.calculateBackoff(msg.Attempts)
		nextRetry := time.Now().UTC().Add(backoff)

		w.logger.Warn("failed to publish outbox message",
			"id", msg.ID,
			"topic", msg.Topic,
			"attempts", msg.Attempts,
			"next_retry", nextRetry,
			"error", pubErr,
		)

		if err := w.repo.MarkFailed(ctx, msg.ID, msg.Attempts, nextRetry, pubErr); err != nil {
			w.logger.Error("failed to mark outbox failure", "id", msg.ID, "error", err)
		}
	}
}

func (w *Worker) calculateBackoff(attempt int) time.Duration {
	mult := math.Pow(2, float64(attempt-1))
	backoff := time.Duration(float64(w.cfg.InitialBackoff) * mult)
	if backoff > w.cfg.MaxBackoff {
		backoff = w.cfg.MaxBackoff
	}
	// Add 20% jitter
	jitter := time.Duration(rand.Int63n(int64(backoff / 5)))
	return backoff + jitter
}
