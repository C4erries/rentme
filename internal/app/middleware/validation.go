package middleware

import (
	"context"

	"rentme/internal/app/commands"
	"rentme/internal/app/queries"
)

type Validator interface {
	Validate(ctx context.Context, message any) error
}

func Validation(v Validator) CommandMiddleware {
	if v == nil {
		panic("middleware: validator required")
	}
	return func(next commands.Bus) commands.Bus {
		nextFn := wrapCommand(next)
		return commandFunc(func(ctx context.Context, cmd commands.Command) (any, error) {
			if err := v.Validate(ctx, cmd); err != nil {
				return nil, err
			}
			return nextFn(ctx, cmd)
		})
	}
}

func QueryValidation(v Validator) QueryMiddleware {
	if v == nil {
		panic("middleware: validator required")
	}
	return func(next queries.Bus) queries.Bus {
		nextFn := wrapQuery(next)
		return queryFunc(func(ctx context.Context, q queries.Query) (any, error) {
			if err := v.Validate(ctx, q); err != nil {
				return nil, err
			}
			return nextFn(ctx, q)
		})
	}
}
