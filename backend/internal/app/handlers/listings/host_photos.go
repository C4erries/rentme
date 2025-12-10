package listings

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"rentme/internal/app/commands"
	"rentme/internal/app/dto"
	"rentme/internal/app/uow"
	domainlistings "rentme/internal/domain/listings"
	"rentme/internal/infra/storage/s3"
)

const uploadHostListingPhotoKey = "host.listings.photos.upload"

type UploadHostListingPhotoCommand struct {
	HostID      string
	ListingID   string
	ObjectKey   string
	ContentType string
	Reader      io.Reader
}

func (c UploadHostListingPhotoCommand) Key() string { return uploadHostListingPhotoKey }

type UploadHostListingPhotoHandler struct {
	Logger   *slog.Logger
	Uploader s3.Uploader
	Now      func() time.Time
}

func (h *UploadHostListingPhotoHandler) Handle(ctx context.Context, cmd UploadHostListingPhotoCommand) (*dto.HostListingPhotoUploadResult, error) {
	if h.Uploader == nil {
		return nil, errors.New("photo uploader unavailable")
	}
	if strings.TrimSpace(cmd.HostID) == "" {
		return nil, errors.New("host id is required")
	}
	if strings.TrimSpace(cmd.ListingID) == "" {
		return nil, errors.New("listing id is required")
	}
	if cmd.Reader == nil {
		return nil, errors.New("photo reader is required")
	}
	if strings.TrimSpace(cmd.ObjectKey) == "" {
		return nil, errors.New("object key is required")
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

	publicURL, err := h.Uploader.Upload(ctx, cmd.ObjectKey, cmd.Reader, cmd.ContentType)
	if err != nil {
		return nil, fmt.Errorf("upload photo: %w", err)
	}

	now := time.Now()
	if h.Now != nil {
		now = h.Now()
	}
	if err := listing.AddPhoto(publicURL, now); err != nil {
		return nil, err
	}
	if err := unit.Listings().Save(ctx, listing); err != nil {
		return nil, err
	}

	if h.Logger != nil {
		h.Logger.Info("listing photo added", "listing_id", listing.ID, "host_id", cmd.HostID, "object_key", cmd.ObjectKey)
	}

	result := dto.HostListingPhotoUploadResult{
		ListingID:    cmd.ListingID,
		Photos:       append([]string(nil), listing.Photos...),
		ThumbnailURL: listing.ThumbnailURL,
	}
	return &result, nil
}

var _ commands.Handler[UploadHostListingPhotoCommand, *dto.HostListingPhotoUploadResult] = (*UploadHostListingPhotoHandler)(nil)
