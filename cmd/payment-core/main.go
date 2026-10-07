package main

import (
	"context"
	"time"

	"payment-gate/internal/config"
	"payment-gate/pkg/logger"
	"payment-gate/pkg/shutdown"
)

func main() {
	cfg := config.Load()

	l := logger.New(logger.Config{
		Level:  cfg.LogLevel,
		Format: cfg.LogFormat,
	})
	l.Info("starting payment-core processing engine", "env", cfg.AppEnv)

	coordinator := shutdown.NewCoordinator(10*time.Second, l)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	coordinator.Register("core_cleanup", func(ctx context.Context) error {
		cancel()
		l.Info("payment-core state machines drained successfully")
		return nil
	})

	l.Info("payment-core engine ready to process transactions")

	if err := coordinator.ListenAndWait(ctx); err != nil {
		l.Error("shutdown error", "error", err)
	}
}
