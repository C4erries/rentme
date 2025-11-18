package memory

import (
	"context"
	"errors"
	"time"

	"rentme/internal/app/policies"
	domainlistings "rentme/internal/domain/listings"
	domainpricing "rentme/internal/domain/pricing"
	domainrange "rentme/internal/domain/shared/daterange"
	"rentme/internal/domain/shared/money"
)

// PricingEngine is a deterministic calculator used for local demos.
type PricingEngine struct {
	BaseNightly money.Money
	CleaningFee money.Money
}

// NewPricingEngine returns an engine with sane defaults when zero values are provided.
func NewPricingEngine() *PricingEngine {
	return &PricingEngine{
		BaseNightly: money.Must(15000, "USD"), // $150.00
		CleaningFee: money.Must(5000, "USD"),  // $50.00
	}
}

func (p *PricingEngine) Quote(ctx context.Context, input domainpricing.QuoteInput) (domainpricing.PriceBreakdown, error) {
	nights := nightsBetween(input.Range)
	if nights < 1 {
		nights = 1
	}
	breakdown := domainpricing.PriceBreakdown{
		Nights:  nights,
		Nightly: p.BaseNightly,
		Fees: []domainpricing.Fee{{
			Name:   "cleaning_fee",
			Amount: p.CleaningFee,
		}},
	}
	if err := breakdown.RecalculateTotal(); err != nil {
		return domainpricing.PriceBreakdown{}, err
	}
	return breakdown, nil
}

func nightsBetween(dr domainrange.DateRange) int {
	if dr.CheckIn.IsZero() || dr.CheckOut.IsZero() {
		return 0
	}
	return int(dr.CheckOut.Sub(dr.CheckIn) / (24 * time.Hour))
}

// PricingPortAdapter bridges domain calculator into the application policy port.
type PricingPortAdapter struct {
	Calculator domainpricing.Calculator
}

var ErrPricingCalculatorMissing = errors.New("pricing: calculator missing")

func (p PricingPortAdapter) Quote(ctx context.Context, listingID domainlistings.ListingID, dr domainrange.DateRange, guests int) (domainpricing.PriceBreakdown, error) {
	if p.Calculator == nil {
		return domainpricing.PriceBreakdown{}, ErrPricingCalculatorMissing
	}
	return p.Calculator.Quote(ctx, domainpricing.QuoteInput{
		ListingID: listingID,
		Range:     dr,
		Guests:    guests,
	})
}

var (
	_ domainpricing.Calculator = (*PricingEngine)(nil)
	_ policies.PricingPort     = PricingPortAdapter{}
)
