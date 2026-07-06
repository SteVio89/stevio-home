package middleware

import (
	"context"
	"database/sql"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/auth"
	"github.com/SteVio89/stevio-home/crypto"
	"github.com/SteVio89/stevio-home/settings"
)

var errMaintenance = &apierr.APIError{
	Code:    "maintenance",
	Status:  503,
	Message: "The service is temporarily unavailable for maintenance.",
}

// MaintenanceChecker reads the maintenance_mode setting with a short TTL
// cache to avoid a DB round-trip on every request.
type MaintenanceChecker struct {
	db               *sql.DB
	adminEmailHashes []string
	secret           []byte
	settings         *settings.Store

	mu       sync.Mutex
	enabled  bool
	cachedAt time.Time
}

const maintenanceCacheTTL = 5 * time.Second

// NewMaintenanceChecker creates a MaintenanceChecker backed by the stevio-home
// site_settings table, keyed by the default session cookie.
func NewMaintenanceChecker(db *sql.DB, adminEmailHashes []string, secret []byte) (*MaintenanceChecker, error) {
	s, err := settings.NewStore(db, "site_settings")
	if err != nil {
		return nil, err
	}
	return &MaintenanceChecker{
		db:               db,
		adminEmailHashes: adminEmailHashes,
		secret:           secret,
		settings:         s,
	}, nil
}

func (mc *MaintenanceChecker) isEnabled(ctx context.Context) bool {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	if time.Since(mc.cachedAt) < maintenanceCacheTTL {
		return mc.enabled
	}
	val, err := mc.settings.Get(ctx, "maintenance_mode")
	enabled := err == nil && val == "1"
	// Safety coupling: the mock payment provider mints real signed licenses
	// without payment, so the store must never be publicly reachable while it is
	// active. Force maintenance mode whenever mock is the selected provider. This
	// keeps the public store and the maintenance-blocked mock trigger route behind
	// the wall (admins remain exempt and can rehearse checkout), and it cannot be
	// switched off in the UI until the provider is changed away from mock.
	if !enabled {
		if provider, perr := mc.settings.Get(ctx, "payment_provider"); perr == nil && provider == "mock" {
			enabled = true
		}
	}
	mc.enabled = enabled
	mc.cachedAt = time.Now()
	return mc.enabled
}

// isRequestFromAdmin returns true if the request carries a valid admin session cookie.
func (mc *MaintenanceChecker) isRequestFromAdmin(r *http.Request) bool {
	cookie, err := r.Cookie(DefaultCookieName)
	if err != nil {
		return false
	}
	sessionID, ok := crypto.VerifySession(cookie.Value, mc.secret)
	if !ok {
		return false
	}
	session, err := auth.GetSession(r.Context(), mc.db, sessionID)
	if err != nil || session == nil {
		return false
	}
	return slices.Contains(mc.adminEmailHashes, session.EmailHash)
}

// Middleware returns a Middleware that blocks non-admin requests with 503
// when maintenance mode is enabled.
func (mc *MaintenanceChecker) Middleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if mc.isEnabled(r.Context()) && !mc.isRequestFromAdmin(r) {
				apierr.Write(w, errMaintenance)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
