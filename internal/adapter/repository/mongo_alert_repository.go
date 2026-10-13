package repository

import (
	"fmt"
	"sync"

	"github.com/shopally-ai/pkg/domain"
	"github.com/shopally-ai/pkg/usecase"

	"github.com/google/uuid"
)

// MockAlertRepository keeps alerts in memory only; nothing survives a restart.
// It satisfies usecase.AlertRepository so the API can run without Mongo during
// development. The durable implementation lands with the alerts feature.
type MockAlertRepository struct {
	alerts sync.Map // map[string]*domain.Alert
}

func NewMockAlertRepository() *MockAlertRepository {
	return &MockAlertRepository{}
}
func (r *MockAlertRepository) CreateAlert(alert *domain.Alert) error {
	alert.ID = uuid.New().String()
	r.alerts.Store(alert.ID, alert)
	return nil
}
func (r *MockAlertRepository) GetAlert(alertID string) (*domain.Alert, error) {
	if value, ok := r.alerts.Load(alertID); ok {
		if alert, ok := value.(*domain.Alert); ok {
			return alert, nil
		}
	}
	return nil, fmt.Errorf("alert %s: %w", alertID, usecase.ErrAlertNotFound)
}
func (r *MockAlertRepository) DeleteAlert(alertID string) error {
	if _, ok := r.alerts.Load(alertID); !ok {
		return fmt.Errorf("alert %s: %w", alertID, usecase.ErrAlertNotFound)
	}
	r.alerts.Delete(alertID)
	return nil
}
