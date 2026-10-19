package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/shopally-ai/pkg/domain"
)

type compareGatewayStub struct {
	products  []*domain.Product
	requested []string
	callCount int
	err       error
}

func (g *compareGatewayStub) FetchProducts(context.Context, string, map[string]interface{}) ([]*domain.Product, error) {
	return nil, errors.New("unexpected search call")
}

func (g *compareGatewayStub) FetchProductsByIDs(_ context.Context, ids []string) ([]*domain.Product, error) {
	g.callCount++
	g.requested = append([]string(nil), ids...)
	return g.products, g.err
}

type compareCacheStub struct {
	values   map[string]string
	setCount int
}

func (c *compareCacheStub) Get(_ context.Context, key string) (string, bool, error) {
	value, found := c.values[key]
	return value, found, nil
}

func (c *compareCacheStub) Set(_ context.Context, key, value string, _ time.Duration) error {
	if c.values == nil {
		c.values = make(map[string]string)
	}
	c.values[key] = value
	c.setCount++
	return nil
}

func TestCompareProductsUseCase_ValidatesAndDeduplicatesIDs(t *testing.T) {
	gateway := &compareGatewayStub{products: []*domain.Product{
		{ID: "p1", Price: domain.Price{ETB: 20}},
		{ID: "p2", Price: domain.Price{ETB: 10}},
	}}
	uc := NewCompareProductsUseCase(gateway, nil)

	result, err := uc.Compare(context.Background(), []string{" p2 ", "p1", "p2"})
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if len(gateway.requested) != 2 || gateway.requested[0] != "p2" || gateway.requested[1] != "p1" {
		t.Fatalf("requested IDs = %v, want [p2 p1]", gateway.requested)
	}
	if len(result.Products) != 2 || result.Products[0].ID != "p2" || result.Products[1].ID != "p1" {
		t.Fatalf("result product order = %v", result.Products)
	}

	_, err = uc.Compare(context.Background(), []string{"p1", "p1"})
	if !errors.Is(err, ErrInvalidComparisonCount) {
		t.Fatalf("duplicate-only comparison error = %v, want %v", err, ErrInvalidComparisonCount)
	}
	if gateway.callCount != 1 {
		t.Fatalf("gateway called %d times, want 1", gateway.callCount)
	}
}

func TestCompareProductsUseCase_CachesComparison(t *testing.T) {
	products := []*domain.Product{{ID: "p1"}, {ID: "p2"}}
	gateway := &compareGatewayStub{products: products}
	cache := &compareCacheStub{}
	uc := NewCompareProductsUseCase(gateway, cache)

	first, err := uc.Compare(context.Background(), []string{"p1", "p2"})
	if err != nil {
		t.Fatalf("first Compare() error = %v", err)
	}
	second, err := uc.Compare(context.Background(), []string{"p1", "p2"})
	if err != nil {
		t.Fatalf("cached Compare() error = %v", err)
	}
	if gateway.callCount != 1 {
		t.Fatalf("gateway called %d times, want 1", gateway.callCount)
	}
	if cache.setCount != 1 {
		t.Fatalf("cache set called %d times, want 1", cache.setCount)
	}
	firstJSON, _ := json.Marshal(first)
	secondJSON, _ := json.Marshal(second)
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("cached result differs: first=%s second=%s", firstJSON, secondJSON)
	}
}
