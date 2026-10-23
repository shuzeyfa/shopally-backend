package repository

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/shopally-ai/pkg/domain"
	"github.com/shopally-ai/pkg/usecase"
)

func TestRepositoriesAcceptLateClientDuringRequests(t *testing.T) {
	// Cancelled requests avoid a live database requirement while exercising
	// publication of the collection during concurrent operations.
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://127.0.0.1:1"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Disconnect(context.Background())) })
	alerts := NewMongoAlertRepository(nil, "", "")
	saved := NewMongoSavedRepository(nil, "", "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			alerts.Rebind(nil, "", "")
			saved.Rebind(nil, "", "")
			alerts.Rebind(client, "", "")
			saved.Rebind(client, "", "")
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_ = alerts.CreateAlert(ctx, &domain.Alert{ID: "alert"})
			_, _ = alerts.ListAlerts(ctx, "user")
			_, _ = alerts.GetAlert(ctx, "user", "alert")
			_ = alerts.DeleteAlert(ctx, "user", "alert")
			_, _ = saved.Save(ctx, "user", &domain.Product{ID: "product"})
			_, _ = saved.List(ctx, "user")
			_ = saved.Delete(ctx, "user", "product")
		}
	}()
	wg.Wait()
	_, err = alerts.ListAlerts(ctx, "user")
	require.Error(t, err)
	require.NotErrorIs(t, err, usecase.ErrAlertStoreUnavailable)
	_, err = saved.List(ctx, "user")
	require.Error(t, err)
	require.NotErrorIs(t, err, usecase.ErrSavedStoreUnavailable)
}
