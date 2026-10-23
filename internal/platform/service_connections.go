package platform

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
)

var errNotConnected = errors.New("not connected")

const defaultReconnectInterval = 5 * time.Second

// ServiceConnections retries initial connections independently. Once connected,
// clients are retained: both drivers reconnect after later network outages.
type ServiceConnections struct {
	mu       sync.RWMutex
	notifyMu sync.Mutex
	wg       sync.WaitGroup
	cancel   context.CancelFunc
	closed   bool

	mongo *mongo.Client
	redis *Client

	connectMongo func(context.Context) (*mongo.Client, error)
	connectRedis func(context.Context) (*Client, error)
}

// NewServiceConnections does no network I/O so the API can listen immediately.
func NewServiceConnections(mongoURI string, redisHost, redisPort, redisPassword string, redisDB int, redisTLS bool) *ServiceConnections {
	return &ServiceConnections{
		connectMongo: func(ctx context.Context) (*mongo.Client, error) {
			return ConnectContext(ctx, mongoURI)
		},
		connectRedis: func(ctx context.Context) (*Client, error) {
			client := NewRedisClient(redisHost, redisPort, redisPassword, redisDB, redisTLS)
			if err := client.Ping(ctx); err != nil {
				_ = client.Close()
				return nil, err
			}
			return client, nil
		},
	}
}

// StartBackgroundRetry starts at most once. afterConnect is serialized across
// dependencies and runs after each successful connection, allowing repositories
// and cache ports to pick up the new client. It must not call Close.
func (s *ServiceConnections) StartBackgroundRetry(ctx context.Context, interval time.Duration, afterConnect func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.cancel != nil {
		return
	}
	if interval <= 0 {
		interval = defaultReconnectInterval
	}
	ctx, s.cancel = context.WithCancel(ctx)
	s.wg.Add(2)
	go s.retry(ctx, interval, "mongo", s.tryMongo, afterConnect)
	go s.retry(ctx, interval, "redis", s.tryRedis, afterConnect)
}

func (s *ServiceConnections) retry(ctx context.Context, interval time.Duration, name string, connect func(context.Context) error, afterConnect func()) {
	defer s.wg.Done()
	for ctx.Err() == nil {
		if err := connect(ctx); err == nil {
			s.notifyMu.Lock()
			if ctx.Err() == nil && afterConnect != nil {
				afterConnect()
			}
			s.notifyMu.Unlock()
			log.Printf("%s connection ready", name)
			return
		}
		if ctx.Err() != nil {
			return
		}
		// Driver errors may contain connection strings. Keep credentials out of logs.
		log.Printf("%s unavailable; retrying in %s", name, interval)
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (s *ServiceConnections) tryMongo(ctx context.Context) error {
	client, err := s.connectMongo(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if s.closed || ctx.Err() != nil {
		s.mu.Unlock()
		_ = Disconnect(client)
		return context.Canceled
	}
	s.mongo = client
	s.mu.Unlock()
	return nil
}

func (s *ServiceConnections) tryRedis(ctx context.Context) error {
	client, err := s.connectRedis(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if s.closed || ctx.Err() != nil {
		s.mu.Unlock()
		_ = client.Close()
		return context.Canceled
	}
	s.redis = client
	s.mu.Unlock()
	return nil
}

func (s *ServiceConnections) MongoClient() *mongo.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mongo
}

func (s *ServiceConnections) RedisClient() *Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.redis
}

func (s *ServiceConnections) PingMongo(ctx context.Context) error {
	client := s.MongoClient()
	if client == nil {
		return errNotConnected
	}
	return client.Ping(ctx, nil)
}

func (s *ServiceConnections) PingRedis(ctx context.Context) error {
	client := s.RedisClient()
	if client == nil {
		return errNotConnected
	}
	return client.Ping(ctx)
}

// Close cancels and joins retries before releasing clients. No connection or
// callback can be published after it returns.
func (s *ServiceConnections) Close() error {
	s.mu.Lock()
	s.closed = true
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()
	s.wg.Wait()

	s.mu.Lock()
	mongoClient, redisClient := s.mongo, s.redis
	s.mongo, s.redis = nil, nil
	s.mu.Unlock()
	var mongoErr, redisErr error
	if mongoClient != nil {
		mongoErr = Disconnect(mongoClient)
	}
	if redisClient != nil {
		redisErr = redisClient.Close()
	}
	return errors.Join(mongoErr, redisErr)
}
