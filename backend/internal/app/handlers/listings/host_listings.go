package listings

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"rentme/internal/app/dto"
	handlersupport "rentme/internal/app/handlers/support"
	"rentme/internal/app/queries"
	"rentme/internal/app/uow"
	domainlistings "rentme/internal/domain/listings"
)

const (
	listHostListingsKey = "host.listings.list"
	getHostListingKey   = "host.listings.get"
)

var ErrListingNotOwned = errors.New("listing not found for host")

type ListHostListingsQuery struct {
	HostID string
	Status string
	Limit  int
	Offset int
}

func (q ListHostListingsQuery) Key() string { return listHostListingsKey }

type ListHostListingsHandler struct {
	UoWFactory uow.UoWFactory
	Logger     *slog.Logger
}

func (h *ListHostListingsHandler) Handle(ctx context.Context, q ListHostListingsQuery) (dto.HostListingCatalog, error) {
	if strings.TrimSpace(q.HostID) == "" {
		return dto.HostListingCatalog{}, errors.New("host id is required")
	}
	unit, execCtx, cleanup, err := handlersupport.BeginReadOnlyUnit(ctx, h.UoWFactory)
	if err != nil {
		return dto.HostListingCatalog{}, err
	}
	if cleanup != nil {
		defer cleanup()
	}

	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := q.Offset
	if offset < 0 {
		offset = 0
	}

	params := domainlistings.SearchParams{
		Host:       domainlistings.HostID(q.HostID),
		Limit:      limit,
		Offset:     offset,
		States:     statesForStatus(q.Status),
		Sort:       domainlistings.SortByUpdated,
		OnlyActive: false,
	}

	result, err := unit.Listings().Search(execCtx, params)
	if err != nil {
		return dto.HostListingCatalog{}, err
	}

	items := make([]dto.HostListingSummary, 0, len(result.Items))
	for _, listing := range result.Items {
		items = append(items, dto.MapHostListingSummary(listing))
	}
	if h.Logger != nil {
		h.Logger.Debug("host listings queried", "host_id", q.HostID, "count", len(items))
	}

	return dto.HostListingCatalog{
		Items: items,
		Meta: dto.HostListingCatalogMeta{
			Total:  result.Total,
			Limit:  limit,
			Offset: offset,
		},
	}, nil
}

type GetHostListingQuery struct {
	HostID    string
	ListingID string
}

func (q GetHostListingQuery) Key() string { return getHostListingKey }

type GetHostListingHandler struct {
	UoWFactory uow.UoWFactory
	Logger     *slog.Logger
}

func (h *GetHostListingHandler) Handle(ctx context.Context, q GetHostListingQuery) (dto.HostListingDetail, error) {
	if strings.TrimSpace(q.HostID) == "" {
		return dto.HostListingDetail{}, errors.New("host id is required")
	}
	if strings.TrimSpace(q.ListingID) == "" {
		return dto.HostListingDetail{}, errors.New("listing id is required")
	}

	unit, execCtx, cleanup, err := handlersupport.BeginReadOnlyUnit(ctx, h.UoWFactory)
	if err != nil {
		return dto.HostListingDetail{}, err
	}
	if cleanup != nil {
		defer cleanup()
	}

	listing, err := unit.Listings().ByID(execCtx, domainlistings.ListingID(q.ListingID))
	if err != nil {
		return dto.HostListingDetail{}, err
	}
	if listing.Host != domainlistings.HostID(q.HostID) {
		return dto.HostListingDetail{}, ErrListingNotOwned
	}

	if h.Logger != nil {
		h.Logger.Debug("host listing loaded", "listing_id", listing.ID, "host_id", q.HostID)
	}

	return dto.MapHostListingDetail(listing), nil
}

func statesForStatus(raw string) []domainlistings.ListingState {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "draft":
		return []domainlistings.ListingState{domainlistings.ListingDraft}
	case "published":
		return []domainlistings.ListingState{domainlistings.ListingActive}
	case "archived":
		return []domainlistings.ListingState{domainlistings.ListingSuspended}
	default:
		return nil
	}
}

var _ queries.Handler[ListHostListingsQuery, dto.HostListingCatalog] = (*ListHostListingsHandler)(nil)
var _ queries.Handler[GetHostListingQuery, dto.HostListingDetail] = (*GetHostListingHandler)(nil)
