package listings

import (
	"time"
)

type ListingCreatedEvent struct {
	ListingID ListingID
	HostID    HostID
	At        time.Time
}

func (e ListingCreatedEvent) EventName() string     { return "listing.created" }
func (e ListingCreatedEvent) AggregateID() string   { return string(e.ListingID) }
func (e ListingCreatedEvent) OccurredAt() time.Time { return e.At }

type ListingActivatedEvent struct {
	ListingID ListingID
	HostID    HostID
	At        time.Time
}

func (e ListingActivatedEvent) EventName() string     { return "listing.activated" }
func (e ListingActivatedEvent) AggregateID() string   { return string(e.ListingID) }
func (e ListingActivatedEvent) OccurredAt() time.Time { return e.At }

type ListingSuspendedEvent struct {
	ListingID ListingID
	Reason    string
	At        time.Time
}

func (e ListingSuspendedEvent) EventName() string     { return "listing.suspended" }
func (e ListingSuspendedEvent) AggregateID() string   { return string(e.ListingID) }
func (e ListingSuspendedEvent) OccurredAt() time.Time { return e.At }

type ListingUpdatedEvent struct {
	ListingID ListingID
	At        time.Time
}

func (e ListingUpdatedEvent) EventName() string     { return "listing.updated" }
func (e ListingUpdatedEvent) AggregateID() string   { return string(e.ListingID) }
func (e ListingUpdatedEvent) OccurredAt() time.Time { return e.At }
