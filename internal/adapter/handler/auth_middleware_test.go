package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/suite"
)

type AuthMiddlewareSuite struct {
	suite.Suite
	secret []byte
}

func (s *AuthMiddlewareSuite) SetupSuite() {
	gin.SetMode(gin.TestMode)
	s.secret = []byte("super-secret-key-that-is-at-least-32-bytes-long!")
}

func (s *AuthMiddlewareSuite) signToken(claims *apiTokenClaims, secret []byte) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	s.Require().NoError(err)
	return signed
}

func (s *AuthMiddlewareSuite) validClaims() *apiTokenClaims {
	now := time.Now()
	return &apiTokenClaims{
		Email: "user@example.com",
		Name:  "Test User",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    TokenIssuer,
			Audience:  jwt.ClaimStrings{TokenAudience},
			Subject:   "user-123",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
		},
	}
}

func (s *AuthMiddlewareSuite) TestRequireAuth_ShortSecret_FailsClosed() {
	shortSecret := []byte("too-short")
	r := gin.New()
	r.Use(RequireAuth(shortSecret))
	r.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	s.Equal(http.StatusUnauthorized, rr.Code)
	var resp Envelope
	s.Require().NoError(json.Unmarshal(rr.Body.Bytes(), &resp))
	s.Equal(CodeUnauthorized, resp.Error.Code)
	s.Equal("authentication required", resp.Error.Message)
}

func (s *AuthMiddlewareSuite) TestRequireAuth_MissingAuthorizationHeader() {
	r := gin.New()
	r.Use(RequireAuth(s.secret))
	r.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	s.Equal(http.StatusUnauthorized, rr.Code)
	s.Contains(rr.Header().Get("WWW-Authenticate"), `error="invalid_token"`)
}

func (s *AuthMiddlewareSuite) TestRequireAuth_MalformedAuthorizationHeader() {
	r := gin.New()
	r.Use(RequireAuth(s.secret))
	r.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for _, badHeader := range []string{
		"Basic abcdef",
		"Bearer",
		"Bearer ",
		"SomethingElse token",
	} {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", badHeader)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		s.Equal(http.StatusUnauthorized, rr.Code, "header: %s", badHeader)
	}
}

func (s *AuthMiddlewareSuite) TestRequireAuth_InvalidSignature() {
	r := gin.New()
	r.Use(RequireAuth(s.secret))
	r.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	otherSecret := []byte("another-secret-key-that-is-at-least-32-bytes-long!")
	token := s.signToken(s.validClaims(), otherSecret)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	s.Equal(http.StatusUnauthorized, rr.Code)
}

func (s *AuthMiddlewareSuite) TestRequireAuth_ExpiredToken() {
	r := gin.New()
	r.Use(RequireAuth(s.secret))
	r.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	claims := s.validClaims()
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-2 * time.Hour))
	token := s.signToken(claims, s.secret)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	s.Equal(http.StatusUnauthorized, rr.Code)
}

func (s *AuthMiddlewareSuite) TestRequireAuth_WrongIssuer() {
	r := gin.New()
	r.Use(RequireAuth(s.secret))
	r.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	claims := s.validClaims()
	claims.Issuer = "wrong-issuer"
	token := s.signToken(claims, s.secret)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	s.Equal(http.StatusUnauthorized, rr.Code)
}

func (s *AuthMiddlewareSuite) TestRequireAuth_WrongAudience() {
	r := gin.New()
	r.Use(RequireAuth(s.secret))
	r.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	claims := s.validClaims()
	claims.Audience = jwt.ClaimStrings{"wrong-audience"}
	token := s.signToken(claims, s.secret)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	s.Equal(http.StatusUnauthorized, rr.Code)
}

func (s *AuthMiddlewareSuite) TestRequireAuth_EmptySubject() {
	r := gin.New()
	r.Use(RequireAuth(s.secret))
	r.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	claims := s.validClaims()
	claims.Subject = "   "
	token := s.signToken(claims, s.secret)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	s.Equal(http.StatusUnauthorized, rr.Code)
}

func (s *AuthMiddlewareSuite) TestRequireAuth_RetiredDemoToken() {
	r := gin.New()
	r.Use(RequireAuth(s.secret))
	called := false
	r.GET("/protected", func(c *gin.Context) {
		called = true
		c.Status(http.StatusOK)
	})

	claims := s.validClaims()
	claims.Subject = "demo-previously-issued-account"
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+s.signToken(claims, s.secret))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	s.Equal(http.StatusUnauthorized, rr.Code)
	s.False(called)
	s.Contains(rr.Body.String(), `"code":"UNAUTHORIZED"`)
}

func (s *AuthMiddlewareSuite) TestRequireAuth_Success_SetsContext() {
	r := gin.New()
	r.Use(RequireAuth(s.secret))

	var gotUserID, gotEmail, gotName string
	r.GET("/protected", func(c *gin.Context) {
		uid, _ := UserIDFrom(c)
		gotUserID = uid
		gotEmail = EmailFrom(c)
		gotName = NameFrom(c)
		respondOK(c, http.StatusOK, gin.H{"status": "ok"})
	})

	claims := s.validClaims()
	claims.Subject = "user-999"
	claims.Email = "alice@example.com"
	claims.Name = "Alice Doe"
	token := s.signToken(claims, s.secret)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	s.Equal("user-999", gotUserID)
	s.Equal("alice@example.com", gotEmail)
	s.Equal("Alice Doe", gotName)
}

func (s *AuthMiddlewareSuite) TestRequireAuth_ClaimAndAlgorithmRules() {
	cases := []struct {
		name   string
		change func(*apiTokenClaims)
		method jwt.SigningMethod
	}{
		{name: "missing expiry", change: func(c *apiTokenClaims) { c.ExpiresAt = nil }},
		{name: "missing issued at", change: func(c *apiTokenClaims) { c.IssuedAt = nil }},
		{name: "missing issuer", change: func(c *apiTokenClaims) { c.Issuer = "" }},
		{name: "missing audience", change: func(c *apiTokenClaims) { c.Audience = nil }},
		{name: "future issued at", change: func(c *apiTokenClaims) {
			c.IssuedAt = jwt.NewNumericDate(time.Now().Add(5 * time.Minute))
		}},
		{name: "future not before", change: func(c *apiTokenClaims) {
			c.NotBefore = jwt.NewNumericDate(time.Now().Add(5 * time.Minute))
		}},
		{name: "inverted interval", change: func(c *apiTokenClaims) {
			c.IssuedAt = jwt.NewNumericDate(time.Now().Add(20 * time.Second))
			c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(10 * time.Second))
		}},
		{name: "wrong algorithm", method: jwt.SigningMethodHS384},
		{name: "unsigned", method: jwt.SigningMethodNone},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			claims := s.validClaims()
			if tc.change != nil {
				tc.change(claims)
			}
			method := tc.method
			var key interface{} = s.secret
			if method == nil {
				method = jwt.SigningMethodHS256
			}
			if method == jwt.SigningMethodNone {
				key = jwt.UnsafeAllowNoneSignatureType
			}
			token, err := jwt.NewWithClaims(method, claims).SignedString(key)
			s.Require().NoError(err)
			r := gin.New()
			r.Use(RequireAuth(s.secret))
			called := false
			r.GET("/protected", func(c *gin.Context) { called = true })
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)
			s.Equal(http.StatusUnauthorized, rr.Code)
			s.False(called)
			var resp Envelope
			s.Require().NoError(json.Unmarshal(rr.Body.Bytes(), &resp))
			s.Require().NotNil(resp.Error)
			s.Equal("authentication required", resp.Error.Message)
		})
	}
}

func (s *AuthMiddlewareSuite) TestRequireAuth_ClockSkewAndBearerCase() {
	claims := s.validClaims()
	claims.IssuedAt = jwt.NewNumericDate(time.Now().Add(10 * time.Second))
	r := gin.New()
	r.Use(RequireAuth(s.secret))
	r.GET("/me", Me)
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "bEaReR "+s.signToken(claims, s.secret))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	s.Equal(http.StatusOK, rr.Code)
}

func (s *AuthMiddlewareSuite) TestMeHandler_Success() {
	r := gin.New()
	r.Use(RequireAuth(s.secret))
	r.GET("/me", Me)

	claims := s.validClaims()
	claims.Subject = "user-abc"
	claims.Email = "me@example.com"
	claims.Name = "Me User"
	token := s.signToken(claims, s.secret)

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	var resp struct {
		Data  mePayload `json:"data"`
		Error *APIError `json:"error"`
	}
	s.Require().NoError(json.Unmarshal(rr.Body.Bytes(), &resp))
	s.Nil(resp.Error)
	s.Equal("user-abc", resp.Data.UserID)
	s.Equal("me@example.com", resp.Data.Email)
	s.Equal("Me User", resp.Data.Name)
}

func (s *AuthMiddlewareSuite) TestMeHandler_Unauthenticated_Fails() {
	r := gin.New()
	r.GET("/me", Me)

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	s.Equal(http.StatusUnauthorized, rr.Code)
}

func TestAuthMiddlewareSuite(t *testing.T) {
	suite.Run(t, new(AuthMiddlewareSuite))
}
