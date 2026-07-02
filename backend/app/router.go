package app

import (
	"net/http"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/middleware"
)

// AuthScope is a scoped route registrar bound to a specific auth requirement.
// Obtain instances only through App.Public(), App.Authenticated(), or App.Role().
// There is no exported constructor — this enforces that every route has an explicit
// auth decision at compile time.
type AuthScope struct {
	app      *App
	authKind string // "public", "authenticated", "role"
	role     string // only set when authKind == "role"
	prefix   string
	extraMW  []middleware.Middleware
}

// Handle registers a route with the scope's auth requirement applied.
// Pattern follows Go 1.22+ ServeMux syntax: "METHOD /path/{param}".
func (s *AuthScope) Handle(pattern string, h HandlerFunc, opts ...RouteOption) {
	rc := &routeCfg{}
	for _, o := range opts {
		o(rc)
	}

	fullPattern := pattern
	if s.prefix != "" {
		method, path := splitPattern(pattern)
		fullPattern = method + s.prefix + path
	}

	// Build the per-route middleware chain.
	var mws []middleware.Middleware

	// Rate limit (per-route override or global default).
	if rc.rateOverride != nil {
		mws = append(mws, rc.rateOverride)
	} else if s.app.globalRate != nil {
		mws = append(mws, s.app.globalRate)
	}

	// Auth middleware based on scope kind.
	switch s.authKind {
	case "authenticated":
		mws = append(mws, middleware.RequireAuthWithCookie(s.app.db.DB, s.app.secret, s.app.cookieName, s.app.logger))
	case "role":
		mws = append(mws, middleware.RequireAuthWithCookie(s.app.db.DB, s.app.secret, s.app.cookieName, s.app.logger))
		mws = append(mws, requireRole(s.role))
	}

	// Scope-level extra middleware.
	mws = append(mws, s.extraMW...)

	// Per-route extra middleware.
	mws = append(mws, rc.extraMW...)

	isPublic := s.authKind == "public"
	handler := s.app.adapt(h, isPublic)

	s.app.mux.Handle(fullPattern, middleware.Chain(handler, mws...))
}

// With returns a copy of the AuthScope with additional middleware appended.
func (s *AuthScope) With(mws ...middleware.Middleware) *AuthScope {
	cp := *s
	cp.extraMW = make([]middleware.Middleware, len(s.extraMW)+len(mws))
	copy(cp.extraMW, s.extraMW)
	copy(cp.extraMW[len(s.extraMW):], mws)
	return &cp
}

// RouteOption modifies a single route registration.
type RouteOption func(*routeCfg)

type routeCfg struct {
	rateOverride middleware.Middleware
	extraMW      []middleware.Middleware
}

// WithRateLimit overrides the global rate limiter for a single route.
// The rate limiter is created when the option is applied to a route via Handle(),
// and tracked by the App for cleanup on Close().
func WithRateLimit(app *App, requestsPerMinute int, trustedProxy bool) RouteOption {
	return func(rc *routeCfg) {
		rl := middleware.NewRateLimiter(requestsPerMinute, trustedProxy)
		app.extraRLs = append(app.extraRLs, rl)
		rc.rateOverride = middleware.RateLimit(rl)
	}
}

// WithMiddleware appends additional middleware to a single route.
func WithMiddleware(mws ...middleware.Middleware) RouteOption {
	return func(rc *routeCfg) {
		rc.extraMW = append(rc.extraMW, mws...)
	}
}

// RouterGroup is a prefix-scoped collection of AuthScopes.
type RouterGroup struct {
	app    *App
	prefix string
}

// Public returns an AuthScope for public (no auth) routes in this group.
func (g *RouterGroup) Public() *AuthScope {
	return &AuthScope{app: g.app, authKind: "public", prefix: g.prefix}
}

// Authenticated returns an AuthScope for authenticated routes in this group.
func (g *RouterGroup) Authenticated() *AuthScope {
	return &AuthScope{app: g.app, authKind: "authenticated", prefix: g.prefix}
}

// Role returns an AuthScope for role-restricted routes in this group.
func (g *RouterGroup) Role(role string) *AuthScope {
	return &AuthScope{app: g.app, authKind: "role", role: role, prefix: g.prefix}
}

// requireRole returns middleware that checks user_type from context equals role.
func requireRole(role string) middleware.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if middleware.UserTypeFromContext(r.Context()) != role {
				apierr.Write(w, apierr.ErrForbidden())
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// splitPattern extracts "METHOD " prefix and path from a Go 1.22 pattern.
// e.g. "GET /items/{id}" → ("GET ", "/items/{id}")
// e.g. "/items/{id}" → ("", "/items/{id}")
func splitPattern(pattern string) (method, path string) {
	for i, c := range pattern {
		if c == '/' {
			return pattern[:i], pattern[i:]
		}
	}
	return "", pattern
}
