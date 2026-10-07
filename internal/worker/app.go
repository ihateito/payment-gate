package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"payment-gate/internal/config"
	"payment-gate/pkg/outbox"
	"payment-gate/pkg/postgres"
	"payment-gate/pkg/shutdown"
)

// App manages the lifecycle of the outbox background worker.
type App struct {
	cfg         *config.Config
	logger      *slog.Logger
	coordinator *shutdown.Coordinator
	worker      *outbox.Worker
}

// NewApp initializes database connections and configures the outbox poller.
func NewApp(
	ctx context.Context,
	cfg *config.Config,
	logger *slog.Logger,
	coordinator *shutdown.Coordinator,
) (*App, error) {
	pgClient, err := postgres.NewClient(ctx, postgres.Config{
		URL:           cfg.DatabaseURL,
		MaxConns:      cfg.DBMaxConns,
		MinConns:      cfg.DBMinConns,
		HealthTimeout: cfg.DBHealthTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	logger.Info("connected to postgresql successfully")

	coordinator.Register("postgres", func(sCtx context.Context) error {
		pgClient.Close()
		logger.Info("postgres connection pool closed")
		return nil
	})

	outboxRepo := outbox.NewPostgresRepository(pgClient.Pool)
	publisher := NewConsolePublisher(logger)
	worker := outbox.NewWorker(outbox.DefaultWorkerConfig(), outboxRepo, publisher, logger)

	return &App{
		cfg:         cfg,
		logger:      logger,
		coordinator: coordinator,
		worker:      worker,
	}, nil
}

// Run starts the outbox polling loop and blocks until graceful shutdown is complete.
func (a *App) Run(ctx context.Context) error {
	workerCtx, workerCancel := context.WithCancel(ctx)
	defer workerCancel()

	a.coordinator.Register("worker_cancel", func(sCtx context.Context) error {
		workerCancel()
		return nil
	})

	workerErr := make(chan error, 1)
	go func() {
		a.logger.Info("outbox worker initialized and polling outbox_messages table")
		if err := a.worker.Run(workerCtx); err != nil && !errors.Is(err, context.Canceled) {
			workerErr <- err
		}
	}()

	select {
	case err := <-workerErr:
		return fmt.Errorf("worker run failure: %w", err)
	default:
	}

	return a.coordinator.ListenAndWait(ctx)
}
