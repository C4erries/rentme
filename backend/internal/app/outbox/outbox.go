package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"rentme/internal/domain/shared/events"
)

type EventRecord struct {
	ID         string
	Name       string
	Payload    []byte
	OccurredAt time.Time
	Aggregate  string
	Headers    map[string]string
}

type Outbox interface {
	Add(ctx context.Context, record EventRecord) error
	Flush(ctx context.Context) error
}

type EventEncoder interface {
	Encode(ev events.DomainEvent) (EventRecord, error)
}

type JSONEventEncoder struct {
	IDGenerator func() string
}

func (e JSONEventEncoder) Encode(ev events.DomainEvent) (EventRecord, error) {
	payload, err := json.Marshal(ev)
	if err != nil {
		return EventRecord{}, err
	}
	idGen := e.IDGenerator
	if idGen == nil {
		idGen = defaultIDGenerator
	}
	return EventRecord{
		ID:         idGen(),
		Name:       ev.EventName(),
		Payload:    payload,
		OccurredAt: ev.OccurredAt(),
		Aggregate:  ev.AggregateID(),
		Headers:    map[string]string{},
	}, nil
}

func RecordDomainEvents(ctx context.Context, box Outbox, encoder EventEncoder, evs []events.DomainEvent) error {
	if box == nil || len(evs) == 0 {
		return nil
	}
	if encoder == nil {
		encoder = JSONEventEncoder{}
	}
	for _, ev := range evs {
		rec, err := encoder.Encode(ev)
		if err != nil {
			return err
		}
		if err := box.Add(ctx, rec); err != nil {
			return err
		}
	}
	return nil
}

func defaultIDGenerator() string {
	return fmt.Sprintf("evt-%d", time.Now().UnixNano())
}
