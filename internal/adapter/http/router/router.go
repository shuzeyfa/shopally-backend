// Package router owns the service's complete route table.
//
// Routes are declared here and nowhere else: handlers no longer register
// themselves, so there is one place to see the whole API surface.
package router

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	apphandler "github.com/shopally-ai/internal/adapter/handler"
)

// Deps contains the handlers the router should mount.
type Deps struct {
	Health *apphandler.HealthHandler
	Search *apphandler.SearchHandler
	FX     *apphandler.FXHandler

	// Alerts is accepted but deliberately not mounted yet: its handlers
	// require an authenticated user ID, and the auth middleware does not
	// exist. Mounting it now would expose unowned alert mutations.
	Alerts *apphandler.AlertHandler
}

// Options control router behavior.
type Options struct {
	// BasePath prefixes every route except /healthz, e.g. "/api/v1".
	BasePath string
	// AllowedOrigins is the CORS allowlist. Empty rejects all cross-origin calls.
	AllowedOrigins []string
	// Production switches gin out of debug mode.
	Production bool
}

// Build constructs the gin engine with middleware and all routes mounted.
func Build(d Deps, opts Options) *gin.Engine {
	if opts.Production {
		gin.SetMode(gin.ReleaseMode)
	}

	// gin.New rather than gin.Default: Default installs its own logger and
	// recovery, which would duplicate ours and emit non-envelope error bodies.
	r := gin.New()

	// RequestID first so every later middleware can log with it. Recovery
	// wraps everything downstream of it.
	r.Use(apphandler.RequestID())
	r.Use(apphandler.Recovery())
	r.Use(apphandler.RequestLogger())
	r.Use(apphandler.CORS(opts.AllowedOrigins))

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, apphandler.Envelope{
			Error: &apphandler.APIError{Code: apphandler.CodeNotFound, Message: "route not found"},
		})
	})

	// Unprefixed: platform health probes expect a stable, versionless path.
	if d.Health != nil {
		r.GET("/healthz", d.Health.Health)
	}

	base := strings.TrimRight(opts.BasePath, "/")
	api := r.Group(base)

	if d.Search != nil {
		api.GET("/search", d.Search.Search)
	}
	if d.FX != nil {
		api.GET("/fx", d.FX.GetFX)
	}

	return r
}
