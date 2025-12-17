package dto

import (
	"time"

	domainreviews "rentme/internal/domain/reviews"
)

// Review represents a public review payload.
type Review struct {
	ID        string    `json:"id"`
	BookingID string    `json:"booking_id"`
	ListingID string    `json:"listing_id"`
	AuthorID  string    `json:"author_id"`
	Rating    int       `json:"rating"`
	Text      string    `json:"text,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type ReviewCollection struct {
	Items []Review `json:"items"`
	Total int      `json:"total"`
}

// MapReview builds a DTO from a domain review.
func MapReview(review *domainreviews.Review) Review {
	if review == nil {
		return Review{}
	}
	return Review{
		ID:        string(review.ID),
		BookingID: string(review.BookingID),
		ListingID: string(review.ListingID),
		AuthorID:  review.AuthorID,
		Rating:    review.Rating,
		Text:      review.Text,
		CreatedAt: review.CreatedAt,
	}
}
