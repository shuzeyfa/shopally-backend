package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/shopally-ai/pkg/domain"
)

type evaluatorRepo struct {
	alerts []*domain.Alert
	prices map[string]float64
}

func (r *evaluatorRepo) CreateAlert(context.Context, *domain.Alert) error { return nil }
func (r *evaluatorRepo) ListAlerts(context.Context, string) ([]*domain.Alert, error) {
	return nil, nil
}
func (r *evaluatorRepo) GetAlert(context.Context, string, string) (*domain.Alert, error) {
	return nil, nil
}
func (r *evaluatorRepo) DeleteAlert(context.Context, string, string) error { return nil }
func (r *evaluatorRepo) ListActiveAlerts(context.Context) ([]*domain.Alert, error) {
	return r.alerts, nil
}
func (r *evaluatorRepo) MarkAlertTriggered(_ context.Context, id string, price float64, at time.Time) error {
	r.prices[id] = price
	return nil
}

type evaluatorProducts struct{ items []*domain.Product }

func (g evaluatorProducts) FetchProducts(context.Context, string, map[string]interface{}) ([]*domain.Product, error) {
	return g.items, nil
}
func (g evaluatorProducts) FetchProductsByIDs(context.Context, []string) ([]*domain.Product, error) {
	return g.items, nil
}

func TestAlertEvaluatorTriggersMatchingCurrencyOnlyWhenTargetReached(t *testing.T) {
	repo := &evaluatorRepo{
		alerts: []*domain.Alert{
			{ID: "etb-hit", ProductID: "phone", TargetPrice: 5000, Currency: "ETB", IsActive: true},
			{ID: "usd-miss", ProductID: "phone", TargetPrice: 40, Currency: "USD", IsActive: true},
		},
		prices: make(map[string]float64),
	}
	evaluator := NewAlertEvaluator(repo, evaluatorProducts{items: []*domain.Product{{ID: "phone", Price: domain.Price{ETB: 4999, USD: 45}}}})

	count, err := evaluator.Evaluate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || repo.prices["etb-hit"] != 4999 {
		t.Fatalf("triggered=%d prices=%v, want one ETB trigger at 4999", count, repo.prices)
	}
	if _, triggered := repo.prices["usd-miss"]; triggered {
		t.Fatal("USD alert triggered despite price being above target")
	}
}
