package memory

import (
	"context"
	"sync"

	"rentme/internal/app/middleware"
)

// IdempotencyStore stores results in memory.
type IdempotencyStore struct {
	mu    sync.RWMutex
	items map[string]middleware.IdempotencyRecord
}

func NewIdempotencyStore() *IdempotencyStore {
	return &IdempotencyStore{items: make(map[string]middleware.IdempotencyRecord)}
}

func (s *IdempotencyStore) Get(ctx context.Context, key string) (middleware.IdempotencyRecord, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.items[key]
	return rec, ok, nil
}

func (s *IdempotencyStore) Save(ctx context.Context, rec middleware.IdempotencyRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[rec.Key] = rec
	return nil
}

var _ middleware.IdempotencyStore = (*IdempotencyStore)(nil)
