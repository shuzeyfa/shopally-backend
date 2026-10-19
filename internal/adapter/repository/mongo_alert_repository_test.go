package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/shopally-ai/pkg/domain"
	"github.com/shopally-ai/pkg/usecase"
)

func TestMongoAlertRepositoryWithoutMongo(t *testing.T) {
	repo := NewMongoAlertRepository(nil, "", "")
	if err := repo.CreateAlert(context.Background(), &domain.Alert{}); !errors.Is(err, usecase.ErrAlertStoreUnavailable) {
		t.Fatalf("CreateAlert error = %v, want storage unavailable", err)
	}
	if _, err := repo.ListAlerts(context.Background(), "user"); !errors.Is(err, usecase.ErrAlertStoreUnavailable) {
		t.Fatalf("ListAlerts error = %v, want storage unavailable", err)
	}
}

func TestMockAlertRepositoryScopesAlertsByOwner(t *testing.T) {
	repo := NewMockAlertRepository()
	alert := &domain.Alert{UserID: "owner", ProductID: "p1", TargetPrice: 20, Currency: "USD", IsActive: true}
	if err := repo.CreateAlert(context.Background(), alert); err != nil {
		t.Fatal(err)
	}
	if alert.ID == "" {
		t.Fatal("CreateAlert did not assign an ID")
	}
	if _, err := repo.GetAlert(context.Background(), "other", alert.ID); !errors.Is(err, usecase.ErrAlertNotFound) {
		t.Fatalf("foreign alert lookup error = %v, want not found", err)
	}
	if err := repo.DeleteAlert(context.Background(), "other", alert.ID); !errors.Is(err, usecase.ErrAlertNotFound) {
		t.Fatalf("foreign alert delete error = %v, want not found", err)
	}
	if _, err := repo.GetAlert(context.Background(), "owner", alert.ID); err != nil {
		t.Fatalf("owner cannot load alert: %v", err)
	}
}
