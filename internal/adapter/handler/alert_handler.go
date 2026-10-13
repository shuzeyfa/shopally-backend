// Package handler provides HTTP handlers for alert-related endpoints.
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

// AlertHandler handles HTTP requests for alert operations.
type AlertHandler struct {
	alertManager *usecase.AlertManager
}

// NewAlertHandler creates a new AlertHandler with the given AlertManager.
func NewAlertHandler(am *usecase.AlertManager) *AlertHandler {
	return &AlertHandler{alertManager: am}
}

// createAlertPayload is the expected body for creating an alert.
//
// UserID is intentionally absent: trusting a client-supplied owner would let
// any caller create alerts for anyone. It is taken from the authenticated
// context once RequireAuth is in place, which is also when these routes get
// mounted.
type createAlertPayload struct {
	ProductID   string  `json:"productId"`
	TargetPrice float64 `json:"targetPrice"`
}

// CreateAlert handles POST /alerts.
func (h *AlertHandler) CreateAlert(c *gin.Context) {
	var payload createAlertPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		respondError(c, http.StatusBadRequest, CodeInvalidInput, "request body is not valid JSON")
		return
	}

	if strings.TrimSpace(payload.ProductID) == "" {
		respondError(c, http.StatusBadRequest, CodeInvalidInput, "productId is required")
		return
	}
	if payload.TargetPrice <= 0 {
		respondError(c, http.StatusBadRequest, CodeInvalidInput, "targetPrice must be greater than zero")
		return
	}

	userID, ok := UserIDFrom(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "authentication required")
		return
	}

	newAlert := &domain.Alert{
		UserID:      userID,
		ProductID:   payload.ProductID,
		TargetPrice: payload.TargetPrice,
		IsActive:    true,
	}

	if err := h.alertManager.CreateAlert(newAlert); err != nil {
		log.Printf("create alert request_id=%s user=%s error=%v", RequestIDFrom(c), userID, err)
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not create alert")
		return
	}

	respondOK(c, http.StatusCreated, map[string]string{"alertId": newAlert.ID})
}

// GetAlert handles GET /alerts/:id.
func (h *AlertHandler) GetAlert(c *gin.Context) {
	alert, ok := h.loadOwnedAlert(c)
	if !ok {
		return
	}
	respondOK(c, http.StatusOK, alert)
}

// DeleteAlert handles DELETE /alerts/:id.
func (h *AlertHandler) DeleteAlert(c *gin.Context) {
	alert, ok := h.loadOwnedAlert(c)
	if !ok {
		return
	}

	if err := h.alertManager.DeleteAlert(alert.ID); err != nil {
		// A not-found here means it was removed between the read and the
		// delete, which is the caller's desired end state anyway.
		if errors.Is(err, usecase.ErrAlertNotFound) {
			respondOK(c, http.StatusOK, map[string]string{"alertId": alert.ID})
			return
		}
		log.Printf("delete alert request_id=%s id=%s error=%v", RequestIDFrom(c), alert.ID, err)
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not delete alert")
		return
	}

	respondOK(c, http.StatusOK, map[string]string{"alertId": alert.ID})
}

// loadOwnedAlert fetches the alert named in the path and verifies the caller
// owns it. It writes the error response itself and reports false when the
// request should not proceed.
//
// A caller who does not own an alert gets 404 rather than 403, so alert IDs
// cannot be probed for existence.
func (h *AlertHandler) loadOwnedAlert(c *gin.Context) (*domain.Alert, bool) {
	userID, ok := UserIDFrom(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "authentication required")
		return nil, false
	}

	alertID := strings.TrimSpace(c.Param("id"))
	if alertID == "" {
		respondError(c, http.StatusBadRequest, CodeInvalidInput, "alert id is required")
		return nil, false
	}

	alert, err := h.alertManager.GetAlert(alertID)
	if err != nil {
		if errors.Is(err, usecase.ErrAlertNotFound) {
			respondError(c, http.StatusNotFound, CodeNotFound, "alert not found")
			return nil, false
		}
		// A storage failure is not a missing alert.
		log.Printf("get alert request_id=%s id=%s error=%v", RequestIDFrom(c), alertID, err)
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not load alert")
		return nil, false
	}

	if alert.UserID != userID {
		respondError(c, http.StatusNotFound, CodeNotFound, "alert not found")
		return nil, false
	}

	return alert, true
}
