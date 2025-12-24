package ginserver

import (
	"net/http"
	"time"

	gin "github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"rentme/internal/app/commands"
	BookingApp "rentme/internal/app/handlers/booking"
)

type BookingHandler struct {
	Commands commands.Bus
}

type createBookingRequest struct {
	ListingID string    `json:"listing_id"`
	CheckIn   time.Time `json:"check_in"`
	CheckOut  time.Time `json:"check_out"`
	Months    int       `json:"months"`
	Guests    int       `json:"guests"`
}

func (h BookingHandler) Create(c *gin.Context) {
	user, ok := requireRole(c, "")
	if !ok {
		return
	}
	if h.Commands == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "commands unavailable"})
		return
	}
	var req createBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cmd := BookingApp.RequestBookingCommand{
		CommandID:       generateCommandID(),
		ListingID:       req.ListingID,
		GuestID:         user.ID,
		CheckIn:         req.CheckIn,
		CheckOut:        req.CheckOut,
		Months:          req.Months,
		Guests:          req.Guests,
		IdempotencyKeyV: c.GetHeader("Idempotency-Key"),
	}
	result, err := commands.Dispatch[BookingApp.RequestBookingCommand, *BookingApp.RequestBookingResult](c.Request.Context(), h.Commands, cmd)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, result)
}

func (h BookingHandler) Accept(c *gin.Context) {
	c.Status(http.StatusNotImplemented)
}

func generateCommandID() string {
	return uuid.NewString()
}

var _ BookingHTTP = BookingHandler{}
