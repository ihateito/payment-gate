package gateway

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"payment-gate/internal/config"
	"payment-gate/internal/payment"
	"payment-gate/pkg/idempotency"
	"payment-gate/pkg/postgres"
	"payment-gate/pkg/shutdown"
)

// App manages the API Gateway runtime components.
type App struct {
	cfg         *config.Config
	logger      *slog.Logger
	coordinator *shutdown.Coordinator
	server      *http.Server
}

// NewApp initializes dependencies, database, redis, and routes.
func NewApp(
	ctx context.Context,
	cfg *config.Config,
	logger *slog.Logger,
	coordinator *shutdown.Coordinator,
) (*App, error) {
	// 1. Establish PostgreSQL connection pool
	pgClient, err := postgres.NewClient(ctx, postgres.Config{
		URL:             cfg.DatabaseURL,
		MaxConns:        cfg.DBMaxConns,
		MinConns:        cfg.DBMinConns,
		HealthTimeout:   cfg.DBHealthTimeout,
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

	// 2. Setup Idempotency storage (Redis with in-memory fallback)
	var idempStorage idempotency.Storage = idempotency.NewMemoryStorage()
	if cfg.RedisAddr != "" {
		rdb := redis.NewClient(&redis.Options{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
		})

		pingCtx, pingCancel := context.WithTimeout(ctx, 2*time.Second)
		if err := rdb.Ping(pingCtx).Err(); err == nil {
			idempStorage = idempotency.NewRedisStorage(rdb, "idempotency:")
			logger.Info("connected to redis for idempotency storage", "addr", cfg.RedisAddr)
			coordinator.Register("redis", func(sCtx context.Context) error {
				return rdb.Close()
			})
		} else {
			logger.Warn("redis unavailable, using in-memory idempotency fallback", "error", err)
		}
		pingCancel()
	}

	// 3. Domain repositories and HTTP handlers
	paymentRepo := payment.NewPostgresRepository(pgClient)
	paymentHandler := payment.NewHandler(paymentRepo, pgClient, idempStorage, cfg.KafkaPaymentTopic, logger)

	mux := http.NewServeMux()
	paymentHandler.RegisterRoutes(mux)

	// 4. Serve web dashboard frontend if directory exists
	mux.Handle("GET /", http.FileServer(http.Dir("./web")))

	// 5. Wrap router with CORS middleware for seamless browser integration
	corsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Idempotency-Key")
		w.Header().Set("Access-Control-Expose-Headers", "Idempotent-Replayed")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		mux.ServeHTTP(w, r)
	})

	server := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      corsHandler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	coordinator.Register("http_server", func(sCtx context.Context) error {
		return server.Shutdown(sCtx)
	})

	return &App{
		cfg:         cfg,
		logger:      logger,
		coordinator: coordinator,
		server:      server,
	}, nil
}

// Run starts the HTTP server and blocks until graceful shutdown is complete.
func (a *App) Run(ctx context.Context) error {
	serverErr := make(chan error, 1)

	go func() {
		a.logger.Info("api-gateway listening", "addr", a.server.Addr)
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("http server failed: %w", err)
	default:
	}

	return a.coordinator.ListenAndWait(ctx)
}
