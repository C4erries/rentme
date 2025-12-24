package pricing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"time"

	domainlistings "rentme/internal/domain/listings"
	domainpricing "rentme/internal/domain/pricing"
	domainrange "rentme/internal/domain/shared/daterange"
	"rentme/internal/domain/shared/money"
)

// MLPricingEngine delegates price suggestions to an external ML service.
type MLPricingEngine struct {
	Client   *http.Client
	Endpoint string
	Listings domainlistings.ListingRepository
	Logger   *slog.Logger
	Clamps   ClampConfig
}

type mlPredictRequest struct {
	ListingID        string  `json:"listing_id,omitempty"`
	City             string  `json:"city"`
	Minutes          float64 `json:"minutes"`
	Way              string  `json:"way"`
	Rooms            int     `json:"rooms"`
	TotalArea        float64 `json:"total_area"`
	Storey           int     `json:"storey"`
	Storeys          int     `json:"storeys"`
	Renovation       int     `json:"renovation"`
	BuildingAgeYears int     `json:"building_age_years"`
	CurrentPrice     float64 `json:"current_price,omitempty"`
	RentalTerm       string  `json:"rental_term,omitempty"`
}

type mlPredictResponse struct {
	ListingID        string   `json:"listing_id"`
	RecommendedPrice float64  `json:"recommended_price"`
	CurrentPrice     *float64 `json:"current_price"`
	Diff             *float64 `json:"diff"`
}

// Quote calls the ML pricing service and maps its response to domain pricing.
func (e *MLPricingEngine) Quote(ctx context.Context, input domainpricing.QuoteInput) (domainpricing.PriceBreakdown, error) {
	var zero domainpricing.PriceBreakdown

	if e == nil || e.Client == nil {
		return zero, errors.New("pricing: http client not configured")
	}
	if e.Endpoint == "" {
		return zero, errors.New("pricing: ml endpoint not configured")
	}
	if e.Listings == nil {
		return zero, errors.New("pricing: listings repository missing")
	}

	listing := input.Listing
	if listing == nil {
		var err error
		listing, err = e.Listings.ByID(ctx, input.ListingID)
		if err != nil {
			return zero, err
		}
	}
	if listing == nil {
		return zero, errors.New("pricing: listing missing")
	}

	rentalTerm := input.RentalTerm
	if rentalTerm == "" && listing != nil {
		rentalTerm = listing.RentalTermType
	}
	if rentalTerm == "" {
		rentalTerm = domainlistings.RentalTermLong
	}
	travelMinutes := listing.TravelMinutes
	if travelMinutes <= 0 {
		travelMinutes = 20
	}
	travelMode := strings.TrimSpace(strings.ToLower(listing.TravelMode))
	if travelMode == "" {
		travelMode = "car"
	}
	if travelMode == "public" {
		travelMode = "transit"
	}
	reqPayload := mlPredictRequest{
		ListingID:        string(listing.ID),
		City:             NormalizeCity(listing.Address.City),
		Minutes:          travelMinutes,
		Way:              travelMode,
		Rooms:            listing.Bedrooms,
		TotalArea:        listing.AreaSquareMeters,
		Storey:           listing.Floor,
		Storeys:          listing.FloorsTotal,
		Renovation:       listing.RenovationScore,
		BuildingAgeYears: listing.BuildingAgeYears,
		CurrentPrice:     float64(listing.RateRub),
		RentalTerm:       string(rentalTerm),
	}

	body, err := json.Marshal(reqPayload)
	if err != nil {
		return zero, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, e.Endpoint, bytes.NewReader(body))
	if err != nil {
		return zero, err
	}
	request.Header.Set("Content-Type", "application/json")

	resp, err := e.Client.Do(request)
	if err != nil {
		e.logError("ml pricing request failed", listing.ID, err)
		return zero, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		err := fmt.Errorf("ml pricing returned status %d: %s", resp.StatusCode, string(snippet))
		e.logError("ml pricing returned error", listing.ID, err)
		return zero, err
	}

	var mlResp mlPredictResponse
	if err := json.NewDecoder(resp.Body).Decode(&mlResp); err != nil {
		e.logError("ml pricing decode failed", listing.ID, err)
		return zero, err
	}

	cityRaw := listing.Address.City
	cityNormalized := NormalizeCity(cityRaw)
	recommendedRaw := int64(math.Round(mlResp.RecommendedPrice))
	recommendedFinal, clampMin, clampMax, clamped := applyClamps(recommendedRaw, e.clamps(), cityNormalized, rentalTerm)
	nights := nightsBetween(input.Range)
	if nights < 1 {
		nights = 1
	}

	breakdown := domainpricing.PriceBreakdown{
		Nights:  nights,
		Nightly: money.Must(recommendedFinal, "RUB"),
	}
	if err := breakdown.RecalculateTotal(); err != nil {
		return zero, err
	}

	if e.Logger != nil {
		e.Logger.Info(
			"ml price post-processed",
			"listing_id", listing.ID,
			"city_raw", cityRaw,
			"city_normalized", cityNormalized,
			"rental_term", rentalTerm,
			"ml_price_raw", recommendedRaw,
			"ml_price_final", recommendedFinal,
			"clamped", clamped,
			"clamp_min", clampMin,
			"clamp_max", clampMax,
		)
	}
	return breakdown, nil
}

func (e *MLPricingEngine) logError(msg string, listingID domainlistings.ListingID, err error) {
	if e.Logger == nil {
		return
	}
	e.Logger.Error(msg, "listing_id", listingID, "error", err)
}

func nightsBetween(dr domainrange.DateRange) int {
	if dr.CheckIn.IsZero() || dr.CheckOut.IsZero() {
		return 0
	}
	return int(dr.CheckOut.Sub(dr.CheckIn) / (24 * time.Hour))
}

var _ domainpricing.Calculator = (*MLPricingEngine)(nil)

func (e *MLPricingEngine) clamps() ClampConfig {
	if e == nil {
		return DefaultClampConfig()
	}
	if e.Clamps.Defaults == nil && e.Clamps.Cities == nil {
		return DefaultClampConfig()
	}
	return e.Clamps
}
