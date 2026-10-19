package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/shopally-ai/pkg/domain"
	"github.com/shopally-ai/pkg/usecase"
)

func TestMongoSavedRepositoryWithoutClientReturnsUnavailable(t *testing.T) {
	repository := NewMongoSavedRepository(nil, "", "")
	ctx := context.Background()

	if _, err := repository.Save(ctx, "user-1", &domain.Product{ID: "product-1"}); !errors.Is(err, usecase.ErrSavedStoreUnavailable) {
		t.Fatalf("Save() error = %v, want ErrSavedStoreUnavailable", err)
	}
	if _, err := repository.List(ctx, "user-1"); !errors.Is(err, usecase.ErrSavedStoreUnavailable) {
		t.Fatalf("List() error = %v, want ErrSavedStoreUnavailable", err)
	}
	if err := repository.Delete(ctx, "user-1", "product-1"); !errors.Is(err, usecase.ErrSavedStoreUnavailable) {
		t.Fatalf("Delete() error = %v, want ErrSavedStoreUnavailable", err)
	}
}

func TestSavedItemIDIsStableAndUserScoped(t *testing.T) {
	first := savedItemID("user-1", "product-1")
	if first != savedItemID("user-1", "product-1") {
		t.Fatal("saved item ID changed for the same user and product")
	}
	if first == savedItemID("user-2", "product-1") {
		t.Fatal("saved item ID should be scoped to the user")
	}
}
