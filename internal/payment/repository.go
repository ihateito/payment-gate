package payment

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"payment-gate/pkg/money"
	"payment-gate/pkg/outbox"
	"payment-gate/pkg/postgres"
)

// Repository encapsulates database persistence for payments and transactional outbox.
type Repository interface {
	CreateWithOutbox(ctx context.Context, p *Payment, eventPayload any, outboxTopic string) error
	ListRecent(ctx context.Context, limit int) ([]PaymentDetail, error)
}

// PostgresRepository implements Repository using pgxpool within atomic transactions.
type PostgresRepository struct {
	client     *postgres.Client
	outboxRepo *outbox.PostgresRepository
}

// NewPostgresRepository creates a payment postgres repository.
func NewPostgresRepository(client *postgres.Client) *PostgresRepository {
	return &PostgresRepository{
		client:     client,
		outboxRepo: outbox.NewPostgresRepository(client.Pool),
	}
}

// CreateWithOutbox atomically records the payment and creates an outbox event in a single ACID transaction.
func (r *PostgresRepository) CreateWithOutbox(ctx context.Context, p *Payment, eventPayload any, outboxTopic string) error {
	return r.client.WithTx(ctx, func(tx pgx.Tx) error {
		const insertPaymentQuery = `
			INSERT INTO payments (
				id, idempotency_key, merchant_id, customer_id, amount, currency, status, provider, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`
		_, err := tx.Exec(ctx, insertPaymentQuery,
			p.ID,
			p.IdempotencyKey,
			p.MerchantID,
			p.CustomerID,
			p.Amount,
			p.Currency,
			string(p.Status),
			p.Provider,
			p.CreatedAt,
			p.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert payment: %w", err)
		}

		msg, err := outbox.NewMessage(
			outboxTopic,
			p.ID.String(),
			eventPayload,
			map[string]string{
				"idempotency_key": p.IdempotencyKey,
				"source":          "api-gateway",
			},
		)
		if err != nil {
			return fmt.Errorf("create outbox message: %w", err)
		}

		if err := r.outboxRepo.StoreTx(ctx, tx, msg); err != nil {
			return fmt.Errorf("store outbox message: %w", err)
		}

		return nil
	})
}

// ListRecent retrieves the latest payments along with their Transactional Outbox status.
func (r *PostgresRepository) ListRecent(ctx context.Context, limit int) ([]PaymentDetail, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	const query = `
		SELECT 
			p.id, 
			p.idempotency_key, 
			p.merchant_id, 
			COALESCE(p.customer_id, ''), 
			p.amount, 
			p.currency, 
			p.status, 
			p.created_at,
			COALESCE(o.status, 'NONE') AS outbox_status
		FROM payments p
		LEFT JOIN outbox_messages o ON o.key = p.id::text
		ORDER BY p.created_at DESC
		LIMIT $1
	`

	rows, err := r.client.Pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("query payments: %w", err)
	}
	defer rows.Close()

	var results []PaymentDetail
	for rows.Next() {
		var (
			d        PaymentDetail
			amount   int64
			currCode string
			status   string
		)

		err := rows.Scan(
			&d.ID,
			&d.IdempotencyKey,
			&d.MerchantID,
			&d.CustomerID,
			&amount,
			&currCode,
			&status,
			&d.CreatedAt,
			&d.OutboxStatus,
		)
		if err != nil {
			return nil, fmt.Errorf("scan payment detail: %w", err)
		}

		m, _ := money.New(amount, money.Currency(currCode))
		d.Amount = m
		d.Status = Status(status)
		results = append(results, d)
	}

	return results, rows.Err()
}

