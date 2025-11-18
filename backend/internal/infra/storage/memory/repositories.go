package memory

import (
	"context"
	"errors"
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
