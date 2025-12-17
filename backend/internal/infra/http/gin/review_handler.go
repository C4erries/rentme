package ginserver

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	gin "github.com/gin-gonic/gin"

	"rentme/internal/app/commands"
	"rentme/internal/app/dto"
	reviewsapp "rentme/internal/app/handlers/reviews"
	"rentme/internal/app/queries"
	"rentme/internal/app/uow"
	domainbooking "rentme/internal/domain/booking"
	domainreviews "rentme/internal/domain/reviews"
)

type ReviewsHandler struct {
	Commands commands.Bus
	Queries  queries.Bus
	Logger   *slog.Logger
}

type submitReviewRequest struct {
	Rating int    `json:"rating"`
	Text   string `json:"text"`
}

func (h ReviewsHandler) Submit(c *gin.Context) {
	user, ok := requireRole(c, "")
	if !ok {
		return
	}
	if h.Commands == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "reviews: commands unavailable"})
		return
	}
	bookingID := c.Param("id")
	if bookingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "booking id is required"})
		return
	}
	var req submitReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cmd := reviewsapp.SubmitReviewCommand{
		BookingID: bookingID,
		AuthorID:  user.ID,
		Rating:    req.Rating,
		Text:      req.Text,
		Now:       time.Now().UTC(),
	}
	review, err := commands.Dispatch[reviewsapp.SubmitReviewCommand, dto.Review](c.Request.Context(), h.Commands, cmd)
	if err != nil {
		h.handleSubmitError(c, err)
		return
	}
	c.JSON(http.StatusCreated, review)
}

func (h ReviewsHandler) handleSubmitError(c *gin.Context, err error) {
	var status int
	switch {
	case errors.Is(err, domainreviews.ErrInvalidRating):
		status = http.StatusBadRequest
	case errors.Is(err, reviewsapp.ErrStayNotFinished):
		status = http.StatusBadRequest
	case errors.Is(err, reviewsapp.ErrBookingOwnership):
		status = http.StatusForbidden
	case errors.Is(err, reviewsapp.ErrDuplicateReview):
		status = http.StatusConflict
	case errors.Is(err, domainbooking.ErrBookingNotFound):
		status = http.StatusNotFound
	case errors.Is(err, uow.ErrUnitOfWorkMissing):
		status = http.StatusServiceUnavailable
	default:
		status = http.StatusInternalServerError
	}
	if h.Logger != nil {
		h.Logger.Warn("review submit failed", "status", status, "error", err)
	}
	c.JSON(status, gin.H{"error": err.Error()})
}

func (h ReviewsHandler) ListByListing(c *gin.Context) {
	if h.Queries == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "reviews: queries unavailable"})
		return
	}
	listingID := c.Param("id")
	if listingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "listing id is required"})
		return
	}
	limit := parsePositiveInt(c.Query("limit"), 20)
	offset := parsePositiveInt(c.Query("offset"), 0)

	query := reviewsapp.ListListingReviewsQuery{
		ListingID: listingID,
		Limit:     limit,
		Offset:    offset,
	}
	result, err := queries.Ask[reviewsapp.ListListingReviewsQuery, dto.ReviewCollection](c.Request.Context(), h.Queries, query)
	if err != nil {
		if errors.Is(err, reviewsapp.ErrListingNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "listing not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func parsePositiveInt(raw string, fallback int) int {
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return fallback
	}
	return value
}

var _ ReviewsHTTP = ReviewsHandler{}
