package queries

import (
	"context"
	"errors"
)

// Query is a read request.
type Query interface {
	Key() string
}

// Handler handles a query and produces a result.
type Handler[Q Query, R any] interface {
	Handle(ctx context.Context, query Q) (R, error)
}

// HandlerFunc is a helper to use functions as handlers.
type HandlerFunc[Q Query, R any] func(ctx context.Context, query Q) (R, error)

// Handle executes f(ctx, query).
func (f HandlerFunc[Q, R]) Handle(ctx context.Context, query Q) (R, error) {
	return f(ctx, query)
}

// Bus routes queries to registered handlers.
type Bus interface {
	Ask(ctx context.Context, query Query) (any, error)
}

var (
	ErrHandlerNotFound = errors.New("queries: handler not found")
	ErrInvalidQuery    = errors.New("queries: invalid query for handler")
	ErrResultType      = errors.New("queries: result type mismatch")
	ErrNilBus          = errors.New("queries: nil bus")
)

// Ask runs the query through the provided bus, returning a typed result.
func Ask[Q Query, R any](ctx context.Context, bus Bus, query Q) (R, error) {
	var zero R
	if bus == nil {
		return zero, ErrNilBus
	}
	res, err := bus.Ask(ctx, query)
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
