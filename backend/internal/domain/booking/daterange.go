package booking

import (
	"errors"
	"time"

	"rentme/internal/domain/shared/daterange"
)

var ErrCheckInInPast = errors.New("booking: check-in date is in the past")

func ValidateDateRange(dr daterange.DateRange, now time.Time) error {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	checkInDate := time.Date(dr.CheckIn.Year(), dr.CheckIn.Month(), dr.CheckIn.Day(), 0, 0, 0, 0, time.UTC)
	if checkInDate.Before(today) {
		return ErrCheckInInPast
	}
	return nil
}
