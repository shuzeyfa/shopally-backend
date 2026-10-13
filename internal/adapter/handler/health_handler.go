package handler

import (
	"context"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
)

// DependencyCheck reports whether one external dependency is reachable.
type DependencyCheck func(ctx context.Context) error

// HealthHandler reports service liveness plus the state of each registered
// dependency. Checks are supplied by the caller so this package stays free of
// Mongo and Redis driver imports.
type HealthHandler struct {
	checks map[string]DependencyCheck
	// timeout bounds each individual check so one hung dependency cannot
	// make the health endpoint itself hang.
	timeout time.Duration
}

func NewHealthHandler(checks map[string]DependencyCheck) *HealthHandler {
	return &HealthHandler{checks: checks, timeout: 2 * time.Second}
}

type healthPayload struct {
	Status       string            `json:"status"`
	Dependencies map[string]string `json:"dependencies"`
}

// Health handles GET /healthz.
//
// Returns 200 when every dependency is reachable and 503 otherwise, so a
// platform health probe can act on it directly. Dependency errors are logged
// rather than returned, since probe output is often publicly reachable.
func (h *HealthHandler) Health(c *gin.Context) {
	deps := make(map[string]string, len(h.checks))
	healthy := true

	// Sorted for deterministic output, which keeps logs and tests stable.
	names := make([]string, 0, len(h.checks))
	for name := range h.checks {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
		err := h.checks[name](ctx)
		cancel()

		if err != nil {
			healthy = false
			deps[name] = "unavailable"
			log.Printf("healthz request_id=%s dependency=%s error=%v", RequestIDFrom(c), name, err)
			continue
		}
		deps[name] = "ok"
	}

	status := http.StatusOK
	overall := "ok"
	if !healthy {
		status = http.StatusServiceUnavailable
		overall = "degraded"
	}

	respondOK(c, status, healthPayload{Status: overall, Dependencies: deps})
}
