package availability

import (
	"context"
	"time"

	"rentme/internal/app/dto"
	"rentme/internal/app/queries"
	"rentme/internal/app/uow"
	domainlistings "rentme/internal/domain/listings"
)

const getCalendarKey = "availability.calendar"

type GetCalendarQuery struct {
	ListingID string
	From      time.Time
	To        time.Time
}

func (q GetCalendarQuery) Key() string { return getCalendarKey }

type GetCalendarHandler struct {
	UoWFactory uow.UoWFactory
}

func (h *GetCalendarHandler) Handle(ctx context.Context, q GetCalendarQuery) (dto.Calendar, error) {
	unit, ok := uow.FromContext(ctx)
	if !ok {
		if h.UoWFactory == nil {
			return dto.Calendar{}, uow.ErrUnitOfWorkMissing
		}
		var err error
		unit, err = h.UoWFactory.Begin(ctx, uow.TxOptions{ReadOnly: true})
		if err != nil {
			return dto.Calendar{}, err
		}
		ctx = uow.ContextWithUnitOfWork(ctx, unit)
		defer unit.Rollback(ctx)
	}

	calendar, err := unit.Availability().Calendar(ctx, domainlistings.ListingID(q.ListingID))
	if err != nil {
		return dto.Calendar{}, err
	}

	return dto.MapCalendar(calendar), nil
}

var _ queries.Handler[GetCalendarQuery, dto.Calendar] = (*GetCalendarHandler)(nil)
