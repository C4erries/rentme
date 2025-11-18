package mongo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Client struct {
	DB *mongo.Database
}

func New(uri, database string) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	opts := options.Client().ApplyURI(uri).SetRetryWrites(true)
	m, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &Client{DB: m.Database(database)}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	return c.DB.Client().Ping(ctx, nil)
}
