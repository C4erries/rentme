package dto

import (
	"strings"
	"time"

	domainlistings "rentme/internal/domain/listings"
)

type HostListingCatalog struct {
	Items []HostListingSummary   `json:"items"`
	Meta  HostListingCatalogMeta `json:"meta"`
}

type HostListingCatalogMeta struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type HostListingSummary struct {
	ID               string    `json:"id"`
	Title            string    `json:"title"`
	Status           string    `json:"status"`
	City             string    `json:"city"`
	Country          string    `json:"country"`
	NightlyRateCents int64     `json:"nightly_rate_cents"`
	GuestsLimit      int       `json:"guests_limit"`
	Bedrooms         int       `json:"bedrooms"`
	Bathrooms        int       `json:"bathrooms"`
	AreaSquareMeters float64   `json:"area_sq_m"`
	AvailableFrom    time.Time `json:"available_from"`
	ThumbnailURL     string    `json:"thumbnail_url"`
	Photos           []string  `json:"photos"`
	UpdatedAt        time.Time `json:"updated_at"`
	State            string    `json:"state"`
}

type HostListingDetail struct {
	ID                   string         `json:"id"`
	Title                string         `json:"title"`
	Description          string         `json:"description"`
	PropertyType         string         `json:"property_type"`
	Address              ListingAddress `json:"address"`
	Amenities            []string       `json:"amenities"`
	GuestsLimit          int            `json:"guests_limit"`
	MinNights            int            `json:"min_nights"`
	MaxNights            int            `json:"max_nights"`
	HouseRules           []string       `json:"house_rules"`
	Host                 ListingHost    `json:"host"`
	State                string         `json:"state"`
	Tags                 []string       `json:"tags"`
	Highlights           []string       `json:"highlights"`
	NightlyRateCents     int64          `json:"nightly_rate_cents"`
	Bedrooms             int            `json:"bedrooms"`
	Bathrooms            int            `json:"bathrooms"`
	AreaSquareMeters     float64        `json:"area_sq_m"`
	ThumbnailURL         string         `json:"thumbnail_url"`
	Photos               []string       `json:"photos"`
	CancellationPolicyID string         `json:"cancellation_policy_id"`
	AvailableFrom        time.Time      `json:"available_from"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	StateLabel           string         `json:"status"`
}

func MapHostListingSummary(listing *domainlistings.Listing) HostListingSummary {
	if listing == nil {
		return HostListingSummary{}
	}
	return HostListingSummary{
		ID:               string(listing.ID),
		Title:            listing.Title,
		Status:           toStatus(listing.State),
		City:             listing.Address.City,
		Country:          listing.Address.Country,
		NightlyRateCents: listing.NightlyRateCents,
		GuestsLimit:      listing.GuestsLimit,
		Bedrooms:         listing.Bedrooms,
		Bathrooms:        listing.Bathrooms,
		AreaSquareMeters: listing.AreaSquareMeters,
		AvailableFrom:    listing.AvailableFrom,
		ThumbnailURL:     listing.ThumbnailURL,
		Photos:           append([]string(nil), listing.Photos...),
		UpdatedAt:        listing.UpdatedAt,
		State:            string(listing.State),
	}
}

func MapHostListingDetail(listing *domainlistings.Listing) HostListingDetail {
	if listing == nil {
		return HostListingDetail{}
	}
	address := ListingAddress{
		Line1:   listing.Address.Line1,
		Line2:   listing.Address.Line2,
		City:    listing.Address.City,
		Country: listing.Address.Country,
		Lat:     listing.Address.Lat,
		Lon:     listing.Address.Lon,
	}
	return HostListingDetail{
		ID:                   string(listing.ID),
		Title:                listing.Title,
		Description:          listing.Description,
		PropertyType:         listing.PropertyType,
		Address:              address,
		Amenities:            append([]string(nil), listing.Amenities...),
		GuestsLimit:          listing.GuestsLimit,
		MinNights:            listing.MinNights,
		MaxNights:            listing.MaxNights,
		HouseRules:           append([]string(nil), listing.HouseRules...),
		Host:                 ListingHost{ID: string(listing.Host)},
		State:                string(listing.State),
		Tags:                 append([]string(nil), listing.Tags...),
		Highlights:           append([]string(nil), listing.Highlights...),
		NightlyRateCents:     listing.NightlyRateCents,
		Bedrooms:             listing.Bedrooms,
		Bathrooms:            listing.Bathrooms,
		AreaSquareMeters:     listing.AreaSquareMeters,
		ThumbnailURL:         listing.ThumbnailURL,
		Photos:               append([]string(nil), listing.Photos...),
		CancellationPolicyID: listing.CancellationPolicyID,
		AvailableFrom:        listing.AvailableFrom,
		CreatedAt:            listing.CreatedAt,
		UpdatedAt:            listing.UpdatedAt,
		StateLabel:           toStatus(listing.State),
	}
}

func toStatus(state domainlistings.ListingState) string {
	switch state {
	case domainlistings.ListingDraft:
		return "draft"
	case domainlistings.ListingActive:
		return "published"
	case domainlistings.ListingSuspended:
		return "archived"
	default:
		return strings.ToLower(string(state))
	}
}
