package queries

import (
	"context"
	"fmt"
)

type queryHandler func(ctx context.Context, q Query) (any, error)

// InMemoryBus is a simple query bus implementation with in-memory registrations.
type InMemoryBus struct {
	handlers map[string]queryHandler
}

func NewInMemoryBus() *InMemoryBus {
	return &InMemoryBus{handlers: make(map[string]queryHandler)}
}

func (b *InMemoryBus) RegisterRaw(key string, handler queryHandler) {
	if key == "" {
		panic("queries: empty key registration")
	}
	b.handlers[key] = handler
}

func (b *InMemoryBus) Ask(ctx context.Context, query Query) (any, error) {
	h, ok := b.handlers[query.Key()]
	if !ok {
		return nil, ErrHandlerNotFound
	}
	return h(ctx, query)
}

func RegisterHandler[Q Query, R any](bus *InMemoryBus, key string, handler Handler[Q, R]) {
	if bus == nil {
		panic("queries: nil bus")
	}
	bus.RegisterRaw(key, func(ctx context.Context, raw Query) (any, error) {
		q, ok := any(raw).(Q)
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrInvalidQuery, key)
		}
		return handler.Handle(ctx, q)
	})
}
