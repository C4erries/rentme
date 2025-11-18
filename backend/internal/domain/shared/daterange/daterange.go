package daterange

import (
	"errors"
	"time"
)

var (
	ErrInvalidRange = errors.New("daterange: checkout must be after checkin")
)

// DateRange represents a half-open interval [checkIn, checkOut)
type DateRange struct {
	CheckIn  time.Time
	CheckOut time.Time
}

func New(checkIn, checkOut time.Time) (DateRange, error) {
	dr := DateRange{CheckIn: checkIn.UTC(), CheckOut: checkOut.UTC()}
	if err := dr.Validate(); err != nil {
		return DateRange{}, err
	}
	return dr, nil
}

func (dr DateRange) Validate() error {
	if dr.CheckOut.IsZero() || dr.CheckIn.IsZero() {
		return ErrInvalidRange
	}
	if !dr.CheckOut.After(dr.CheckIn) {
		return ErrInvalidRange
	}
	return nil
}

func (dr DateRange) Nights() int {
	return int(dr.CheckOut.Sub(dr.CheckIn).Hours() / 24)
}

func (dr DateRange) Overlaps(other DateRange) bool {
	return dr.CheckIn.Before(other.CheckOut) && other.CheckIn.Before(dr.CheckOut)
}

func (dr DateRange) Contains(other DateRange) bool {
	return (dr.CheckIn.Before(other.CheckIn) || dr.CheckIn.Equal(other.CheckIn)) &&
		(dr.CheckOut.After(other.CheckOut) || dr.CheckOut.Equal(other.CheckOut))
}

func (dr DateRange) ContainsDate(t time.Time) bool {
	t = t.UTC()
	return (t.Equal(dr.CheckIn) || t.After(dr.CheckIn)) && t.Before(dr.CheckOut)
}

func (dr DateRange) Adjacent(other DateRange) bool {
	return dr.CheckOut.Equal(other.CheckIn) || dr.CheckIn.Equal(other.CheckOut)
}

func (dr DateRange) Merge(other DateRange) (DateRange, bool) {
	if !(dr.Overlaps(other) || dr.Adjacent(other)) {
		return DateRange{}, false
	}
	start := dr.CheckIn
	if other.CheckIn.Before(start) {
		start = other.CheckIn
	}
	end := dr.CheckOut
	if other.CheckOut.After(end) {
		end = other.CheckOut
	}
	return DateRange{CheckIn: start, CheckOut: end}, true
}
