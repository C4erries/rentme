package uow

import (
	"context"

	domainavailability "rentme/internal/domain/availability"
	domainbooking "rentme/internal/domain/booking"
	domainlistings "rentme/internal/domain/listings"
	domainpricing "rentme/internal/domain/pricing"
	domainreviews "rentme/internal/domain/reviews"
)

// UnitOfWork coordinates repositories inside a transaction boundary.
type UnitOfWork interface {
	Listings() domainlistings.ListingRepository
	Availability() domainavailability.Repository
	Booking() domainbooking.Repository
	Pricing() domainpricing.Calculator
	Reviews() domainreviews.Repository

	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// UoWFactory starts unit of work instances.
type UoWFactory interface {
	Begin(ctx context.Context, opts TxOptions) (UnitOfWork, error)
}

// TxOptions configure transaction boundaries.
type TxOptions struct {
	ReadOnly bool
}
