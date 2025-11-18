package commands

import (
	"context"
	"errors"
)

// Command represents a write intent routed through the application bus.
type Command interface {
	Key() string
}

// Handler processes a command and returns a value (if any).
type Handler[C Command, R any] interface {
	Handle(ctx context.Context, cmd C) (R, error)
}

// HandlerFunc is an adapter to allow the use of ordinary functions as command handlers.
type HandlerFunc[C Command, R any] func(ctx context.Context, cmd C) (R, error)

// Handle calls f(ctx, cmd).
func (f HandlerFunc[C, R]) Handle(ctx context.Context, cmd C) (R, error) {
	return f(ctx, cmd)
}

// Bus dispatches commands through optional middleware pipeline.
type Bus interface {
	Dispatch(ctx context.Context, cmd Command) (any, error)
}

var (
	ErrHandlerNotFound = errors.New("commands: handler not found")
	ErrInvalidCommand  = errors.New("commands: invalid command for handler")
	ErrResultType      = errors.New("commands: result type mismatch")
	ErrNilBus          = errors.New("commands: nil bus")
)

// Dispatch is a helper that performs type-safe command invocation against a bus.
func Dispatch[C Command, R any](ctx context.Context, bus Bus, cmd C) (R, error) {
	var zero R
	if bus == nil {
		return zero, ErrNilBus
	}
	res, err := bus.Dispatch(ctx, cmd)
	if err != nil {
		return zero, err
	}
	if res == nil {
		return zero, nil
	}
	value, ok := res.(R)
	if !ok {
		return zero, ErrResultType
	}
	return value, nil
}
