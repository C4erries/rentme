package reviews

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"rentme/internal/app/commands"
	"rentme/internal/app/dto"
	"rentme/internal/app/uow"
	domainreviews "rentme/internal/domain/reviews"
)

const updateReviewKey = "reviews.update"

var ErrReviewOwnership = errors.New("reviews: review does not belong to current user")

// UpdateReviewCommand updates an existing review (demo: editing is always allowed).
type UpdateReviewCommand struct {
	ReviewID string
	AuthorID string
	Rating   int
	Text     string
	Now      time.Time
}

func (c UpdateReviewCommand) Key() string { return updateReviewKey }

// UpdateReviewHandler updates the review and recalculates listing rating.
type UpdateReviewHandler struct {
	UoWFactory uow.UoWFactory
	Logger     *slog.Logger
}

func (h *UpdateReviewHandler) Handle(ctx context.Context, cmd UpdateReviewCommand) (dto.Review, error) {
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

	if cmd.ReviewID == "" {
		return dto.Review{}, errors.New("review id is required")
	}

	now := cmd.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	review, err := unit.Reviews().ByID(ctx, domainreviews.ReviewID(cmd.ReviewID))
	if err != nil {
		return dto.Review{}, err
	}
	if review.AuthorID != cmd.AuthorID {
		return dto.Review{}, ErrReviewOwnership
	}
	if err := review.Update(cmd.Rating, cmd.Text, now); err != nil {
		return dto.Review{}, err
	}
	if err := unit.Reviews().Save(ctx, review); err != nil {
		return dto.Review{}, err
	}
	if err := recalculateListingRating(ctx, unit, review.ListingID, now); err != nil {
		return dto.Review{}, err
	}

	if managed {
		if err := unit.Commit(ctx); err != nil {
			return dto.Review{}, err
		}
		committed = true
	}

	if h.Logger != nil {
		h.Logger.Info("review updated", "review_id", review.ID, "listing_id", review.ListingID, "author_id", review.AuthorID)
	}

	return dto.MapReview(review), nil
}

var _ commands.Handler[UpdateReviewCommand, dto.Review] = (*UpdateReviewHandler)(nil)
