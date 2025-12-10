package ginserver

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"time"

	gin "github.com/gin-gonic/gin"

	"rentme/internal/app/commands"
	"rentme/internal/app/dto"
	listingapp "rentme/internal/app/handlers/listings"
	"rentme/internal/app/queries"
	domainlistings "rentme/internal/domain/listings"
)

type HostListingHandler struct {
	Commands commands.Bus
	Queries  queries.Bus
	Logger   *slog.Logger
}

func (h HostListingHandler) List(c *gin.Context) {
	principal, ok := requireRole(c, "host")
	if !ok {
		return
	}
	hostID := principal.ID
	if h.Queries == nil {
		h.respondWithError(c, http.StatusServiceUnavailable, errors.New("queries bus unavailable"))
		return
	}

	limit := parseIntWithDefault(c.Query("limit"), 20)
	page := parseIntWithDefault(c.Query("page"), 1)
	offset := parseInt(c.Query("offset"))
	if offset == 0 && page > 1 {
		offset = (page - 1) * limit
	}

	query := listingapp.ListHostListingsQuery{
		HostID: hostID,
		Status: strings.TrimSpace(c.Query("status")),
		Limit:  limit,
		Offset: offset,
	}
	result, err := queries.Ask[listingapp.ListHostListingsQuery, dto.HostListingCatalog](c.Request.Context(), h.Queries, query)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h HostListingHandler) Create(c *gin.Context) {
	principal, ok := requireRole(c, "host")
	if !ok {
		return
	}
	hostID := principal.ID
	if h.Commands == nil {
		h.respondWithError(c, http.StatusServiceUnavailable, errors.New("commands bus unavailable"))
		return
	}

	var req hostListingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondWithError(c, http.StatusBadRequest, err)
		return
	}

	payload, err := buildHostListingPayload(req)
	if err != nil {
		h.respondWithError(c, http.StatusBadRequest, err)
		return
	}

	cmd := listingapp.CreateHostListingCommand{HostID: hostID, Payload: payload}
	result, err := commands.Dispatch[listingapp.CreateHostListingCommand, *dto.HostListingDetail](c.Request.Context(), h.Commands, cmd)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.Header("Location", fmt.Sprintf("/api/v1/host/listings/%s", result.ID))
	c.JSON(http.StatusCreated, result)
}

func (h HostListingHandler) Get(c *gin.Context) {
	principal, ok := requireRole(c, "host")
	if !ok {
		return
	}
	hostID := principal.ID
	if h.Queries == nil {
		h.respondWithError(c, http.StatusServiceUnavailable, errors.New("queries bus unavailable"))
		return
	}

	query := listingapp.GetHostListingQuery{
		HostID:    hostID,
		ListingID: c.Param("id"),
	}
	result, err := queries.Ask[listingapp.GetHostListingQuery, dto.HostListingDetail](c.Request.Context(), h.Queries, query)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h HostListingHandler) Update(c *gin.Context) {
	principal, ok := requireRole(c, "host")
	if !ok {
		return
	}
	hostID := principal.ID
	if h.Commands == nil {
		h.respondWithError(c, http.StatusServiceUnavailable, errors.New("commands bus unavailable"))
		return
	}

	var req hostListingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondWithError(c, http.StatusBadRequest, err)
		return
	}

	payload, err := buildHostListingPayload(req)
	if err != nil {
		h.respondWithError(c, http.StatusBadRequest, err)
		return
	}

	cmd := listingapp.UpdateHostListingCommand{
		HostID:    hostID,
		ListingID: c.Param("id"),
		Payload:   payload,
	}
	result, err := commands.Dispatch[listingapp.UpdateHostListingCommand, *dto.HostListingDetail](c.Request.Context(), h.Commands, cmd)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h HostListingHandler) Publish(c *gin.Context) {
	principal, ok := requireRole(c, "host")
	if !ok {
		return
	}
	hostID := principal.ID
	if h.Commands == nil {
		h.respondWithError(c, http.StatusServiceUnavailable, errors.New("commands bus unavailable"))
		return
	}

	cmd := listingapp.PublishHostListingCommand{
		HostID:    hostID,
		ListingID: c.Param("id"),
	}
	result, err := commands.Dispatch[listingapp.PublishHostListingCommand, *dto.HostListingDetail](c.Request.Context(), h.Commands, cmd)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h HostListingHandler) Unpublish(c *gin.Context) {
	principal, ok := requireRole(c, "host")
	if !ok {
		return
	}
	hostID := principal.ID
	if h.Commands == nil {
		h.respondWithError(c, http.StatusServiceUnavailable, errors.New("commands bus unavailable"))
		return
	}

	cmd := listingapp.UnpublishHostListingCommand{
		HostID:    hostID,
		ListingID: c.Param("id"),
	}
	result, err := commands.Dispatch[listingapp.UnpublishHostListingCommand, *dto.HostListingDetail](c.Request.Context(), h.Commands, cmd)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h HostListingHandler) PriceSuggestion(c *gin.Context) {
	principal, ok := requireRole(c, "host")
	if !ok {
		return
	}
	hostID := principal.ID
	if h.Queries == nil {
		h.respondWithError(c, http.StatusServiceUnavailable, errors.New("queries bus unavailable"))
		return
	}

	var payload priceSuggestionRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&payload); err != nil {
			h.respondWithError(c, http.StatusBadRequest, err)
			return
		}
	}

	checkIn, checkOut, err := parseRange(payload.CheckIn, payload.CheckOut)
	if err != nil {
		h.respondWithError(c, http.StatusBadRequest, err)
		return
	}

	query := listingapp.HostListingPriceSuggestionQuery{
		HostID:    hostID,
		ListingID: c.Param("id"),
		CheckIn:   checkIn,
		CheckOut:  checkOut,
		Guests:    payload.Guests,
	}
	result, err := queries.Ask[listingapp.HostListingPriceSuggestionQuery, dto.HostListingPriceSuggestion](c.Request.Context(), h.Queries, query)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h HostListingHandler) handleError(c *gin.Context, err error) {
	if errors.Is(err, listingapp.ErrListingNotOwned) {
		h.respondWithError(c, http.StatusNotFound, err)
		return
	}
	if isValidationError(err) {
		h.respondWithError(c, http.StatusBadRequest, err)
		return
	}
	h.respondWithError(c, http.StatusInternalServerError, err)
}

func (h HostListingHandler) respondWithError(c *gin.Context, status int, err error) {
	if h.Logger != nil {
		fields := []any{"status", status, "error", err, "path", c.FullPath()}
		if host, ok := currentPrincipal(c); ok {
			fields = append(fields, "host_id", host.ID)
		}
		h.Logger.Error("host listing request failed", fields...)
	}
	c.JSON(status, gin.H{"error": err.Error()})
}

func parseRange(checkInRaw, checkOutRaw string) (time.Time, time.Time, error) {
	if checkInRaw == "" && checkOutRaw == "" {
		return time.Time{}, time.Time{}, nil
	}
	checkIn, ok := parseFlexibleTime(checkInRaw)
	if checkInRaw != "" && !ok {
		return time.Time{}, time.Time{}, errors.New("check_in must be a valid date")
	}
	checkOut, ok := parseFlexibleTime(checkOutRaw)
	if checkOutRaw != "" && !ok {
		return time.Time{}, time.Time{}, errors.New("check_out must be a valid date")
	}
	return checkIn, checkOut, nil
}

func buildHostListingPayload(req hostListingRequest) (listingapp.HostListingPayload, error) {
	availableFrom := time.Time{}
	if req.AvailableFrom != "" {
		if parsed, ok := parseFlexibleTime(req.AvailableFrom); ok {
			availableFrom = parsed
		} else {
			return listingapp.HostListingPayload{}, errors.New("available_from must be a valid date")
		}
	}

	rate := req.NightlyRateCents
	if rate == 0 && req.NightlyRate > 0 {
		rate = int64(math.Round(req.NightlyRate * 100))
	}

	address := domainlistings.Address{
		Line1:   strings.TrimSpace(req.Address.Line1),
		Line2:   strings.TrimSpace(req.Address.Line2),
		City:    strings.TrimSpace(req.Address.City),
		Region:  strings.TrimSpace(req.Address.Region),
		Country: strings.TrimSpace(req.Address.Country),
		Lat:     req.Address.Lat,
		Lon:     req.Address.Lon,
	}
	if address.Region == "" {
		address.Region = address.Country
	}

	payload := listingapp.HostListingPayload{
		Title:                req.Title,
		Description:          req.Description,
		PropertyType:         strings.TrimSpace(req.PropertyType),
		Address:              address,
		Amenities:            cleanStrings(req.Amenities),
		HouseRules:           cleanStrings(req.HouseRules),
		Tags:                 cleanStrings(req.Tags),
		Highlights:           cleanStrings(req.Highlights),
		ThumbnailURL:         strings.TrimSpace(req.ThumbnailURL),
		CancellationPolicyID: strings.TrimSpace(req.CancellationPolicyID),
		GuestsLimit:          req.GuestsLimit,
		MinNights:            req.MinNights,
		MaxNights:            req.MaxNights,
		NightlyRateCents:     rate,
		Bedrooms:             req.Bedrooms,
		Bathrooms:            req.Bathrooms,
		Floor:                req.Floor,
		FloorsTotal:          req.FloorsTotal,
		RenovationScore:      req.RenovationScore,
		BuildingAgeYears:     req.BuildingAgeYears,
		AreaSquareMeters:     req.AreaSquareMeters,
		AvailableFrom:        availableFrom,
		Photos:               cleanStrings(req.Photos),
	}
	return payload, nil
}

func cleanStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func isValidationError(err error) bool {
	switch {
	case errors.Is(err, domainlistings.ErrTitleRequired),
		errors.Is(err, domainlistings.ErrGuestsLimit),
		errors.Is(err, domainlistings.ErrNightsRange),
		errors.Is(err, domainlistings.ErrNightlyRate),
		errors.Is(err, domainlistings.ErrInvalidFloor),
		errors.Is(err, domainlistings.ErrFloorsTotal),
		errors.Is(err, domainlistings.ErrRenovationScore),
		errors.Is(err, domainlistings.ErrBuildingAge),
		errors.Is(err, domainlistings.ErrAddressRequired),
		errors.Is(err, domainlistings.ErrInvalidState):
		return true
	}
	return false
}

type hostListingRequest struct {
	Title                string             `json:"title"`
	Description          string             `json:"description"`
	PropertyType         string             `json:"property_type"`
	Address              hostListingAddress `json:"address"`
	Amenities            []string           `json:"amenities"`
	HouseRules           []string           `json:"house_rules"`
	Tags                 []string           `json:"tags"`
	Highlights           []string           `json:"highlights"`
	ThumbnailURL         string             `json:"thumbnail_url"`
	CancellationPolicyID string             `json:"cancellation_policy_id"`
	GuestsLimit          int                `json:"guests_limit"`
	MinNights            int                `json:"min_nights"`
	MaxNights            int                `json:"max_nights"`
	NightlyRateCents     int64              `json:"nightly_rate_cents"`
	NightlyRate          float64            `json:"nightly_rate"`
	Bedrooms             int                `json:"bedrooms"`
	Bathrooms            int                `json:"bathrooms"`
	Floor                int                `json:"floor"`
	FloorsTotal          int                `json:"floors_total"`
	RenovationScore      int                `json:"renovation_score"`
	BuildingAgeYears     int                `json:"building_age_years"`
	AreaSquareMeters     float64            `json:"area_sq_m"`
	AvailableFrom        string             `json:"available_from"`
	Photos               []string           `json:"photos"`
}

type hostListingAddress struct {
	Line1   string  `json:"line1"`
	Line2   string  `json:"line2"`
	City    string  `json:"city"`
	Region  string  `json:"region"`
	Country string  `json:"country"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
}

type priceSuggestionRequest struct {
	CheckIn  string `json:"check_in"`
	CheckOut string `json:"check_out"`
	Guests   int    `json:"guests"`
}
