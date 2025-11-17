package policies

import (
	"context"

	"rentme/internal/domain/shared/money"
)

type PaymentsPort interface {
	PlaceHold(ctx context.Context, bookingID string, amount money.Money) (string, error)
	Capture(ctx context.Context, holdID string) error
	Refund(ctx context.Context, bookingID string, amount money.Money) error
}
