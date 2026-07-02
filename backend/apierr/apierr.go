// Package apierr defines structured HTTP error responses.
//
// Base errors (ErrNotFound, ErrUnauthorized, etc.) are factory functions that
// return fresh *APIError values, safe to use without risk of mutating shared state.
//
// App-specific sentinel errors (ErrSlotsFull, ErrLicenseInvalid, etc.) are
// pointer values — acceptable because handlers never mutate them and the
// framework's error dispatch only reads their fields.
package apierr

import (
	"encoding/json"
	"net/http"
)

// APIError is a structured error that maps to an HTTP response.
// Code is the machine-readable error code (serialized as "error" in JSON).
// Message is the human-readable description.
// Status is the HTTP status code (not included in JSON output).
type APIError struct {
	Code    string `json:"error"`
	Message string `json:"message"`
	Status  int    `json:"-"`
}

func (e *APIError) Error() string { return e.Code + ": " + e.Message }

// Sentinel error templates — unexported values to prevent mutation.
// Use the exported factory functions below, which return fresh copies.
var (
	errNotFound     = APIError{Code: "not_found", Status: 404, Message: "Resource not found"}
	errUnauthorized = APIError{Code: "unauthorized", Status: 401, Message: "Authentication required"}
	errForbidden    = APIError{Code: "forbidden", Status: 403, Message: "Access denied"}
	errBadRequest   = APIError{Code: "bad_request", Status: 400, Message: "Invalid request"}
	errRateLimit    = APIError{Code: "rate_limited", Status: 429, Message: "Too many requests, try again later"}
	errInternal     = APIError{Code: "internal_error", Status: 500, Message: "Internal server error"}
	errConflict     = APIError{Code: "conflict", Status: 409, Message: "Resource already exists"}
	errTokenInvalid = APIError{Code: "token_invalid", Status: 401, Message: "Token is invalid or has expired"}
)

// Error factory functions — each call returns a fresh *APIError, safe to use
// without risk of mutating shared state.

func ErrNotFound() *APIError     { e := errNotFound; return &e }
func ErrUnauthorized() *APIError { e := errUnauthorized; return &e }
func ErrForbidden() *APIError    { e := errForbidden; return &e }
func ErrBadRequest() *APIError   { e := errBadRequest; return &e }
func ErrRateLimit() *APIError    { e := errRateLimit; return &e }
func ErrInternal() *APIError     { e := errInternal; return &e }
func ErrConflict() *APIError     { e := errConflict; return &e }
func ErrTokenInvalid() *APIError { e := errTokenInvalid; return &e }

// Write sends an APIError as a JSON HTTP response.
func Write(w http.ResponseWriter, e *APIError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.Status)
	_ = json.NewEncoder(w).Encode(e)
}

// JSON sends an arbitrary value as a JSON HTTP response.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// App-specific sentinel errors for stevio-home.
var (
	ErrSlotsFull            = &APIError{Code: "activation_slots_full", Status: 422, Message: "Maximum device activations reached"}
	ErrLicenseInvalid       = &APIError{Code: "license_invalid", Status: 422, Message: "License key not found"}
	ErrMagicLinkPending     = &APIError{Code: "magic_link_pending", Status: http.StatusTooManyRequests, Message: "A login link was already sent. Please check your email or try again later."}
	ErrDownloadTokenUsed    = &APIError{Code: "download_token_used", Status: http.StatusGone, Message: "Download link has already been used"}
	ErrDownloadTokenExpired = &APIError{Code: "download_token_expired", Status: http.StatusGone, Message: "Download link has expired"}
	ErrDiscountInvalid      = &APIError{Code: "discount_invalid", Status: 422, Message: "Discount code is invalid, expired, or not applicable"}
	ErrAlreadyOwned         = &APIError{Code: "already_owned", Status: http.StatusConflict, Message: "You already own a license for this app"}
	ErrLoginRequired        = &APIError{Code: "login_required", Status: http.StatusUnauthorized, Message: "You must be logged in to purchase this app"}
	ErrPurchaseNotAvailable = &APIError{Code: "purchase_not_available", Status: http.StatusForbidden, Message: "This app is not available for purchase"}
	ErrLicenseRevoked       = &APIError{Code: "license_revoked", Status: http.StatusForbidden, Message: "This license has been revoked"}
	ErrNoActiveSigningKey   = &APIError{Code: "no_active_signing_key", Status: http.StatusServiceUnavailable, Message: "License signing is not configured. Please contact support."}
)
