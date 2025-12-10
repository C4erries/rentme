package listings

import (
	"context"
	"strings"
	"time"

	"rentme/internal/app/dto"
	"rentme/internal/app/queries"
	"rentme/internal/app/uow"
	domainlistings "rentme/internal/domain/listings"
	"rentme/internal/domain/shared/daterange"
)

const searchCatalogKey = "listings.catalog"

// SearchCatalogQuery describes request filters.
type SearchCatalogQuery struct {
	City          string
	Region        string
	Country       string
	Location      string
	Tags          []string
	Amenities     []string
	MinGuests     int
	PriceMinCents int64
	PriceMaxCents int64
	PropertyTypes []string
	RentalTerms   []string
	Sort          string
	Limit         int
	Offset        int
	CheckIn       time.Time
	CheckOut      time.Time
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
		Region:        q.Region,
		Country:       q.Country,
		LocationQuery: q.Location,
		Tags:          append([]string(nil), q.Tags...),
		Amenities:     append([]string(nil), q.Amenities...),
		MinGuests:     q.MinGuests,
		PriceMinCents: q.PriceMinCents,
		PriceMaxCents: q.PriceMaxCents,
		PropertyTypes: append([]string(nil), q.PropertyTypes...),
		RentalTerms:   parseRentalTerms(q.RentalTerms),
		Sort:          domainlistings.CatalogSort(q.Sort),
		Limit:         q.Limit,
		Offset:        q.Offset,
		CheckIn:       q.CheckIn,
		CheckOut:      q.CheckOut,
		OnlyActive:    true,
	}

	result, err := unit.Listings().Search(ctx, searchParams)
	if err != nil {
		return dto.ListingCatalog{}, err
	}

	var availability map[domainlistings.ListingID]dto.ListingAvailability
	if !q.CheckIn.IsZero() && !q.CheckOut.IsZero() {
		dateRange, err := daterange.New(q.CheckIn, q.CheckOut)
		if err != nil {
			return dto.ListingCatalog{}, err
		}
		availability = make(map[domainlistings.ListingID]dto.ListingAvailability, len(result.Items))
		for _, listing := range result.Items {
			cal, err := unit.Availability().Calendar(ctx, listing.ID)
			if err != nil {
				return dto.ListingCatalog{}, err
			}
			isAvailable := cal.CanReserve(dateRange)
			availability[listing.ID] = dto.ListingAvailability{
				CheckIn:     dateRange.CheckIn,
				CheckOut:    dateRange.CheckOut,
				Nights:      dateRange.Nights(),
				Guests:      searchParams.MinGuests,
				IsAvailable: isAvailable,
				Reason: func() string {
					if isAvailable {
						return ""
					}
					return "unavailable"
				}(),
			}
		}
	}

	return dto.MapCatalog(result, searchParams, availability), nil
}

var _ queries.Handler[SearchCatalogQuery, dto.ListingCatalog] = (*SearchCatalogHandler)(nil)

func parseRentalTerms(tokens []string) []domainlistings.RentalTermType {
	if len(tokens) == 0 {
		return nil
	}
	seen := make(map[domainlistings.RentalTermType]struct{}, len(tokens))
	out := make([]domainlistings.RentalTermType, 0, len(tokens))
	for _, token := range tokens {
		normalized := domainlistings.RentalTermType(strings.ToLower(strings.TrimSpace(token)))
		switch normalized {
		case domainlistings.RentalTermShort, domainlistings.RentalTermLong:
			if _, ok := seen[normalized]; ok {
				continue
			}
			seen[normalized] = struct{}{}
			out = append(out, normalized)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
