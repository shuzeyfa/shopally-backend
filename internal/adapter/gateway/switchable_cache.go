package gateway

import (
	"context"
	"sync"
	"time"

	"github.com/shopally-ai/pkg/usecase"
)

// SwitchableCache implements ICachePort and allows swapping the backing store
// when Redis connects after startup without restarting the API.
type SwitchableCache struct {
	mu   sync.RWMutex
	impl usecase.ICachePort
}

var _ usecase.ICachePort = (*SwitchableCache)(nil)

func NewSwitchableCache(initial usecase.ICachePort) *SwitchableCache {
	return &SwitchableCache{impl: initial}
}

func (s *SwitchableCache) Replace(impl usecase.ICachePort) {
	s.mu.Lock()
	s.impl = impl
	s.mu.Unlock()
}

func (s *SwitchableCache) Get(ctx context.Context, key string) (string, bool, error) {
	s.mu.RLock()
	impl := s.impl
	s.mu.RUnlock()
	if impl == nil {
		return "", false, nil
	}
	return impl.Get(ctx, key)
}

func (s *SwitchableCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	s.mu.RLock()
	impl := s.impl
	s.mu.RUnlock()
	if impl == nil {
		return nil
	}
	return impl.Set(ctx, key, value, ttl)
}
