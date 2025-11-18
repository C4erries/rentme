package ginserver

import (
	"net/http"
	"time"

	gin "github.com/gin-gonic/gin"

	"rentme/internal/app/dto"
	availabilityapp "rentme/internal/app/handlers/availability"
	"rentme/internal/app/queries"
)

type AvailabilityHandler struct {
	Queries queries.Bus
}

func (h AvailabilityHandler) Calendar(c *gin.Context) {
	listingID := c.Param("id")
	from, _ := time.Parse(time.RFC3339, c.Query("from"))
	to, _ := time.Parse(time.RFC3339, c.Query("to"))
	query := availabilityapp.GetCalendarQuery{ListingID: listingID, From: from, To: to}
	result, err := queries.Ask[availabilityapp.GetCalendarQuery, dto.Calendar](c.Request.Context(), h.Queries, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

var _ AvailabilityHTTP = AvailabilityHandler{}
