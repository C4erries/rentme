package reviews

import (
	"context"
	"time"

	"rentme/internal/app/uow"
	domainlistings "rentme/internal/domain/listings"
)

func recalculateListingRating(ctx context.Context, unit uow.UnitOfWork, listingID domainlistings.ListingID, now time.Time) error {
	reviews, err := unit.Reviews().ListByListing(ctx, listingID, 0, 0)
	if err != nil {
		return err
	}
	var total int
	for _, review := range reviews {
		total += review.Rating
	}
	average := 0.0
	if len(reviews) > 0 {
		average = float64(total) / float64(len(reviews))
	}

	listing, err := unit.Listings().ByID(ctx, listingID)
	if err != nil {
		return err
	}
	listing.UpdateRating(average, now)
	return unit.Listings().Save(ctx, listing)
}
