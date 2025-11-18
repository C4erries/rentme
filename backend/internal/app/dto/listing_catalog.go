package dto

import (
	"time"

	domainlistings "rentme/internal/domain/listings"
)

// ListingCatalog is a paginated collection of listings.
type ListingCatalog struct {
	Items   []ListingCard   `json:"items"`
	Filters CatalogFilters  `json:"filters"`
	Meta    CatalogMetadata `json:"meta"`
}

// ListingCard is a lightweight representation for catalog cards.
type ListingCard struct {
	ID               string    `json:"id"`
	Title            string    `json:"title"`
	City             string    `json:"city"`
	Country          string    `json:"country"`
	AddressLine      string    `json:"address_line"`
	GuestsLimit      int       `json:"guests_limit"`
	MinNights        int       `json:"min_nights"`
	MaxNights        int       `json:"max_nights"`
	NightlyRateCents int64     `json:"nightly_rate_cents"`
	Bedrooms         int       `json:"bedrooms"`
	Bathrooms        int       `json:"bathrooms"`
	AreaSquareMeters float64   `json:"area_sq_m"`
	Tags             []string  `json:"tags"`
	Amenities        []string  `json:"amenities"`
	Highlights       []string  `json:"highlights"`
	ThumbnailURL     string    `json:"thumbnail_url"`
	Rating           float64   `json:"rating"`
	AvailableFrom    time.Time `json:"available_from"`
	State            string    `json:"state"`
}

// CatalogFilters echoes back the applied filters.
type CatalogFilters struct {
	City          string   `json:"city"`
	Country       string   `json:"country"`
	Tags          []string `json:"tags"`
	Amenities     []string `json:"amenities"`
	MinGuests     int      `json:"min_guests"`
	PriceMinCents int64    `json:"price_min_cents"`
	PriceMaxCents int64    `json:"price_max_cents"`
}

// CatalogMetadata describes pagination.
type CatalogMetadata struct {
	Total  int    `json:"total"`
	Count  int    `json:"count"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
	Sort   string `json:"sort"`
}

// MapCatalog builds a DTO collection based on a search result.
func MapCatalog(result domainlistings.SearchResult, params domainlistings.SearchParams) ListingCatalog {
	normalized := params.Normalized()
	items := make([]ListingCard, 0, len(result.Items))
	for _, listing := range result.Items {
		items = append(items, MapListingCard(listing))
	}
	return ListingCatalog{
		Items: items,
		Filters: CatalogFilters{
			City:          normalized.City,
			Country:       normalized.Country,
			Tags:          append([]string(nil), normalized.Tags...),
			Amenities:     append([]string(nil), normalized.Amenities...),
			MinGuests:     normalized.MinGuests,
			PriceMinCents: normalized.PriceMinCents,
			PriceMaxCents: normalized.PriceMaxCents,
		},
		Meta: CatalogMetadata{
			Total:  result.Total,
			Count:  len(items),
			Limit:  normalized.Limit,
			Offset: normalized.Offset,
			Sort:   string(normalized.Sort),
		},
	}
}

// MapListingCard copies domain data for frontend consumption.
func MapListingCard(listing *domainlistings.Listing) ListingCard {
	if listing == nil {
		return ListingCard{}
	}
	return ListingCard{
		ID:               string(listing.ID),
		Title:            listing.Title,
		City:             listing.Address.City,
		Country:          listing.Address.Country,
		AddressLine:      listing.Address.Line1,
		GuestsLimit:      listing.GuestsLimit,
		MinNights:        listing.MinNights,
		MaxNights:        listing.MaxNights,
		NightlyRateCents: listing.NightlyRateCents,
		Bedrooms:         listing.Bedrooms,
		Bathrooms:        listing.Bathrooms,
		AreaSquareMeters: listing.AreaSquareMeters,
		Tags:             append([]string(nil), listing.Tags...),
		Amenities:        append([]string(nil), listing.Amenities...),
		Highlights:       append([]string(nil), listing.Highlights...),
		ThumbnailURL:     listing.ThumbnailURL,
		Rating:           listing.Rating,
		AvailableFrom:    listing.AvailableFrom,
		State:            string(listing.State),
	}
}
