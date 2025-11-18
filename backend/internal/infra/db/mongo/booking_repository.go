package mongo

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	domainbooking "rentme/internal/domain/booking"
	"rentme/internal/domain/listings"
	domainpricing "rentme/internal/domain/pricing"
	domainrange "rentme/internal/domain/shared/daterange"
)

var ErrConcurrentUpdate = errors.New("mongo: concurrent update detected")

type BookingRepository struct {
	col *mongo.Collection
}

func NewBookingRepository(db *mongo.Database) *BookingRepository {
	return &BookingRepository{col: db.Collection("agg_booking")}
}

func (r *BookingRepository) ByID(ctx context.Context, id domainbooking.BookingID) (*domainbooking.Booking, error) {
	var doc bookingDocument
	if err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&doc); err != nil {
		return nil, err
	}
	return doc.toAggregate()
}

func (r *BookingRepository) Save(ctx context.Context, b *domainbooking.Booking) error {
	doc := newBookingDocument(b)
	filter := bson.M{"_id": doc.ID, "version": b.Version}
	doc.Version = b.Version + 1
	update := bson.M{"$set": doc}
	opts := options.Update().SetUpsert(true)
	res, err := r.col.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrConcurrentUpdate
		}
		return err
	}
	if res.MatchedCount == 0 && res.UpsertedCount == 0 {
		return ErrConcurrentUpdate
	}
	b.Version = doc.Version
	return nil
}

type bookingDocument struct {
	ID          string                                   `bson:"_id"`
	ListingID   string                                   `bson:"listing_id"`
	GuestID     string                                   `bson:"guest_id"`
	Range       rangeDocument                            `bson:"range"`
	Guests      int                                      `bson:"guests"`
	Price       domainpricing.PriceBreakdown             `bson:"price"`
	State       string                                   `bson:"state"`
	PaymentHold string                                   `bson:"payment_hold"`
	Policy      domainbooking.CancellationPolicySnapshot `bson:"policy"`
	CreatedAt   int64                                    `bson:"created_at"`
	UpdatedAt   int64                                    `bson:"updated_at"`
	Version     int64                                    `bson:"version"`
}

func newBookingDocument(b *domainbooking.Booking) bookingDocument {
	return bookingDocument{
		ID:          string(b.ID),
		ListingID:   string(b.ListingID),
		GuestID:     b.GuestID,
		Range:       rangeDocument{CheckIn: b.Range.CheckIn.UnixMilli(), CheckOut: b.Range.CheckOut.UnixMilli()},
		Guests:      b.Guests,
		Price:       b.Price,
		State:       string(b.State),
		PaymentHold: b.PaymentHold,
		Policy:      b.Policy,
		CreatedAt:   b.CreatedAt.UnixMilli(),
		UpdatedAt:   b.UpdatedAt.UnixMilli(),
		Version:     b.Version,
	}
}

func (d bookingDocument) toAggregate() (*domainbooking.Booking, error) {
	dr := domainrange.DateRange{CheckIn: timestampToTime(d.Range.CheckIn), CheckOut: timestampToTime(d.Range.CheckOut)}
	agg := &domainbooking.Booking{
		ID:          domainbooking.BookingID(d.ID),
		ListingID:   listings.ListingID(d.ListingID),
		GuestID:     d.GuestID,
		Range:       dr,
		Guests:      d.Guests,
		Price:       d.Price,
		State:       domainbooking.BookingState(d.State),
		PaymentHold: d.PaymentHold,
		Policy:      d.Policy,
		CreatedAt:   timestampToTime(d.CreatedAt),
		UpdatedAt:   timestampToTime(d.UpdatedAt),
		Version:     d.Version,
	}
	return agg, nil
}

type rangeDocument struct {
	CheckIn  int64 `bson:"check_in"`
	CheckOut int64 `bson:"check_out"`
}

func timestampToTime(ms int64) time.Time {
	return time.UnixMilli(ms).UTC()
}
