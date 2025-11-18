package listings

import (
	"context"
	"time"

	"rentme/internal/app/dto"
	"rentme/internal/app/queries"
	"rentme/internal/app/uow"
	domainlistings "rentme/internal/domain/listings"
)

const getOverviewKey = "listings.overview"

// GetOverviewQuery loads a listing with availability metadata.
type GetOverviewQuery struct {
	ListingID string
	From      time.Time
	To        time.Time
}

func (q GetOverviewQuery) Key() string { return getOverviewKey }

// GetOverviewHandler resolves the overview DTO.
type GetOverviewHandler struct {
	UoWFactory uow.UoWFactory
}

func (h *GetOverviewHandler) Handle(ctx context.Context, q GetOverviewQuery) (dto.ListingOverview, error) {
	unit, ok := uow.FromContext(ctx)
	if !ok {
		if h.UoWFactory == nil {
			return dto.ListingOverview{}, uow.ErrUnitOfWorkMissing
		}
		var err error
		unit, err = h.UoWFactory.Begin(ctx, uow.TxOptions{ReadOnly: true})
		if err != nil {
			return dto.ListingOverview{}, err
		}
		ctx = uow.ContextWithUnitOfWork(ctx, unit)
		defer unit.Rollback(ctx)
	}

	listing, err := unit.Listings().ByID(ctx, domainlistings.ListingID(q.ListingID))
	if err != nil {
		return dto.ListingOverview{}, err
	}

	calendar, err := unit.Availability().Calendar(ctx, listing.ID)
	if err != nil {
		return dto.ListingOverview{}, err
	}

	return dto.MapListingOverview(listing, calendar, q.From, q.To), nil
}

var _ queries.Handler[GetOverviewQuery, dto.ListingOverview] = (*GetOverviewHandler)(nil)
