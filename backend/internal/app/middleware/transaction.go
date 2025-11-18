package middleware

import (
	"context"
	"errors"

	"rentme/internal/app/commands"
	"rentme/internal/app/uow"
)

var ErrUnitOfWorkMissing = errors.New("middleware: unit of work not found")

type TxOptionsProvider func(cmd commands.Command) uow.TxOptions

func Transaction(factory uow.UoWFactory, optsProvider TxOptionsProvider) CommandMiddleware {
	if factory == nil {
		panic("middleware: uow factory required")
	}
	return func(next commands.Bus) commands.Bus {
		nextFn := wrapCommand(next)
		return commandFunc(func(ctx context.Context, cmd commands.Command) (any, error) {
			opts := uow.TxOptions{}
			if optsProvider != nil {
				opts = optsProvider(cmd)
			}
			unit, err := factory.Begin(ctx, opts)
			if err != nil {
				return nil, err
			}
			execCtx := ctx
			if injector, ok := unit.(interface {
				InjectContext(context.Context) context.Context
			}); ok {
				execCtx = injector.InjectContext(ctx)
			}
			execCtx = uow.ContextWithUnitOfWork(execCtx, unit)
			committed := false
			defer func() {
				if !committed {
					_ = unit.Rollback(execCtx)
				}
			}()

			res, err := nextFn(execCtx, cmd)
			if err != nil {
				return nil, err
			}
			if err := unit.Commit(execCtx); err != nil {
				return nil, err
			}
			committed = true
			return res, nil
		})
	}
}
