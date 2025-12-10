package policies

import (
	"context"

	domainlistings "rentme/internal/domain/listings"
	domainpricing "rentme/internal/domain/pricing"
	domainrange "rentme/internal/domain/shared/daterange"
)

type PricingPort interface {
	Quote(ctx context.Context, listing *domainlistings.Listing, dr domainrange.DateRange, guests int) (domainpricing.PriceBreakdown, error)
}
