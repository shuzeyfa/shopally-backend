package repository

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/shopally-ai/pkg/domain"
	"github.com/shopally-ai/pkg/usecase"
)

type MockAlertRepository struct {
	mu     sync.RWMutex
	alerts map[string]*domain.Alert
}

func NewMockAlertRepository() *MockAlertRepository {
	return &MockAlertRepository{alerts: make(map[string]*domain.Alert)}
}

func (r *MockAlertRepository) CreateAlert(_ context.Context, alert *domain.Alert) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if alert.ID == "" {
		alert.ID = uuid.NewString()
	}
	copy := *alert
	r.alerts[alert.ID] = &copy
	return nil
}

func (r *MockAlertRepository) ListAlerts(_ context.Context, userID string) ([]*domain.Alert, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	alerts := make([]*domain.Alert, 0)
	for _, alert := range r.alerts {
		if alert.UserID == userID {
			copy := *alert
			alerts = append(alerts, &copy)
		}
	}
	return alerts, nil
}

func (r *MockAlertRepository) GetAlert(_ context.Context, userID, alertID string) (*domain.Alert, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	alert, ok := r.alerts[alertID]
	if !ok || alert.UserID != userID {
		return nil, fmt.Errorf("alert %s: %w", alertID, usecase.ErrAlertNotFound)
	}
	copy := *alert
	return &copy, nil
}

func (r *MockAlertRepository) DeleteAlert(_ context.Context, userID, alertID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	alert, ok := r.alerts[alertID]
	if !ok || alert.UserID != userID {
		return fmt.Errorf("alert %s: %w", alertID, usecase.ErrAlertNotFound)
	}
	delete(r.alerts, alertID)
	return nil
}

func (r *MockAlertRepository) ListActiveAlerts(_ context.Context) ([]*domain.Alert, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	alerts := make([]*domain.Alert, 0)
	for _, alert := range r.alerts {
		if alert.IsActive {
			copy := *alert
			alerts = append(alerts, &copy)
		}
	}
	return alerts, nil
}

func (r *MockAlertRepository) MarkAlertTriggered(_ context.Context, alertID string, price float64, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	alert, ok := r.alerts[alertID]
	if !ok || !alert.IsActive {
		return fmt.Errorf("alert %s: %w", alertID, usecase.ErrAlertNotFound)
	}
	alert.IsActive = false
	alert.TriggeredAt = &at
	alert.TriggeredPrice = price
	return nil
}

var _ usecase.AlertRepository = (*MockAlertRepository)(nil)
