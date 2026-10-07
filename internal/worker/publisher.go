package worker

import (
	"context"
	"fmt"
	"log/slog"

	"payment-gate/pkg/outbox"
)

// ConsolePublisher is a development broker publisher that outputs dispatched events to stdout and structured logs.
type ConsolePublisher struct {
	logger *slog.Logger
}

// NewConsolePublisher creates a new ConsolePublisher instance.
func NewConsolePublisher(l *slog.Logger) *ConsolePublisher {
	return &ConsolePublisher{
		logger: l.With("publisher", "console"),
	}
}

// Publish prints and logs the outbox message.
func (c *ConsolePublisher) Publish(ctx context.Context, msg *outbox.Message) error {
	c.logger.InfoContext(ctx, "dispatched outbox event to message broker",
		"topic", msg.Topic,
		"key", msg.Key,
		"payload_size", len(msg.Payload),
	)

	fmt.Printf("\n[OUTBOX -> BROKER DISPATCH]\n  Topic:   %s\n  Key:     %s\n  Payload: %s\n\n",
		msg.Topic, msg.Key, string(msg.Payload),
	)
	return nil
}
