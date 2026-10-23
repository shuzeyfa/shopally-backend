package handler

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	// MinTokenSecretLength is the shortest HMAC secret accepted for API tokens.
	// 32 bytes is the output size of the underlying hash, so anything shorter
	// adds no security while looking like it does.
	MinTokenSecretLength = 32

	// TokenIssuer is the exact iss claim the frontend stamps on minted tokens.
	// Anywhere else signing with the shared secret is rejected.
	TokenIssuer = "shopally-web"

	// TokenAudience is the exact aud claim the frontend stamps on minted tokens.
	TokenAudience = "shopally-api"

	// tokenClockLeeway absorbs small clock differences between the frontend
	// host and the API host when checking exp and iat.
	tokenClockLeeway = 30 * time.Second
)

// Email and name ride along on the token so /me can answer without a user
// store. They are display data only; identity decisions use the subject.
const (
	userEmailKey = "userEmail"
	userNameKey  = "userName"
)

// apiTokenClaims is the payload the frontend mints. RegisteredClaims supplies
// iss, aud, sub, iat and exp, all of which the parser validates.
type apiTokenClaims struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	jwt.RegisteredClaims
}

// RequireAuth rejects any request without a valid bearer token, and on success
// puts the token's subject in the context as the authenticated user ID.
//
// The token is the only source of identity: nothing a client sends in a body,
// query string or header other than Authorization can change who the caller is.
//
// A secret shorter than MinTokenSecretLength is a deployment error. The
// middleware fails closed — every request is refused — because the alternative,
// serving traffic protected by a guessable key, is worse than an outage that
// alerts loudly.
func RequireAuth(secret []byte) gin.HandlerFunc {
	if len(secret) < MinTokenSecretLength {
		log.Printf("auth misconfigured: AUTH_JWT_SECRET is %d bytes, need at least %d; rejecting every authenticated request until it is fixed",
			len(secret), MinTokenSecretLength)
		return func(c *gin.Context) {
			unauthorized(c, "server has no usable token secret")
		}
	}

	// One parser for the lifetime of the process: it is stateless, and building
	// it once keeps the accepted-algorithm and claim rules in a single place.
	parser := jwt.NewParser(
		// Pinning the algorithm is what prevents an attacker from presenting a
		// token signed with "none" or an asymmetric algorithm keyed with our
		// HMAC secret as if it were a public key.
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(TokenIssuer),
		jwt.WithAudience(TokenAudience),
		// Without this, a token with no exp never expires.
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(tokenClockLeeway),
	)

	return func(c *gin.Context) {
		raw, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok {
			unauthorized(c, "missing or malformed Authorization header")
			return
		}

		claims := &apiTokenClaims{}
		_, err := parser.ParseWithClaims(raw, claims, func(*jwt.Token) (interface{}, error) {
			return secret, nil
		})
		if err != nil {
			// The parser's messages name the failure (expired, wrong audience,
			// bad signature) and never echo the token itself.
			unauthorized(c, "invalid token: "+err.Error())
			return
		}
		if strings.TrimSpace(claims.Subject) == "" {
			unauthorized(c, "token has no subject")
			return
		}
		// The retired demo provider derived identities from unverified emails.
		// Reject its already-issued tokens as well as disabling new sign-ins.
		if strings.HasPrefix(claims.Subject, "demo-") {
			unauthorized(c, "demo accounts are no longer supported")
			return
		}

		c.Set(userIDKey, claims.Subject)
		c.Set(userEmailKey, claims.Email)
		c.Set(userNameKey, claims.Name)
		c.Next()
	}
}

// bearerToken extracts the credential from an Authorization header.
func bearerToken(header string) (string, bool) {
	const prefix = "bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	return token, token != ""
}

// unauthorized writes the 401 envelope and stops the chain.
func unauthorized(c *gin.Context, reason string) {
	log.Printf("auth rejected request_id=%s method=%s path=%s reason=%s",
		RequestIDFrom(c), c.Request.Method, c.Request.URL.Path, reason)

	// RFC 6750: a bearer-protected resource should say so, and should not say
	// why beyond "invalid_token", which would help an attacker refine guesses.
	c.Header("WWW-Authenticate", `Bearer realm="shopally-api", error="invalid_token"`)
	respondError(c, http.StatusUnauthorized, CodeUnauthorized, "authentication required")
	c.Abort()
}

// EmailFrom returns the authenticated caller's email, or "" if none.
func EmailFrom(c *gin.Context) string {
	return contextString(c, userEmailKey)
}

// NameFrom returns the authenticated caller's display name, or "" if none.
func NameFrom(c *gin.Context) string {
	return contextString(c, userNameKey)
}

func contextString(c *gin.Context, key string) string {
	if v, ok := c.Get(key); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
