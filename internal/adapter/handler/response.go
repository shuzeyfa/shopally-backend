package handler

import "github.com/gin-gonic/gin"

// Error codes returned in the envelope's error.code field. The frontend keys
// its error handling off these, so treat them as part of the API contract.
const (
	CodeInvalidInput       = "INVALID_INPUT"
	CodeUnauthorized       = "UNAUTHORIZED"
	CodeForbidden          = "FORBIDDEN"
	CodeNotFound           = "NOT_FOUND"
	CodeFXUnavailable      = "FX_UNAVAILABLE"
	CodeServiceUnavailable = "SERVICE_UNAVAILABLE"
	CodeInternal           = "INTERNAL_SERVER_ERROR"
)

// APIError is the error half of the response envelope.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Envelope is the single response shape for every endpoint. Exactly one of
// Data and Error is populated, so the frontend can branch on error != null.
type Envelope struct {
	Data  interface{} `json:"data"`
	Error *APIError   `json:"error"`
}

// respondOK writes a success envelope.
func respondOK(c *gin.Context, status int, data interface{}) {
	c.JSON(status, Envelope{Data: data, Error: nil})
}

// respondError writes an error envelope.
//
// message is client-facing: never pass a raw err.Error() that could carry an
// upstream URL, credential, or driver internal. Log those server-side instead.
func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, Envelope{Data: nil, Error: &APIError{Code: code, Message: message}})
}
