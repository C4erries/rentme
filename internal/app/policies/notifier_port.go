package policies

import "context"

type Notifier interface {
	Send(ctx context.Context, to string, template string, data any) error
}
