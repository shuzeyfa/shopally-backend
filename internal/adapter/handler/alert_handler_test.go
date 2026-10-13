package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/shopally-ai/internal/adapter/repository"
	"github.com/shopally-ai/pkg/domain"
	"github.com/shopally-ai/pkg/usecase"
)

// newAlertTestServer builds an engine with the alert routes mounted behind a
// stub auth middleware that authenticates every request as userID. Routing is
// exercised too, so the :id path param behaves as it will in production.
//
// Pass an empty userID to simulate an unauthenticated caller.
func newAlertTestServer(userID string) *gin.Engine {
	gin.SetMode(gin.TestMode)

	h := NewAlertHandler(usecase.NewAlertManager(repository.NewMockAlertRepository()))

	r := gin.New()
	r.Use(func(c *gin.Context) {
		if userID != "" {
			c.Set(userIDKey, userID)
		}
		c.Next()
	})
	r.POST("/alerts", h.CreateAlert)
	r.GET("/alerts/:id", h.GetAlert)
	r.DELETE("/alerts/:id", h.DeleteAlert)
	return r
}

func doJSON(r *gin.Engine, method, target, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

// decodeAlertID pulls the alertId out of a success envelope.
func decodeAlertID(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()
	var resp struct {
		Data struct {
			AlertID string `json:"alertId"`
		} `json:"data"`
		Error *APIError `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("could not decode response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error in envelope: %+v", resp.Error)
	}
	if resp.Data.AlertID == "" {
		t.Fatal("response contained no alertId")
	}
	return resp.Data.AlertID
}

func TestAlertLifecycle(t *testing.T) {
	r := newAlertTestServer("user-123")

	rr := doJSON(r, http.MethodPost, "/alerts", `{"productId":"prod-abc","targetPrice":500}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: got status %d, want %d (body: %s)", rr.Code, http.StatusCreated, rr.Body)
	}
	alertID := decodeAlertID(t, rr)

	rr = doJSON(r, http.MethodGet, "/alerts/"+alertID, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("get: got status %d, want %d", rr.Code, http.StatusOK)
	}
	var got struct {
		Data domain.Alert `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("could not decode alert: %v", err)
	}
	if got.Data.ID != alertID {
		t.Errorf("got alertId %q, want %q", got.Data.ID, alertID)
	}
	if got.Data.UserID != "user-123" {
		t.Errorf("got userId %q, want the authenticated user", got.Data.UserID)
	}

	rr = doJSON(r, http.MethodDelete, "/alerts/"+alertID, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("delete: got status %d, want %d", rr.Code, http.StatusOK)
	}

	rr = doJSON(r, http.MethodGet, "/alerts/"+alertID, "")
	if rr.Code != http.StatusNotFound {
		t.Errorf("get after delete: got status %d, want %d", rr.Code, http.StatusNotFound)
	}
}

// The owner is taken from the authenticated context, so a body that tries to
// claim a different user must not influence the stored alert.
func TestCreateAlertIgnoresUserIDInBody(t *testing.T) {
	r := newAlertTestServer("real-user")

	rr := doJSON(r, http.MethodPost, "/alerts",
		`{"userId":"attacker","productId":"prod-abc","targetPrice":500}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: got status %d, want %d", rr.Code, http.StatusCreated)
	}
	alertID := decodeAlertID(t, rr)

	rr = doJSON(r, http.MethodGet, "/alerts/"+alertID, "")
	var got struct {
		Data domain.Alert `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("could not decode alert: %v", err)
	}
	if got.Data.UserID != "real-user" {
		t.Errorf("got userId %q, want %q", got.Data.UserID, "real-user")
	}
}

func TestAlertRequiresAuth(t *testing.T) {
	r := newAlertTestServer("")

	cases := []struct {
		name, method, target, body string
	}{
		{"create", http.MethodPost, "/alerts", `{"productId":"p","targetPrice":1}`},
		{"get", http.MethodGet, "/alerts/some-id", ""},
		{"delete", http.MethodDelete, "/alerts/some-id", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := doJSON(r, tc.method, tc.target, tc.body)
			if rr.Code != http.StatusUnauthorized {
				t.Errorf("got status %d, want %d", rr.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestAlertInvalidPayload(t *testing.T) {
	r := newAlertTestServer("user-123")

	cases := []struct{ name, body string }{
		{"malformed json", `{`},
		{"missing productId", `{"targetPrice":500}`},
		{"zero targetPrice", `{"productId":"p","targetPrice":0}`},
		{"negative targetPrice", `{"productId":"p","targetPrice":-5}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := doJSON(r, http.MethodPost, "/alerts", tc.body)
			if rr.Code != http.StatusBadRequest {
				t.Errorf("got status %d, want %d", rr.Code, http.StatusBadRequest)
			}
		})
	}
}

// One user must not be able to read or delete another's alert, and the
// response must not reveal that the ID exists.
func TestAlertOwnershipIsEnforced(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// A single handler shared by two engines, so both users hit one store.
	h := NewAlertHandler(usecase.NewAlertManager(repository.NewMockAlertRepository()))

	serverFor := func(userID string) *gin.Engine {
		r := gin.New()
		r.Use(func(c *gin.Context) { c.Set(userIDKey, userID); c.Next() })
		r.POST("/alerts", h.CreateAlert)
		r.GET("/alerts/:id", h.GetAlert)
		r.DELETE("/alerts/:id", h.DeleteAlert)
		return r
	}

	owner := serverFor("owner")
	intruder := serverFor("intruder")

	rr := doJSON(owner, http.MethodPost, "/alerts", `{"productId":"prod-abc","targetPrice":500}`)
	alertID := decodeAlertID(t, rr)

	if rr := doJSON(intruder, http.MethodGet, "/alerts/"+alertID, ""); rr.Code != http.StatusNotFound {
		t.Errorf("intruder get: got status %d, want %d", rr.Code, http.StatusNotFound)
	}
	if rr := doJSON(intruder, http.MethodDelete, "/alerts/"+alertID, ""); rr.Code != http.StatusNotFound {
		t.Errorf("intruder delete: got status %d, want %d", rr.Code, http.StatusNotFound)
	}

	// The owner's alert must have survived the intruder's delete attempt.
	if rr := doJSON(owner, http.MethodGet, "/alerts/"+alertID, ""); rr.Code != http.StatusOK {
		t.Errorf("owner get after intruder delete: got status %d, want %d", rr.Code, http.StatusOK)
	}
}
