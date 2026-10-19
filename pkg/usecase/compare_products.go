package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/shopally-ai/pkg/domain"
)

// AttributeComparison represents a single comparison row across multiple products.
type AttributeComparison struct {
	Key    string            `json:"key"`
	Label  string            `json:"label"`
	Values map[string]string `json:"values"` // productID -> display value
}

// CompareResult is the normalized attribute matrix returned by CompareProductsUseCase.
type CompareResult struct {
	Products   []*domain.Product      `json:"products"`
	Attributes []*AttributeComparison `json:"attributes"`
}

// CompareProductsUseCase orchestrates product comparison across 2 to 4 items.
type CompareProductsUseCase struct {
	alibabaGateway AlibabaGateway
	cache          ICachePort
}

// NewCompareProductsUseCase constructs a new CompareProductsUseCase.
func NewCompareProductsUseCase(ag AlibabaGateway, cache ICachePort) *CompareProductsUseCase {
	return &CompareProductsUseCase{
		alibabaGateway: ag,
		cache:          cache,
	}
}

// Compare validates product IDs, fetches catalog products, and builds a normalized attribute matrix.
func (uc *CompareProductsUseCase) Compare(ctx context.Context, productIDs []string) (*CompareResult, error) {
	// Deduplicate IDs
	seen := make(map[string]bool)
	uniqueIDs := make([]string, 0, len(productIDs))
	for _, id := range productIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" && !seen[trimmed] {
			seen[trimmed] = true
			uniqueIDs = append(uniqueIDs, trimmed)
		}
	}

	if len(uniqueIDs) < 2 || len(uniqueIDs) > 4 {
		return nil, ErrInvalidComparisonCount
	}
	idsJSON, _ := json.Marshal(uniqueIDs)
	keyHash := sha256.Sum256(idsJSON)
	cacheKey := "compare:" + hex.EncodeToString(keyHash[:])
	if uc.cache != nil {
		if cached, found, err := uc.cache.Get(ctx, cacheKey); err == nil && found && cached != "" {
			var result CompareResult
			if err := json.Unmarshal([]byte(cached), &result); err == nil {
				return &result, nil
			}
			log.Printf("compare: cache decode failed for key=%s: %v", cacheKey, err)
		} else if err != nil {
			log.Printf("compare: cache read failed for key=%s: %v", cacheKey, err)
		}
	}

	catalog, err := uc.alibabaGateway.FetchProductsByIDs(ctx, uniqueIDs)
	if err != nil {
		return nil, err
	}

	catalogByID := make(map[string]*domain.Product)
	for _, p := range catalog {
		catalogByID[p.ID] = p
	}

	products := make([]*domain.Product, 0, len(uniqueIDs))
	for _, id := range uniqueIDs {
		p, found := catalogByID[id]
		if !found {
			return nil, fmt.Errorf("%w: %s", ErrProductNotFound, id)
		}
		products = append(products, p)
	}

	// Build normalized comparison matrix
	priceETB := &AttributeComparison{Key: "price_etb", Label: "Price (ETB)", Values: make(map[string]string)}
	priceUSD := &AttributeComparison{Key: "price_usd", Label: "Price (USD)", Values: make(map[string]string)}
	aiMatch := &AttributeComparison{Key: "ai_match", Label: "AI Match Score", Values: make(map[string]string)}
	rating := &AttributeComparison{Key: "rating", Label: "Customer Rating", Values: make(map[string]string)}
	seller := &AttributeComparison{Key: "seller_score", Label: "Seller Reputation", Values: make(map[string]string)}
	delivery := &AttributeComparison{Key: "delivery", Label: "Delivery Estimate", Values: make(map[string]string)}
	highlights := &AttributeComparison{Key: "highlights", Label: "Key Highlights", Values: make(map[string]string)}

	for _, p := range products {
		priceETB.Values[p.ID] = fmt.Sprintf("ETB %.2f", p.Price.ETB)
		priceUSD.Values[p.ID] = fmt.Sprintf("$%.2f", p.Price.USD)
		aiMatch.Values[p.ID] = fmt.Sprintf("%d%%", p.AIMatchPercentage)
		rating.Values[p.ID] = fmt.Sprintf("%.1f / 5.0", p.ProductRating)
		seller.Values[p.ID] = fmt.Sprintf("%d%%", p.SellerScore)
		delivery.Values[p.ID] = p.DeliveryEstimate
		highlights.Values[p.ID] = strings.Join(p.SummaryBullets, "; ")
	}

	attributes := []*AttributeComparison{
		priceETB,
		priceUSD,
		aiMatch,
		rating,
		seller,
		delivery,
		highlights,
	}

	result := &CompareResult{
		Products:   products,
		Attributes: attributes,
	}
	if uc.cache != nil {
		if encoded, err := json.Marshal(result); err == nil {
			if err := uc.cache.Set(ctx, cacheKey, string(encoded), 10*time.Minute); err != nil {
				log.Printf("compare: cache write failed for key=%s: %v", cacheKey, err)
			}
		}
	}
	return result, nil
}
