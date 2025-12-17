package booking

import (
	"context"
	"errors"
	"time"

	"rentme/internal/domain/listings"
	"rentme/internal/domain/pricing"
	"rentme/internal/domain/shared/daterange"
	"rentme/internal/domain/shared/events"
	"rentme/internal/domain/shared/money"
)

var (
	ErrInvalidGuests       = errors.New("booking: guests count must be positive")
	ErrInvalidState        = errors.New("booking: invalid state transition")
	ErrPaymentHoldRequired = errors.New("booking: payment hold required before confirmation")
	ErrBookingNotFound     = errors.New("booking: not found")
)

type BookingID string

type BookingState string

const (
	StatePending    BookingState = "PENDING"
	StateAccepted   BookingState = "ACCEPTED"
	StateDeclined   BookingState = "DECLINED"
	StateExpired    BookingState = "EXPIRED"
	StateConfirmed  BookingState = "CONFIRMED"
	StateCancelled  BookingState = "CANCELLED"
	StateCheckedIn  BookingState = "CHECKED_IN"
	StateCheckedOut BookingState = "CHECKED_OUT"
	StateNoShow     BookingState = "NO_SHOW"
)

type Booking struct {
	ID          BookingID
	ListingID   listings.ListingID
	GuestID     string
	Range       daterange.DateRange
	Guests      int
	Price       pricing.PriceBreakdown
	State       BookingState
	PaymentHold string
	Policy      CancellationPolicySnapshot
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Version     int64
	events.EventRecorder
}

type Repository interface {
	ByID(ctx context.Context, id BookingID) (*Booking, error)
	Save(ctx context.Context, booking *Booking) error
	ListByGuest(ctx context.Context, guestID string) ([]*Booking, error)
}

type CreateParams struct {
	ID        BookingID
	ListingID listings.ListingID
	GuestID   string
	Range     daterange.DateRange
	Guests    int
	Price     pricing.PriceBreakdown
	Policy    CancellationPolicySnapshot
	CreatedAt time.Time
	AllowZero bool
}

func NewBooking(params CreateParams) (*Booking, error) {
	if params.Guests <= 0 {
		return nil, ErrInvalidGuests
	}
	if params.GuestID == "" {
		return nil, errors.New("booking: guest id required")
	}
	if err := params.Price.RecalculateTotal(); err != nil {
		return nil, err
	}
	if params.Price.Total.Amount <= 0 && !params.AllowZero {
		return nil, errors.New("booking: total must be positive")
	}
	now := params.CreatedAt.UTC()
	b := &Booking{
		ID:        params.ID,
		ListingID: params.ListingID,
		GuestID:   params.GuestID,
		Range:     params.Range,
		Guests:    params.Guests,
		Price:     params.Price.Copy(),
		Policy:    params.Policy,
		State:     StatePending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	b.Record(BookingRequested{BookingID: b.ID, ListingID: b.ListingID, GuestID: b.GuestID, Range: b.Range, GuestsCount: b.Guests, QuotedPrice: b.Price.Total, At: now})
	return b, nil
}

func (b *Booking) Accept(now time.Time) error {
	if b.State != StatePending {
		return ErrInvalidState
	}
	b.State = StateAccepted
	b.UpdatedAt = now.UTC()
	b.Record(BookingAccepted{BookingID: b.ID, At: b.UpdatedAt})
	return nil
}

func (b *Booking) Decline(reason string, now time.Time) error {
	if b.State != StatePending && b.State != StateAccepted {
		return ErrInvalidState
	}
	b.State = StateDeclined
	b.UpdatedAt = now.UTC()
	b.Record(BookingDeclined{BookingID: b.ID, Reason: reason, At: b.UpdatedAt})
	return nil
}

func (b *Booking) Confirm(paymentHoldID string, now time.Time) error {
	if b.State != StateAccepted && b.State != StatePending {
		return ErrInvalidState
	}
	if b.Price.Total.Amount > 0 && paymentHoldID == "" {
		return ErrPaymentHoldRequired
	}
	b.PaymentHold = paymentHoldID
	b.State = StateConfirmed
	b.UpdatedAt = now.UTC()
	b.Record(BookingConfirmed{BookingID: b.ID, ListingID: b.ListingID, Range: b.Range, Total: b.Price.Total, At: b.UpdatedAt})
	return nil
}

func (b *Booking) Cancel(reason string, now time.Time) (money.Money, money.Money, error) {
	switch b.State {
	case StatePending, StateAccepted, StateConfirmed:
	default:
		return money.Money{}, money.Money{}, ErrInvalidState
	}
	refund, penalty, err := b.Policy.CalculateRefund(b.Price.Total, now, b.Range.CheckIn)
	if err != nil {
		return money.Money{}, money.Money{}, err
	}
	b.State = StateCancelled
	b.UpdatedAt = now.UTC()
	b.Record(BookingCancelled{BookingID: b.ID, Refund: refund, Penalty: penalty, Reason: reason, At: b.UpdatedAt})
	return refund, penalty, nil
}

func (b *Booking) CheckIn(now time.Time) error {
	if b.State != StateConfirmed {
		return ErrInvalidState
	}
	b.State = StateCheckedIn
	b.UpdatedAt = now.UTC()
	b.Record(CheckInCompleted{BookingID: b.ID, At: b.UpdatedAt})
	return nil
}

func (b *Booking) CheckOut(now time.Time) error {
	if b.State != StateCheckedIn {
		return ErrInvalidState
	}
	b.State = StateCheckedOut
	b.UpdatedAt = now.UTC()
	b.Record(CheckOutCompleted{BookingID: b.ID, At: b.UpdatedAt})
	return nil
}

func (b *Booking) MarkNoShow(now time.Time) error {
	if b.State != StateConfirmed {
		return ErrInvalidState
	}
	b.State = StateNoShow
	b.UpdatedAt = now.UTC()
	b.Record(NoShowRecorded{BookingID: b.ID, At: b.UpdatedAt})
	return nil
}
