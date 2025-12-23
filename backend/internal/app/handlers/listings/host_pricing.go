package listings

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"strings"
	"time"

	"rentme/internal/app/dto"
	handlersupport "rentme/internal/app/handlers/support"
	"rentme/internal/app/policies"
	"rentme/internal/app/queries"
	"rentme/internal/app/uow"
	domainlistings "rentme/internal/domain/listings"
	domainrange "rentme/internal/domain/shared/daterange"
)

const priceSuggestionKey = "host.listings.price_suggestion"

type HostListingPriceSuggestionQuery struct {
	HostID    string
	ListingID string
	CheckIn   time.Time
	CheckOut  time.Time
	Guests    int
}

func (q HostListingPriceSuggestionQuery) Key() string { return priceSuggestionKey }

type HostListingPriceSuggestionHandler struct {
	Logger     *slog.Logger
	Pricing    policies.PricingPort
	UoWFactory uow.UoWFactory
}

func (h *HostListingPriceSuggestionHandler) Handle(ctx context.Context, q HostListingPriceSuggestionQuery) (dto.HostListingPriceSuggestion, error) {
	var zero dto.HostListingPriceSuggestion
	if strings.TrimSpace(q.HostID) == "" {
		return zero, errors.New("host id is required")
	}
	if strings.TrimSpace(q.ListingID) == "" {
		return zero, errors.New("listing id is required")
	}
	unit, execCtx, cleanup, err := handlersupport.BeginReadOnlyUnit(ctx, h.UoWFactory)
	if err != nil {
		return zero, err
	}
	if cleanup != nil {
		defer cleanup()
	}

	listing, err := unit.Listings().ByID(execCtx, domainlistings.ListingID(q.ListingID))
	if err != nil {
		return zero, err
	}
	if listing.Host != domainlistings.HostID(q.HostID) {
		return zero, ErrListingNotOwned
	}

	if h.Pricing == nil {
		return zero, errors.New("pricing service unavailable")
	}

	checkIn := q.CheckIn
	checkOut := q.CheckOut
	if checkIn.IsZero() || checkOut.IsZero() || !checkOut.After(checkIn) {
		checkIn = time.Now().UTC()
		checkOut = checkIn.AddDate(0, 0, 7)
	}

	dr, err := domainrange.New(checkIn, checkOut)
	if err != nil {
		return zero, err
	}

	guests := q.Guests
	if guests <= 0 {
		guests = listing.GuestsLimit
	}

	breakdown, err := h.Pricing.Quote(execCtx, listing, dr, guests)
	if err != nil {
		return zero, err
	}

	recommended := breakdown.Nightly.Amount
	current := listing.RateRub
	level := priceLevelFor(current, recommended)
	gapPercent := priceGapPercent(current, recommended)

	message := priceMessage(level)

	result := dto.HostListingPriceSuggestion{
		ListingID:             string(listing.ID),
		RecommendedPriceRub:   recommended,
		CurrentPriceRub:       current,
		PriceLevel:            level,
		PriceGapPercent:       gapPercent,
		Message:               message,
		Range: dto.ListingDateRange{
			CheckIn:  dr.CheckIn,
			CheckOut: dr.CheckOut,
		},
	}

	if h.Logger != nil {
		h.Logger.Info("price suggestion generated", "listing_id", listing.ID, "host_id", q.HostID, "level", level)
	}

	return result, nil
}

func priceLevelFor(current, recommended int64) string {
	if recommended == 0 {
		return dto.PriceLevelFair
	}
	diff := float64(current-recommended) / float64(recommended)
	if diff <= -0.1 {
		return dto.PriceLevelBelowMarket
	}
	if diff >= 0.1 {
		return dto.PriceLevelAboveMarket
	}
	return dto.PriceLevelFair
}

func priceGapPercent(current, recommended int64) float64 {
	if recommended == 0 {
		return 0
	}
	const percentBase = 100.0
	const precisionBase = 100.0
	percent := (float64(current-recommended) / float64(recommended)) * percentBase
	return math.Round(percent*precisionBase) / precisionBase
}

func priceMessage(level string) string {
	switch level {
	case dto.PriceLevelBelowMarket:
		return "Текущая цена ниже средней по району — Rentme рекомендует немного поднять её, чтобы заработать больше без потери спроса."
	case dto.PriceLevelAboveMarket:
		return "Цена выше средней — вы можете получать больше за бронь, но спрос может быть ниже. Вы всегда можете оставить свой вариант."
	default:
		return "Цена выглядит справедливой для этого района и типа жилья, но вы всегда можете скорректировать её вручную."
	}
}

var _ queries.Handler[HostListingPriceSuggestionQuery, dto.HostListingPriceSuggestion] = (*HostListingPriceSuggestionHandler)(nil)
