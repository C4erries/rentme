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
	domainpricing "rentme/internal/domain/pricing"
	domainrange "rentme/internal/domain/shared/daterange"
	"rentme/internal/domain/shared/money"
)

const requestBookingKey = "booking.request"

type RequestBookingCommand struct {
	CommandID       string
	ListingID       string
	GuestID         string
	CheckIn         time.Time
	CheckOut        time.Time
	Months          int
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

	listing, err := unit.Listings().ByID(ctx, domainlistings.ListingID(cmd.ListingID))
	if err != nil {
		return nil, err
	}

	rentalTerm := listing.RentalTermType
	if rentalTerm == "" {
		rentalTerm = domainlistings.RentalTermLong
	}

	dr, months, priceUnit, err := resolveBookingRange(rentalTerm, cmd.CheckIn, cmd.CheckOut, cmd.Months)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if err := domainbooking.ValidateDateRange(dr, now); err != nil {
		return nil, err
	}

	units := dr.Nights()
	if priceUnit == "month" {
		units = months
	}
	price, err := buildBookingPrice(listing.RateRub, units)
	if err != nil {
		return nil, err
	}

	booking, err := domainbooking.NewBooking(domainbooking.CreateParams{
		ID:        domainbooking.BookingID(cmd.CommandID),
		ListingID: listing.ID,
		GuestID:   cmd.GuestID,
		Range:     dr,
		Guests:    cmd.Guests,
		Months:    months,
		PriceUnit: priceUnit,
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

func resolveBookingRange(term domainlistings.RentalTermType, checkIn, checkOut time.Time, months int) (domainrange.DateRange, int, string, error) {
	switch term {
	case domainlistings.RentalTermLong:
		if months < 1 || months > 12 {
			return domainrange.DateRange{}, 0, "", errors.New("months must be between 1 and 12")
		}
		computedOut := checkIn.AddDate(0, months, 0)
		dr, err := domainrange.New(checkIn, computedOut)
		if err != nil {
			return domainrange.DateRange{}, 0, "", err
		}
		return dr, months, "month", nil
	default:
		if checkOut.IsZero() {
			return domainrange.DateRange{}, 0, "", errors.New("check_out is required")
		}
		dr, err := domainrange.New(checkIn, checkOut)
		if err != nil {
			return domainrange.DateRange{}, 0, "", err
		}
		return dr, 0, "night", nil
	}
}

func buildBookingPrice(rateRub int64, units int) (domainpricing.PriceBreakdown, error) {
	if units <= 0 {
		return domainpricing.PriceBreakdown{}, errors.New("booking: units must be positive")
	}
	breakdown := domainpricing.PriceBreakdown{
		Nights:  units,
		Nightly: money.Must(rateRub, "RUB"),
	}
	if err := breakdown.RecalculateTotal(); err != nil {
		return domainpricing.PriceBreakdown{}, err
	}
	return breakdown, nil
}

var _ commands.Handler[RequestBookingCommand, *RequestBookingResult] = (*RequestBookingHandler)(nil)
var _ middleware.IdempotentCommand = (*RequestBookingCommand)(nil)
