package listings

import "strings"

// CatalogSort defines a supported ordering.
type CatalogSort string

const (
	SortByPriceAsc  CatalogSort = "price_asc"
	SortByPriceDesc CatalogSort = "price_desc"
	SortByRating    CatalogSort = "rating_desc"
	SortByNewest    CatalogSort = "newest"

	defaultSearchLimit = 24
	maxSearchLimit     = 60
)

// SearchParams describe catalog filters and paging options.
type SearchParams struct {
	City          string
	Country       string
	Tags          []string
	Amenities     []string
	MinGuests     int
	PriceMinCents int64
	PriceMaxCents int64
	Sort          CatalogSort
	Limit         int
	Offset        int
	OnlyActive    bool
}

// Normalized returns a sanitized copy of params.
func (p SearchParams) Normalized() SearchParams {
	normalized := p
	normalized.City = strings.TrimSpace(strings.ToLower(normalized.City))
	normalized.Country = strings.TrimSpace(strings.ToLower(normalized.Country))
	normalized.Tags = normalizeTokens(normalized.Tags)
	normalized.Amenities = normalizeTokens(normalized.Amenities)
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

// SearchResult wraps search hits with meta.
type SearchResult struct {
	Items []*Listing
	Total int
}
