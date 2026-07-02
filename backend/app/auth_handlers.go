package app

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/auth"
	"github.com/SteVio89/stevio-home/crypto"
	"github.com/SteVio89/stevio-home/dbutil"
	"github.com/SteVio89/stevio-home/validate"
)

// Built-in i18n keys used by the auth handlers.
const (
	KeyLoginSubject = "auth.login_subject"
	KeyLoginBody    = "auth.login_body"
)

// loginHandler handles POST /auth/login.
// Accepts JSON: {"email": "user@example.com"}
func (a *App) loginHandler(c *Ctx) error {
	var req struct {
		Email string `json:"email"`
	}
	if err := c.Decode(&req); err != nil {
		return err
	}

	req.Email = strings.TrimSpace(req.Email)
	if err := validate.Email(req.Email); err != nil {
		return apierr.ErrBadRequest()
	}

	emailHash := crypto.HashEmail(req.Email, a.salt)

	// Check for an existing valid token to prevent email spam. If one exists we
	// skip sending a second email, but we still return the same "sent" response
	// as a fresh request: revealing "already_sent" would let an attacker probe
	// whether an address recently initiated a login (recent-activity enumeration).
	// Residual response-time differences between the two paths are not equalized.
	exists, err := auth.HasValidAuthToken(c.R.Context(), a.db.DB, emailHash)
	if err != nil {
		return err
	}
	if exists {
		return c.JSON(http.StatusOK, map[string]string{
			"status": "sent",
		})
	}

	token := dbutil.NewSecureToken()
	expiresAt := time.Now().UTC().Add(a.tokenDuration)

	if err := auth.InsertAuthToken(c.R.Context(), a.db.DB, token, emailHash, expiresAt); err != nil {
		return err
	}

	// Build the magic link.
	link := a.buildMagicLink(c.R, token)

	// Send the email using i18n translations.
	var subject, body string
	if a.mailTpl != nil {
		var resolveErr error
		subject, body, resolveErr = a.mailTpl.Resolve(c.R.Context(), c.lang, "magic_link")
		if resolveErr != nil {
			a.logger.Printf("stevio: resolve mail template: %v", resolveErr)
		}
		if subject != "" && body != "" {
			body = fmt.Sprintf(body, link)
		}
	}
	if subject == "" {
		subject = a.catalog.Lookup(c.lang, KeyLoginSubject)
		body = a.catalog.Lookup(c.lang, KeyLoginBody, link)
	}

	if a.mailer != nil {
		if err := a.mailer.Send(req.Email, subject, body); err != nil {
			a.logger.Printf("stevio: send login email: %v", err)
			return apierr.ErrInternal()
		}
	}

	return c.JSON(http.StatusOK, map[string]string{
		"status": "sent",
	})
}

// verifyHandler handles POST /auth/verify.
// Accepts JSON: {"token": "..."}
func (a *App) verifyHandler(c *Ctx) error {
	var req struct {
		Token string `json:"token"`
	}
	if err := c.Decode(&req); err != nil {
		return err
	}

	req.Token = strings.TrimSpace(req.Token)
	if req.Token == "" {
		return apierr.ErrBadRequest()
	}

	emailHash, _, err := auth.ConsumeAuthTokenFull(c.R.Context(), a.db.DB, req.Token)
	if err != nil {
		// Distinguish auth sentinel errors from actual DB failures.
		if errors.Is(err, auth.ErrTokenNotFound) || errors.Is(err, auth.ErrTokenUsed) || errors.Is(err, auth.ErrTokenExpired) {
			return apierr.ErrTokenInvalid()
		}
		return err // surfaces as 500
	}

	// Find or create the user.
	user, _, err := a.users.FindOrCreate(c.R.Context(), emailHash, a.defaultRole)
	if err != nil {
		return err
	}

	// Create session with role-specific duration.
	sessionID := dbutil.NewID()
	dur := a.sessionDurationForRole(user.Role)
	expiresAt := time.Now().UTC().Add(dur)

	if err := auth.CreateSession(c.R.Context(), a.db.DB, sessionID, emailHash, expiresAt,
		auth.WithSessionUser(user.ID, user.Role)); err != nil {
		return err
	}

	// Sign and set cookie.
	// CSRF posture: SameSite=Lax prevents cross-site form submissions.
	// Combined with JSON-only endpoints (Content-Type check), this provides
	// sufficient CSRF protection for most SaaS use cases. If sub-domain
	// isolation is needed, add explicit CSRF token middleware.
	signed := crypto.SignSession(sessionID, a.secret)
	http.SetCookie(c.W, &http.Cookie{
		Name:     a.cookieName,
		Value:    signed,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(dur.Seconds()),
	})

	return c.JSON(http.StatusOK, map[string]any{
		"user_id": user.ID,
		"role":    user.Role,
	})
}

// logoutHandler handles POST /auth/logout.
func (a *App) logoutHandler(c *Ctx) error {
	if err := auth.DeleteSession(c.R.Context(), a.db.DB, c.user.SessionID); err != nil {
		return err
	}

	// Clear the cookie.
	http.SetCookie(c.W, &http.Cookie{
		Name:     a.cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   a.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	return c.NoContent()
}

// buildMagicLink assembles the magic link URL.
// Prefers Config.BaseURL for security. Falls back to request Host if not configured.
func (a *App) buildMagicLink(r *http.Request, token string) string {
	if a.baseURL != "" {
		return a.baseURL + "/auth/verify?token=" + token
	}

	// Fallback: derive from request (less secure — Host header can be spoofed).
	scheme := "https"
	if a.cfg.TrustedProxy {
		if proto := r.Header.Get("X-Forwarded-Proto"); proto == "http" || proto == "https" {
			scheme = proto
		}
	}
	return fmt.Sprintf("%s://%s/auth/verify?token=%s", scheme, r.Host, token)
}
