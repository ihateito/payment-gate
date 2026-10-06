package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository implements outbox.Repository using pgxpool.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository creates a new outbox repository.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// Store inserts a message into the outbox table. If tx is nil, uses pool.
func (r *PostgresRepository) Store(ctx context.Context, msg *Message) error {
	headersJSON, err := json.Marshal(msg.Headers)
	if err != nil {
		return fmt.Errorf("marshal outbox headers: %w", err)
	}

	query := `
		INSERT INTO outbox_messages (
			id, topic, key, payload, headers, status, attempts, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err = r.pool.Exec(ctx, query,
		msg.ID,
		msg.Topic,
		msg.Key,
		msg.Payload,
		headersJSON,
		msg.Status,
		msg.Attempts,
		msg.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert outbox message: %w", err)
	}

	return nil
}

// StoreTx inserts a message as part of an ongoing database transaction.
func (r *PostgresRepository) StoreTx(ctx context.Context, tx pgx.Tx, msg *Message) error {
	headersJSON, err := json.Marshal(msg.Headers)
	if err != nil {
		return fmt.Errorf("marshal outbox headers: %w", err)
	}

	query := `
		INSERT INTO outbox_messages (
			id, topic, key, payload, headers, status, attempts, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err = tx.Exec(ctx, query,
		msg.ID,
		msg.Topic,
		msg.Key,
		msg.Payload,
		headersJSON,
		msg.Status,
		msg.Attempts,
		msg.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert outbox message in tx: %w", err)
	}

	return nil
}

// FetchBatch retrieves pending messages locked for update using SKIP LOCKED.
func (r *PostgresRepository) FetchBatch(ctx context.Context, batchSize int) ([]*Message, error) {
	query := `
		SELECT id, topic, key, payload, headers, status, attempts, created_at, next_retry_at
		FROM outbox_messages
		WHERE status = 'PENDING' AND (next_retry_at IS NULL OR next_retry_at <= NOW())
		ORDER BY created_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`

	rows, err := r.pool.Query(ctx, query, batchSize)
	if err != nil {
		return nil, fmt.Errorf("fetch outbox batch: %w", err)
	}
	defer rows.Close()

	var messages []*Message
	for rows.Next() {
		var (
			msg         Message
			headersJSON []byte
		)
		err := rows.Scan(
			&msg.ID,
			&msg.Topic,
			&msg.Key,
			&msg.Payload,
			&headersJSON,
			&msg.Status,
			&msg.Attempts,
			&msg.CreatedAt,
			&msg.NextRetryAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan outbox row: %w", err)
		}

		if len(headersJSON) > 0 {
			_ = json.Unmarshal(headersJSON, &msg.Headers)
		}
		messages = append(messages, &msg)
	}

	return messages, rows.Err()
}

// MarkPublished marks message as successfully published.
func (r *PostgresRepository) MarkPublished(ctx context.Context, id uuid.UUID, publishedAt time.Time) error {
	query := `
		UPDATE outbox_messages
		SET status = 'PUBLISHED', published_at = $2
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, id, publishedAt)
	return err
}

// MarkFailed updates retry counter and next retry time.
func (r *PostgresRepository) MarkFailed(ctx context.Context, id uuid.UUID, attempts int, nextRetryAt time.Time, lastErr error) error {
	errMsg := ""
	if lastErr != nil {
		errMsg = lastErr.Error()
	}

	query := `
		UPDATE outbox_messages
		SET attempts = $2, next_retry_at = $3, last_error = $4
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, id, attempts, nextRetryAt, errMsg)
	return err
}
