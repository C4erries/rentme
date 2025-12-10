package listings

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"rentme/internal/app/commands"
	"rentme/internal/app/dto"
	"rentme/internal/app/uow"
	domainlistings "rentme/internal/domain/listings"
)

const (
	createHostListingKey    = "host.listings.create"
	updateHostListingKey    = "host.listings.update"
	publishHostListingKey   = "host.listings.publish"
	unpublishHostListingKey = "host.listings.unpublish"
)

type HostListingPayload struct {
	Title                string
	Description          string
	PropertyType         string
	Address              domainlistings.Address
	Amenities            []string
	HouseRules           []string
	Tags                 []string
	Highlights           []string
	ThumbnailURL         string
	CancellationPolicyID string
	GuestsLimit          int
	MinNights            int
	MaxNights            int
	NightlyRateCents     int64
	Bedrooms             int
	Bathrooms            int
	Floor                int
	FloorsTotal          int
	RenovationScore      int
	BuildingAgeYears     int
	AreaSquareMeters     float64
	AvailableFrom        time.Time
	Photos               []string
}

type CreateHostListingCommand struct {
	HostID  string
	Payload HostListingPayload
}

func (c CreateHostListingCommand) Key() string { return createHostListingKey }

type CreateHostListingHandler struct {
	Logger *slog.Logger
}

func (h *CreateHostListingHandler) Handle(ctx context.Context, cmd CreateHostListingCommand) (*dto.HostListingDetail, error) {
	if strings.TrimSpace(cmd.HostID) == "" {
		return nil, errors.New("host id is required")
	}
	unit, ok := uow.FromContext(ctx)
	if !ok {
		return nil, uow.ErrUnitOfWorkMissing
	}

	listingID := domainlistings.ListingID(uuid.NewString())
	listing, err := domainlistings.NewListing(domainlistings.CreateListingParams{
		ID:                   listingID,
		Host:                 domainlistings.HostID(cmd.HostID),
		Title:                cmd.Payload.Title,
		Description:          cmd.Payload.Description,
		PropertyType:         cmd.Payload.PropertyType,
		Address:              cmd.Payload.Address,
		Amenities:            cmd.Payload.Amenities,
		GuestsLimit:          cmd.Payload.GuestsLimit,
		MinNights:            cmd.Payload.MinNights,
		MaxNights:            cmd.Payload.MaxNights,
		HouseRules:           cmd.Payload.HouseRules,
		CancellationPolicyID: cmd.Payload.CancellationPolicyID,
		Tags:                 cmd.Payload.Tags,
		Highlights:           cmd.Payload.Highlights,
		NightlyRateCents:     cmd.Payload.NightlyRateCents,
		Bedrooms:             cmd.Payload.Bedrooms,
		Bathrooms:            cmd.Payload.Bathrooms,
		Floor:                cmd.Payload.Floor,
		FloorsTotal:          cmd.Payload.FloorsTotal,
		RenovationScore:      cmd.Payload.RenovationScore,
		BuildingAgeYears:     cmd.Payload.BuildingAgeYears,
		AreaSquareMeters:     cmd.Payload.AreaSquareMeters,
		ThumbnailURL:         cmd.Payload.ThumbnailURL,
		Photos:               cmd.Payload.Photos,
		AvailableFrom:        cmd.Payload.AvailableFrom,
		Now:                  time.Now(),
	})
	if err != nil {
		return nil, err
	}

	if err := unit.Listings().Save(ctx, listing); err != nil {
		return nil, err
	}

	if h.Logger != nil {
		h.Logger.Info("host listing created", "listing_id", listing.ID, "host_id", cmd.HostID)
	}

	result := dto.MapHostListingDetail(listing)
	return &result, nil
}

type UpdateHostListingCommand struct {
	HostID    string
	ListingID string
	Payload   HostListingPayload
}

func (c UpdateHostListingCommand) Key() string { return updateHostListingKey }

type UpdateHostListingHandler struct {
	Logger *slog.Logger
}

func (h *UpdateHostListingHandler) Handle(ctx context.Context, cmd UpdateHostListingCommand) (*dto.HostListingDetail, error) {
	if strings.TrimSpace(cmd.HostID) == "" {
		return nil, errors.New("host id is required")
	}
	if strings.TrimSpace(cmd.ListingID) == "" {
		return nil, errors.New("listing id is required")
	}
	unit, ok := uow.FromContext(ctx)
	if !ok {
		return nil, uow.ErrUnitOfWorkMissing
	}

	listing, err := unit.Listings().ByID(ctx, domainlistings.ListingID(cmd.ListingID))
	if err != nil {
		return nil, err
	}
	if listing.Host != domainlistings.HostID(cmd.HostID) {
		return nil, ErrListingNotOwned
	}

	if err := listing.UpdateAttributes(domainlistings.UpdateListingParams{
		Title:                cmd.Payload.Title,
		Description:          cmd.Payload.Description,
		PropertyType:         cmd.Payload.PropertyType,
		Address:              cmd.Payload.Address,
		Amenities:            cmd.Payload.Amenities,
		HouseRules:           cmd.Payload.HouseRules,
		Tags:                 cmd.Payload.Tags,
		Highlights:           cmd.Payload.Highlights,
		ThumbnailURL:         cmd.Payload.ThumbnailURL,
		CancellationPolicyID: cmd.Payload.CancellationPolicyID,
		GuestsLimit:          cmd.Payload.GuestsLimit,
		MinNights:            cmd.Payload.MinNights,
		MaxNights:            cmd.Payload.MaxNights,
		NightlyRateCents:     cmd.Payload.NightlyRateCents,
		Bedrooms:             cmd.Payload.Bedrooms,
		Bathrooms:            cmd.Payload.Bathrooms,
		Floor:                cmd.Payload.Floor,
		FloorsTotal:          cmd.Payload.FloorsTotal,
		RenovationScore:      cmd.Payload.RenovationScore,
		BuildingAgeYears:     cmd.Payload.BuildingAgeYears,
		AreaSquareMeters:     cmd.Payload.AreaSquareMeters,
		AvailableFrom:        cmd.Payload.AvailableFrom,
		Photos:               cmd.Payload.Photos,
		Now:                  time.Now(),
	}); err != nil {
		return nil, err
	}

	if err := unit.Listings().Save(ctx, listing); err != nil {
		return nil, err
	}

	if h.Logger != nil {
		h.Logger.Info("host listing updated", "listing_id", listing.ID, "host_id", cmd.HostID)
	}

	result := dto.MapHostListingDetail(listing)
	return &result, nil
}

type PublishHostListingCommand struct {
	HostID    string
	ListingID string
}

func (c PublishHostListingCommand) Key() string { return publishHostListingKey }

type PublishHostListingHandler struct {
	Logger *slog.Logger
}

func (h *PublishHostListingHandler) Handle(ctx context.Context, cmd PublishHostListingCommand) (*dto.HostListingDetail, error) {
	if strings.TrimSpace(cmd.HostID) == "" {
		return nil, errors.New("host id is required")
	}
	if strings.TrimSpace(cmd.ListingID) == "" {
		return nil, errors.New("listing id is required")
	}
	unit, ok := uow.FromContext(ctx)
	if !ok {
		return nil, uow.ErrUnitOfWorkMissing
	}

	listing, err := unit.Listings().ByID(ctx, domainlistings.ListingID(cmd.ListingID))
	if err != nil {
		return nil, err
	}
	if listing.Host != domainlistings.HostID(cmd.HostID) {
		return nil, ErrListingNotOwned
	}

	if err := listing.Activate(time.Now()); err != nil {
		return nil, err
	}
	if err := unit.Listings().Save(ctx, listing); err != nil {
		return nil, err
	}

	if h.Logger != nil {
		h.Logger.Info("host listing published", "listing_id", listing.ID, "host_id", cmd.HostID)
	}

	result := dto.MapHostListingDetail(listing)
	return &result, nil
}

type UnpublishHostListingCommand struct {
	HostID    string
	ListingID string
}

func (c UnpublishHostListingCommand) Key() string { return unpublishHostListingKey }

type UnpublishHostListingHandler struct {
	Logger *slog.Logger
}

func (h *UnpublishHostListingHandler) Handle(ctx context.Context, cmd UnpublishHostListingCommand) (*dto.HostListingDetail, error) {
	if strings.TrimSpace(cmd.HostID) == "" {
		return nil, errors.New("host id is required")
	}
	if strings.TrimSpace(cmd.ListingID) == "" {
		return nil, errors.New("listing id is required")
	}
	unit, ok := uow.FromContext(ctx)
	if !ok {
		return nil, uow.ErrUnitOfWorkMissing
	}

	listing, err := unit.Listings().ByID(ctx, domainlistings.ListingID(cmd.ListingID))
	if err != nil {
		return nil, err
	}
	if listing.Host != domainlistings.HostID(cmd.HostID) {
		return nil, ErrListingNotOwned
	}

	if err := listing.Suspend(time.Now(), "host-request"); err != nil {
		return nil, err
	}
	if err := unit.Listings().Save(ctx, listing); err != nil {
		return nil, err
	}

	if h.Logger != nil {
		h.Logger.Info("host listing unpublished", "listing_id", listing.ID, "host_id", cmd.HostID)
	}

	result := dto.MapHostListingDetail(listing)
	return &result, nil
}

var (
	_ commands.Handler[CreateHostListingCommand, *dto.HostListingDetail]    = (*CreateHostListingHandler)(nil)
	_ commands.Handler[UpdateHostListingCommand, *dto.HostListingDetail]    = (*UpdateHostListingHandler)(nil)
	_ commands.Handler[PublishHostListingCommand, *dto.HostListingDetail]   = (*PublishHostListingHandler)(nil)
	_ commands.Handler[UnpublishHostListingCommand, *dto.HostListingDetail] = (*UnpublishHostListingHandler)(nil)
)
