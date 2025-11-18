package mongo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"rentme/internal/app/middleware"
)

type IdempotencyStore struct {
	col *mongo.Collection
}

func NewIdempotencyStore(db *mongo.Database) *IdempotencyStore {
	col := db.Collection("app_idempotency")
	ttl := time.Hour * 24 * 7
	idx := mongo.IndexModel{
		Keys:    bson.D{{Key: "created_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(int32(ttl.Seconds())),
	}
	_, _ = col.Indexes().CreateOne(context.Background(), mongo.IndexModel{Keys: bson.D{{Key: "key", Value: 1}}, Options: options.Index().SetUnique(true)})
	_, _ = col.Indexes().CreateOne(context.Background(), idx)
	return &IdempotencyStore{col: col}
}

func (s *IdempotencyStore) Get(ctx context.Context, key string) (middleware.IdempotencyRecord, bool, error) {
	var doc idempotencyDocument
	if err := s.col.FindOne(ctx, bson.M{"key": key}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return middleware.IdempotencyRecord{}, false, nil
		}
		return middleware.IdempotencyRecord{}, false, err
	}
	return doc.toRecord(), true, nil
}

func (s *IdempotencyStore) Save(ctx context.Context, rec middleware.IdempotencyRecord) error {
	doc := idempotencyDocument{
		ID:         rec.Key,
		Key:        rec.Key,
		Payload:    rec.Payload,
		Error:      rec.Error,
		OccurredAt: rec.OccurredAt,
		CreatedAt:  time.Now().UTC(),
	}
	_, err := s.col.UpdateByID(ctx, doc.ID, bson.M{"$set": doc}, options.Update().SetUpsert(true))
	return err
}

type idempotencyDocument struct {
	ID         string    `bson:"_id"`
	Key        string    `bson:"key"`
	Payload    []byte    `bson:"payload"`
	Error      string    `bson:"error"`
	OccurredAt time.Time `bson:"occurred_at"`
	CreatedAt  time.Time `bson:"created_at"`
}

func (d idempotencyDocument) toRecord() middleware.IdempotencyRecord {
	return middleware.IdempotencyRecord{Key: d.Key, Payload: d.Payload, Error: d.Error, OccurredAt: d.OccurredAt}
}
