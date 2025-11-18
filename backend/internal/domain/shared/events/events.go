package events

import "time"

type EventID string

type DomainEvent interface {
	EventName() string
	AggregateID() string
	OccurredAt() time.Time
}

type EventRecorder struct {
	pending []DomainEvent
}

func (r *EventRecorder) Record(event DomainEvent) {
	if event == nil {
		return
	}
	r.pending = append(r.pending, event)
}

func (r *EventRecorder) PendingEvents() []DomainEvent {
	out := make([]DomainEvent, len(r.pending))
	copy(out, r.pending)
	return out
}

func (r *EventRecorder) ClearEvents() {
	r.pending = nil
}

type BaseEvent struct {
	Name      string
	Aggregate string
	Time      time.Time
}

func (e BaseEvent) EventName() string {
	return e.Name
}

func (e BaseEvent) AggregateID() string {
	return e.Aggregate
}

func (e BaseEvent) OccurredAt() time.Time {
	return e.Time
}
