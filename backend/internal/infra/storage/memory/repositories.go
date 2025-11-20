package memory

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"

	domainavailability "rentme/internal/domain/availability"
	domainbooking "rentme/internal/domain/booking"
	domainlistings "rentme/internal/domain/listings"
	domainreviews "rentme/internal/domain/reviews"
)

var (
	// ErrListingNotFound is returned when a listing cannot be located in memory.
	ErrListingNotFound = errors.New("memory: listing not found")
	// ErrBookingNotFound is returned when a booking does not exist.
	ErrBookingNotFound = errors.New("memory: booking not found")
)

// ListingRepository is an in-memory implementation for demo purposes.
type ListingRepository struct {
	mu    sync.RWMutex
	items map[domainlistings.ListingID]*domainlistings.Listing
}

// NewListingRepository builds an empty repository.
func NewListingRepository() *ListingRepository {
	return &ListingRepository{
		items: make(map[domainlistings.ListingID]*domainlistings.Listing),
	}
}

// ByID returns a listing or ErrListingNotFound.
func (r *ListingRepository) ByID(ctx context.Context, id domainlistings.ListingID) (*domainlistings.Listing, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	listing, ok := r.items[id]
	if !ok {
		return nil, ErrListingNotFound
	}
	return listing, nil
}

// Save stores/updates a listing entry.
func (r *ListingRepository) Save(ctx context.Context, listing *domainlistings.Listing) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[listing.ID] = listing
	return nil
}

// Search returns listings that satisfy provided filters.
func (r *ListingRepository) Search(ctx context.Context, params domainlistings.SearchParams) (domainlistings.SearchResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	opts := params.Normalized()
	matches := make([]*domainlistings.Listing, 0, len(r.items))
	for _, listing := range r.items {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return domainlistings.SearchResult{}, ctx.Err()
			default:
			}
		}

		if opts.OnlyActive && listing.State != domainlistings.ListingActive {
			continue
		}
		if opts.Host != "" && listing.Host != opts.Host {
			continue
		}
		if len(opts.States) > 0 && !stateIncluded(listing.State, opts.States) {
			continue
		}
		if opts.City != "" && !strings.EqualFold(listing.Address.City, opts.City) {
			continue
		}
		if opts.Country != "" && !strings.EqualFold(listing.Address.Country, opts.Country) {
			continue
		}
		if opts.LocationQuery != "" {
			if !matchLocation(listing, opts.LocationQuery) {
				continue
			}
		}
		if opts.MinGuests > 0 && listing.GuestsLimit < opts.MinGuests {
			continue
		}
		if opts.PriceMinCents > 0 && listing.NightlyRateCents < opts.PriceMinCents {
			continue
		}
		if opts.PriceMaxCents > 0 && listing.NightlyRateCents > opts.PriceMaxCents {
			continue
		}
		if !opts.CheckIn.IsZero() && listing.AvailableFrom.After(opts.CheckIn) {
			continue
		}
		if !tokensMatch(listing.Amenities, opts.Amenities) {
			continue
		}
		if !tokensMatch(listing.Tags, opts.Tags) {
			continue
		}
		if len(opts.PropertyTypes) > 0 && !propertyTypeMatches(listing.PropertyType, opts.PropertyTypes) {
			continue
		}
		matches = append(matches, listing)
	}

	sort.Slice(matches, func(i, j int) bool {
		switch opts.Sort {
		case domainlistings.SortByPriceDesc:
			if matches[i].NightlyRateCents == matches[j].NightlyRateCents {
				return matches[i].Rating > matches[j].Rating
			}
			return matches[i].NightlyRateCents > matches[j].NightlyRateCents
		case domainlistings.SortByRating:
			if matches[i].Rating == matches[j].Rating {
				return matches[i].NightlyRateCents < matches[j].NightlyRateCents
			}
			return matches[i].Rating > matches[j].Rating
		case domainlistings.SortByNewest:
			if matches[i].AvailableFrom.Equal(matches[j].AvailableFrom) {
				return matches[i].NightlyRateCents < matches[j].NightlyRateCents
			}
			return matches[i].AvailableFrom.After(matches[j].AvailableFrom)
		case domainlistings.SortByUpdated:
			if matches[i].UpdatedAt.Equal(matches[j].UpdatedAt) {
				return matches[i].NightlyRateCents < matches[j].NightlyRateCents
			}
			return matches[i].UpdatedAt.After(matches[j].UpdatedAt)
		default:
			if matches[i].NightlyRateCents == matches[j].NightlyRateCents {
				return matches[i].Rating > matches[j].Rating
			}
			return matches[i].NightlyRateCents < matches[j].NightlyRateCents
		}
	})

	total := len(matches)
	start := opts.Offset
	if start > total {
		start = total
	}
	end := start + opts.Limit
	if end > total {
		end = total
	}

	return domainlistings.SearchResult{
		Items: matches[start:end],
		Total: total,
	}, nil
}

func tokensMatch(values []string, required []string) bool {
	if len(required) == 0 {
		return true
	}
	if len(values) == 0 {
		return false
	}
	index := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.ToLower(value))
		if value == "" {
			continue
		}
		index[value] = struct{}{}
	}
	for _, token := range required {
		token = strings.TrimSpace(strings.ToLower(token))
		if token == "" {
			continue
		}
		if _, ok := index[token]; !ok {
			return false
		}
	}
	return true
}

func matchLocation(listing *domainlistings.Listing, needle string) bool {
	if listing == nil {
		return false
	}
	full := strings.ToLower(strings.Join([]string{
		listing.Address.City,
		listing.Address.Country,
		listing.Address.Line1,
		listing.Title,
	}, " "))
	return strings.Contains(full, needle)
}

func propertyTypeMatches(value string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	current := strings.TrimSpace(strings.ToLower(value))
	if current == "" {
		return false
	}
	for _, option := range allowed {
		if current == option {
			return true
		}
	}
	return false
}

func stateIncluded(state domainlistings.ListingState, allowed []domainlistings.ListingState) bool {
	for _, candidate := range allowed {
		if state == candidate {
			return true
		}
	}
	return false
}

// AvailabilityRepository keeps availability calendars in memory.
type AvailabilityRepository struct {
	mu        sync.RWMutex
	calendars map[domainlistings.ListingID]*domainavailability.AvailabilityCalendar
}

// NewAvailabilityRepository returns a repository initialized with empty calendars.
func NewAvailabilityRepository() *AvailabilityRepository {
	return &AvailabilityRepository{
		calendars: make(map[domainlistings.ListingID]*domainavailability.AvailabilityCalendar),
	}
}

// Calendar retrieves an availability calendar, lazily creating it.
func (r *AvailabilityRepository) Calendar(ctx context.Context, id domainlistings.ListingID) (*domainavailability.AvailabilityCalendar, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if cal, ok := r.calendars[id]; ok {
		return cal, nil
	}
	cal := domainavailability.NewCalendar(id, 1)
	r.calendars[id] = cal
	return cal, nil
}

// Save persists a calendar snapshot.
func (r *AvailabilityRepository) Save(ctx context.Context, calendar *domainavailability.AvailabilityCalendar) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calendars[calendar.ListingID] = calendar
	return nil
}

// BookingRepository stores bookings in memory.
type BookingRepository struct {
	mu    sync.RWMutex
	items map[domainbooking.BookingID]*domainbooking.Booking
}

// NewBookingRepository builds an empty booking repo.
func NewBookingRepository() *BookingRepository {
	return &BookingRepository{items: make(map[domainbooking.BookingID]*domainbooking.Booking)}
}

// ByID fetches a booking.
func (r *BookingRepository) ByID(ctx context.Context, id domainbooking.BookingID) (*domainbooking.Booking, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	booking, ok := r.items[id]
	if !ok {
		return nil, ErrBookingNotFound
	}
	return booking, nil
}

// Save stores the current booking state.
func (r *BookingRepository) Save(ctx context.Context, booking *domainbooking.Booking) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	booking.Version++
	r.items[booking.ID] = booking
	return nil
}

func (r *BookingRepository) ListByGuest(ctx context.Context, guestID string) ([]*domainbooking.Booking, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id := strings.TrimSpace(guestID)
	if id == "" {
		return nil, errors.New("memory: guest id required")
	}
	matches := make([]*domainbooking.Booking, 0)
	for _, booking := range r.items {
		if booking.GuestID == id {
			matches = append(matches, booking)
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].CreatedAt.After(matches[j].CreatedAt)
	})
	result := make([]*domainbooking.Booking, len(matches))
	copy(result, matches)
	return result, nil
}

// ReviewsRepository is a lightweight in-memory review store.
type ReviewsRepository struct {
	mu    sync.RWMutex
	items map[string]*domainreviews.Review
}

// NewReviewsRepository builds an empty reviews store.
func NewReviewsRepository() *ReviewsRepository {
	return &ReviewsRepository{items: make(map[string]*domainreviews.Review)}
}

// ByBooking locates a review using booking and author identifiers.
func (r *ReviewsRepository) ByBooking(ctx context.Context, bookingID domainbooking.BookingID, authorID string) (*domainreviews.Review, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	key := bookingReviewKey(bookingID, authorID)
	if review, ok := r.items[key]; ok {
		return review, nil
	}
	return nil, errors.New("memory: review not found")
}

// Save writes the review entry.
func (r *ReviewsRepository) Save(ctx context.Context, review *domainreviews.Review) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := bookingReviewKey(review.BookingID, review.AuthorID)
	r.items[key] = review
	return nil
}

func bookingReviewKey(bookingID domainbooking.BookingID, authorID string) string {
	return string(bookingID) + ":" + authorID
}
