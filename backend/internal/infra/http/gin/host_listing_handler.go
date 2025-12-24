package ginserver

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	gin "github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"rentme/internal/app/commands"
	"rentme/internal/app/dto"
	listingapp "rentme/internal/app/handlers/listings"
	"rentme/internal/app/queries"
	domainlistings "rentme/internal/domain/listings"
)

const maxListingPhotoSizeBytes int64 = 10 * 1024 * 1024

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

func (h HostListingHandler) UploadPhoto(c *gin.Context) {
	principal, ok := requireRole(c, "host")
	if !ok {
		return
	}
	if h.Commands == nil {
		h.respondWithError(c, http.StatusServiceUnavailable, errors.New("commands bus unavailable"))
		return
	}

	listingID := strings.TrimSpace(c.Param("id"))
	if listingID == "" {
		h.respondWithError(c, http.StatusBadRequest, errors.New("listing id is required"))
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		h.respondWithError(c, http.StatusBadRequest, fmt.Errorf("file is required: %w", err))
		return
	}
	if fileHeader.Size <= 0 {
		h.respondWithError(c, http.StatusBadRequest, errors.New("file is empty"))
		return
	}
	if fileHeader.Size > maxListingPhotoSizeBytes {
		h.respondWithError(c, http.StatusBadRequest, fmt.Errorf("file too large (max %d MB)", maxListingPhotoSizeBytes/1024/1024))
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		h.respondWithError(c, http.StatusBadRequest, err)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxListingPhotoSizeBytes+1024))
	if err != nil {
		h.respondWithError(c, http.StatusInternalServerError, fmt.Errorf("cannot read file: %w", err))
		return
	}
	if len(data) == 0 {
		h.respondWithError(c, http.StatusBadRequest, errors.New("file is empty"))
		return
	}
	if int64(len(data)) > maxListingPhotoSizeBytes {
		h.respondWithError(c, http.StatusBadRequest, fmt.Errorf("file too large (max %d MB)", maxListingPhotoSizeBytes/1024/1024))
		return
	}

	contentType := http.DetectContentType(data)
	if !isAllowedImageType(contentType) {
		h.respondWithError(c, http.StatusBadRequest, fmt.Errorf("unsupported content type: %s", contentType))
		return
	}

	objectKey := buildPhotoObjectKey(listingID, fileHeader.Filename, contentType)
	cmd := listingapp.UploadHostListingPhotoCommand{
		HostID:      principal.ID,
		ListingID:   listingID,
		ObjectKey:   objectKey,
		ContentType: contentType,
		Reader:      bytes.NewReader(data),
	}
	result, err := commands.Dispatch[listingapp.UploadHostListingPhotoCommand, *dto.HostListingPhotoUploadResult](c.Request.Context(), h.Commands, cmd)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, result)
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

func isAllowedImageType(contentType string) bool {
	switch strings.ToLower(contentType) {
	case "image/jpeg", "image/jpg", "image/png", "image/webp":
		return true
	default:
		return false
	}
}

func extensionForContentType(contentType string) string {
	switch strings.ToLower(contentType) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	default:
		return ""
	}
}

func buildPhotoObjectKey(listingID, filename, contentType string) string {
	ext := extensionForContentType(contentType)
	if ext == "" {
		ext = strings.ToLower(path.Ext(filename))
	}
	if ext == "" {
		ext = ".img"
	}
	safeListing := sanitizePathToken(listingID)
	return fmt.Sprintf("listings/%s/%s%s", safeListing, uuid.NewString(), ext)
}

func sanitizePathToken(value string) string {
	if strings.TrimSpace(value) == "" {
		return "listing"
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		return "listing"
	}
	return result
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

	rate := req.RateRub

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

	travelMinutes := req.TravelMinutes
	if travelMinutes < 0 {
		travelMinutes = 0
	}
	travelMode := strings.TrimSpace(strings.ToLower(req.TravelMode))
	if travelMode == "" {
		travelMode = "car"
	}
	if travelMode == "public" {
		travelMode = "transit"
	}

	var rentalTerm domainlistings.RentalTermType
	if strings.TrimSpace(req.RentalTerm) != "" {
		value := strings.ToLower(strings.TrimSpace(req.RentalTerm))
		switch value {
		case string(domainlistings.RentalTermShort):
			rentalTerm = domainlistings.RentalTermShort
		case string(domainlistings.RentalTermLong):
			rentalTerm = domainlistings.RentalTermLong
		default:
			return listingapp.HostListingPayload{}, fmt.Errorf("rental_term must be %q or %q", domainlistings.RentalTermShort, domainlistings.RentalTermLong)
		}
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
		RateRub:              rate,
		Bedrooms:             req.Bedrooms,
		Bathrooms:            req.Bathrooms,
		Floor:                req.Floor,
		FloorsTotal:          req.FloorsTotal,
		RenovationScore:      req.RenovationScore,
		BuildingAgeYears:     req.BuildingAgeYears,
		AreaSquareMeters:     req.AreaSquareMeters,
		TravelMinutes:        travelMinutes,
		TravelMode:           travelMode,
		RentalTermType:       rentalTerm,
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
		errors.Is(err, domainlistings.ErrRate),
		errors.Is(err, domainlistings.ErrInvalidFloor),
		errors.Is(err, domainlistings.ErrFloorsTotal),
		errors.Is(err, domainlistings.ErrRenovationScore),
		errors.Is(err, domainlistings.ErrBuildingAge),
		errors.Is(err, domainlistings.ErrRentalTerm),
		errors.Is(err, domainlistings.ErrAddressRequired),
		errors.Is(err, domainlistings.ErrInvalidState),
		errors.Is(err, domainlistings.ErrPhotoURL):
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
	RateRub              int64              `json:"rate_rub"`
	Bedrooms             int                `json:"bedrooms"`
	Bathrooms            int                `json:"bathrooms"`
	Floor                int                `json:"floor"`
	FloorsTotal          int                `json:"floors_total"`
	RenovationScore      int                `json:"renovation_score"`
	BuildingAgeYears     int                `json:"building_age_years"`
	AreaSquareMeters     float64            `json:"area_sq_m"`
	AvailableFrom        string             `json:"available_from"`
	Photos               []string           `json:"photos"`
	RentalTerm           string             `json:"rental_term"`
	TravelMinutes        float64            `json:"travel_minutes"`
	TravelMode           string             `json:"travel_mode"`
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
