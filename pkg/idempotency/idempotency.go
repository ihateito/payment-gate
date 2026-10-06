package idempotency

import (
	"context"
	"errors"
	"time"
)

var (
	ErrLocked       = errors.New("concurrent request in progress for idempotency key")
	ErrKeyNotFound  = errors.New("idempotency key not found")
	ErrInvalidState = errors.New("invalid idempotency state")
)

type Status string

const (
	StatusInProgress Status = "IN_PROGRESS"
	StatusCompleted  Status = "COMPLETED"
	StatusFailed     Status = "FAILED"
)

// ResponseRecord stores the captured response to replay upon repeated requests.
type ResponseRecord struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       []byte            `json:"body"`
	CreatedAt  time.Time         `json:"created_at"`
}

// AcquireResult indicates the outcome of acquiring an idempotency key.
type AcquireResult struct {
	Status   Status
	Response *ResponseRecord // Populated if Status == StatusCompleted
}

// Storage is the contract for storing and locking idempotency keys.
type Storage interface {
	// TryAcquire attempts to claim the key atomically.
	// If the key is new: creates it with StatusInProgress and returns StatusInProgress.
	// If the key is already InProgress: returns ErrLocked.
	// If the key was already Completed: returns StatusCompleted and the cached ResponseRecord.
	TryAcquire(ctx context.Context, key string, ttl time.Duration) (*AcquireResult, error)

	// Complete saves the finalized response and marks the key as StatusCompleted.
	Complete(ctx context.Context, key string, resp ResponseRecord, ttl time.Duration) error

	// Release removes or frees the key (e.g. if an early pre-processing error occurred).
	Release(ctx context.Context, key string) error
}
