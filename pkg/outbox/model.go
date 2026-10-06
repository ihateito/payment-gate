package outbox

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type MessageStatus string

const (
	StatusPending   MessageStatus = "PENDING"
	StatusPublished MessageStatus = "PUBLISHED"
	StatusFailed    MessageStatus = "FAILED"
)

// Message represents an event to be reliably dispatched via message broker.
type Message struct {
	ID          uuid.UUID         `json:"id"`
	Topic       string            `json:"topic"`
	Key         string            `json:"key"`
	Payload     []byte            `json:"payload"`
	Headers     map[string]string `json:"headers"`
	Status      MessageStatus     `json:"status"`
	Attempts    int               `json:"attempts"`
	LastError   string            `json:"last_error,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	NextRetryAt *time.Time        `json:"next_retry_at,omitempty"`
	PublishedAt *time.Time        `json:"published_at,omitempty"`
}

// NewMessage creates a pending outbox message with UUID v7 or v4.
func NewMessage(topic, key string, payload any, headers map[string]string) (*Message, error) {
	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	if headers == nil {
		headers = make(map[string]string)
	}

	return &Message{
		ID:        uuid.New(),
		Topic:     topic,
		Key:       key,
		Payload:   bytes,
		Headers:   headers,
		Status:    StatusPending,
		Attempts:  0,
		CreatedAt: time.Now().UTC(),
	}, nil
}
