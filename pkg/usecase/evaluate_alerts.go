package usecase

import (
	"context"
	"time"

	"github.com/shopally-ai/pkg/domain"
)

type AlertEvaluator struct {
	repo     AlertRepository
	products AlibabaGateway
	now      func() time.Time
}

func NewAlertEvaluator(repo AlertRepository, products AlibabaGateway) *AlertEvaluator {
	return &AlertEvaluator{repo: repo, products: products, now: time.Now}
}

func (e *AlertEvaluator) Evaluate(ctx context.Context) (int, error) {
	alerts, err := e.repo.ListActiveAlerts(ctx)
	if err != nil {
		return 0, err
	}
	ids := make([]string, 0, len(alerts))
	for _, alert := range alerts {
		ids = append(ids, alert.ProductID)
	}
	products, err := e.products.FetchProductsByIDs(ctx, ids)
	if err != nil {
		return 0, err
	}
	byID := make(map[string]*domain.Product, len(products))
	for _, product := range products {
		byID[product.ID] = product
	}
	triggered := 0
	for _, alert := range alerts {
		product := byID[alert.ProductID]
		if product == nil {
			continue
		}
		price := product.Price.ETB
		if alert.Currency == "USD" {
			price = product.Price.USD
		}
		if price <= 0 || price > alert.TargetPrice {
			continue
		}
		if err := e.repo.MarkAlertTriggered(ctx, alert.ID, price, e.now().UTC()); err != nil {
			return triggered, err
		}
		triggered++
	}
	return triggered, nil
}
