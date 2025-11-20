package dto

import "time"

const (
	PriceLevelBelowMarket = "below_market"
	PriceLevelFair        = "fair"
	PriceLevelAboveMarket = "above_market"
)

type ListingDateRange struct {
	CheckIn  time.Time `json:"check_in"`
	CheckOut time.Time `json:"check_out"`
}

type HostListingPriceSuggestion struct {
	ListingID             string           `json:"listing_id"`
	RecommendedPriceCents int64            `json:"recommended_price_cents"`
	CurrentPriceCents     int64            `json:"current_price_cents"`
	PriceLevel            string           `json:"price_level"`
	PriceGapPercent       float64          `json:"price_gap_percent"`
	Message               string           `json:"message"`
	Range                 ListingDateRange `json:"range"`
}
