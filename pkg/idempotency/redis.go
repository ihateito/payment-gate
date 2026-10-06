package idempotency

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type storedPayload struct {
	Status   Status          `json:"status"`
	Response *ResponseRecord `json:"response,omitempty"`
}

// RedisStorage implements Storage backed by Redis.
type RedisStorage struct {
	client redis.UniversalClient
	prefix string
}

// NewRedisStorage creates a Redis-backed idempotency storage.
func NewRedisStorage(client redis.UniversalClient, keyPrefix string) *RedisStorage {
	if keyPrefix == "" {
		keyPrefix = "idempotency:"
	}
	return &RedisStorage{
		client: client,
		prefix: keyPrefix,
	}
}

func (r *RedisStorage) rKey(key string) string {
	return r.prefix + key
}

func (r *RedisStorage) TryAcquire(ctx context.Context, key string, ttl time.Duration) (*AcquireResult, error) {
	redisKey := r.rKey(key)

	inProgressData, err := json.Marshal(storedPayload{
		Status: StatusInProgress,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal initial state: %w", err)
	}

	// Try atomic SET NX PX
	set, err := r.client.SetNX(ctx, redisKey, inProgressData, ttl).Result()
	if err != nil {
		return nil, fmt.Errorf("redis setnx: %w", err)
	}

	if set {
		// Key was successfully locked
		return &AcquireResult{Status: StatusInProgress}, nil
	}

	// Key already exists, read its state
	val, err := r.client.Get(ctx, redisKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// Expired between setnx and get, retry acquire
			return r.TryAcquire(ctx, key, ttl)
		}
		return nil, fmt.Errorf("redis get: %w", err)
	}

	var payload storedPayload
	if err := json.Unmarshal([]byte(val), &payload); err != nil {
		return nil, fmt.Errorf("unmarshal existing record: %w", err)
	}

	if payload.Status == StatusInProgress {
		return nil, ErrLocked
	}

	if payload.Status == StatusCompleted {
		return &AcquireResult{
			Status:   StatusCompleted,
			Response: payload.Response,
		}, nil
	}

	return nil, ErrInvalidState
}

func (r *RedisStorage) Complete(ctx context.Context, key string, resp ResponseRecord, ttl time.Duration) error {
	redisKey := r.rKey(key)
	data, err := json.Marshal(storedPayload{
		Status:   StatusCompleted,
		Response: &resp,
	})
	if err != nil {
		return fmt.Errorf("marshal complete record: %w", err)
	}

	return r.client.Set(ctx, redisKey, data, ttl).Err()
}

func (r *RedisStorage) Release(ctx context.Context, key string) error {
	redisKey := r.rKey(key)
	return r.client.Del(ctx, redisKey).Err()
}
