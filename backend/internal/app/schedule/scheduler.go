package schedule

import (
	"context"
	"time"
)

type Scheduler interface {
	Schedule(ctx context.Context, name string, payload any, runAt time.Time) error
}
