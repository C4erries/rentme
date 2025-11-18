package listings

import (
	"context"

	"rentme/internal/app/dto"
	"rentme/internal/app/queries"
	"rentme/internal/app/uow"
	domainlistings "rentme/internal/domain/listings"
)

const searchCatalogKey = "listings.catalog"

// SearchCatalogQuery describes request filters.
type SearchCatalogQuery struct {
	City          string
	Country       string
	Tags          []string
	Amenities     []string
	MinGuests     int
	PriceMinCents int64
	PriceMaxCents int64
	Sort          string
	Limit         int
	Offset        int
}

func (q SearchCatalogQuery) Key() string { return searchCatalogKey }

// SearchCatalogHandler loads listings with applied filters.
type SearchCatalogHandler struct {
	UoWFactory uow.UoWFactory
}

func (h *SearchCatalogHandler) Handle(ctx context.Context, q SearchCatalogQuery) (dto.ListingCatalog, error) {
	unit, ok := uow.FromContext(ctx)
	if !ok {
		if h.UoWFactory == nil {
			return dto.ListingCatalog{}, uow.ErrUnitOfWorkMissing
		}
		var err error
		unit, err = h.UoWFactory.Begin(ctx, uow.TxOptions{ReadOnly: true})
		if err != nil {
			return dto.ListingCatalog{}, err
		}
		ctx = uow.ContextWithUnitOfWork(ctx, unit)
		defer unit.Rollback(ctx)
	}

	searchParams := domainlistings.SearchParams{
		City:          q.City,
		Country:       q.Country,
		Tags:          append([]string(nil), q.Tags...),
		Amenities:     append([]string(nil), q.Amenities...),
		MinGuests:     q.MinGuests,
		PriceMinCents: q.PriceMinCents,
		PriceMaxCents: q.PriceMaxCents,
		Sort:          domainlistings.CatalogSort(q.Sort),
		Limit:         q.Limit,
		Offset:        q.Offset,
		OnlyActive:    true,
	}

	result, err := unit.Listings().Search(ctx, searchParams)
	if err != nil {
		return dto.ListingCatalog{}, err
	}

	return dto.MapCatalog(result, searchParams), nil
}

var _ queries.Handler[SearchCatalogQuery, dto.ListingCatalog] = (*SearchCatalogHandler)(nil)
