package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Producer interface {
	Publish(ctx context.Context, topic string, key string, payload []byte, headers map[string]string) error
}

type Worker struct {
	Store       *Store
	Producer    Producer
	Interval    time.Duration
	TopicPrefix string
	Source      string
	ID          string
	Backoff     []time.Duration
}

func (w *Worker) Run(ctx context.Context) error {
	if w.Store == nil || w.Producer == nil {
		return ErrWorkerNotConfigured
	}
	ticker := time.NewTicker(w.interval())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := w.processOnce(ctx); err != nil {
				return err
			}
		}
	}
}

func (w *Worker) processOnce(ctx context.Context) error {
	doc, err := w.Store.Claim(ctx, w.workerID())
	if err != nil || doc == nil {
		return err
	}
	topic := w.topicFor(doc.Name)
	payload, headers, err := w.formatPayload(doc)
	if err != nil {
		_ = w.Store.MarkFailed(ctx, doc.ID, w.nextRetry(doc.Attempts), err.Error())
		return nil
	}
	if err := w.Producer.Publish(ctx, topic, doc.Aggregate, payload, headers); err != nil {
		_ = w.Store.MarkFailed(ctx, doc.ID, w.nextRetry(doc.Attempts), err.Error())
		return nil
	}
	return w.Store.MarkSent(ctx, doc.ID)
}

func (w *Worker) formatPayload(doc *EventDocument) ([]byte, map[string]string, error) {
	if doc.Headers == nil {
		doc.Headers = map[string]string{}
	}
	data := map[string]any{}
	if err := json.Unmarshal(doc.Payload, &data); err != nil {
		return nil, nil, err
	}
	evt := map[string]any{
		"specversion":     "1.0",
		"id":              uuid.NewString(),
		"type":            doc.Name + ".v1",
		"source":          w.source(),
		"time":            doc.OccurredAt,
		"datacontenttype": "application/json",
		"data":            data,
	}
	if trace, ok := doc.Headers["traceparent"]; ok {
		evt["traceparent"] = trace
	}
	payload, err := json.Marshal(evt)
	if err != nil {
		return nil, nil, err
	}
	headers := map[string]string{
		"content-type": "application/cloudevents+json",
	}
	for k, v := range doc.Headers {
		headers[k] = v
	}
	return payload, headers, nil
}

func (w *Worker) topicFor(name string) string {
	base := name
	if idx := strings.IndexRune(name, '.'); idx > 0 {
		base = name[:idx]
	}
	topic := base + ".events.v1"
	if w.TopicPrefix != "" {
		topic = w.TopicPrefix + topic
	}
	return topic
}

func (w *Worker) workerID() string {
	if w.ID != "" {
		return w.ID
	}
	return uuid.NewString()
}

func (w *Worker) interval() time.Duration {
	if w.Interval <= 0 {
		return 500 * time.Millisecond
	}
	return w.Interval
}

func (w *Worker) nextRetry(attempts int) time.Time {
	if attempts < len(w.Backoff) {
		return time.Now().Add(w.Backoff[attempts])
	}
	if len(w.Backoff) > 0 {
		return time.Now().Add(w.Backoff[len(w.Backoff)-1])
	}
	return time.Now().Add(5 * time.Second)
}

func (w *Worker) source() string {
	if w.Source != "" {
		return w.Source
	}
	return "app://rentme"
}

var ErrWorkerNotConfigured = errors.New("outbox: worker missing dependencies")
