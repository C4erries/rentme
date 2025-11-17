package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"time"

	"rentme/internal/app/commands"
)

// IdempotentCommand must be implemented by commands that want idempotency guarantees.
type IdempotentCommand interface {
	commands.Command
	IdempotencyKey() string
	ResultPrototype() any // should match the handler result type
}

type IdempotencyRecord struct {
	Key        string
	Payload    []byte
	Error      string
	OccurredAt time.Time
}

type IdempotencyStore interface {
	Get(ctx context.Context, key string) (IdempotencyRecord, bool, error)
	Save(ctx context.Context, rec IdempotencyRecord) error
}

type ResultCodec interface {
	Encode(v any) ([]byte, error)
	Decode(data []byte, out any) error
}

type JSONResultCodec struct{}

func (JSONResultCodec) Encode(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (JSONResultCodec) Decode(data []byte, out any) error {
	return json.Unmarshal(data, out)
}

var (
	errMissingPrototype = errors.New("middleware: idempotent command requires result prototype")
)

func Idempotency(store IdempotencyStore, codec ResultCodec) CommandMiddleware {
	if store == nil {
		panic("middleware: idempotency store required")
	}
	if codec == nil {
		codec = JSONResultCodec{}
	}
	return func(next commands.Bus) commands.Bus {
		nextFn := wrapCommand(next)
		return commandFunc(func(ctx context.Context, cmd commands.Command) (any, error) {
			idCmd, ok := cmd.(IdempotentCommand)
			if !ok {
				return nextFn(ctx, cmd)
			}
			key := idCmd.IdempotencyKey()
			if key == "" {
				return nextFn(ctx, cmd)
			}
			rec, found, err := store.Get(ctx, key)
			if err != nil {
				return nil, err
			}
			if found {
				if rec.Error != "" {
					return nil, errors.New(rec.Error)
				}
				proto := idCmd.ResultPrototype()
				if proto == nil {
					return nil, errMissingPrototype
				}
				if err := codec.Decode(rec.Payload, proto); err != nil {
					return nil, err
				}
				return normalizePrototype(proto), nil
			}
			result, err := nextFn(ctx, cmd)
			record := IdempotencyRecord{
				Key:        key,
				OccurredAt: time.Now().UTC(),
			}
			if err != nil {
				record.Error = err.Error()
				if saveErr := store.Save(ctx, record); saveErr != nil {
					return nil, errors.Join(err, saveErr)
				}
				return nil, err
			}
			if result != nil {
				payload, encErr := codec.Encode(result)
				if encErr != nil {
					return nil, encErr
				}
				record.Payload = payload
			}
			if saveErr := store.Save(ctx, record); saveErr != nil {
				return nil, saveErr
			}
			return result, nil
		})
	}
}

func normalizePrototype(proto any) any {
	rv := reflect.ValueOf(proto)
	if rv.Kind() == reflect.Ptr && !rv.IsNil() {
		return rv.Interface()
	}
	return proto
}
