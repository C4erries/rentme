package me

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"rentme/internal/app/dto"
	handlersupport "rentme/internal/app/handlers/support"
	"rentme/internal/app/queries"
	"rentme/internal/app/uow"
	domainlistings "rentme/internal/domain/listings"
	domainreviews "rentme/internal/domain/reviews"
)

const listGuestBookingsKey = "me.bookings.list"

type ListGuestBookingsQuery struct {
	GuestID string
}

func (q ListGuestBookingsQuery) Key() string { return listGuestBookingsKey }

type ListGuestBookingsHandler struct {
	UoWFactory uow.UoWFactory
	Logger     *slog.Logger
}

func (h *ListGuestBookingsHandler) Handle(ctx context.Context, q ListGuestBookingsQuery) (dto.GuestBookingCollection, error) {
	guestID := strings.TrimSpace(q.GuestID)
	if guestID == "" {
		return dto.GuestBookingCollection{}, errors.New("guest id is required")
	}
	unit, execCtx, cleanup, err := handlersupport.BeginReadOnlyUnit(ctx, h.UoWFactory)
	if err != nil {
		return dto.GuestBookingCollection{}, err
	}
	if cleanup != nil {
		defer cleanup()
	}

	bookings, err := unit.Booking().ListByGuest(execCtx, guestID)
	if err != nil {
		return dto.GuestBookingCollection{}, err
	}

	now := time.Now().UTC()
	listingCache := make(map[domainlistings.ListingID]*domainlistings.Listing)
	items := make([]dto.GuestBookingSummary, 0, len(bookings))
	for _, booking := range bookings {
		listing, err := loadListing(execCtx, unit.Listings(), booking.ListingID, listingCache)
		if err != nil {
			if h.Logger != nil {
				h.Logger.Warn("listing snapshot missing for booking", "booking_id", booking.ID, "listing_id", booking.ListingID, "error", err)
			}
		}
		canReview := !booking.Range.CheckOut.After(now)
		var review *domainreviews.Review
		if reviews := unit.Reviews(); reviews != nil {
			if existing, err := reviews.ByBooking(execCtx, booking.ID, guestID); err == nil {
				review = existing
				canReview = false
			} else if err != nil && !errors.Is(err, domainreviews.ErrNotFound) && h.Logger != nil {
				h.Logger.Warn("failed to check review", "booking_id", booking.ID, "guest_id", guestID, "error", err)
			}
		}
		items = append(items, dto.MapGuestBookingSummary(booking, listing, review, canReview))
	}

	if h.Logger != nil {
		h.Logger.Debug("guest bookings listed", "guest_id", guestID, "count", len(items))
	}

	return dto.GuestBookingCollection{Items: items}, nil
}

func loadListing(
	ctx context.Context,
	repo domainlistings.ListingRepository,
	id domainlistings.ListingID,
	cache map[domainlistings.ListingID]*domainlistings.Listing,
) (*domainlistings.Listing, error) {
	if listing, ok := cache[id]; ok {
		return listing, nil
	}
	listing, err := repo.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	cache[id] = listing
	return listing, nil
}

var _ queries.Handler[ListGuestBookingsQuery, dto.GuestBookingCollection] = (*ListGuestBookingsHandler)(nil)
