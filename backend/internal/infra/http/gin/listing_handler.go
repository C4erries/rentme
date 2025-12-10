package ginserver

import (
	"math"
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
	location := c.Query("location")
	checkInRaw := c.Query("check_in")
	checkOutRaw := c.Query("check_out")
	checkIn, _ := parseFlexibleTime(checkInRaw)
	checkOut, _ := parseFlexibleTime(checkOutRaw)
	if (checkInRaw != "" || checkOutRaw != "") && (checkIn.IsZero() || checkOut.IsZero()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "both check_in and check_out must be valid dates"})
		return
	}
	if !checkIn.IsZero() && !checkOut.IsZero() && !checkOut.After(checkIn) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "check_out must be after check_in"})
		return
	}
	guests := parseInt(c.Query("guests"))
	if guests == 0 {
		guests = parseInt(c.Query("min_guests"))
	}
	limit := parseIntWithDefault(c.Query("limit"), 24)
	page := parseIntWithDefault(c.Query("page"), 1)
	if page < 1 {
		page = 1
	}
	offset := parseInt(c.Query("offset"))
	if offset == 0 && page > 1 {
		offset = (page - 1) * limit
	}
	priceMin := parseInt64(c.Query("price_min_cents"))
	priceMax := parseInt64(c.Query("price_max_cents"))
	if priceMin == 0 {
		priceMin = parseMajorCurrencyToCents(c.Query("price_min"))
	}
	if priceMax == 0 {
		priceMax = parseMajorCurrencyToCents(c.Query("price_max"))
	}
	propertyTypes := mergeSlices(splitCSV(c.Query("type")), splitCSV(c.Query("types")))
	rentalTerms := mergeSlices(splitCSV(c.Query("rental_term")), splitCSV(c.Query("rental_terms")))

	query := listingapp.SearchCatalogQuery{
		City:          c.Query("city"),
		Region:        c.Query("region"),
		Country:       c.Query("country"),
		Location:      location,
		Tags:          splitCSV(c.Query("tags")),
		Amenities:     splitCSV(c.Query("amenities")),
		MinGuests:     guests,
		PriceMinCents: priceMin,
		PriceMaxCents: priceMax,
		PropertyTypes: propertyTypes,
		RentalTerms:   rentalTerms,
		Limit:         limit,
		Offset:        offset,
		Sort:          c.Query("sort"),
		CheckIn:       checkIn,
		CheckOut:      checkOut,
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

func parseMajorCurrencyToCents(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}
	return int64(math.Round(value * 100))
}

func mergeSlices(parts ...[]string) []string {
	var merged []string
	for _, slice := range parts {
		if len(slice) == 0 {
			continue
		}
		merged = append(merged, slice...)
	}
	if len(merged) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(merged))
	result := make([]string, 0, len(merged))
	for _, item := range merged {
		lower := strings.ToLower(strings.TrimSpace(item))
		if lower == "" {
			continue
		}
		if _, ok := seen[lower]; ok {
			continue
		}
		seen[lower] = struct{}{}
		result = append(result, lower)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
