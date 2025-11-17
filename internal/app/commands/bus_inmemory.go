package commands

import (
	"context"
	"fmt"
)

type commandHandler func(ctx context.Context, cmd Command) (any, error)

// InMemoryBus is a simple registry-backed bus that keeps handlers in memory.
type InMemoryBus struct {
	handlers map[string]commandHandler
}

// NewInMemoryBus creates an empty bus instance.
func NewInMemoryBus() *InMemoryBus {
	return &InMemoryBus{handlers: make(map[string]commandHandler)}
}

// RegisterRaw attaches a raw handler function to the provided command key.
func (b *InMemoryBus) RegisterRaw(key string, handler commandHandler) {
	if key == "" {
		panic("commands: empty key registration")
	}
	b.handlers[key] = handler
}

// Dispatch executes the registered handler for the provided command.
func (b *InMemoryBus) Dispatch(ctx context.Context, cmd Command) (any, error) {
	h, ok := b.handlers[cmd.Key()]
	if !ok {
		return nil, ErrHandlerNotFound
	}
	return h(ctx, cmd)
}

// RegisterHandler is a helper to register strongly typed handlers on the in-memory bus.
func RegisterHandler[C Command, R any](bus *InMemoryBus, key string, handler Handler[C, R]) {
	if bus == nil {
		panic("commands: nil bus")
	}
	bus.RegisterRaw(key, func(ctx context.Context, raw Command) (any, error) {
		cmd, ok := any(raw).(C)
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrInvalidCommand, key)
		}
		return handler.Handle(ctx, cmd)
	})
}
