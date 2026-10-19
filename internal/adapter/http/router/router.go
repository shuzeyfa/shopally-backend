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
	Health  *apphandler.HealthHandler
	Search  *apphandler.SearchHandler
	FX      *apphandler.FXHandler
	Compare *apphandler.CompareHandler

	// Alerts is mounted behind RequireAuth and takes its owner from the
	// verified token, never from the request body.
	Alerts *apphandler.AlertHandler
	Saved  *apphandler.SavedHandler
}

// Options control router behavior.
type Options struct {
	// BasePath prefixes every route except /healthz, e.g. "/api/v1".
	BasePath string
	// AllowedOrigins is the CORS allowlist. Empty rejects all cross-origin calls.
	AllowedOrigins []string
	// AuthSecret verifies the frontend's HS256 API tokens. Anything shorter
	// than apphandler.MinTokenSecretLength makes every protected route answer
	// 401 rather than serve traffic behind a weak key.
	AuthSecret []byte
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
	if d.Compare != nil {
		api.POST("/compare", d.Compare.Compare)
	}

	// Everything below identifies the caller, so it sits behind one group:
	// adding a route to `protected` is the whole opt-in, and nothing can be
	// mounted authenticated by accident.
	protected := api.Group("")
	protected.Use(apphandler.RequireAuth(opts.AuthSecret))

	protected.GET("/me", apphandler.Me)

	if d.Alerts != nil {
		protected.POST("/alerts", d.Alerts.CreateAlert)
		protected.GET("/alerts", d.Alerts.ListAlerts)
		protected.GET("/alerts/:id", d.Alerts.GetAlert)
		protected.DELETE("/alerts/:id", d.Alerts.DeleteAlert)
	}
	if d.Saved != nil {
		protected.POST("/saved", d.Saved.Save)
		protected.GET("/saved", d.Saved.List)
		protected.DELETE("/saved/:productId", d.Saved.Delete)
	}

	return r
}
