package handler

import (
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

func TestSearchHandler_MissingQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	uc := usecase.NewSearchProductsUseCase(
		gateway.NewMockAlibabaGateway(),
		gateway.NewMockLLMGateway(),
		nil,
	)
	h := NewSearchHandler(uc)

	r := gin.New()
	r.GET("/search", h.Search)

	req := httptest.NewRequest(http.MethodGet, "/search?q=", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	var resp Envelope
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, CodeInvalidInput, resp.Error.Code)
}

func TestSearchHandler_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	uc := usecase.NewSearchProductsUseCase(
		gateway.NewMockAlibabaGateway(),
		gateway.NewMockLLMGateway(),
		nil,
	)
	h := NewSearchHandler(uc)

	r := gin.New()
	r.GET("/search", h.Search)

	req := httptest.NewRequest(http.MethodGet, "/search?q=phone", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp struct {
		Data  usecase.SearchResult `json:"data"`
		Error *APIError            `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Nil(t, resp.Error)
	assert.NotEmpty(t, resp.Data.Products)
	assert.Equal(t, len(resp.Data.Products), resp.Data.Total)
}

func TestSearchHandler_WithLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	uc := usecase.NewSearchProductsUseCase(
		gateway.NewMockAlibabaGateway(),
		gateway.NewMockLLMGateway(),
		nil,
	)
	h := NewSearchHandler(uc)

	r := gin.New()
	r.GET("/search", h.Search)

	req := httptest.NewRequest(http.MethodGet, "/search?q=phone&limit=2", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp struct {
		Data  usecase.SearchResult `json:"data"`
		Error *APIError            `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Nil(t, resp.Error)
	assert.Equal(t, 2, len(resp.Data.Products))
	assert.Equal(t, 2, resp.Data.Total)
}
