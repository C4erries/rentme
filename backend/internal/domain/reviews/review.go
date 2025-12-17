package reviews

import (
	"context"
	"errors"
	"strings"
	"time"

	"rentme/internal/domain/booking"
	"rentme/internal/domain/listings"
	"rentme/internal/domain/shared/events"
)

var (
	ErrInvalidRating = errors.New("reviews: rating must be between 1 and 5")
	ErrNotFound      = errors.New("reviews: not found")
)

type ReviewID string

type Review struct {
	ID        ReviewID
	BookingID booking.BookingID
	AuthorID  string
	ListingID listings.ListingID
	Rating    int
	Text      string
	CreatedAt time.Time
	Submitted bool
	events.EventRecorder
}

type Repository interface {
	ByBooking(ctx context.Context, bookingID booking.BookingID, authorID string) (*Review, error)
	ListByListing(ctx context.Context, listingID listings.ListingID, limit, offset int) ([]*Review, error)
	Save(ctx context.Context, review *Review) error
}

type SubmitParams struct {
	ID        ReviewID
	BookingID booking.BookingID
	AuthorID  string
	ListingID listings.ListingID
	Rating    int
	Text      string
	CreatedAt time.Time
}

func Submit(params SubmitParams) (*Review, error) {
	if params.Rating < 1 || params.Rating > 5 {
		return nil, ErrInvalidRating
	}
	review := &Review{
		ID:        params.ID,
		BookingID: params.BookingID,
		AuthorID:  params.AuthorID,
		ListingID: params.ListingID,
		Rating:    params.Rating,
		Text:      strings.TrimSpace(params.Text),
		CreatedAt: params.CreatedAt.UTC(),
		Submitted: true,
	}
	review.Record(ReviewSubmitted{ReviewID: review.ID, BookingID: review.BookingID, ListingID: review.ListingID, Rating: review.Rating, At: review.CreatedAt})
	return review, nil
}

func (r *Review) UpdateText(text string, now time.Time) error {
	if !r.Submitted {
		return errors.New("reviews: cannot update draft state")
	}
	r.Text = strings.TrimSpace(text)
	r.Record(ReviewUpdated{ReviewID: r.ID, At: now.UTC()})
	return nil
}
