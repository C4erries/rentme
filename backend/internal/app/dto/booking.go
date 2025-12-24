package dto

import (
	"time"

	domainbooking "rentme/internal/domain/booking"
	domainlistings "rentme/internal/domain/listings"
	"rentme/internal/domain/shared/money"
)

type MoneyDTO struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

type BookingListingSnapshot struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	AddressLine1 string `json:"address_line1"`
	City         string `json:"city"`
	Region       string `json:"region"`
	Country      string `json:"country"`
	ThumbnailURL string `json:"thumbnail_url"`
}

type GuestBookingSummary struct {
	ID              string                 `json:"id"`
	Listing         BookingListingSnapshot `json:"listing"`
	CheckIn         time.Time              `json:"check_in"`
	CheckOut        time.Time              `json:"check_out"`
	Guests          int                    `json:"guests"`
	Months          int                    `json:"months,omitempty"`
	PriceUnit       string                 `json:"price_unit"`
	Status          string                 `json:"status"`
	Total           MoneyDTO               `json:"total"`
	CreatedAt       time.Time              `json:"created_at"`
	ReviewSubmitted bool                   `json:"review_submitted"`
	CanReview       bool                   `json:"can_review"`
}

type GuestBookingCollection struct {
	Items []GuestBookingSummary `json:"items"`
}

type HostBookingSummary struct {
	ID        string                 `json:"id"`
	Listing   BookingListingSnapshot `json:"listing"`
	GuestID   string                 `json:"guest_id"`
	CheckIn   time.Time              `json:"check_in"`
	CheckOut  time.Time              `json:"check_out"`
	Guests    int                    `json:"guests"`
	Months    int                    `json:"months,omitempty"`
	PriceUnit string                 `json:"price_unit"`
	Status    string                 `json:"status"`
	Total     MoneyDTO               `json:"total"`
	CreatedAt time.Time              `json:"created_at"`
}

type HostBookingCollection struct {
	Items []HostBookingSummary `json:"items"`
}

func MapMoney(value money.Money) MoneyDTO {
	return MoneyDTO{
		Amount:   value.Amount,
		Currency: value.Currency,
	}
}

func MapGuestBookingSummary(
	booking *domainbooking.Booking,
	listing *domainlistings.Listing,
	reviewSubmitted bool,
	canReview bool,
) GuestBookingSummary {
	snapshot := BookingListingSnapshot{
		ID: string(booking.ListingID),
	}
	if listing != nil {
		snapshot.Title = listing.Title
		snapshot.AddressLine1 = listing.Address.Line1
		snapshot.City = listing.Address.City
		snapshot.Region = listing.Address.Region
		snapshot.Country = listing.Address.Country
		snapshot.ThumbnailURL = listing.ThumbnailURL
	}
	return GuestBookingSummary{
		ID:              string(booking.ID),
		Listing:         snapshot,
		CheckIn:         booking.Range.CheckIn,
		CheckOut:        booking.Range.CheckOut,
		Guests:          booking.Guests,
		Months:          booking.Months,
		PriceUnit:       resolvePriceUnit(booking.PriceUnit),
		Status:          string(booking.State),
		Total:           MapMoney(booking.Price.Total),
		CreatedAt:       booking.CreatedAt,
		ReviewSubmitted: reviewSubmitted,
		CanReview:       canReview,
	}
}

func MapHostBookingSummary(booking *domainbooking.Booking, listing *domainlistings.Listing) HostBookingSummary {
	snapshot := BookingListingSnapshot{
		ID: string(booking.ListingID),
	}
	if listing != nil {
		snapshot.Title = listing.Title
		snapshot.AddressLine1 = listing.Address.Line1
		snapshot.City = listing.Address.City
		snapshot.Region = listing.Address.Region
		snapshot.Country = listing.Address.Country
		snapshot.ThumbnailURL = listing.ThumbnailURL
	}
	return HostBookingSummary{
		ID:        string(booking.ID),
		Listing:   snapshot,
		GuestID:   booking.GuestID,
		CheckIn:   booking.Range.CheckIn,
		CheckOut:  booking.Range.CheckOut,
		Guests:    booking.Guests,
		Months:    booking.Months,
		PriceUnit: resolvePriceUnit(booking.PriceUnit),
		Status:    string(booking.State),
		Total:     MapMoney(booking.Price.Total),
		CreatedAt: booking.CreatedAt,
	}
}

func resolvePriceUnit(value string) string {
	switch value {
	case "night", "month":
		return value
	default:
		return "night"
	}
}
