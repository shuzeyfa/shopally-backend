package platform

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestConnectionsRecoverIndependently(t *testing.T) {
	server := miniredis.RunT(t)
	s := NewServiceConnections("", server.Host(), server.Port(), "", 0, false)
	t.Cleanup(func() { require.NoError(t, s.Close()) })
	mongoClient, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://127.0.0.1:1"))
	require.NoError(t, err)
	var mongoReady atomic.Bool
	var mongoAttempts atomic.Int32
	s.connectMongo = func(ctx context.Context) (*mongo.Client, error) {
		mongoAttempts.Add(1)
		if !mongoReady.Load() {
			return nil, errNotConnected
		}
		return mongoClient, nil
	}
	redisDial := s.connectRedis
	var redisAttempts atomic.Int32
	s.connectRedis = func(ctx context.Context) (*Client, error) {
		if redisAttempts.Add(1) == 1 {
			return nil, errNotConnected
		}
		return redisDial(ctx)
	}
	var notifications atomic.Int32
	s.StartBackgroundRetry(context.Background(), time.Millisecond, func() {
		notifications.Add(1)
	})
	require.Eventually(t, func() bool { return notifications.Load() == 1 }, 3*time.Second, time.Millisecond)
	require.Nil(t, s.MongoClient(), "Mongo must not prevent Redis recovery")
	require.NoError(t, s.PingRedis(context.Background()))
	require.GreaterOrEqual(t, redisAttempts.Load(), int32(2))
	require.ErrorIs(t, s.PingMongo(context.Background()), errNotConnected)

	mongoReady.Store(true)
	require.Eventually(t, func() bool { return notifications.Load() == 2 }, 3*time.Second, time.Millisecond)
	require.Same(t, mongoClient, s.MongoClient())
	require.GreaterOrEqual(t, mongoAttempts.Load(), int32(2))

	// Retained Redis clients also reconnect after a later outage.
	client := s.RedisClient()
	address := server.Addr()
	server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	require.Error(t, s.PingRedis(ctx))
	cancel()
	require.NoError(t, server.StartAddr(address))
	require.Eventually(t, func() bool { return s.PingRedis(context.Background()) == nil }, 3*time.Second, 10*time.Millisecond)
	require.Same(t, client, s.RedisClient())
}

func TestCloseCancelsInflightConnections(t *testing.T) {
	s := NewServiceConnections("", "", "", "", 0, false)
	started := make(chan struct{}, 2)
	var callbacks atomic.Int32
	s.connectMongo = func(ctx context.Context) (*mongo.Client, error) {
		started <- struct{}{}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	s.connectRedis = func(ctx context.Context) (*Client, error) {
		started <- struct{}{}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	s.StartBackgroundRetry(context.Background(), time.Hour, func() { callbacks.Add(1) })
	t.Cleanup(func() { require.NoError(t, s.Close()) })
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(3 * time.Second):
			t.Fatal("connection attempts did not start")
		}
	}
	done := make(chan error, 1)
	go func() { done <- s.Close() }()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("Close did not cancel connection attempts")
	}
	require.Zero(t, callbacks.Load())
	require.Nil(t, s.MongoClient())
	require.Nil(t, s.RedisClient())
	// Repeated starts after shutdown cannot reopen clients.
	s.StartBackgroundRetry(context.Background(), time.Millisecond, nil)
	require.NoError(t, s.Close())
}

func TestMongoConnectHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client, err := ConnectContext(ctx, "mongodb://127.0.0.1:1")
	require.Nil(t, client)
	require.True(t, errors.Is(err, context.Canceled), "expected cancellation, got %v", err)
}
