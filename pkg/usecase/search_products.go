package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/shopally-ai/pkg/domain"
)

// SearchResult is the typed payload returned by the product search use case.
type SearchResult struct {
	Products []*domain.Product `json:"products"`
	Total    int               `json:"total"`
}

// SearchProductsUseCase contains the business logic for searching products.
// It orchestrates calls to external gateways (LLM, Alibaba, Cache).
type SearchProductsUseCase struct {
	alibabaGateway AlibabaGateway
	llmGateway     LLMGateway
	cache          ICachePort
}

// NewSearchProductsUseCase creates a new SearchProductsUseCase.
// cache may be nil, in which case lookups are served straight from the gateways.
func NewSearchProductsUseCase(ag AlibabaGateway, lg LLMGateway, cache ICachePort) *SearchProductsUseCase {
	return &SearchProductsUseCase{
		alibabaGateway: ag,
		llmGateway:     lg,
		cache:          cache,
	}
}

// Search runs the search pipeline: checks the cache first, parses intent with LLM,
// fetches matching products from Alibaba, and populates the cache.
func (uc *SearchProductsUseCase) Search(ctx context.Context, query string) (*SearchResult, error) {
	normQuery := strings.ToLower(strings.TrimSpace(query))
	cacheKey := fmt.Sprintf("search:%s", normQuery)

	// Check cache if configured
	if uc.cache != nil {
		if cached, found, err := uc.cache.Get(ctx, cacheKey); err == nil && found && cached != "" {
			var cachedRes SearchResult
			if err := json.Unmarshal([]byte(cached), &cachedRes); err == nil {
				return &cachedRes, nil
			}
			log.Printf("search: cache unmarshal failed for key=%s: %v", cacheKey, err)
		} else if err != nil {
			log.Printf("search: cache read error for key=%s: %v", cacheKey, err)
		}
	}

	// Parse user query intent using LLM gateway
	intent, err := uc.llmGateway.ParseIntent(ctx, query)
	if err != nil {
		// Intent parsing is an enhancement, not a hard requirement: fall back
		// to unfiltered results rather than failing the search. Logged so the
		// degradation is visible instead of silent.
		log.Printf("search: intent parsing failed, continuing unfiltered: q=%q error=%v", query, err)
		intent = map[string]interface{}{}
	}

	products, err := uc.alibabaGateway.FetchProducts(ctx, query, intent)
	if err != nil {
		return nil, err
	}

	res := &SearchResult{
		Products: products,
		Total:    len(products),
	}

	// Store in cache for 10 minutes
	if uc.cache != nil {
		if bytes, err := json.Marshal(res); err == nil {
			if err := uc.cache.Set(ctx, cacheKey, string(bytes), 10*time.Minute); err != nil {
				log.Printf("search: cache write error for key=%s: %v", cacheKey, err)
			}
		}
	}

	return res, nil
}
