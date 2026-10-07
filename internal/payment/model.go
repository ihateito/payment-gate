package payment

import (
	"time"

	"github.com/google/uuid"
	"payment-gate/pkg/money"
)

type Status string

const (
	StatusPending    Status = "PENDING"
	StatusAuthorized Status = "AUTHORIZED"
	StatusCaptured   Status = "CAPTURED"
	StatusFailed     Status = "FAILED"
)

// Payment represents the core domain model for a transaction.
type Payment struct {
	ID             uuid.UUID
	IdempotencyKey string
	MerchantID     uuid.UUID
	CustomerID     string
	Amount         int64
	Currency       string
	Status         Status
	Provider       string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// CreatePaymentRequest represents client payload for charging payments.
type CreatePaymentRequest struct {
	MerchantID uuid.UUID `json:"merchant_id"`
	CustomerID string    `json:"customer_id"`
	Amount     int64     `json:"amount"` // in minor currency units (cents, kopecks)
	Currency   string    `json:"currency"`
	Method     string    `json:"method"`
}

// PaymentResponse represents client response after charge initiation.
type PaymentResponse struct {
	ID        uuid.UUID   `json:"id"`
	Status    Status      `json:"status"`
	Amount    money.Money `json:"amount"`
	CreatedAt time.Time   `json:"created_at"`
}

// PaymentDetail represents enriched payment details including outbox dispatch status.
type PaymentDetail struct {
	ID             uuid.UUID   `json:"id"`
	IdempotencyKey string      `json:"idempotency_key"`
	MerchantID     uuid.UUID   `json:"merchant_id"`
	CustomerID     string      `json:"customer_id"`
	Amount         money.Money `json:"amount"`
	Status         Status      `json:"status"`
	OutboxStatus   string      `json:"outbox_status"`
	CreatedAt      time.Time   `json:"created_at"`
}
