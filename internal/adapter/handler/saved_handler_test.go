package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopally-ai/pkg/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type savedItemsRepositoryStub struct {
	userID  string
	product *domain.Product
	item    *domain.SavedItem
	items   []*domain.SavedItem
	deleted string
}

func (r *savedItemsRepositoryStub) Save(_ context.Context, userID string, product *domain.Product) (*domain.SavedItem, error) {
	r.userID = userID
	r.product = product
	r.item = &domain.SavedItem{Product: product, SavedAt: time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)}
	return r.item, nil
}

func (r *savedItemsRepositoryStub) List(_ context.Context, userID string) ([]*domain.SavedItem, error) {
	r.userID = userID
	return r.items, nil
}

func (r *savedItemsRepositoryStub) Delete(_ context.Context, userID, productID string) error {
	r.userID = userID
	r.deleted = productID
	return nil
}

func TestSavedHandler_SavesForAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &savedItemsRepositoryStub{}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(userIDKey, "verified-user")
		c.Next()
	})
	router.POST("/saved", NewSavedHandler(repo).Save)

	body := []byte(`{"userId":"spoofed-user","product":{"id":"product-1","title":"Phone"}}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/saved", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusCreated, recorder.Code)
	assert.Equal(t, "verified-user", repo.userID)
	assert.Equal(t, "product-1", repo.product.ID)
}

func TestSavedHandler_ListsAndDeletesWithinAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &savedItemsRepositoryStub{items: []*domain.SavedItem{{Product: &domain.Product{ID: "product-1"}}}}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(userIDKey, "verified-user")
		c.Next()
	})
	handler := NewSavedHandler(repo)
	router.GET("/saved", handler.List)
	router.DELETE("/saved/:productId", handler.Delete)

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, httptest.NewRequest(http.MethodGet, "/saved", nil))
	assert.Equal(t, http.StatusOK, listRecorder.Code)
	assert.Equal(t, "verified-user", repo.userID)
	var response struct {
		Data []*domain.SavedItem `json:"data"`
	}
	require.NoError(t, json.Unmarshal(listRecorder.Body.Bytes(), &response))
	assert.Len(t, response.Data, 1)

	deleteRecorder := httptest.NewRecorder()
	router.ServeHTTP(deleteRecorder, httptest.NewRequest(http.MethodDelete, "/saved/product-1", nil))
	assert.Equal(t, http.StatusOK, deleteRecorder.Code)
	assert.Equal(t, "verified-user", repo.userID)
	assert.Equal(t, "product-1", repo.deleted)
}

func TestSavedHandler_RequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/saved", NewSavedHandler(&savedItemsRepositoryStub{}).List)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/saved", nil))
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}
