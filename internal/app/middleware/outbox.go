package middleware

import (
	"context"

	"rentme/internal/app/commands"
	"rentme/internal/app/outbox"
)

func OutboxFlush(box outbox.Outbox) CommandMiddleware {
	if box == nil {
		panic("middleware: outbox required")
	}
	return func(next commands.Bus) commands.Bus {
		nextFn := wrapCommand(next)
		return commandFunc(func(ctx context.Context, cmd commands.Command) (any, error) {
			res, err := nextFn(ctx, cmd)
			if err != nil {
				return nil, err
			}
			if err := box.Flush(ctx); err != nil {
				return nil, err
			}
			return res, nil
		})
	}
}
