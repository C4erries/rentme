package ginserver

import (
	"log/slog"
	"net/http"

	gin "github.com/gin-gonic/gin"

	"rentme/internal/app/dto"
	meapp "rentme/internal/app/handlers/me"
	"rentme/internal/app/queries"
)

type MeHTTP interface {
	ListBookings(c *gin.Context)
}

type MeHandler struct {
	Queries queries.Bus
	Logger  *slog.Logger
}

func (h MeHandler) ListBookings(c *gin.Context) {
	user, ok := requireRole(c, "")
	if !ok {
		return
	}
	if h.Queries == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "queries unavailable"})
		return
	}
	query := meapp.ListGuestBookingsQuery{GuestID: user.ID}
	result, err := queries.Ask[meapp.ListGuestBookingsQuery, dto.GuestBookingCollection](c.Request.Context(), h.Queries, query)
	if err != nil {
		if h.Logger != nil {
			h.Logger.Error("me bookings query failed", "error", err, "user_id", user.ID)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load bookings"})
		return
	}
	c.JSON(http.StatusOK, result)
}

var _ MeHTTP = (*MeHandler)(nil)
