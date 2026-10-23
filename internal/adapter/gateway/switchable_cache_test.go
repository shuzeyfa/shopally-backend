package gateway

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestSwitchableCacheLateConnection(t *testing.T) {
	ctx := context.Background()
	cache := NewSwitchableCache(nil)
	_, found, err := cache.Get(ctx, "query")
	require.NoError(t, err)
	require.False(t, found)
	require.NoError(t, cache.Set(ctx, "query", "uncached", time.Minute))

	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	backing := NewRedisCache(client, "test:")
	cache.Replace(backing)
	require.NoError(t, cache.Set(ctx, "query", "products", time.Minute))
	value, found, err := cache.Get(ctx, "query")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "products", value)
	require.Equal(t, time.Minute, server.TTL("test:query"))

	// Requests may be in flight when a background connection is attached.
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			cache.Replace(nil)
			cache.Replace(backing)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_, _, _ = cache.Get(ctx, "query")
			_ = cache.Set(ctx, "another", "value", time.Minute)
		}
	}()
	wg.Wait()
}
