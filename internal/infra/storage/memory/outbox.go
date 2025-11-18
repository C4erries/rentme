package memory

import (
	"context"
	"sync"

	appoutbox "rentme/internal/app/outbox"
)

// Outbox is a no-op implementation that merely keeps events in memory until flushed.
type Outbox struct {
	mu      sync.Mutex
	records []appoutbox.EventRecord
}

func NewOutbox() *Outbox {
	return &Outbox{}
}

func (o *Outbox) Add(ctx context.Context, record appoutbox.EventRecord) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.records = append(o.records, record)
	return nil
}

func (o *Outbox) Flush(ctx context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.records = nil
	return nil
}

var _ appoutbox.Outbox = (*Outbox)(nil)
