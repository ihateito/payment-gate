package outbox

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// BrokerPublisher is the interface for pushing messages to the event bus (Kafka, Redpanda, RabbitMQ).
type BrokerPublisher interface {
	Publish(ctx context.Context, msg *Message) error
}

// Repository defines database operations for the transactional outbox table.
type Repository interface {
	// Store adds a new outbox message in the context of an existing DB transaction.
	Store(ctx context.Context, msg *Message) error

	// FetchBatch retrieves pending messages locked for update (SKIP LOCKED).
	FetchBatch(ctx context.Context, batchSize int) ([]*Message, error)

	// MarkPublished updates message status to PUBLISHED.
	MarkPublished(ctx context.Context, id uuid.UUID, publishedAt time.Time) error

	// MarkFailed updates error and schedules next retry or marks as FAILED.
	MarkFailed(ctx context.Context, id uuid.UUID, attempts int, nextRetryAt time.Time, lastErr error) error
}
