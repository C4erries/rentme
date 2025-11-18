package middleware

import (
	"context"

	"rentme/internal/app/commands"
	"rentme/internal/app/queries"
)

type Authorizer interface {
	Authorize(ctx context.Context, message any) error
}

func Authorization(a Authorizer) CommandMiddleware {
	if a == nil {
		panic("middleware: authorizer required")
	}
	return func(next commands.Bus) commands.Bus {
		nextFn := wrapCommand(next)
		return commandFunc(func(ctx context.Context, cmd commands.Command) (any, error) {
			if err := a.Authorize(ctx, cmd); err != nil {
				return nil, err
			}
			return nextFn(ctx, cmd)
		})
	}
}

func QueryAuthorization(a Authorizer) QueryMiddleware {
	if a == nil {
		panic("middleware: authorizer required")
	}
	return func(next queries.Bus) queries.Bus {
		nextFn := wrapQuery(next)
		return queryFunc(func(ctx context.Context, q queries.Query) (any, error) {
			if err := a.Authorize(ctx, q); err != nil {
				return nil, err
			}
			return nextFn(ctx, q)
		})
	}
}
