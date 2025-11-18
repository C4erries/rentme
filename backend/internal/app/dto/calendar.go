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
	return Calendar{
		ListingID: string(cal.ListingID),
		Blocks:    mapCalendarBlocks(cal.Blocks),
	}
}

// MapCalendarWithin returns only the blocks overlapping the provided window.
func MapCalendarWithin(cal *availability.AvailabilityCalendar, from, to time.Time) Calendar {
	if cal == nil {
		return Calendar{}
	}
	if from.IsZero() && to.IsZero() {
		return MapCalendar(cal)
	}
	from = normalizeUTC(from)
	to = normalizeUTC(to)
	filtered := make([]availability.Block, 0, len(cal.Blocks))
	for _, block := range cal.Blocks {
		if !from.IsZero() && !block.Range.CheckOut.After(from) {
			continue
		}
		if !to.IsZero() && !block.Range.CheckIn.Before(to) {
			continue
		}
		filtered = append(filtered, block)
	}
	return Calendar{ListingID: string(cal.ListingID), Blocks: mapCalendarBlocks(filtered)}
}

func mapCalendarBlocks(blocks []availability.Block) []CalendarBlock {
	if len(blocks) == 0 {
		return nil
	}
	result := make([]CalendarBlock, 0, len(blocks))
	for _, b := range blocks {
		result = append(result, CalendarBlock{
			From:   b.Range.CheckIn,
			To:     b.Range.CheckOut,
			Reason: string(b.Reason),
		})
	}
	return result
}

func normalizeUTC(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return t.UTC()
}
