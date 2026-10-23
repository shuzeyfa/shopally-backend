package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync/atomic"
	"time"

	"github.com/shopally-ai/pkg/domain"
	"github.com/shopally-ai/pkg/usecase"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoSavedRepository struct {
	collection atomic.Pointer[mongo.Collection]
}

func NewMongoSavedRepository(client *mongo.Client, database, collection string) *MongoSavedRepository {
	r := &MongoSavedRepository{}
	r.Rebind(client, database, collection)
	return r
}

// Rebind attaches the repository to a Mongo client after a late connection.
func (r *MongoSavedRepository) Rebind(client *mongo.Client, database, collection string) {
	if client == nil {
		r.collection.Store(nil)
		return
	}
	if database == "" {
		database = "shopally"
	}
	if collection == "" {
		collection = "saved_items"
	}
	r.collection.Store(client.Database(database).Collection(collection))
}

func (r *MongoSavedRepository) Save(ctx context.Context, userID string, product *domain.Product) (*domain.SavedItem, error) {
	collection := r.collection.Load()
	if collection == nil {
		return nil, usecase.ErrSavedStoreUnavailable
	}
	if product == nil || product.ID == "" {
		return nil, errors.New("product id is required")
	}

	now := time.Now().UTC()
	itemID := savedItemID(userID, product.ID)
	filter := bson.M{"_id": itemID}
	update := bson.M{
		"$set":         bson.M{"product": product},
		"$setOnInsert": bson.M{"user_id": userID, "saved_at": now},
	}
	options := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)

	var item domain.SavedItem
	if err := collection.FindOneAndUpdate(ctx, filter, update, options).Decode(&item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *MongoSavedRepository) List(ctx context.Context, userID string) ([]*domain.SavedItem, error) {
	collection := r.collection.Load()
	if collection == nil {
		return nil, usecase.ErrSavedStoreUnavailable
	}

	cursor, err := collection.Find(ctx, bson.M{"user_id": userID}, options.Find().SetSort(bson.D{{Key: "saved_at", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	items := make([]*domain.SavedItem, 0)
	for cursor.Next(ctx) {
		var item domain.SavedItem
		if err := cursor.Decode(&item); err != nil {
			return nil, err
		}
		items = append(items, &item)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *MongoSavedRepository) Delete(ctx context.Context, userID, productID string) error {
	collection := r.collection.Load()
	if collection == nil {
		return usecase.ErrSavedStoreUnavailable
	}
	_, err := collection.DeleteOne(ctx, bson.M{
		"_id":     savedItemID(userID, productID),
		"user_id": userID,
	})
	return err
}

func savedItemID(userID, productID string) string {
	digest := sha256.Sum256([]byte(userID + "\x00" + productID))
	return hex.EncodeToString(digest[:])
}

var _ usecase.SavedItemsRepository = (*MongoSavedRepository)(nil)
