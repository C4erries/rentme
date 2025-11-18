package inbox

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Store struct {
	col      *mongo.Collection
	consumer string
}

func NewStore(db *mongo.Database, consumer string) *Store {
	col := db.Collection("app_inbox")
	_, _ = col.Indexes().CreateOne(context.Background(), mongo.IndexModel{Keys: bson.D{{Key: "event_id", Value: 1}, {Key: "consumer", Value: 1}}, Options: options.Index().SetUnique(true)})
	return &Store{col: col, consumer: consumer}
}

func (s *Store) Seen(ctx context.Context, eventID string) (bool, error) {
	doc := bson.M{"event_id": eventID, "consumer": s.consumer, "received_at": time.Now().UTC()}
	_, err := s.col.InsertOne(ctx, doc)
	if err == nil {
		return false, nil
	}
	if mongo.IsDuplicateKeyError(err) {
		return true, nil
	}
	return false, err
}
