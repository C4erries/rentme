package listings

import (
	"context"
	"errors"
	"strings"
	"time"

	"rentme/internal/domain/shared/events"
)

var (
	ErrGuestsLimit     = errors.New("listings: guests limit must be at least 1")
	ErrNightsRange     = errors.New("listings: min nights must be <= max nights")
	ErrInvalidState    = errors.New("listings: invalid state transition")
	ErrAddressRequired = errors.New("listings: address must be provided when activating")
	ErrTitleRequired   = errors.New("listings: title is required")
	ErrNightlyRate     = errors.New("listings: nightly rate must be non-negative")
)

type ListingID string
type HostID string

type ListingState string

const (
	ListingDraft     ListingState = "DRAFT"
	ListingActive    ListingState = "ACTIVE"
	ListingSuspended ListingState = "SUSPENDED"
)

type Address struct {
	Line1   string
	Line2   string
	City    string
	Country string
	Lat     float64
	Lon     float64
}

func (a Address) Valid() bool {
	return strings.TrimSpace(a.Line1) != "" && strings.TrimSpace(a.City) != "" && strings.TrimSpace(a.Country) != ""
}

type Listing struct {
	ID                   ListingID
	Host                 HostID
	Title                string
	Description          string
	PropertyType         string
	Address              Address
	Amenities            []string
	GuestsLimit          int
	MinNights            int
	MaxNights            int
	HouseRules           []string
	CancellationPolicyID string
	State                ListingState
	Tags                 []string
	Highlights           []string
	NightlyRateCents     int64
	Bedrooms             int
	Bathrooms            int
	AreaSquareMeters     float64
	ThumbnailURL         string
	Rating               float64
	AvailableFrom        time.Time
	Version              int64
	CreatedAt            time.Time
	UpdatedAt            time.Time
	events.EventRecorder
}

type ListingRepository interface {
	ByID(ctx context.Context, id ListingID) (*Listing, error)
	Save(ctx context.Context, listing *Listing) error
	Search(ctx context.Context, params SearchParams) (SearchResult, error)
}

type CreateListingParams struct {
	ID                   ListingID
	Host                 HostID
	Title                string
	Description          string
	PropertyType         string
	Address              Address
	Amenities            []string
	GuestsLimit          int
	MinNights            int
	MaxNights            int
	HouseRules           []string
	CancellationPolicyID string
	Tags                 []string
	Highlights           []string
	NightlyRateCents     int64
	Bedrooms             int
	Bathrooms            int
	AreaSquareMeters     float64
	ThumbnailURL         string
	Rating               float64
	AvailableFrom        time.Time
	Now                  time.Time
}

func NewListing(params CreateListingParams) (*Listing, error) {
	if strings.TrimSpace(string(params.ID)) == "" {
		return nil, errors.New("listings: id is required")
	}
	if strings.TrimSpace(string(params.Host)) == "" {
		return nil, errors.New("listings: host is required")
	}
	if strings.TrimSpace(params.Title) == "" {
		return nil, ErrTitleRequired
	}
	if params.GuestsLimit < 1 {
		return nil, ErrGuestsLimit
	}
	if params.MinNights > params.MaxNights {
		return nil, ErrNightsRange
	}
	if params.NightlyRateCents < 0 {
		return nil, ErrNightlyRate
	}
	availableFrom := params.AvailableFrom
	if availableFrom.IsZero() {
		availableFrom = params.Now
	}

	listing := &Listing{
		ID:                   params.ID,
		Host:                 params.Host,
		Title:                strings.TrimSpace(params.Title),
		Description:          strings.TrimSpace(params.Description),
		PropertyType:         strings.TrimSpace(params.PropertyType),
		Address:              params.Address,
		Amenities:            append([]string(nil), params.Amenities...),
		GuestsLimit:          params.GuestsLimit,
		MinNights:            params.MinNights,
		MaxNights:            params.MaxNights,
		HouseRules:           append([]string(nil), params.HouseRules...),
		CancellationPolicyID: params.CancellationPolicyID,
		State:                ListingDraft,
		Tags:                 append([]string(nil), params.Tags...),
		Highlights:           append([]string(nil), params.Highlights...),
		NightlyRateCents:     params.NightlyRateCents,
		Bedrooms:             params.Bedrooms,
		Bathrooms:            params.Bathrooms,
		AreaSquareMeters:     params.AreaSquareMeters,
		ThumbnailURL:         strings.TrimSpace(params.ThumbnailURL),
		Rating:               params.Rating,
		AvailableFrom:        availableFrom.UTC(),
		CreatedAt:            params.Now.UTC(),
		UpdatedAt:            params.Now.UTC(),
	}

	listing.Record(newListingCreatedEvent(listing.ID, listing.Host, listing.CreatedAt))
	return listing, nil
}

func (l *Listing) Activate(now time.Time) error {
	if l.State == ListingActive {
		return nil
	}
	if !l.Address.Valid() {
		return ErrAddressRequired
	}
	if l.GuestsLimit < 1 {
		return ErrGuestsLimit
	}
	if l.MinNights > l.MaxNights {
		return ErrNightsRange
	}
	l.State = ListingActive
	l.UpdatedAt = now.UTC()
	l.Record(newListingActivatedEvent(l.ID, l.Host, l.UpdatedAt))
	return nil
}

func (l *Listing) Suspend(now time.Time, reason string) error {
	if l.State != ListingActive {
		return ErrInvalidState
	}
	l.State = ListingSuspended
	l.UpdatedAt = now.UTC()
	l.Record(newListingSuspendedEvent(l.ID, reason, l.UpdatedAt))
	return nil
}

func (l *Listing) UpdateDetails(title, description string, rules, amenities []string, now time.Time) error {
	if strings.TrimSpace(title) == "" {
		return ErrTitleRequired
	}
	l.Title = strings.TrimSpace(title)
	l.Description = strings.TrimSpace(description)
	l.Amenities = append([]string(nil), amenities...)
	l.HouseRules = append([]string(nil), rules...)
	l.UpdatedAt = now.UTC()
	l.Record(newListingUpdatedEvent(l.ID, now.UTC()))
	return nil
}

func newListingCreatedEvent(id ListingID, host HostID, at time.Time) events.DomainEvent {
	return ListingCreatedEvent{ListingID: id, HostID: host, At: at}
}

func newListingActivatedEvent(id ListingID, host HostID, at time.Time) events.DomainEvent {
	return ListingActivatedEvent{ListingID: id, HostID: host, At: at}
}

func newListingSuspendedEvent(id ListingID, reason string, at time.Time) events.DomainEvent {
	return ListingSuspendedEvent{ListingID: id, Reason: reason, At: at}
}

func newListingUpdatedEvent(id ListingID, at time.Time) events.DomainEvent {
	return ListingUpdatedEvent{ListingID: id, At: at}
}
