package dto

import (
	"time"

	"rentme/internal/domain/availability"
)

type CalendarBlock struct {
	From   time.Time `json:"from"`
	To     time.Time `json:"to"`
	Reason string    `json:"reason"`
}

type Calendar struct {
	ListingID string          `json:"listing_id"`
	Blocks    []CalendarBlock `json:"blocks"`
}

func MapCalendar(cal *availability.AvailabilityCalendar) Calendar {
	if cal == nil {
		return Calendar{}
	}
	blocks := make([]CalendarBlock, 0, len(cal.Blocks))
	for _, b := range cal.Blocks {
		blocks = append(blocks, CalendarBlock{
			From:   b.Range.CheckIn,
			To:     b.Range.CheckOut,
			Reason: string(b.Reason),
		})
	}
	return Calendar{ListingID: string(cal.ListingID), Blocks: blocks}
}
