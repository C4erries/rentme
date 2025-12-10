package pricing

import (
	"context"
	"errors"

	"rentme/internal/domain/listings"
	"rentme/internal/domain/shared/daterange"
	"rentme/internal/domain/shared/money"
)

var (
	ErrNegativeComponent = errors.New("pricing: components cannot be negative unless modeled as discount")
	ErrCurrencyUnset     = errors.New("pricing: currency must be defined")
)

type Fee struct {
	Name   string
	Amount money.Money
}

type Tax struct {
	Name   string
	Amount money.Money
}

type Discount struct {
	Name   string
	Amount money.Money
}

type PriceBreakdown struct {
	Nights    int
	Nightly   money.Money
	Fees      []Fee
	Taxes     []Tax
	Discounts []Discount
	Total     money.Money
}

func (p *PriceBreakdown) Validate() error {
	if p.Nightly.Currency == "" {
		return ErrCurrencyUnset
	}
	if p.Nights <= 0 {
		return errors.New("pricing: nights must be positive")
	}
	return nil
}

func (p *PriceBreakdown) RecalculateTotal() error {
	if err := p.Validate(); err != nil {
		return err
	}
	total := p.Nightly.Multiply(int64(p.Nights))
	addMoney := func(m money.Money) {
		res, _ := total.Add(m)
		total = res
	}
	for _, fee := range p.Fees {
		if fee.Amount.Amount < 0 {
			return ErrNegativeComponent
		}
		addMoney(fee.Amount)
	}
	for _, tax := range p.Taxes {
		if tax.Amount.Amount < 0 {
			return ErrNegativeComponent
		}
		addMoney(tax.Amount)
	}
	for _, discount := range p.Discounts {
		if discount.Amount.Amount > 0 {
			discount.Amount = discount.Amount.Neg()
		}
		addMoney(discount.Amount)
	}
	if total.Amount < 0 {
		total = money.Money{Amount: 0, Currency: total.Currency}
	}
	p.Total = total
	return nil
}

func (p PriceBreakdown) Copy() PriceBreakdown {
	clone := p
	clone.Fees = append([]Fee(nil), p.Fees...)
	clone.Taxes = append([]Tax(nil), p.Taxes...)
	clone.Discounts = append([]Discount(nil), p.Discounts...)
	return clone
}

type QuoteInput struct {
	ListingID listings.ListingID
	Listing   *listings.Listing
	RentalTerm listings.RentalTermType
	Range     daterange.DateRange
	Guests    int
}

type Calculator interface {
	Quote(ctx context.Context, input QuoteInput) (PriceBreakdown, error)
}
