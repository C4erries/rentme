package memory

import (
	"context"
	"errors"

	"rentme/internal/app/uow"
	domainavailability "rentme/internal/domain/availability"
	domainbooking "rentme/internal/domain/booking"
	domainlistings "rentme/internal/domain/listings"
	domainpricing "rentme/internal/domain/pricing"
	domainreviews "rentme/internal/domain/reviews"
)

// Factory wires in-memory repositories into a unit-of-work boundary.
type Factory struct {
	ListingsRepo     domainlistings.ListingRepository
	AvailabilityRepo domainavailability.Repository
	BookingRepo      domainbooking.Repository
	PricingSvc       domainpricing.Calculator
	ReviewsRepo      domainreviews.Repository
}

// ErrFactoryMisconfigured indicates missing repositories.
var ErrFactoryMisconfigured = errors.New("memory: unit of work factory misconfigured")

// Begin starts a lightweight transaction boundary. No isolation is provided but
// the abstraction matches the application ports.
func (f Factory) Begin(ctx context.Context, opts uow.TxOptions) (uow.UnitOfWork, error) {
	if f.ListingsRepo == nil || f.AvailabilityRepo == nil || f.BookingRepo == nil || f.ReviewsRepo == nil {
		return nil, ErrFactoryMisconfigured
	}
	return &Unit{
		listings:     f.ListingsRepo,
		availability: f.AvailabilityRepo,
		booking:      f.BookingRepo,
		pricing:      f.PricingSvc,
		reviews:      f.ReviewsRepo,
	}, nil
}

// Unit is a lightweight uow.UnitOfWork backed by in-memory stores.
type Unit struct {
	listings     domainlistings.ListingRepository
	availability domainavailability.Repository
	booking      domainbooking.Repository
	pricing      domainpricing.Calculator
	reviews      domainreviews.Repository
}

func (u *Unit) Listings() domainlistings.ListingRepository {
	return u.listings
}

func (u *Unit) Availability() domainavailability.Repository {
	return u.availability
}

func (u *Unit) Booking() domainbooking.Repository {
	return u.booking
}

func (u *Unit) Pricing() domainpricing.Calculator {
	return u.pricing
}

func (u *Unit) Reviews() domainreviews.Repository {
	return u.reviews
}

func (u *Unit) Commit(ctx context.Context) error {
	return nil
}

func (u *Unit) Rollback(ctx context.Context) error {
	return nil
}
