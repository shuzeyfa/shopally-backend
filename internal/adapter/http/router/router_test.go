package router

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/shopally-ai/internal/adapter/gateway"
	"github.com/shopally-ai/internal/adapter/handler"
	"github.com/shopally-ai/pkg/usecase"
)

func TestFoundationRoutes(t *testing.T) {
	var healthy atomic.Bool
	r := Build(Deps{
		Health: handler.NewHealthHandler(map[string]handler.DependencyCheck{
			"mongo": func(context.Context) error {
				if !healthy.Load() {
					return errors.New("dependency unavailable")
				}
				return nil
			},
		}),
		Search: handler.NewSearchHandler(usecase.NewSearchProductsUseCase(
			gateway.NewMockAlibabaGateway(), gateway.NewMockLLMGateway(), gateway.NewSwitchableCache(nil),
		)),
	}, Options{BasePath: "/api/v1", AllowedOrigins: []string{"http://localhost:3000"}})
	r.GET("/panic", func(*gin.Context) { panic("internal detail") })

	for _, test := range []struct {
		path string
		code int
	}{
		{"/healthz", http.StatusServiceUnavailable},
		{"/api/v1/search?q=phone", http.StatusOK},
		{"/api/v1/search", http.StatusBadRequest},
		{"/api/v1/me", http.StatusUnauthorized},
		{"/unknown", http.StatusNotFound},
		{"/panic", http.StatusInternalServerError},
	} {
		t.Run(test.path, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, test.path, nil)
			req.Header.Set("Origin", "http://localhost:3000")
			r.ServeHTTP(rr, req)
			require.Equal(t, test.code, rr.Code)
			require.Equal(t, "http://localhost:3000", rr.Header().Get("Access-Control-Allow-Origin"))
			require.NotEmpty(t, rr.Header().Get("X-Request-ID"))
			var envelope map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope))
			require.Contains(t, envelope, "data")
			require.Contains(t, envelope, "error")
			require.NotContains(t, rr.Body.String(), "internal detail")
		})
	}
	healthy.Store(true)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	require.Equal(t, http.StatusOK, rr.Code, "health must reflect recovery without rebuilding routes")

	for _, origin := range []string{"http://localhost:3000", "https://untrusted.test"} {
		req := httptest.NewRequest(http.MethodOptions, "/api/v1/saved", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "authorization,content-type")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		require.Equal(t, http.StatusNoContent, rr.Code)
		if origin == "http://localhost:3000" {
			require.Equal(t, origin, rr.Header().Get("Access-Control-Allow-Origin"))
			require.Contains(t, rr.Header().Get("Access-Control-Allow-Headers"), "Authorization")
		} else {
			require.Empty(t, rr.Header().Get("Access-Control-Allow-Origin"))
		}
	}
}
