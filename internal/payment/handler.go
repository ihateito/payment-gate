package payment

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"payment-gate/pkg/idempotency"
	"payment-gate/pkg/money"
	"payment-gate/pkg/postgres"
)

// Handler handles HTTP requests related to payment processing.
type Handler struct {
	repo         Repository
	pgClient     *postgres.Client
	idempStorage idempotency.Storage
	topic        string
	logger       *slog.Logger
}

// NewHandler constructs a new Payment HTTP handler.
func NewHandler(
	repo Repository,
	pgClient *postgres.Client,
	idempStorage idempotency.Storage,
	topic string,
	l *slog.Logger,
) *Handler {
	return &Handler{
		repo:         repo,
		pgClient:     pgClient,
		idempStorage: idempStorage,
		topic:        topic,
		logger:       l.With("handler", "payment"),
	}
}

// RegisterRoutes registers payment endpoints on the HTTP router.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", h.HealthCheck)
	mux.HandleFunc("GET /v1/payments", h.ListPayments)
	mux.Handle("POST /v1/payments", idempotency.Middleware(h.idempStorage, 24*time.Hour)(http.HandlerFunc(h.CreatePayment)))
}

// ListPayments returns recent payments for visualization in the UI.
func (h *Handler) ListPayments(w http.ResponseWriter, r *http.Request) {
	payments, err := h.repo.ListRecent(r.Context(), 50)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to list payments", "error", err)
		http.Error(w, fmt.Sprintf(`{"error":"failed to query payments: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payments)
}


// HealthCheck verifies service and database liveness.
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.pgClient != nil {
		if err := h.pgClient.Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write(fmt.Appendf(nil, `{"status":"DOWN","service":"api-gateway","database":"error: %s"}`, err))
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"UP","service":"api-gateway","database":"healthy"}`))
}

// CreatePayment handles initial payment authorization and outbox event queuing.
func (h *Handler) CreatePayment(w http.ResponseWriter, r *http.Request) {
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

	idempKey := r.Header.Get(idempotency.HeaderIdempotencyKey)
	if idempKey == "" {
		idempKey = uuid.New().String()
	}

	paymentID := uuid.New()
	now := time.Now().UTC()

	p := &Payment{
		ID:             paymentID,
		IdempotencyKey: idempKey,
		MerchantID:     req.MerchantID,
		CustomerID:     req.CustomerID,
		Amount:         mon.Amount,
		Currency:       string(mon.Currency),
		Status:         StatusPending,
		Provider:       "internal",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	eventPayload := map[string]any{
		"event_type":  "payment.created",
		"payment_id":  paymentID,
		"merchant_id": req.MerchantID,
		"customer_id": req.CustomerID,
		"amount":      mon.Amount,
		"currency":    string(mon.Currency),
		"created_at":  now,
	}

	if err := h.repo.CreateWithOutbox(r.Context(), p, eventPayload, h.topic); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to persist payment transaction", "error", err)
		http.Error(w, fmt.Sprintf(`{"error":"database transaction error: %s"}`, err), http.StatusInternalServerError)
		return
	}

	h.logger.InfoContext(r.Context(), "payment created and outbox event queued in postgres",
		"payment_id", paymentID,
		"merchant_id", req.MerchantID,
		"amount", mon.String(),
		"idempotency_key", idempKey,
	)

	resp := PaymentResponse{
		ID:        paymentID,
		Status:    StatusPending,
		Amount:    mon,
		CreatedAt: now,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}
