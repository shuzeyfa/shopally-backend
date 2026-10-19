package handler

import (
	"log"
	"net/http"
	"slices"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const requestIDHeader = "X-Request-ID"

// requestIDKey is the gin context key holding the current request's ID.
const requestIDKey = "requestID"

// CORS allows browser clients from the configured origins.
//
// The matched origin is echoed back rather than "*" because the frontend sends
// an Authorization header; browsers reject wildcard origins on credentialed
// requests. An empty allowlist rejects all cross-origin calls.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		// Vary unconditionally: the response body is origin-independent but the
		// CORS headers are not, so shared caches must key on Origin.
		c.Header("Vary", "Origin")

		if origin != "" && slices.Contains(allowedOrigins, origin) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, "+requestIDHeader)
			c.Header("Access-Control-Expose-Headers", requestIDHeader)
			c.Header("Access-Control-Max-Age", "600")
		}

		// Preflight never reaches a route handler.
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// RequestID attaches a request ID to the context and response, reusing an
// inbound one so a trace survives across the frontend/backend boundary.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(requestIDHeader)
		if id == "" {
			id = uuid.NewString()
		}
		c.Set(requestIDKey, id)
		c.Header(requestIDHeader, id)
		c.Next()
	}
}

// Recovery converts a panic into a 500 in the standard envelope, so clients
// get parseable JSON instead of gin's default plain-text crash output.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic request_id=%s method=%s path=%s: %v",
					RequestIDFrom(c), c.Request.Method, c.Request.URL.Path, r)
				respondError(c, http.StatusInternalServerError, CodeInternal, "internal server error")
				c.Abort()
			}
		}()
		c.Next()
	}
}

// RequestLogger logs one line per request after it completes.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		log.Printf("request_id=%s method=%s path=%s status=%d duration=%s",
			RequestIDFrom(c), c.Request.Method, c.Request.URL.Path,
			c.Writer.Status(), time.Since(start).Round(time.Millisecond))
	}
}

// RequestIDFrom returns the current request's ID, or "" if unset.
func RequestIDFrom(c *gin.Context) string {
	if v, ok := c.Get(requestIDKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// userIDKey is the gin context key holding the authenticated user's ID.
const userIDKey = "userID"

// UserIDFrom returns the authenticated user's ID and whether one is present.
//
// The value is populated by RequireAuth from a verified bearer token, never
// from client-supplied request data. Handlers that call this must sit behind
// RequireAuth; the check here reports false only as a backstop, and those
// handlers answer 401 rather than falling back to a caller-supplied ID.
func UserIDFrom(c *gin.Context) (string, bool) {
	v, ok := c.Get(userIDKey)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return "", false
	}
	return s, true
}
