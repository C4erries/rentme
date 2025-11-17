package policies

import (
	"context"

	domainlistings "rentme/internal/domain/listings"
	domainpricing "rentme/internal/domain/pricing"
	domainrange "rentme/internal/domain/shared/daterange"
)

type PricingPort interface {
	Quote(ctx context.Context, listingID domainlistings.ListingID, dr domainrange.DateRange, guests int) (domainpricing.PriceBreakdown, error)
}
