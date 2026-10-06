package main

import (
	"context"
	"fmt"
	"time"

	"payment-gate/pkg/logger"
	"payment-gate/pkg/outbox"
	"payment-gate/pkg/shutdown"
)

// ConsolePublisher is a mock/development broker publisher.
type ConsolePublisher struct{}

func (c *ConsolePublisher) Publish(ctx context.Context, msg *outbox.Message) error {
	fmt.Printf("[KAFKA PUBLISH] Topic: %s | Key: %s | Payload: %s\n", msg.Topic, msg.Key, string(msg.Payload))
	return nil
}

func main() {
	l := logger.New(logger.Config{
		Level:  "DEBUG",
		Format: "json",
	})
	l.Info("starting outbox-worker microservice")

	coordinator := shutdown.NewCoordinator(10*time.Second, l)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	coordinator.Register("worker_cancel", func(ctx context.Context) error {
		cancel()
		return nil
	})

	l.Info("outbox worker initialized and listening for events")

	if err := coordinator.ListenAndWait(ctx); err != nil {
		l.Error("shutdown error", "error", err)
	}
}
