package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// mePayload echoes the identity the API derived from the bearer token. It is
// deliberately limited to what the token actually carries: there is no user
// store behind it yet.
type mePayload struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}

// Me handles GET /me.
//
// Its real job is to be the smallest possible protected route: if this answers,
// the frontend's token reached the API intact and the middleware accepted it.
// It must only ever be mounted behind RequireAuth; the UserIDFrom guard below
// is a backstop, not the protection.
func Me(c *gin.Context) {
	userID, ok := UserIDFrom(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "authentication required")
		return
	}

	respondOK(c, http.StatusOK, mePayload{
		UserID: userID,
		Email:  EmailFrom(c),
		Name:   NameFrom(c),
	})
}
