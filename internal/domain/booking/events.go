package booking

import (
	"time"

	"rentme/internal/domain/listings"
	"rentme/internal/domain/shared/daterange"
	"rentme/internal/domain/shared/money"
)

type BookingRequested struct {
	BookingID   BookingID
	ListingID   listings.ListingID
	GuestID     string
	Range       daterange.DateRange
	GuestsCount int
	QuotedPrice money.Money
	At          time.Time
}

func (e BookingRequested) EventName() string     { return "booking.requested" }
func (e BookingRequested) AggregateID() string   { return string(e.BookingID) }
func (e BookingRequested) OccurredAt() time.Time { return e.At }

type BookingAccepted struct {
	BookingID BookingID
	At        time.Time
}

func (e BookingAccepted) EventName() string     { return "booking.accepted" }
func (e BookingAccepted) AggregateID() string   { return string(e.BookingID) }
func (e BookingAccepted) OccurredAt() time.Time { return e.At }

type BookingDeclined struct {
	BookingID BookingID
	Reason    string
	At        time.Time
}

func (e BookingDeclined) EventName() string     { return "booking.declined" }
func (e BookingDeclined) AggregateID() string   { return string(e.BookingID) }
func (e BookingDeclined) OccurredAt() time.Time { return e.At }

type BookingConfirmed struct {
	BookingID BookingID
	ListingID listings.ListingID
	Range     daterange.DateRange
	Total     money.Money
	At        time.Time
}

func (e BookingConfirmed) EventName() string     { return "booking.confirmed" }
func (e BookingConfirmed) AggregateID() string   { return string(e.BookingID) }
func (e BookingConfirmed) OccurredAt() time.Time { return e.At }

type BookingCancelled struct {
	BookingID BookingID
	Refund    money.Money
	Penalty   money.Money
	Reason    string
	At        time.Time
}

func (e BookingCancelled) EventName() string     { return "booking.cancelled" }
func (e BookingCancelled) AggregateID() string   { return string(e.BookingID) }
func (e BookingCancelled) OccurredAt() time.Time { return e.At }

type CheckInCompleted struct {
	BookingID BookingID
	At        time.Time
}

func (e CheckInCompleted) EventName() string     { return "booking.checkin_completed" }
func (e CheckInCompleted) AggregateID() string   { return string(e.BookingID) }
func (e CheckInCompleted) OccurredAt() time.Time { return e.At }

type CheckOutCompleted struct {
	BookingID BookingID
	At        time.Time
}

func (e CheckOutCompleted) EventName() string     { return "booking.checkout_completed" }
func (e CheckOutCompleted) AggregateID() string   { return string(e.BookingID) }
func (e CheckOutCompleted) OccurredAt() time.Time { return e.At }

type NoShowRecorded struct {
	BookingID BookingID
	At        time.Time
}

func (e NoShowRecorded) EventName() string     { return "booking.no_show" }
func (e NoShowRecorded) AggregateID() string   { return string(e.BookingID) }
func (e NoShowRecorded) OccurredAt() time.Time { return e.At }
