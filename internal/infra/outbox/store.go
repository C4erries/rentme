package outbox

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	appoutbox "rentme/internal/app/outbox"
)

const (
	stateNew     = "NEW"
	stateClaimed = "CLAIMED"
	stateSent    = "SENT"
	stateFailed  = "FAILED"
)

type Store struct {
	col *mongo.Collection
}

func NewStore(db *mongo.Database) *Store {
	col := db.Collection("app_outbox")
	idx := mongo.IndexModel{Keys: bson.D{{Key: "state", Value: 1}, {Key: "next_attempt_at", Value: 1}}}
	_, _ = col.Indexes().CreateOne(context.Background(), idx)
	return &Store{col: col}
}

func (s *Store) Add(ctx context.Context, record appoutbox.EventRecord) error {
	doc := bson.M{
		"_id":             record.ID,
		"name":            record.Name,
		"payload":         record.Payload,
		"occurred_at":     record.OccurredAt,
		"aggregate":       record.Aggregate,
		"headers":         record.Headers,
		"state":           stateNew,
		"attempts":        0,
		"next_attempt_at": time.Now().UTC(),
		"created_at":      time.Now().UTC(),
	}
	_, err := s.col.InsertOne(ctx, doc)
	return err
}

func (s *Store) Flush(context.Context) error {
	return nil
}

type EventDocument struct {
	ID          string            `bson:"_id"`
	Name        string            `bson:"name"`
	Payload     []byte            `bson:"payload"`
	OccurredAt  time.Time         `bson:"occurred_at"`
	Aggregate   string            `bson:"aggregate"`
	Headers     map[string]string `bson:"headers"`
	State       string            `bson:"state"`
	Attempts    int               `bson:"attempts"`
	NextAttempt time.Time         `bson:"next_attempt_at"`
	ClaimedBy   string            `bson:"claimed_by"`
	ClaimedAt   time.Time         `bson:"claimed_at"`
	SentAt      time.Time         `bson:"sent_at"`
	LastError   string            `bson:"last_error"`
}

func (s *Store) Claim(ctx context.Context, workerID string) (*EventDocument, error) {
	now := time.Now().UTC()
	filter := bson.M{"state": bson.M{"$in": []string{stateNew, stateFailed}}, "next_attempt_at": bson.M{"$lte": now}}
	update := bson.M{"$set": bson.M{"state": stateClaimed, "claimed_by": workerID, "claimed_at": now}}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var doc EventDocument
	err := s.col.FindOneAndUpdate(ctx, filter, update, opts).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &doc, nil
}

func (s *Store) MarkSent(ctx context.Context, id string) error {
	_, err := s.col.UpdateByID(ctx, id, bson.M{"$set": bson.M{"state": stateSent, "sent_at": time.Now().UTC()}})
	return err
}

func (s *Store) MarkFailed(ctx context.Context, id string, next time.Time, errMsg string) error {
	update := bson.M{
		"$set": bson.M{
			"state":           stateFailed,
			"next_attempt_at": next,
			"last_error":      errMsg,
		},
		"$inc": bson.M{"attempts": 1},
	}
	_, err := s.col.UpdateByID(ctx, id, update)
	return err
}
