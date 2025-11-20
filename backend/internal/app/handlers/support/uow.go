package support

import (
	"context"

	"rentme/internal/app/uow"
)

func BeginReadOnlyUnit(ctx context.Context, factory uow.UoWFactory) (uow.UnitOfWork, context.Context, func(), error) {
	unit, ok := uow.FromContext(ctx)
	if ok {
		return unit, ctx, nil, nil
	}
	if factory == nil {
		return nil, ctx, nil, uow.ErrUnitOfWorkMissing
	}
	newUnit, err := factory.Begin(ctx, uow.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, ctx, nil, err
	}
	execCtx := ctx
	if injector, ok := newUnit.(interface {
		InjectContext(context.Context) context.Context
	}); ok {
		execCtx = injector.InjectContext(ctx)
	}
	execCtx = uow.ContextWithUnitOfWork(execCtx, newUnit)
	cleanup := func() {
		_ = newUnit.Rollback(execCtx)
	}
	return newUnit, execCtx, cleanup, nil
}
