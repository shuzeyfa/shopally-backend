package platform

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Connect establishes a new MongoDB client connection
func Connect(uri string) (*mongo.Client, error) {
	return ConnectContext(context.Background(), uri)
}

// ConnectContext bounds connection attempts and releases failed clients.
func ConnectContext(parent context.Context, uri string) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}

	// Ping to check connection
	if err := client.Ping(ctx, nil); err != nil {
		_ = Disconnect(client)
		return nil, err
	}
	return client, nil
}

// Disconnect closes the MongoDB client connection
func Disconnect(client *mongo.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return client.Disconnect(ctx)
}
