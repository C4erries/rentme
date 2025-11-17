package availability

import (
	"time"

	"rentme/internal/domain/listings"
	"rentme/internal/domain/shared/daterange"
)

type CalendarBlocked struct {
	ListingID string
	Range     daterange.DateRange
	Reason    BlockReason
	At        time.Time
}

func (e CalendarBlocked) EventName() string     { return "calendar.blocked" }
func (e CalendarBlocked) AggregateID() string   { return e.ListingID }
func (e CalendarBlocked) OccurredAt() time.Time { return e.At }

type CalendarReleased struct {
	ListingID string
	Range     daterange.DateRange
	Reason    BlockReason
	At        time.Time
}

func (e CalendarReleased) EventName() string     { return "calendar.released" }
func (e CalendarReleased) AggregateID() string   { return e.ListingID }
func (e CalendarReleased) OccurredAt() time.Time { return e.At }

type CalendarOverbookingPrevented struct {
	ListingID string
	Range     daterange.DateRange
	At        time.Time
}

func (e CalendarOverbookingPrevented) EventName() string     { return "calendar.overbooking_prevented" }
func (e CalendarOverbookingPrevented) AggregateID() string   { return e.ListingID }
func (e CalendarOverbookingPrevented) OccurredAt() time.Time { return e.At }

func CalendarBlockedEvent(id listings.ListingID, r daterange.DateRange, reason BlockReason, at time.Time) CalendarBlocked {
	return CalendarBlocked{ListingID: string(id), Range: r, Reason: reason, At: at}
}

func CalendarReleasedEvent(id listings.ListingID, r daterange.DateRange, reason BlockReason, at time.Time) CalendarReleased {
	return CalendarReleased{ListingID: string(id), Range: r, Reason: reason, At: at}
}

func CalendarOverbookingPreventedEvent(id listings.ListingID, r daterange.DateRange, at time.Time) CalendarOverbookingPrevented {
	return CalendarOverbookingPrevented{ListingID: string(id), Range: r, At: at}
}
