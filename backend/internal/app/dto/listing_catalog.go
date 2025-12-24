package dto

import (
	"math"
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
	ID               string              `json:"id"`
	HostID           string              `json:"host_id"`
	Title            string              `json:"title"`
	City             string              `json:"city"`
	Region           string              `json:"region"`
	Country          string              `json:"country"`
	AddressLine      string              `json:"address_line"`
	PropertyType     string              `json:"property_type"`
	GuestsLimit      int                 `json:"guests_limit"`
	MinNights        int                 `json:"min_nights"`
	MaxNights        int                 `json:"max_nights"`
	RateRub          int64               `json:"rate_rub"`
	PriceUnit        string              `json:"price_unit"`
	Bedrooms         int                 `json:"bedrooms"`
	Bathrooms        int                 `json:"bathrooms"`
	AreaSquareMeters float64             `json:"area_sq_m"`
	RentalTerm       string              `json:"rental_term"`
	Tags             []string            `json:"tags"`
	Amenities        []string            `json:"amenities"`
	Highlights       []string            `json:"highlights"`
	ThumbnailURL     string              `json:"thumbnail_url"`
	Rating           float64             `json:"rating"`
	AvailableFrom    time.Time           `json:"available_from"`
	State            string              `json:"state"`
	Availability     ListingAvailability `json:"availability"`
}

// ListingAvailability describes availability for selected filters.
type ListingAvailability struct {
	CheckIn     time.Time `json:"check_in"`
	CheckOut    time.Time `json:"check_out"`
	Nights      int       `json:"nights"`
	Guests      int       `json:"guests"`
	IsAvailable bool      `json:"is_available"`
	Reason      string    `json:"reason,omitempty"`
}

// CatalogFilters echoes back the applied filters.
type CatalogFilters struct {
	City          string   `json:"city"`
	Region        string   `json:"region"`
	Country       string   `json:"country"`
	Location      string   `json:"location"`
	Tags          []string `json:"tags"`
	Amenities     []string `json:"amenities"`
	MinGuests     int      `json:"min_guests"`
	PriceMinRub   int64    `json:"price_min_rub"`
	PriceMaxRub   int64    `json:"price_max_rub"`
	PropertyTypes []string `json:"property_types"`
	CheckIn       string   `json:"check_in"`
	CheckOut      string   `json:"check_out"`
	RentalTerms   []string `json:"rental_terms"`
}

// CatalogMetadata describes pagination.
type CatalogMetadata struct {
	Total      int    `json:"total"`
	Count      int    `json:"count"`
	Limit      int    `json:"limit"`
	Offset     int    `json:"offset"`
	Sort       string `json:"sort"`
	Page       int    `json:"page"`
	TotalPages int    `json:"total_pages"`
}

// MapCatalog builds a DTO collection based on a search result.
func MapCatalog(result domainlistings.SearchResult, params domainlistings.SearchParams, availability map[domainlistings.ListingID]ListingAvailability) ListingCatalog {
	normalized := params.Normalized()
	items := make([]ListingCard, 0, len(result.Items))
	for _, listing := range result.Items {
		card := MapListingCard(listing)
		if availability != nil {
			if report, ok := availability[listing.ID]; ok {
				card.Availability = report
			}
		}
		items = append(items, card)
	}
	page, totalPages := resolvePaging(normalized.Limit, normalized.Offset, result.Total)
	rentalTerms := make([]string, 0, len(normalized.RentalTerms))
	for _, term := range normalized.RentalTerms {
		rentalTerms = append(rentalTerms, string(term))
	}
	return ListingCatalog{
		Items: items,
		Filters: CatalogFilters{
			City:          normalized.City,
			Region:        normalized.Region,
			Country:       normalized.Country,
			Location:      normalized.LocationQuery,
			Tags:          append([]string(nil), normalized.Tags...),
			Amenities:     append([]string(nil), normalized.Amenities...),
			MinGuests:     normalized.MinGuests,
			PriceMinRub:   normalized.PriceMinRub,
			PriceMaxRub:   normalized.PriceMaxRub,
			PropertyTypes: append([]string(nil), normalized.PropertyTypes...),
			CheckIn:       formatDate(normalized.CheckIn),
			CheckOut:      formatDate(normalized.CheckOut),
			RentalTerms:   rentalTerms,
		},
		Meta: CatalogMetadata{
			Total:      result.Total,
			Count:      len(items),
			Limit:      normalized.Limit,
			Offset:     normalized.Offset,
			Sort:       string(normalized.Sort),
			Page:       page,
			TotalPages: totalPages,
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
		HostID:           string(listing.Host),
		Title:            listing.Title,
		City:             listing.Address.City,
		Region:           listing.Address.Region,
		Country:          listing.Address.Country,
		AddressLine:      listing.Address.Line1,
		PropertyType:     listing.PropertyType,
		GuestsLimit:      listing.GuestsLimit,
		MinNights:        listing.MinNights,
		MaxNights:        listing.MaxNights,
		RateRub:          listing.RateRub,
		PriceUnit:        priceUnit(listing.RentalTermType),
		Bedrooms:         listing.Bedrooms,
		Bathrooms:        listing.Bathrooms,
		AreaSquareMeters: listing.AreaSquareMeters,
		RentalTerm:       string(listing.RentalTermType),
		Tags:             append([]string(nil), listing.Tags...),
		Amenities:        append([]string(nil), listing.Amenities...),
		Highlights:       append([]string(nil), listing.Highlights...),
		ThumbnailURL:     listing.ThumbnailURL,
		Rating:           listing.Rating,
		AvailableFrom:    listing.AvailableFrom,
		State:            string(listing.State),
	}
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02")
}

func resolvePaging(limit, offset, total int) (int, int) {
	if limit <= 0 {
		return 1, 1
	}
	page := (offset / limit) + 1
	totalPages := 1
	if total > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(limit)))
		if totalPages == 0 {
			totalPages = 1
		}
	}
	return page, totalPages
}

func priceUnit(term domainlistings.RentalTermType) string {
	if term == domainlistings.RentalTermLong {
		return "month"
	}
	return "night"
}
