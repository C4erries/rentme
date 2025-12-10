package listings

import (
	"strings"
	"time"
)

// CatalogSort defines a supported ordering.
type CatalogSort string

const (
	SortByPriceAsc  CatalogSort = "price_asc"
	SortByPriceDesc CatalogSort = "price_desc"
	SortByRating    CatalogSort = "rating_desc"
	SortByNewest    CatalogSort = "newest"
	SortByUpdated   CatalogSort = "updated"

	defaultSearchLimit = 24
	maxSearchLimit     = 60
)

// SearchParams describe catalog filters and paging options.
type SearchParams struct {
	Host          HostID
	States        []ListingState
	City          string
	Region        string
	Country       string
	LocationQuery string
	Tags          []string
	Amenities     []string
	MinGuests     int
	PriceMinCents int64
	PriceMaxCents int64
	PropertyTypes []string
	RentalTerms   []RentalTermType
	CheckIn       time.Time
	CheckOut      time.Time
	Sort          CatalogSort
	Limit         int
	Offset        int
	OnlyActive    bool
}

// Normalized returns a sanitized copy of params.
func (p SearchParams) Normalized() SearchParams {
	normalized := p
	normalized.City = strings.TrimSpace(strings.ToLower(normalized.City))
	normalized.Region = strings.TrimSpace(strings.ToLower(normalized.Region))
	normalized.Country = strings.TrimSpace(strings.ToLower(normalized.Country))
	normalized.LocationQuery = strings.TrimSpace(strings.ToLower(normalized.LocationQuery))
	normalized.Tags = normalizeTokens(normalized.Tags)
	normalized.Amenities = normalizeTokens(normalized.Amenities)
	normalized.PropertyTypes = normalizeTokens(normalized.PropertyTypes)
	normalized.RentalTerms = normalizeRentalTerms(normalized.RentalTerms)
	normalized.CheckIn = normalizeDate(normalized.CheckIn)
	normalized.CheckOut = normalizeDate(normalized.CheckOut)
	if !normalized.CheckIn.IsZero() && !normalized.CheckOut.IsZero() && !normalized.CheckOut.After(normalized.CheckIn) {
		normalized.CheckOut = time.Time{}
	}
	if normalized.MinGuests < 0 {
		normalized.MinGuests = 0
	}
	if normalized.PriceMinCents < 0 {
		normalized.PriceMinCents = 0
	}
	if normalized.PriceMaxCents > 0 && normalized.PriceMaxCents < normalized.PriceMinCents {
		normalized.PriceMaxCents = 0
	}
	if normalized.Limit <= 0 {
		normalized.Limit = defaultSearchLimit
	}
	if normalized.Limit > maxSearchLimit {
		normalized.Limit = maxSearchLimit
	}
	if normalized.Offset < 0 {
		normalized.Offset = 0
	}
	switch normalized.Sort {
	case SortByPriceAsc, SortByPriceDesc, SortByRating, SortByNewest:
	case SortByUpdated:
	default:
		normalized.Sort = SortByPriceAsc
	}
	return normalized
}

func normalizeTokens(tokens []string) []string {
	if len(tokens) == 0 {
		return nil
	}
	out := make([]string, 0, len(tokens))
	seen := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		token = strings.TrimSpace(strings.ToLower(token))
		if token == "" {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}
	return out
}

func normalizeRentalTerms(values []RentalTermType) []RentalTermType {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[RentalTermType]struct{}, len(values))
	out := make([]RentalTermType, 0, len(values))
	for _, value := range values {
		normalized := normalizeRentalTerm(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func normalizeDate(value time.Time) time.Time {
	if value.IsZero() {
		return value
	}
	y, m, d := value.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// SearchResult wraps search hits with meta.
type SearchResult struct {
	Items []*Listing
	Total int
}
