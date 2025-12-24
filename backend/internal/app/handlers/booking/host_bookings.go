package booking

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"time"

	"rentme/internal/app/commands"
	"rentme/internal/app/dto"
	handlersupport "rentme/internal/app/handlers/support"
	"rentme/internal/app/queries"
	"rentme/internal/app/uow"
	domainbooking "rentme/internal/domain/booking"
	domainlistings "rentme/internal/domain/listings"
)

const (
	listHostBookingsKey    = "host.bookings.list"
	confirmHostBookingKey  = "host.bookings.confirm"
	declineHostBookingKey  = "host.bookings.decline"
	demoPaymentHoldID      = "demo-hold"
	defaultHostListLimit   = 60
	allStatusesFilterValue = "ALL"
)

var ErrBookingNotOwned = errors.New("booking: not owned by host")

type ListHostBookingsQuery struct {
	HostID string
	Status string
}

func (q ListHostBookingsQuery) Key() string { return listHostBookingsKey }

type ListHostBookingsHandler struct {
	UoWFactory uow.UoWFactory
	Logger     *slog.Logger
}

func (h *ListHostBookingsHandler) Handle(ctx context.Context, q ListHostBookingsQuery) (dto.HostBookingCollection, error) {
	hostID := strings.TrimSpace(q.HostID)
	if hostID == "" {
		return dto.HostBookingCollection{}, errors.New("host id is required")
	}
	unit, execCtx, cleanup, err := handlersupport.BeginReadOnlyUnit(ctx, h.UoWFactory)
	if err != nil {
		return dto.HostBookingCollection{}, err
	}
	if cleanup != nil {
		defer cleanup()
	}

	listingsResult, err := unit.Listings().Search(execCtx, domainlistings.SearchParams{
		Host:   domainlistings.HostID(hostID),
		Limit: defaultHostListLimit,
	})
	if err != nil {
		return dto.HostBookingCollection{}, err
	}

	statusFilter := strings.ToUpper(strings.TrimSpace(q.Status))
	if statusFilter == "" {
		statusFilter = string(domainbooking.StatePending)
	}
	allStatuses := statusFilter == allStatusesFilterValue

	items := make([]dto.HostBookingSummary, 0)
	for _, listing := range listingsResult.Items {
		bookings, err := unit.Booking().ListByListing(execCtx, listing.ID)
		if err != nil {
			return dto.HostBookingCollection{}, err
		}
		for _, booking := range bookings {
			if !allStatuses && string(booking.State) != statusFilter {
				continue
			}
			items = append(items, dto.MapHostBookingSummary(booking, listing))
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})

	if h.Logger != nil {
		h.Logger.Debug("host bookings listed", "host_id", hostID, "count", len(items), "status", statusFilter)
	}

	return dto.HostBookingCollection{Items: items}, nil
}

type ConfirmHostBookingCommand struct {
	HostID    string
	BookingID string
}

func (c ConfirmHostBookingCommand) Key() string { return confirmHostBookingKey }

type DeclineHostBookingCommand struct {
	HostID    string
	BookingID string
	Reason    string
}

func (c DeclineHostBookingCommand) Key() string { return declineHostBookingKey }

type HostBookingActionResult struct {
	BookingID string `json:"booking_id"`
	Status    string `json:"status"`
}

type ConfirmHostBookingHandler struct {
	Logger *slog.Logger
}

func (h *ConfirmHostBookingHandler) Handle(ctx context.Context, cmd ConfirmHostBookingCommand) (*HostBookingActionResult, error) {
	hostID := strings.TrimSpace(cmd.HostID)
	if hostID == "" {
		return nil, errors.New("host id is required")
	}
	bookingID := strings.TrimSpace(cmd.BookingID)
	if bookingID == "" {
		return nil, errors.New("booking id is required")
	}
	unit, ok := uow.FromContext(ctx)
	if !ok {
		return nil, uow.ErrUnitOfWorkMissing
	}

	booking, err := unit.Booking().ByID(ctx, domainbooking.BookingID(bookingID))
	if err != nil {
		return nil, err
	}
	listing, err := unit.Listings().ByID(ctx, booking.ListingID)
	if err != nil {
		return nil, err
	}
	if listing.Host != domainlistings.HostID(hostID) {
		return nil, ErrBookingNotOwned
	}

	now := time.Now().UTC()
	if err := booking.Confirm(demoPaymentHoldID, now); err != nil {
		return nil, err
	}
	if err := unit.Booking().Save(ctx, booking); err != nil {
		return nil, err
	}

	if h.Logger != nil {
		h.Logger.Info("host booking confirmed", "booking_id", booking.ID, "host_id", hostID, "listing_id", booking.ListingID)
	}

	return &HostBookingActionResult{BookingID: string(booking.ID), Status: string(booking.State)}, nil
}

type DeclineHostBookingHandler struct {
	Logger *slog.Logger
}

func (h *DeclineHostBookingHandler) Handle(ctx context.Context, cmd DeclineHostBookingCommand) (*HostBookingActionResult, error) {
	hostID := strings.TrimSpace(cmd.HostID)
	if hostID == "" {
		return nil, errors.New("host id is required")
	}
	bookingID := strings.TrimSpace(cmd.BookingID)
	if bookingID == "" {
		return nil, errors.New("booking id is required")
	}
	unit, ok := uow.FromContext(ctx)
	if !ok {
		return nil, uow.ErrUnitOfWorkMissing
	}

	booking, err := unit.Booking().ByID(ctx, domainbooking.BookingID(bookingID))
	if err != nil {
		return nil, err
	}
	listing, err := unit.Listings().ByID(ctx, booking.ListingID)
	if err != nil {
		return nil, err
	}
	if listing.Host != domainlistings.HostID(hostID) {
		return nil, ErrBookingNotOwned
	}

	reason := strings.TrimSpace(cmd.Reason)
	if reason == "" {
		reason = "host-declined"
	}

	now := time.Now().UTC()
	if err := booking.Decline(reason, now); err != nil {
		return nil, err
	}
	if err := unit.Booking().Save(ctx, booking); err != nil {
		return nil, err
	}

	if h.Logger != nil {
		h.Logger.Info("host booking declined", "booking_id", booking.ID, "host_id", hostID, "listing_id", booking.ListingID, "reason", reason)
	}

	return &HostBookingActionResult{BookingID: string(booking.ID), Status: string(booking.State)}, nil
}

var _ queries.Handler[ListHostBookingsQuery, dto.HostBookingCollection] = (*ListHostBookingsHandler)(nil)
var _ commands.Handler[ConfirmHostBookingCommand, *HostBookingActionResult] = (*ConfirmHostBookingHandler)(nil)
var _ commands.Handler[DeclineHostBookingCommand, *HostBookingActionResult] = (*DeclineHostBookingHandler)(nil)
