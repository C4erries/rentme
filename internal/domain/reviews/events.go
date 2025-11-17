package reviews

import (
	"time"

	"rentme/internal/domain/booking"
	"rentme/internal/domain/listings"
)

type ReviewSubmitted struct {
	ReviewID  ReviewID
	BookingID booking.BookingID
	ListingID listings.ListingID
	Rating    int
	At        time.Time
}

func (e ReviewSubmitted) EventName() string     { return "review.submitted" }
func (e ReviewSubmitted) AggregateID() string   { return string(e.ReviewID) }
func (e ReviewSubmitted) OccurredAt() time.Time { return e.At }

type ReviewUpdated struct {
	ReviewID ReviewID
	At       time.Time
}

func (e ReviewUpdated) EventName() string     { return "review.updated" }
func (e ReviewUpdated) AggregateID() string   { return string(e.ReviewID) }
func (e ReviewUpdated) OccurredAt() time.Time { return e.At }
