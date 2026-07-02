package app

import (
	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/middleware"
	"github.com/SteVio89/stevio-home/settings"
	"github.com/SteVio89/stevio-home/validate"
)

// Re-export types so consumer apps can use them without importing sub-packages.
type (
	APIError         = apierr.APIError
	Middleware       = middleware.Middleware
	ValidationErrors = validate.Errors
)

// Re-export sentinel errors for the single-import pattern.
var (
	ErrSettingNotFound = settings.ErrNotFound
	ErrRequired        = validate.ErrRequired
)

// Error constructors for common HTTP error responses.
// Handlers return these; the framework dispatches them to the appropriate HTTP status.

// NotFound returns a 404 error with the given message.
func NotFound(msg string) *apierr.APIError {
	return &apierr.APIError{Code: "not_found", Status: 404, Message: msg}
}

// Forbidden returns a 403 error with the given message.
func Forbidden(msg string) *apierr.APIError {
	return &apierr.APIError{Code: "forbidden", Status: 403, Message: msg}
}

// BadRequest returns a 400 error with the given message.
func BadRequest(msg string) *apierr.APIError {
	return &apierr.APIError{Code: "bad_request", Status: 400, Message: msg}
}

// Conflict returns a 409 error with the given message.
func Conflict(msg string) *apierr.APIError {
	return &apierr.APIError{Code: "conflict", Status: 409, Message: msg}
}
