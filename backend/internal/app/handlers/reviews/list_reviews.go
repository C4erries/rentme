package reviews

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"rentme/internal/app/dto"
	handlersupport "rentme/internal/app/handlers/support"
	"rentme/internal/app/queries"
	"rentme/internal/app/uow"
	domainlistings "rentme/internal/domain/listings"
)

const listListingReviewsKey = "reviews.listing.list"

var ErrListingNotFound = errors.New("reviews: listing not found")

// ListListingReviewsQuery retrieves reviews for a listing.
type ListListingReviewsQuery struct {
	ListingID string
	Limit     int
	Offset    int
}

func (q ListListingReviewsQuery) Key() string { return listListingReviewsKey }

// ListListingReviewsHandler loads paginated reviews for a listing.
type ListListingReviewsHandler struct {
	UoWFactory uow.UoWFactory
	Logger     *slog.Logger
}

func (h *ListListingReviewsHandler) Handle(ctx context.Context, q ListListingReviewsQuery) (dto.ReviewCollection, error) {
	limit := normalizeLimit(q.Limit)
	offset := q.Offset
	if offset < 0 {
		offset = 0
	}

	unit, execCtx, cleanup, err := handlersupport.BeginReadOnlyUnit(ctx, h.UoWFactory)
	if err != nil {
		return dto.ReviewCollection{}, err
	}
	if cleanup != nil {
		defer cleanup()
	}

	listingID := domainlistings.ListingID(q.ListingID)
	if _, err := unit.Listings().ByID(execCtx, listingID); err != nil {
		return dto.ReviewCollection{}, fmt.Errorf("%w: %v", ErrListingNotFound, err)
	}

	all, err := unit.Reviews().ListByListing(execCtx, listingID, 0, 0)
	if err != nil {
		return dto.ReviewCollection{}, err
	}
	total := len(all)

	windowEnd := total
	if limit > 0 && offset+limit < windowEnd {
		windowEnd = offset + limit
	}
	if offset > windowEnd {
		offset = windowEnd
	}
	slice := all[offset:windowEnd]

	items := make([]dto.Review, 0, len(slice))
	for _, review := range slice {
		items = append(items, dto.MapReview(review))
	}

	if h.Logger != nil {
		h.Logger.Debug("listing reviews listed", "listing_id", listingID, "count", len(items), "total", total)
	}

	return dto.ReviewCollection{Items: items, Total: total}, nil
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}

var _ queries.Handler[ListListingReviewsQuery, dto.ReviewCollection] = (*ListListingReviewsHandler)(nil)
