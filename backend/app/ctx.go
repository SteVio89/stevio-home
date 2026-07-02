package app

import (
	"encoding/json"
	"mime"
	"net/http"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/i18n"
	"github.com/SteVio89/stevio-home/settings"
	"github.com/SteVio89/stevio-home/validate"
)

// User is the authenticated user attached to Authenticated and Role requests.
// It is nil on Public routes.
type User struct {
	ID        string `json:"id"`
	EmailHash string `json:"email_hash"`
	Role      string `json:"role"`
	SessionID string `json:"session_id"`
}

// Ctx is the framework request context passed to every HandlerFunc.
// R and W are always available as escape hatches to the raw stdlib types.
type Ctx struct {
	R    *http.Request
	W    http.ResponseWriter
	app  *App
	user *User
	lang string
}

// User returns the authenticated user. Returns nil on Public routes.
// On Authenticated and Role routes, the user is guaranteed non-nil.
func (c *Ctx) User() *User {
	return c.user
}

// DB returns the application's database connection.
// The returned *DB embeds *sql.DB and signals a sync push after writes.
func (c *Ctx) DB() *DB {
	return c.app.db
}

// T returns the translated string for the given key in the request's language.
// Falls back to the catalog's fallback language, then returns the key itself.
func (c *Ctx) T(key string, args ...any) string {
	return c.app.catalog.Lookup(c.lang, key, args...)
}

// Lang returns the resolved BCP-47 language tag for this request.
func (c *Ctx) Lang() string {
	return c.lang
}

// Param returns a URL path parameter from Go 1.22+ patterns.
// For pattern "GET /items/{id}", c.Param("id") returns the matched segment.
func (c *Ctx) Param(name string) string {
	return c.R.PathValue(name)
}

// JSON writes a JSON response with the given status code.
// Always returns nil — the error return enables `return c.JSON(...)` in handlers.
func (c *Ctx) JSON(status int, v any) error {
	apierr.JSON(c.W, status, v)
	return nil
}

// NoContent writes a 204 No Content response.
func (c *Ctx) NoContent() error {
	c.W.WriteHeader(http.StatusNoContent)
	return nil
}

// maxRequestBody is the maximum allowed request body size (1 MB).
const maxRequestBody = 1 << 20

// Decode decodes the JSON request body into v.
// Limits the body to 1 MB. Requires Content-Type application/json.
// Returns a 400 Bad Request error on wrong content type, malformed,
// or oversized JSON.
func (c *Ctx) Decode(v any) error {
	ct := c.R.Header.Get("Content-Type")
	if ct == "" {
		return apierr.ErrBadRequest()
	}
	mediaType, _, _ := mime.ParseMediaType(ct)
	if mediaType != "application/json" {
		return apierr.ErrBadRequest()
	}
	c.R.Body = http.MaxBytesReader(c.W, c.R.Body, maxRequestBody)
	dec := json.NewDecoder(c.R.Body)
	if err := dec.Decode(v); err != nil {
		return apierr.ErrBadRequest()
	}
	if dec.More() {
		return apierr.ErrBadRequest()
	}
	return nil
}

// DecodeValidate decodes the JSON body into v, then runs the validation function.
// The framework provides a *validate.Errors for fn to populate. Returns the
// validation error (rendered as 422 by the framework) if any errors were added.
func (c *Ctx) DecodeValidate(v any, fn func(*validate.Errors)) error {
	if err := c.Decode(v); err != nil {
		return err
	}
	ve := &validate.Errors{}
	fn(ve)
	return ve.Err()
}

// Locales returns the locale registry cache, or nil if Config.I18n was not set.
func (c *Ctx) Locales() *i18n.LocaleCache {
	return c.app.locales
}

// Settings returns the runtime settings store.
func (c *Ctx) Settings() *settings.Store {
	return c.app.sets
}
