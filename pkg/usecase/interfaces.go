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
	CreateAlert(ctx context.Context, alert *domain.Alert) error
	ListAlerts(ctx context.Context, userID string) ([]*domain.Alert, error)
	GetAlert(ctx context.Context, userID, alertID string) (*domain.Alert, error)
	DeleteAlert(ctx context.Context, userID, alertID string) error
	ListActiveAlerts(ctx context.Context) ([]*domain.Alert, error)
	MarkAlertTriggered(ctx context.Context, alertID string, price float64, at time.Time) error
}

type SavedItemsRepository interface {
	Save(ctx context.Context, userID string, product *domain.Product) (*domain.SavedItem, error)
	List(ctx context.Context, userID string) ([]*domain.SavedItem, error)
	Delete(ctx context.Context, userID, productID string) error
}
