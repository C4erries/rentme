package booking

import (
	"context"
	"errors"
	"time"

	"rentme/internal/app/commands"
	"rentme/internal/app/middleware"
	"rentme/internal/app/outbox"
	"rentme/internal/app/policies"
	"rentme/internal/app/uow"
	domainbooking "rentme/internal/domain/booking"
	domainlistings "rentme/internal/domain/listings"
	domainrange "rentme/internal/domain/shared/daterange"
)

const requestBookingKey = "booking.request"

type RequestBookingCommand struct {
	CommandID       string
	ListingID       string
	GuestID         string
	CheckIn         time.Time
	CheckOut        time.Time
	Guests          int
	IdempotencyKeyV string
}

func (c RequestBookingCommand) Key() string { return requestBookingKey }

func (c RequestBookingCommand) IdempotencyKey() string { return c.IdempotencyKeyV }

func (c RequestBookingCommand) ResultPrototype() any { return &RequestBookingResult{} }

type RequestBookingResult struct {
	BookingID string `json:"booking_id"`
}

type RequestBookingHandler struct {
	UoWFactory uow.UoWFactory
	Pricing    policies.PricingPort
	Outbox     outbox.Outbox
	Encoder    outbox.EventEncoder
}

var ErrUnitOfWorkRequired = errors.New("booking: unit of work required")

func (h *RequestBookingHandler) Handle(ctx context.Context, cmd RequestBookingCommand) (*RequestBookingResult, error) {
	unit, ok := uow.FromContext(ctx)
	managed := false
	committed := false
	if !ok {
		if h.UoWFactory == nil {
			return nil, ErrUnitOfWorkRequired
		}
		var err error
		unit, err = h.UoWFactory.Begin(ctx, uow.TxOptions{})
		if err != nil {
			return nil, err
		}
		ctx = uow.ContextWithUnitOfWork(ctx, unit)
		managed = true
	}
	if managed {
		defer func() {
			if !committed {
				_ = unit.Rollback(ctx)
			}
		}()
	}

	dr, err := domainrange.New(cmd.CheckIn, cmd.CheckOut)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if err := domainbooking.ValidateDateRange(dr, now); err != nil {
		return nil, err
	}

	listing, err := unit.Listings().ByID(ctx, domainlistings.ListingID(cmd.ListingID))
	if err != nil {
		return nil, err
	}

	price, err := h.Pricing.Quote(ctx, listing, dr, cmd.Guests)
	if err != nil {
		return nil, err
	}

	booking, err := domainbooking.NewBooking(domainbooking.CreateParams{
		ID:        domainbooking.BookingID(cmd.CommandID),
		ListingID: listing.ID,
		GuestID:   cmd.GuestID,
		Range:     dr,
		Guests:    cmd.Guests,
		Price:     price,
		Policy: domainbooking.CancellationPolicySnapshot{
			PolicyID: listing.CancellationPolicyID,
		},
		CreatedAt: now,
	})
	if err != nil {
		return nil, err
	}

	if err := unit.Booking().Save(ctx, booking); err != nil {
		return nil, err
	}

	r := booking.PendingEvents()
	booking.ClearEvents()
	if err := outbox.RecordDomainEvents(ctx, h.Outbox, h.encoder(), r); err != nil {
		return nil, err
	}

	if managed {
		if err := unit.Commit(ctx); err != nil {
			return nil, err
		}
		committed = true
	}

	return &RequestBookingResult{BookingID: string(booking.ID)}, nil
}

func (h *RequestBookingHandler) encoder() outbox.EventEncoder {
	if h.Encoder != nil {
		return h.Encoder
	}
	return outbox.JSONEventEncoder{}
}

var _ commands.Handler[RequestBookingCommand, *RequestBookingResult] = (*RequestBookingHandler)(nil)
var _ middleware.IdempotentCommand = (*RequestBookingCommand)(nil)
