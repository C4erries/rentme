package money

import (
	"errors"
	"strings"
)

var (
	ErrInvalidCurrency  = errors.New("money: invalid currency code")
	ErrCurrencyMismatch = errors.New("money: currency mismatch")
)

// Money keeps amounts in integer RUB to avoid floating point issues.
type Money struct {
	Amount   int64
	Currency string
}

// New constructs a Money value validating minimal invariants.
func New(amount int64, currency string) (Money, error) {
	if len(currency) != 3 {
		return Money{}, ErrInvalidCurrency
	}
	currency = strings.ToUpper(currency)
	return Money{Amount: amount, Currency: currency}, nil
}

// Must creates Money and panics if validation fails; useful in tests and fixtures.
func Must(amount int64, currency string) Money {
	m, err := New(amount, currency)
	if err != nil {
		panic(err)
	}
	return m
}

// Add adds two money values ensuring currencies match.
func (m Money) Add(other Money) (Money, error) {
	if err := m.ensureSameCurrency(other); err != nil {
		return Money{}, err
	}
	return Money{Amount: m.Amount + other.Amount, Currency: m.Currency}, nil
}

// Sub subtracts other from the receiver.
func (m Money) Sub(other Money) (Money, error) {
	if err := m.ensureSameCurrency(other); err != nil {
		return Money{}, err
	}
	return Money{Amount: m.Amount - other.Amount, Currency: m.Currency}, nil
}

// Neg returns the negated amount preserving currency.
func (m Money) Neg() Money {
	return Money{Amount: -m.Amount, Currency: m.Currency}
}

// Multiply multiplies the amount by the provided factor.
func (m Money) Multiply(times int64) Money {
	return Money{Amount: m.Amount * times, Currency: m.Currency}
}

// IsZero returns true if the amount equals zero.
func (m Money) IsZero() bool {
	return m.Amount == 0
}

func (m Money) ensureSameCurrency(other Money) error {
	if m.Currency == "" || other.Currency == "" {
		return ErrInvalidCurrency
	}
	if m.Currency != other.Currency {
		return ErrCurrencyMismatch
	}
	return nil
}
