package handler

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/shopally-ai/pkg/domain"
	"github.com/shopally-ai/pkg/usecase"
)

type SavedHandler struct {
	repository usecase.SavedItemsRepository
}

func NewSavedHandler(repository usecase.SavedItemsRepository) *SavedHandler {
	return &SavedHandler{repository: repository}
}

type saveProductRequest struct {
	Product *domain.Product `json:"product" binding:"required"`
}

func (h *SavedHandler) Save(c *gin.Context) {
	userID, ok := UserIDFrom(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "authentication required")
		return
	}

	var request saveProductRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.Product == nil {
		respondError(c, http.StatusBadRequest, CodeInvalidInput, "product is required")
		return
	}
	request.Product.ID = strings.TrimSpace(request.Product.ID)
	if request.Product.ID == "" {
		respondError(c, http.StatusBadRequest, CodeInvalidInput, "product id is required")
		return
	}

	item, err := h.repository.Save(c.Request.Context(), userID, request.Product)
	if err != nil {
		h.respondStorageError(c, userID, "save", err)
		return
	}
	respondOK(c, http.StatusCreated, item)
}

func (h *SavedHandler) List(c *gin.Context) {
	userID, ok := UserIDFrom(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "authentication required")
		return
	}

	items, err := h.repository.List(c.Request.Context(), userID)
	if err != nil {
		h.respondStorageError(c, userID, "list", err)
		return
	}
	respondOK(c, http.StatusOK, items)
}

func (h *SavedHandler) Delete(c *gin.Context) {
	userID, ok := UserIDFrom(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "authentication required")
		return
	}

	productID := strings.TrimSpace(c.Param("productId"))
	if productID == "" {
		respondError(c, http.StatusBadRequest, CodeInvalidInput, "product id is required")
		return
	}
	if err := h.repository.Delete(c.Request.Context(), userID, productID); err != nil {
		h.respondStorageError(c, userID, "delete", err)
		return
	}
	respondOK(c, http.StatusOK, map[string]string{"productId": productID})
}

func (h *SavedHandler) respondStorageError(c *gin.Context, userID, operation string, err error) {
	if errors.Is(err, usecase.ErrSavedStoreUnavailable) {
		respondError(c, http.StatusServiceUnavailable, CodeServiceUnavailable, "saved items are temporarily unavailable")
		return
	}
	log.Printf("saved_items operation=%s request_id=%s user=%s error=%v", operation, RequestIDFrom(c), userID, err)
	respondError(c, http.StatusInternalServerError, CodeInternal, "could not access saved items")
}
