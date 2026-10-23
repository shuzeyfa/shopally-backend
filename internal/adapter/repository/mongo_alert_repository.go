package repository

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/shopally-ai/pkg/domain"
	"github.com/shopally-ai/pkg/usecase"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoAlertRepository struct {
	collection atomic.Pointer[mongo.Collection]
}

func NewMongoAlertRepository(client *mongo.Client, database, collection string) *MongoAlertRepository {
	r := &MongoAlertRepository{}
	r.Rebind(client, database, collection)
	return r
}

// Rebind attaches the repository to a Mongo client after a late connection.
func (r *MongoAlertRepository) Rebind(client *mongo.Client, database, collection string) {
	if client == nil {
		r.collection.Store(nil)
		return
	}
	if database == "" {
		database = "shopally"
	}
	if collection == "" {
		collection = "alerts"
	}
	r.collection.Store(client.Database(database).Collection(collection))
}

func (r *MongoAlertRepository) CreateAlert(ctx context.Context, alert *domain.Alert) error {
	collection := r.collection.Load()
	if collection == nil {
		return usecase.ErrAlertStoreUnavailable
	}
	if alert.ID == "" {
		alert.ID = uuid.NewString()
	}
	_, err := collection.InsertOne(ctx, alert)
	return err
}

func (r *MongoAlertRepository) ListAlerts(ctx context.Context, userID string) ([]*domain.Alert, error) {
	collection := r.collection.Load()
	if collection == nil {
		return nil, usecase.ErrAlertStoreUnavailable
	}
	cursor, err := collection.Find(ctx, bson.M{"user_id": userID}, options.Find().SetSort(bson.D{{Key: "_id", Value: -1}}))
	if err != nil {
		return nil, err
	}
	// Cleanup is best effort; read and decode errors are returned below.
	defer func() { _ = cursor.Close(ctx) }()
	alerts := make([]*domain.Alert, 0)
	for cursor.Next(ctx) {
		var alert domain.Alert
		if err := cursor.Decode(&alert); err != nil {
			return nil, err
		}
		alerts = append(alerts, &alert)
	}
	return alerts, cursor.Err()
}

func (r *MongoAlertRepository) GetAlert(ctx context.Context, userID, alertID string) (*domain.Alert, error) {
	collection := r.collection.Load()
	if collection == nil {
		return nil, usecase.ErrAlertStoreUnavailable
	}
	var alert domain.Alert
	err := collection.FindOne(ctx, bson.M{"_id": alertID, "user_id": userID}).Decode(&alert)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, fmt.Errorf("alert %s: %w", alertID, usecase.ErrAlertNotFound)
	}
	if err != nil {
		return nil, err
	}
	return &alert, nil
}

func (r *MongoAlertRepository) DeleteAlert(ctx context.Context, userID, alertID string) error {
	collection := r.collection.Load()
	if collection == nil {
		return usecase.ErrAlertStoreUnavailable
	}
	result, err := collection.DeleteOne(ctx, bson.M{"_id": alertID, "user_id": userID})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return fmt.Errorf("alert %s: %w", alertID, usecase.ErrAlertNotFound)
	}
	return nil
}

func (r *MongoAlertRepository) ListActiveAlerts(ctx context.Context) ([]*domain.Alert, error) {
	collection := r.collection.Load()
	if collection == nil {
		return nil, usecase.ErrAlertStoreUnavailable
	}
	cursor, err := collection.Find(ctx, bson.M{"is_active": true})
	if err != nil {
		return nil, err
	}
	// Cleanup is best effort; read and decode errors are returned below.
	defer func() { _ = cursor.Close(ctx) }()
	alerts := make([]*domain.Alert, 0)
	for cursor.Next(ctx) {
		var alert domain.Alert
		if err := cursor.Decode(&alert); err != nil {
			return nil, err
		}
		alerts = append(alerts, &alert)
	}
	return alerts, cursor.Err()
}

func (r *MongoAlertRepository) MarkAlertTriggered(ctx context.Context, alertID string, price float64, at time.Time) error {
	collection := r.collection.Load()
	if collection == nil {
		return usecase.ErrAlertStoreUnavailable
	}
	result, err := collection.UpdateOne(ctx, bson.M{"_id": alertID, "is_active": true}, bson.M{"$set": bson.M{
		"is_active": false, "triggered_at": at, "triggered_price": price,
	}})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("alert %s: %w", alertID, usecase.ErrAlertNotFound)
	}
	return nil
}

var _ usecase.AlertRepository = (*MongoAlertRepository)(nil)
