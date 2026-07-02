package app

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/middleware"
	"github.com/SteVio89/stevio-home/validate"
)

// HandlerFunc is the handler signature for stevio routes.
// Returning nil means the handler already wrote the response.
// Returning an error delegates response writing to the framework.
type HandlerFunc func(c *Ctx) error

// adapt converts a HandlerFunc into an http.Handler, constructing
// the Ctx and dispatching errors.
func (a *App) adapt(h HandlerFunc, isPublic bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var lang string
		if a.locales != nil {
			lang = a.locales.FromRequest(r)
		} else {
			lang = a.catalog.Resolve(r.Header.Get("Accept-Language"))
		}

		c := &Ctx{
			R:    r,
			W:    w,
			app:  a,
			lang: lang,
		}

		if !isPublic {
			c.user = &User{
				ID:        middleware.UserIDFromContext(r.Context()),
				EmailHash: middleware.EmailHashFromContext(r.Context()),
				Role:      middleware.UserTypeFromContext(r.Context()),
				SessionID: middleware.SessionIDFromContext(r.Context()),
			}
		}

		err := h(c)
		if err == nil {
			return
		}

		var apiErr *apierr.APIError
		if errors.As(err, &apiErr) {
			apierr.Write(w, apiErr)
			return
		}

		var valErrs *validate.Errors
		if errors.As(err, &valErrs) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error":  "validation_failed",
				"fields": valErrs.Fields(),
			})
			return
		}

		a.logger.Printf("handler error: %v", err)
		apierr.Write(w, apierr.ErrInternal())
	})
}
