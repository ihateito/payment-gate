package main

import (
	"context"
	"os"
	"time"

	"payment-gate/internal/config"
	"payment-gate/internal/worker"
	"payment-gate/pkg/logger"
	"payment-gate/pkg/shutdown"
)

func main() {
	cfg := config.Load()

	l := logger.New(logger.Config{
		Level:  cfg.LogLevel,
		Format: cfg.LogFormat,
	})
	l.Info("starting outbox-worker microservice", "env", cfg.AppEnv)

	coordinator := shutdown.NewCoordinator(10*time.Second, l)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app, err := worker.NewApp(ctx, cfg, l, coordinator)
	if err != nil {
		l.Error("failed to initialize worker application", "error", err)
		os.Exit(1)
	}

	if err := app.Run(ctx); err != nil {
		l.Error("worker application stopped with error", "error", err)
		os.Exit(1)
	}
}
