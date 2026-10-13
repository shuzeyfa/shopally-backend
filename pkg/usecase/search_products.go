package usecase

import (
	"context"
	"log"
)

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

// Search runs the search pipeline: parse intent, then fetch using it as filters.
func (uc *SearchProductsUseCase) Search(ctx context.Context, query string) (interface{}, error) {
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
	return map[string]interface{}{"products": products}, nil
}
