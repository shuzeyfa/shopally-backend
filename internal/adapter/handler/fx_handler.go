package handler

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/shopally-ai/pkg/usecase"
)

type FXHandler struct {
	FX usecase.IFXClient
}

func NewFXHandler(fx usecase.IFXClient) *FXHandler {
	return &FXHandler{FX: fx}
}

type fxResponse struct {
	From      string  `json:"from"`
	To        string  `json:"to"`
	Rate      float64 `json:"rate"`
	Amount    float64 `json:"amount"`
	Converted float64 `json:"converted"`
}

// GetFX handles GET /fx?from=&to=&amount=.
// Defaults to converting 1 USD to ETB, the pairing the product UI needs.
func (h *FXHandler) GetFX(c *gin.Context) {
	from := normalizeCurrency(c.Query("from"))
	to := normalizeCurrency(c.Query("to"))
	if from == "" {
		from = "USD"
	}
	if to == "" {
		to = "ETB"
	}

	amount := 1.0
	if s := strings.TrimSpace(c.Query("amount")); s != "" {
		a, err := strconv.ParseFloat(s, 64)
		if err != nil || a < 0 {
			respondError(c, http.StatusBadRequest, CodeInvalidInput,
				"amount must be a non-negative number")
			return
		}
		amount = a
	}

	rate, err := h.FX.GetRate(c.Request.Context(), from, to)
	if err != nil {
		// Upstream detail is logged by the gateway with the API key redacted;
		// the client only learns that conversion is unavailable.
		if errors.Is(err, usecase.ErrFXUnavailable) || errors.Is(err, usecase.ErrFXUnrecognized) {
			respondError(c, http.StatusBadGateway, CodeFXUnavailable,
				"currency conversion is temporarily unavailable")
			return
		}
		log.Printf("fx request_id=%s from=%s to=%s error=%v", RequestIDFrom(c), from, to, err)
		respondError(c, http.StatusBadGateway, CodeFXUnavailable,
			"currency conversion is temporarily unavailable")
		return
	}

	respondOK(c, http.StatusOK, fxResponse{
		From:      from,
		To:        to,
		Rate:      rate,
		Amount:    amount,
		Converted: rate * amount,
	})
}

// normalizeCurrency upper-cases a currency code and strips the stray quotes
// that shell and URL-encoding mistakes tend to leave behind.
func normalizeCurrency(raw string) string {
	return strings.ToUpper(strings.Trim(strings.TrimSpace(raw), "\"'"))
}
