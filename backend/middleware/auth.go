package middleware

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/auth"
	"github.com/SteVio89/stevio-home/crypto"
)

type contextKey string

const (
	contextKeyEmailHash contextKey = "email_hash"
	contextKeySessionID contextKey = "session_id"
	contextKeyUserID    contextKey = "user_id"
	contextKeyUserType  contextKey = "user_type"

	// DefaultCookieName is the default session cookie name.
	DefaultCookieName = "sid"
)

// RequireAuth returns middleware that validates the session cookie.
// Uses DefaultCookieName ("sid") for the cookie name.
func RequireAuth(db *sql.DB, secret []byte, logger *log.Logger) Middleware {
	return RequireAuthWithCookie(db, secret, DefaultCookieName, logger)
}

// RequireAuthWithCookie returns middleware that validates a session cookie
// with a custom cookie name.
func RequireAuthWithCookie(db *sql.DB, secret []byte, cookieName string, logger *log.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(cookieName)
			if err != nil {
				apierr.Write(w, apierr.ErrUnauthorized())
				return
			}

			sessionID, ok := crypto.VerifySession(cookie.Value, secret)
			if !ok {
				apierr.Write(w, apierr.ErrUnauthorized())
				return
			}

			session, err := auth.GetSession(r.Context(), db, sessionID)
			if err != nil {
				if errors.Is(err, auth.ErrSessionNotFound) {
					apierr.Write(w, apierr.ErrUnauthorized())
					return
				}
				apierr.Write(w, apierr.ErrInternal())
				return
			}

			ctx := context.WithValue(r.Context(), contextKeyEmailHash, session.EmailHash)
			ctx = context.WithValue(ctx, contextKeySessionID, session.ID)
			if session.UserID != nil {
				ctx = context.WithValue(ctx, contextKeyUserID, *session.UserID)
			}
			if session.UserType != nil {
				ctx = context.WithValue(ctx, contextKeyUserType, *session.UserType)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// EmailHashFromContext returns the email hash stored by RequireAuth.
func EmailHashFromContext(ctx context.Context) string {
	v, _ := ctx.Value(contextKeyEmailHash).(string)
	return v
}

// SessionIDFromContext returns the session ID stored by RequireAuth.
func SessionIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(contextKeySessionID).(string)
	return v
}

// UserIDFromContext returns the user_id stored in the session, or empty string
// if the session was created without one.
func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(contextKeyUserID).(string)
	return v
}

// UserTypeFromContext returns the user_type stored in the session, or empty
// string if the session was created without one.
func UserTypeFromContext(ctx context.Context) string {
	v, _ := ctx.Value(contextKeyUserType).(string)
	return v
}
