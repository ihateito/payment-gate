package idempotency

import (
	"context"
	"sync"
	"time"
)

type memoryItem struct {
	status    Status
	response  *ResponseRecord
	expiresAt time.Time
}

// MemoryStorage is an in-memory thread-safe implementation of Storage.
type MemoryStorage struct {
	mu    sync.RWMutex
	items map[string]memoryItem
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage() *MemoryStorage {
	s := &MemoryStorage{
		items: make(map[string]memoryItem),
	}
	// Start periodic eviction
	go s.cleanupLoop()
	return s
}

func (s *MemoryStorage) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for k, v := range s.items {
			if now.After(v.expiresAt) {
				delete(s.items, k)
			}
		}
		s.mu.Unlock()
	}
}

func (s *MemoryStorage) TryAcquire(ctx context.Context, key string, ttl time.Duration) (*AcquireResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, exists := s.items[key]
	now := time.Now()

	if exists && now.Before(item.expiresAt) {
		if item.status == StatusInProgress {
			return nil, ErrLocked
		}
		if item.status == StatusCompleted {
			return &AcquireResult{
				Status:   StatusCompleted,
				Response: item.response,
			}, nil
		}
	}

	// Lock acquired
	s.items[key] = memoryItem{
		status:    StatusInProgress,
		expiresAt: now.Add(ttl),
	}

	return &AcquireResult{Status: StatusInProgress}, nil
}

func (s *MemoryStorage) Complete(ctx context.Context, key string, resp ResponseRecord, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.items[key] = memoryItem{
		status:    StatusCompleted,
		response:  &resp,
		expiresAt: time.Now().Add(ttl),
	}
	return nil
}

func (s *MemoryStorage) Release(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, key)
	return nil
}
