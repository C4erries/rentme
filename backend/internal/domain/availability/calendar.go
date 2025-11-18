package availability

import (
	"context"
	"errors"
	"time"

	"rentme/internal/domain/listings"
	"rentme/internal/domain/shared/daterange"
	"rentme/internal/domain/shared/events"
)

var (
	ErrOverlappingRange = errors.New("availability: range overlaps with an existing block")
	ErrRangeNotFound    = errors.New("availability: range not found")
)

type BlockReason string

const (
	ReasonBooking   BlockReason = "BOOKING"
	ReasonHostBlock BlockReason = "HOST_BLOCK"
	ReasonCleaning  BlockReason = "CLEANING_BUFFER"
)

type Block struct {
	Range     daterange.DateRange
	Reason    BlockReason
	Reference string
	CreatedAt time.Time
}

type AvailabilityCalendar struct {
	ListingID          listings.ListingID
	Blocks             []Block
	Version            int64
	CleaningBufferDays int
	events.EventRecorder
}

type Repository interface {
	Calendar(ctx context.Context, id listings.ListingID) (*AvailabilityCalendar, error)
	Save(ctx context.Context, calendar *AvailabilityCalendar) error
}

func NewCalendar(id listings.ListingID, cleaningBufferDays int) *AvailabilityCalendar {
	return &AvailabilityCalendar{ListingID: id, CleaningBufferDays: cleaningBufferDays}
}

func (c *AvailabilityCalendar) CanReserve(r daterange.DateRange) bool {
	for _, block := range c.Blocks {
		if block.Range.Overlaps(r) {
			return false
		}
	}
	return true
}

func (c *AvailabilityCalendar) Reserve(r daterange.DateRange, bookingID string, now time.Time) error {
	if !c.CanReserve(r) {
		c.Record(CalendarOverbookingPreventedEvent(c.ListingID, r, now))
		return ErrOverlappingRange
	}
	c.appendBlock(Block{Range: r, Reason: ReasonBooking, Reference: bookingID, CreatedAt: now.UTC()})

	if c.CleaningBufferDays > 0 {
		buffer := time.Hour * 24 * time.Duration(c.CleaningBufferDays)
		before := daterange.DateRange{CheckIn: r.CheckIn.Add(-buffer), CheckOut: r.CheckIn}
		if before.CheckOut.After(before.CheckIn) {
			if c.CanReserve(before) {
				c.appendBlock(Block{Range: before, Reason: ReasonCleaning, Reference: bookingID + "-before", CreatedAt: now.UTC()})
			}
		}
		after := daterange.DateRange{CheckIn: r.CheckOut, CheckOut: r.CheckOut.Add(buffer)}
		if after.CheckOut.After(after.CheckIn) {
			if c.CanReserve(after) {
				c.appendBlock(Block{Range: after, Reason: ReasonCleaning, Reference: bookingID + "-after", CreatedAt: now.UTC()})
			}
		}
	}

	c.Record(CalendarBlockedEvent(c.ListingID, r, ReasonBooking, now))
	return nil
}

func (c *AvailabilityCalendar) BlockRange(r daterange.DateRange, reason BlockReason, reference string, now time.Time) error {
	if reason == "" {
		reason = ReasonHostBlock
	}
	if !c.CanReserve(r) {
		return ErrOverlappingRange
	}
	c.appendBlock(Block{Range: r, Reason: reason, Reference: reference, CreatedAt: now.UTC()})
	c.Record(CalendarBlockedEvent(c.ListingID, r, reason, now))
	return nil
}

func (c *AvailabilityCalendar) Release(reference string, now time.Time) error {
	idx := -1
	for i, block := range c.Blocks {
		if block.Reference == reference {
			idx = i
			break
		}
	}
	if idx == -1 {
		return ErrRangeNotFound
	}
	removed := c.Blocks[idx]
	c.Blocks = append(c.Blocks[:idx], c.Blocks[idx+1:]...)
	c.Record(CalendarReleasedEvent(c.ListingID, removed.Range, removed.Reason, now))
	return nil
}

func (c *AvailabilityCalendar) appendBlock(block Block) {
	c.Blocks = append(c.Blocks, block)
}
