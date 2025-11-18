package middleware

import (
	"context"

	"rentme/internal/app/commands"
	"rentme/internal/app/queries"
)

// CommandMiddleware wraps a command bus with additional behavior (logging, tx, etc.).
type CommandMiddleware func(next commands.Bus) commands.Bus

// QueryMiddleware wraps a query bus with extra behavior.
type QueryMiddleware func(next queries.Bus) queries.Bus

// ChainCommands builds a command bus wrapped with the provided middleware (outermost first).
func ChainCommands(base commands.Bus, mws ...CommandMiddleware) commands.Bus {
	wrapped := base
	for i := len(mws) - 1; i >= 0; i-- {
		wrapped = mws[i](wrapped)
	}
	return wrapped
}

// ChainQueries builds a query bus with middleware applied.
func ChainQueries(base queries.Bus, mws ...QueryMiddleware) queries.Bus {
	wrapped := base
	for i := len(mws) - 1; i >= 0; i-- {
		wrapped = mws[i](wrapped)
	}
	return wrapped
}

// commandFunc allows lightweight middleware composition without new structs per wrapper.
type commandFunc func(ctx context.Context, cmd commands.Command) (any, error)

func (f commandFunc) Dispatch(ctx context.Context, cmd commands.Command) (any, error) {
	return f(ctx, cmd)
}

// wrapCommand builds a commandFunc around a bus.
func wrapCommand(next commands.Bus) commandFunc {
	return func(ctx context.Context, cmd commands.Command) (any, error) {
		return next.Dispatch(ctx, cmd)
	}
}

type queryFunc func(ctx context.Context, query queries.Query) (any, error)

func (f queryFunc) Ask(ctx context.Context, q queries.Query) (any, error) {
	return f(ctx, q)
}

func wrapQuery(next queries.Bus) queryFunc {
	return func(ctx context.Context, q queries.Query) (any, error) {
		return next.Ask(ctx, q)
	}
}
