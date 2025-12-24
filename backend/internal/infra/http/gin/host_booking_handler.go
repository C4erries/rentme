package ginserver

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	gin "github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"

	"rentme/internal/app/commands"
	"rentme/internal/app/dto"
	bookingapp "rentme/internal/app/handlers/booking"
	"rentme/internal/app/queries"
	domainbooking "rentme/internal/domain/booking"
)

type HostBookingHandler struct {
	Commands commands.Bus
	Queries  queries.Bus
	Logger   *slog.Logger
}

type declineBookingRequest struct {
	Reason string `json:"reason"`
}

func (h HostBookingHandler) List(c *gin.Context) {
	host, ok := requireRole(c, "host")
	if !ok {
		return
	}
	if h.Queries == nil {
		h.respondWithError(c, http.StatusServiceUnavailable, errors.New("queries bus unavailable"))
		return
	}

	query := bookingapp.ListHostBookingsQuery{
		HostID: host.ID,
		Status: c.Query("status"),
	}
	result, err := queries.Ask[bookingapp.ListHostBookingsQuery, dto.HostBookingCollection](c.Request.Context(), h.Queries, query)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h HostBookingHandler) Confirm(c *gin.Context) {
	host, ok := requireRole(c, "host")
	if !ok {
		return
	}
	if h.Commands == nil {
		h.respondWithError(c, http.StatusServiceUnavailable, errors.New("commands bus unavailable"))
		return
	}

	cmd := bookingapp.ConfirmHostBookingCommand{
		HostID:    host.ID,
		BookingID: strings.TrimSpace(c.Param("id")),
	}
	result, err := commands.Dispatch[bookingapp.ConfirmHostBookingCommand, *bookingapp.HostBookingActionResult](c.Request.Context(), h.Commands, cmd)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h HostBookingHandler) Decline(c *gin.Context) {
	host, ok := requireRole(c, "host")
	if !ok {
		return
	}
	if h.Commands == nil {
		h.respondWithError(c, http.StatusServiceUnavailable, errors.New("commands bus unavailable"))
		return
	}

	var req declineBookingRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			h.respondWithError(c, http.StatusBadRequest, err)
			return
		}
	}

	cmd := bookingapp.DeclineHostBookingCommand{
		HostID:    host.ID,
		BookingID: strings.TrimSpace(c.Param("id")),
		Reason:    strings.TrimSpace(req.Reason),
	}
	result, err := commands.Dispatch[bookingapp.DeclineHostBookingCommand, *bookingapp.HostBookingActionResult](c.Request.Context(), h.Commands, cmd)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h HostBookingHandler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, bookingapp.ErrBookingNotOwned),
		errors.Is(err, domainbooking.ErrBookingNotFound),
		errors.Is(err, mongo.ErrNoDocuments):
		h.respondWithError(c, http.StatusNotFound, err)
	case isHostBookingValidationError(err):
		h.respondWithError(c, http.StatusBadRequest, err)
	default:
		h.respondWithError(c, http.StatusInternalServerError, err)
	}
}

func (h HostBookingHandler) respondWithError(c *gin.Context, status int, err error) {
	if h.Logger != nil {
		fields := []any{"status", status, "error", err, "path", c.FullPath()}
		if host, ok := currentPrincipal(c); ok {
			fields = append(fields, "host_id", host.ID)
		}
		h.Logger.Error("host booking request failed", fields...)
	}
	c.JSON(status, gin.H{"error": err.Error()})
}

func isHostBookingValidationError(err error) bool {
	switch {
	case errors.Is(err, domainbooking.ErrInvalidState),
		errors.Is(err, domainbooking.ErrPaymentHoldRequired),
		errors.Is(err, domainbooking.ErrInvalidGuests),
		errors.Is(err, domainbooking.ErrCheckInInPast):
		return true
	}
	return false
}

var _ HostBookingHTTP = HostBookingHandler{}
