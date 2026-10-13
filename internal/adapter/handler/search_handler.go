package handler

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/shopally-ai/pkg/usecase"
)

// SearchHandler handles incoming HTTP requests for the /search endpoint.
type SearchHandler struct {
	uc *usecase.SearchProductsUseCase
}

// NewSearchHandler creates a new SearchHandler with its dependencies.
func NewSearchHandler(uc *usecase.SearchProductsUseCase) *SearchHandler {
	return &SearchHandler{uc: uc}
}

// Search handles GET /search?q=.
func (h *SearchHandler) Search(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		respondError(c, http.StatusBadRequest, CodeInvalidInput,
			"missing required query parameter: q")
		return
	}

	data, err := h.uc.Search(c.Request.Context(), q)
	if err != nil {
		// The underlying error can name an upstream provider; log it and keep
		// the client-facing message generic.
		log.Printf("search request_id=%s q=%q error=%v", RequestIDFrom(c), q, err)
		respondError(c, http.StatusInternalServerError, CodeInternal, "search failed")
		return
	}

	respondOK(c, http.StatusOK, data)
}
