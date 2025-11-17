package saga

import (
	"context"

	"rentme/internal/app/outbox"
)

type Step interface {
	Execute(ctx context.Context, data any) error
	Compensate(ctx context.Context, data any) error
}

type Saga interface {
	Name() string
	Start(ctx context.Context, payload any) error
	OnEvent(ctx context.Context, ev outbox.EventRecord) error
}
