package usecase

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/shopally-ai/pkg/domain"
)

type mockAlertRepository struct {
	alerts sync.Map
}

func newMockAlertRepository() *mockAlertRepository {
	return &mockAlertRepository{}
}

func (m *mockAlertRepository) CreateAlert(_ context.Context, alert *domain.Alert) error {
	if alert.ID == "" {
		alert.ID = "test-alert-id"
	}
	m.alerts.Store(alert.ID, alert)
	return nil
}

func (m *mockAlertRepository) ListAlerts(_ context.Context, userID string) ([]*domain.Alert, error) {
	result := make([]*domain.Alert, 0)
	m.alerts.Range(func(_, value any) bool {
		alert := value.(*domain.Alert)
		if alert.UserID == userID {
			result = append(result, alert)
		}
		return true
	})
	return result, nil
}

func (m *mockAlertRepository) GetAlert(_ context.Context, userID, alertID string) (*domain.Alert, error) {
	if value, ok := m.alerts.Load(alertID); ok {
		alert := value.(*domain.Alert)
		if alert.UserID == userID {
			return alert, nil
		}
	}
	return nil, fmt.Errorf("alert with ID %s not found", alertID)
}

func (m *mockAlertRepository) DeleteAlert(_ context.Context, userID, alertID string) error {
	if alert, ok := m.alerts.Load(alertID); !ok || alert.(*domain.Alert).UserID != userID {
		return fmt.Errorf("alert with ID %s not found", alertID)
	}
	m.alerts.Delete(alertID)
	return nil
}

func (m *mockAlertRepository) ListActiveAlerts(_ context.Context) ([]*domain.Alert, error) {
	return nil, nil
}
func (m *mockAlertRepository) MarkAlertTriggered(_ context.Context, _ string, _ float64, _ time.Time) error {
	return nil
}

func TestAlertManager_UseCases(t *testing.T) {
	mockRepo := newMockAlertRepository()
	alertManager := NewAlertManager(mockRepo)

	sampleAlert := &domain.Alert{
		UserID:      "user-123",
		ProductID:   "prod-abc",
		TargetPrice: 500.00,
	}

	var createdAlertID string
	t.Run("CreateAlert_Success", func(t *testing.T) {
		err := alertManager.CreateAlert(context.Background(), sampleAlert)
		if err != nil {
			t.Fatalf("CreateAlert failed: %v", err)
		}
		if sampleAlert.ID == "" {
			t.Fatal("Alert ID was not populated by the use case")
		}
		createdAlertID = sampleAlert.ID
	})
	t.Run("GetAlert_Success", func(t *testing.T) {
		retrievedAlert, err := alertManager.GetAlert(context.Background(), "user-123", createdAlertID)
		if err != nil {
			t.Fatalf("GetAlert failed: %v", err)
		}
		if retrievedAlert.ID != createdAlertID {
			t.Errorf("Retrieved alert ID mismatch: got %s, want %s", retrievedAlert.ID, createdAlertID)
		}
	})

	t.Run("GetAlert_NotFound", func(t *testing.T) {
		_, err := alertManager.GetAlert(context.Background(), "user-123", "non-existent-id")
		if err == nil {
			t.Fatal("GetAlert for non-existent ID did not return an error")
		}
	})

	t.Run("DeleteAlert_Success", func(t *testing.T) {
		err := alertManager.DeleteAlert(context.Background(), "user-123", createdAlertID)
		if err != nil {
			t.Fatalf("DeleteAlert failed: %v", err)
		}

		_, err = alertManager.GetAlert(context.Background(), "user-123", createdAlertID)
		if err == nil {
			t.Fatal("Alert was not deleted as expected")
		}
	})

	t.Run("DeleteAlert_NotFound", func(t *testing.T) {
		err := alertManager.DeleteAlert(context.Background(), "user-123", "non-existent-id")
		if err == nil {
			t.Fatal("DeleteAlert for non-existent ID did not return an error")
		}
	})
}
