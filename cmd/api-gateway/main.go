package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"payment-gate/pkg/idempotency"
	"payment-gate/pkg/logger"
	"payment-gate/pkg/money"
	"payment-gate/pkg/shutdown"
)

type CreatePaymentRequest struct {
	MerchantID uuid.UUID `json:"merchant_id"`
	CustomerID string    `json:"customer_id"`
	Amount     int64     `json:"amount"` // in minor units (cents)
	Currency   string    `json:"currency"`
	Method     string    `json:"method"`
}

type PaymentResponse struct {
	ID        uuid.UUID    `json:"id"`
	Status    string       `json:"status"`
	Amount    money.Money  `json:"amount"`
	CreatedAt time.Time    `json:"created_at"`
}

func main() {
	l := logger.New(logger.Config{
		Level:  "DEBUG",
		Format: "json",
	})
	l.Info("starting api-gateway microservice")

	// Shutdown coordinator
	coordinator := shutdown.NewCoordinator(10*time.Second, l)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Idempotency storage (In-memory for dev, Redis in production)
	idempStorage := idempotency.NewMemoryStorage()

	mux := http.NewServeMux()

	// Health check probe
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"UP","service":"api-gateway"}`))
	})

	// Payment charge endpoint (protected by idempotency middleware)
	paymentHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req CreatePaymentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"invalid request payload: %s"}`, err), http.StatusBadRequest)
			return
		}

		mon, err := money.New(req.Amount, money.Currency(req.Currency))
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err), http.StatusBadRequest)
			return
		}

		paymentID := uuid.New()
		l.InfoContext(r.Context(), "processing payment authorization",
			"payment_id", paymentID,
			"merchant_id", req.MerchantID,
			"amount", mon.String(),
		)

		// Emulate processing
		resp := PaymentResponse{
			ID:        paymentID,
			Status:    "AUTHORIZED",
			Amount:    mon,
			CreatedAt: time.Now().UTC(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(resp)
	})

	// Wrap payment endpoint with idempotency middleware
	mux.Handle("POST /v1/payments", idempotency.Middleware(idempStorage, 24*time.Hour)(paymentHandler))

	port := os.Getenv("GATEWAY_HTTP_PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	coordinator.Register("http_server", func(sCtx context.Context) error {
		return server.Shutdown(sCtx)
	})

	go func() {
		l.Info("api-gateway listening", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			l.Error("http server failed", "error", err)
			cancel()
		}
	}()

	if err := coordinator.ListenAndWait(ctx); err != nil {
		l.Error("shutdown error", "error", err)
	}
}
