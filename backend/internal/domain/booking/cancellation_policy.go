package booking

import (
	"time"

	"rentme/internal/domain/shared/money"
)

type CancellationPolicySnapshot struct {
	PolicyID                  string
	FreeCancellationUntil     time.Time
	PreCheckInPenaltyPercent  int
	PostCheckInPenaltyPercent int
}

func (c CancellationPolicySnapshot) CalculateRefund(total money.Money, cancelAt, checkIn time.Time) (refund money.Money, penalty money.Money, err error) {
	if cancelAt.IsZero() {
		cancelAt = time.Now().UTC()
	}
	percent := 0
	if c.PolicyID == "" {
		percent = 0
	} else if cancelAt.Before(checkIn) {
		if !c.FreeCancellationUntil.IsZero() && cancelAt.Before(c.FreeCancellationUntil) {
			percent = 0
		} else {
			percent = clampPercent(c.PreCheckInPenaltyPercent)
		}
	} else {
		percent = clampPercent(c.PostCheckInPenaltyPercent)
	}
	penalty = percentOf(total, percent)
	refund, err = total.Sub(penalty)
	if err != nil {
		return money.Money{}, money.Money{}, err
	}
	return refund, penalty, nil
}

func percentOf(total money.Money, percent int) money.Money {
	if percent <= 0 {
		return money.Money{Amount: 0, Currency: total.Currency}
	}
	const percentBase = int64(100)
	amount := total.Amount * int64(percent) / percentBase
	return money.Money{Amount: amount, Currency: total.Currency}
}

func clampPercent(p int) int {
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}
