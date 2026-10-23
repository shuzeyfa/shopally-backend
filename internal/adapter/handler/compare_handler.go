package handler

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/shopally-ai/pkg/usecase"
)

// CompareHandler handles product comparison requests.
type CompareHandler struct {
	uc *usecase.CompareProductsUseCase
}

// NewCompareHandler constructs a new CompareHandler.
func NewCompareHandler(uc *usecase.CompareProductsUseCase) *CompareHandler {
	return &CompareHandler{uc: uc}
}

type compareRequest struct {
	ProductIDs []string `json:"productIds" binding:"required"`
}

// Compare handles POST /api/v1/compare.
func (h *CompareHandler) Compare(c *gin.Context) {
	var req compareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, CodeInvalidInput, "invalid request body: productIds is required")
		return
	}

	result, err := h.uc.Compare(c.Request.Context(), req.ProductIDs)
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidComparisonCount) {
			respondError(c, http.StatusBadRequest, CodeInvalidInput, err.Error())
			return
		}
		if errors.Is(err, usecase.ErrProductNotFound) {
			respondError(c, http.StatusNotFound, CodeNotFound, err.Error())
			return
		}
		log.Printf("compare request_id=%s error=%v", RequestIDFrom(c), err)
		respondError(c, http.StatusInternalServerError, CodeInternal, "comparison failed")
		return
	}

	respondOK(c, http.StatusOK, result)
}
