package reviews

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"rentme/internal/app/commands"
	"rentme/internal/app/dto"
	"rentme/internal/app/uow"
	domainbooking "rentme/internal/domain/booking"
	domainreviews "rentme/internal/domain/reviews"
)

const submitReviewKey = "reviews.submit"

var (
	ErrBookingOwnership = errors.New("reviews: booking does not belong to current user")
	ErrStayNotFinished  = errors.New("reviews: stay is not finished yet")
	ErrDuplicateReview  = errors.New("reviews: review already exists for booking")
)

// SubmitReviewCommand creates a new review for a booking.
type SubmitReviewCommand struct {
	BookingID string
	AuthorID  string
	Rating    int
	Text      string
	Now       time.Time
}

func (c SubmitReviewCommand) Key() string { return submitReviewKey }

// SubmitReviewHandler validates and stores a new review, updating listing rating.
type SubmitReviewHandler struct {
	UoWFactory uow.UoWFactory
	Logger     *slog.Logger
}

func (h *SubmitReviewHandler) Handle(ctx context.Context, cmd SubmitReviewCommand) (dto.Review, error) {
	unit, ok := uow.FromContext(ctx)
	managed := false
	committed := false
	if !ok {
		if h.UoWFactory == nil {
			return dto.Review{}, uow.ErrUnitOfWorkMissing
		}
		var err error
		unit, err = h.UoWFactory.Begin(ctx, uow.TxOptions{})
		if err != nil {
			return dto.Review{}, err
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

	now := cmd.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	booking, err := unit.Booking().ByID(ctx, domainbooking.BookingID(cmd.BookingID))
	if err != nil {
		return dto.Review{}, err
	}
	if booking.GuestID != cmd.AuthorID {
		return dto.Review{}, ErrBookingOwnership
	}
	if booking.Range.CheckOut.After(now) {
		return dto.Review{}, ErrStayNotFinished
	}

	if existing, err := unit.Reviews().ByBooking(ctx, booking.ID, cmd.AuthorID); err == nil && existing != nil {
		return dto.Review{}, ErrDuplicateReview
	} else if err != nil && !errors.Is(err, domainreviews.ErrNotFound) {
		return dto.Review{}, err
	}

	review, err := domainreviews.Submit(domainreviews.SubmitParams{
		ID:        domainreviews.ReviewID(newReviewID()),
		BookingID: booking.ID,
		AuthorID:  cmd.AuthorID,
		ListingID: booking.ListingID,
		Rating:    cmd.Rating,
		Text:      cmd.Text,
		CreatedAt: now,
	})
	if err != nil {
		return dto.Review{}, err
	}
	if err := unit.Reviews().Save(ctx, review); err != nil {
		return dto.Review{}, err
	}

	if err := recalculateListingRating(ctx, unit, booking.ListingID, now); err != nil {
		return dto.Review{}, err
	}

	if managed {
		if err := unit.Commit(ctx); err != nil {
			return dto.Review{}, err
		}
		committed = true
	}

	if h.Logger != nil {
		h.Logger.Info("review submitted", "booking_id", booking.ID, "listing_id", booking.ListingID, "author_id", cmd.AuthorID, "rating", cmd.Rating)
	}

	return dto.MapReview(review), nil
}

func newReviewID() string {
	return uuid.NewString()
}

var _ commands.Handler[SubmitReviewCommand, dto.Review] = (*SubmitReviewHandler)(nil)
