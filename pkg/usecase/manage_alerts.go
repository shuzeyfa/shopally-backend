package usecase

import (
	"context"
	"time"

	"github.com/shopally-ai/pkg/domain"
)

type AlertManager struct {
	repo AlertRepository
}

func NewAlertManager(repo AlertRepository) *AlertManager {
	return &AlertManager{
		repo: repo,
	}
}
func (m *AlertManager) CreateAlert(ctx context.Context, alert *domain.Alert) error {
	return m.repo.CreateAlert(ctx, alert)
}

func (m *AlertManager) ListAlerts(ctx context.Context, userID string) ([]*domain.Alert, error) {
	return m.repo.ListAlerts(ctx, userID)
}

func (m *AlertManager) GetAlert(ctx context.Context, userID, alertID string) (*domain.Alert, error) {
	return m.repo.GetAlert(ctx, userID, alertID)
}

func (m *AlertManager) DeleteAlert(ctx context.Context, userID, alertID string) error {
	return m.repo.DeleteAlert(ctx, userID, alertID)
}

func (m *AlertManager) ListActiveAlerts(ctx context.Context) ([]*domain.Alert, error) {
	return m.repo.ListActiveAlerts(ctx)
}

func (m *AlertManager) MarkAlertTriggered(ctx context.Context, alertID string, price float64, at time.Time) error {
	return m.repo.MarkAlertTriggered(ctx, alertID, price, at)
}
