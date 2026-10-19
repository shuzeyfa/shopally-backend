package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/shopally-ai/internal/adapter/gateway"
	"github.com/shopally-ai/pkg/usecase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompareHandler_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	uc := usecase.NewCompareProductsUseCase(gateway.NewMockAlibabaGateway(), nil)
	h := NewCompareHandler(uc)

	r := gin.New()
	r.POST("/compare", h.Compare)

	// Case 1: Too few product IDs
	body, _ := json.Marshal(map[string]interface{}{
		"productIds": []string{"MOCK-123"},
	})
	req := httptest.NewRequest(http.MethodPost, "/compare", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	// Case 2: Too many product IDs (> 4)
	body, _ = json.Marshal(map[string]interface{}{
		"productIds": []string{"MOCK-123", "MOCK-124", "MOCK-125", "MOCK-126", "MOCK-127"},
	})
	req = httptest.NewRequest(http.MethodPost, "/compare", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCompareHandler_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	uc := usecase.NewCompareProductsUseCase(gateway.NewMockAlibabaGateway(), nil)
	h := NewCompareHandler(uc)

	r := gin.New()
	r.POST("/compare", h.Compare)

	body, _ := json.Marshal(map[string]interface{}{
		"productIds": []string{"MOCK-123", "MOCK-124"},
	})
	req := httptest.NewRequest(http.MethodPost, "/compare", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp struct {
		Data  usecase.CompareResult `json:"data"`
		Error *APIError             `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Nil(t, resp.Error)
	assert.Equal(t, 2, len(resp.Data.Products))
	assert.NotEmpty(t, resp.Data.Attributes)
	assert.Equal(t, "Price (ETB)", resp.Data.Attributes[0].Label)
	assert.Equal(t, "ETB 4999.00", resp.Data.Attributes[0].Values["MOCK-123"])
}

func TestCompareHandler_ProductNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	uc := usecase.NewCompareProductsUseCase(gateway.NewMockAlibabaGateway(), nil)
	h := NewCompareHandler(uc)

	r := gin.New()
	r.POST("/compare", h.Compare)

	body, _ := json.Marshal(map[string]interface{}{
		"productIds": []string{"MOCK-123", "NONEXISTENT-999"},
	})
	req := httptest.NewRequest(http.MethodPost, "/compare", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}
