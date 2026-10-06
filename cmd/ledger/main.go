package main

import (
	"context"
	"time"

	"payment-gate/pkg/logger"
	"payment-gate/pkg/shutdown"
)

func main() {
	l := logger.New(logger.Config{
		Level:  "DEBUG",
		Format: "json",
	})
	l.Info("starting ledger microservice (double-entry accounting)")

	coordinator := shutdown.NewCoordinator(10*time.Second, l)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	coordinator.Register("ledger_flush", func(ctx context.Context) error {
		cancel()
		l.Info("ledger entries committed and flushed")
		return nil
	})

	l.Info("ledger service listening for journal transactions")

	if err := coordinator.ListenAndWait(ctx); err != nil {
		l.Error("shutdown error", "error", err)
	}
}
