package usecase

import (
	"context"
	"time"

	"github.com/shopally-ai/pkg/domain"
)

// AlibabaGateway defines the contract for fetching products from an external source.
type AlibabaGateway interface {
	FetchProducts(ctx context.Context, query string, filters map[string]interface{}) ([]*domain.Product, error)
	FetchProductsByIDs(ctx context.Context, ids []string) ([]*domain.Product, error)
}

// LLMGateway defines the contract for a Large Language Model service
// to parse user intent from a search query.
type LLMGateway interface {
	ParseIntent(ctx context.Context, query string) (map[string]interface{}, error)
}

// ICachePort defines the contract for a caching service.
//
// This is the only cache abstraction; RedisCache in the gateway package
// implements it. Get's second return distinguishes a cache miss from an
// error, which a plain (value, error) pair cannot express.
type ICachePort interface {
	// Get returns the value, whether it was found, and any error.
	Get(ctx context.Context, key string) (string, bool, error)
	// Set stores the value with a TTL; use 0 for no expiration.
	Set(ctx context.Context, key, val string, ttl time.Duration) error
}

type IFXClient interface {
	GetRate(ctx context.Context, from, to string) (float64, error)
}

type AlertRepository interface {
	CreateAlert(alert *domain.Alert) error
	GetAlert(alertID string) (*domain.Alert, error)
	DeleteAlert(alertID string) error
}
