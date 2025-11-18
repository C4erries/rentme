package ginserver

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	gin "github.com/gin-gonic/gin"

	"rentme/internal/app/dto"
	listingapp "rentme/internal/app/handlers/listings"
	"rentme/internal/app/queries"
)

// ListingHandler wires listing queries to HTTP.
type ListingHandler struct {
	Queries queries.Bus
}

// Catalog responds with a filtered collection of listings.
func (h ListingHandler) Catalog(c *gin.Context) {
	if h.Queries == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "listing handler unavailable"})
		return
	}
	query := listingapp.SearchCatalogQuery{
		City:          c.Query("city"),
		Country:       c.Query("country"),
		Tags:          splitCSV(c.Query("tags")),
		Amenities:     splitCSV(c.Query("amenities")),
		MinGuests:     parseInt(c.Query("min_guests")),
		PriceMinCents: parseInt64(c.Query("price_min_cents")),
		PriceMaxCents: parseInt64(c.Query("price_max_cents")),
		Limit:         parseIntWithDefault(c.Query("limit"), 24),
		Offset:        parseInt(c.Query("offset")),
		Sort:          c.Query("sort"),
	}
	result, err := queries.Ask[listingapp.SearchCatalogQuery, dto.ListingCatalog](c.Request.Context(), h.Queries, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h ListingHandler) Overview(c *gin.Context) {
	if h.Queries == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "listing handler unavailable"})
		return
	}
	listingID := c.Param("id")
	if listingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "listing id is required"})
		return
	}
	windowFrom, windowTo := resolveWindow(c.Query("from"), c.Query("to"))
	query := listingapp.GetOverviewQuery{
		ListingID: listingID,
		From:      windowFrom,
		To:        windowTo,
	}
	result, err := queries.Ask[listingapp.GetOverviewQuery, dto.ListingOverview](c.Request.Context(), h.Queries, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

var _ ListingHTTP = ListingHandler{}

func resolveWindow(fromRaw, toRaw string) (time.Time, time.Time) {
	now := time.Now().UTC()
	from, ok := parseFlexibleTime(fromRaw)
	if !ok {
		from = now
	}
	from = truncateToDay(from)
	to, ok := parseFlexibleTime(toRaw)
	if !ok {
		to = from.AddDate(0, 0, 45)
	}
	to = truncateToDay(to)
	if !to.After(from) {
		to = from.AddDate(0, 0, 45)
	}
	return from, to
}

func parseFlexibleTime(raw string) (time.Time, bool) {
	if raw == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC(), true
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), true
	}
	return time.Time{}, false
}

func truncateToDay(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseInt(raw string) int {
	value, _ := strconv.Atoi(strings.TrimSpace(raw))
	if value < 0 {
		return 0
	}
	return value
}

func parseIntWithDefault(raw string, fallback int) int {
	value := parseInt(raw)
	if value == 0 {
		return fallback
	}
	return value
}

func parseInt64(raw string) int64 {
	value, _ := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if value < 0 {
		return 0
	}
	return value
}
