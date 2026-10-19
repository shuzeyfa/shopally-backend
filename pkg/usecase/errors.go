package usecase

import "errors"

// Domain-level sentinel errors. Adapters wrap these so handlers can map a
// cause to an HTTP status without string-matching, and so a transient storage
// failure is never reported to the client as "not found".
//
// They live here rather than in the adapter packages so that inbound adapters
// (handlers) and outbound adapters (gateways, repositories) both depend on
// this layer instead of on each other.
var (
	// ErrAlertNotFound means no alert exists with the requested ID.
	ErrAlertNotFound = errors.New("alert not found")

	// ErrFXUnavailable means the FX provider could not be reached or refused
	// the request.
	ErrFXUnavailable = errors.New("fx provider unavailable")

	// ErrFXUnrecognized means the FX provider replied in no shape we can parse.
	ErrFXUnrecognized = errors.New("fx provider returned an unrecognized response")

	// ErrProductNotFound means a requested product does not exist.
	ErrProductNotFound = errors.New("product not found")

	// ErrInvalidComparisonCount means fewer than 2 or more than 4 products were requested.
	ErrInvalidComparisonCount = errors.New("comparison requires between 2 and 4 products")

	ErrSavedStoreUnavailable = errors.New("saved items storage unavailable")
	ErrAlertStoreUnavailable = errors.New("alert storage unavailable")
)
