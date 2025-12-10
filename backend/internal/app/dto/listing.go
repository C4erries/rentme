package dto

import (
	"time"

	domainavailability "rentme/internal/domain/availability"
	domainlistings "rentme/internal/domain/listings"
)

// ListingAddress represents the public location snapshot.
type ListingAddress struct {
	Line1   string  `json:"line1"`
	Line2   string  `json:"line2"`
	City    string  `json:"city"`
	Region  string  `json:"region"`
	Country string  `json:"country"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
}

// ListingHost contains owner level metadata.
type ListingHost struct {
	ID string `json:"id"`
}

// AvailabilityWindow describes the time window used to build the response.
type AvailabilityWindow struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// ListingOverview aggregates listing details and calendar information.
type ListingOverview struct {
	ID                 string             `json:"id"`
	Title              string             `json:"title"`
	Description        string             `json:"description"`
	Address            ListingAddress     `json:"address"`
	Amenities          []string           `json:"amenities"`
	GuestsLimit        int                `json:"guests_limit"`
	MinNights          int                `json:"min_nights"`
	MaxNights          int                `json:"max_nights"`
	HouseRules         []string           `json:"house_rules"`
	Host               ListingHost        `json:"host"`
	State              string             `json:"state"`
	Calendar           Calendar           `json:"calendar"`
	AvailabilityWindow AvailabilityWindow `json:"availability_window"`
}

// MapListingOverview builds a DTO that is convenient for the frontend.
func MapListingOverview(
	listing *domainlistings.Listing,
	calendar *domainavailability.AvailabilityCalendar,
	windowFrom, windowTo time.Time,
) ListingOverview {
	if listing == nil {
		return ListingOverview{}
	}
	host := ListingHost{ID: string(listing.Host)}
	address := ListingAddress{
		Line1:   listing.Address.Line1,
		Line2:   listing.Address.Line2,
		City:    listing.Address.City,
		Region:  listing.Address.Region,
		Country: listing.Address.Country,
		Lat:     listing.Address.Lat,
		Lon:     listing.Address.Lon,
	}
	overview := ListingOverview{
		ID:                 string(listing.ID),
		Title:              listing.Title,
		Description:        listing.Description,
		Address:            address,
		Amenities:          append([]string(nil), listing.Amenities...),
		GuestsLimit:        listing.GuestsLimit,
		MinNights:          listing.MinNights,
		MaxNights:          listing.MaxNights,
		HouseRules:         append([]string(nil), listing.HouseRules...),
		Host:               host,
		State:              string(listing.State),
		AvailabilityWindow: AvailabilityWindow{From: windowFrom, To: windowTo},
	}
	overview.Calendar = MapCalendarWithin(calendar, windowFrom, windowTo)
	return overview
}
