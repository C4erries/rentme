package mongo

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"rentme/internal/app/uow"
	domainavailability "rentme/internal/domain/availability"
	domainbooking "rentme/internal/domain/booking"
	domainlistings "rentme/internal/domain/listings"
	domainpricing "rentme/internal/domain/pricing"
	domainreviews "rentme/internal/domain/reviews"
)

// Factory wires Mongo transactions into the generic UnitOfWork interface.
type Factory struct {
	DB *mongo.Database

	ListingsRepo     domainlistings.ListingRepository
	AvailabilityRepo domainavailability.Repository
	BookingRepo      domainbooking.Repository
	PricingSvc       domainpricing.Calculator
	ReviewsRepo      domainreviews.Repository
}

var ErrUnitOfWorkNotConfigured = errors.New("mongo: unit of work factory missing database")

// Begin starts a MongoDB session/transaction.
func (f Factory) Begin(ctx context.Context, opts uow.TxOptions) (uow.UnitOfWork, error) {
	if f.DB == nil {
		return nil, ErrUnitOfWorkNotConfigured
	}
	session, err := f.DB.Client().StartSession()
	if err != nil {
		return nil, err
	}
	txnOpts := options.Transaction().SetReadConcern(f.DB.ReadConcern()).SetWriteConcern(f.DB.WriteConcern())
	if opts.ReadOnly {
		txnOpts = txnOpts.SetReadConcern(f.DB.ReadConcern())
	}
	if err := session.StartTransaction(txnOpts); err != nil {
		session.EndSession(ctx)
		return nil, err
	}
	return &Unit{
		db:           f.DB,
		session:      session,
		listings:     f.ListingsRepo,
		availability: f.AvailabilityRepo,
		booking:      f.BookingRepo,
		pricing:      f.PricingSvc,
		reviews:      f.ReviewsRepo,
	}, nil
}

type Unit struct {
	db      *mongo.Database
	session mongo.Session

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
	defer u.session.EndSession(ctx)
	if err := u.session.CommitTransaction(ctx); err != nil {
		return err
	}
	return nil
}

func (u *Unit) Rollback(ctx context.Context) error {
	defer u.session.EndSession(ctx)
	return u.session.AbortTransaction(ctx)
}

// InjectContext ensures Mongo session is available in context for downstream repos.
func (u *Unit) InjectContext(ctx context.Context) context.Context {
	return mongo.NewSessionContext(ctx, u.session)
}
