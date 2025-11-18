package uow

import (
	"context"
	"errors"
)

var ErrUnitOfWorkMissing = errors.New("uow: unit of work missing from context")

type ctxKey struct{}

// ContextWithUnitOfWork stores the provided unit of work in context.
func ContextWithUnitOfWork(ctx context.Context, unit UnitOfWork) context.Context {
	return context.WithValue(ctx, ctxKey{}, unit)
}

// FromContext retrieves a unit of work from context if present.
func FromContext(ctx context.Context) (UnitOfWork, bool) {
	val := ctx.Value(ctxKey{})
	if val == nil {
		return nil, false
	}
	unit, ok := val.(UnitOfWork)
	return unit, ok
}
