package middleware

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SteVio89/stevio-home/auth"
	"github.com/SteVio89/stevio-home/crypto"
	"github.com/SteVio89/stevio-home/testutil"
)

var logger = log.New(io.Discard, "", 0)

func TestRealIP(t *testing.T) {
	tests := []struct {
		name         string
		remoteAddr   string
		xRealIP      string
		trustedProxy bool
		want         string
	}{
		{"direct - remote addr with port", "192.168.1.1:12345", "", false, "192.168.1.1"},
		{"direct - ignores X-Real-IP", "192.168.1.1:12345", "10.0.0.1", false, "192.168.1.1"},
		{"proxy - uses X-Real-IP", "192.168.1.1:12345", "10.0.0.1", true, "10.0.0.1"},
		{"proxy - falls back to remote addr", "192.168.1.1:12345", "", true, "192.168.1.1"},
		{"no port in remote addr", "192.168.1.1", "", false, "192.168.1.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = tt.remoteAddr
			if tt.xRealIP != "" {
				r.Header.Set("X-Real-IP", tt.xRealIP)
			}
			got := RealIP(r, tt.trustedProxy)
			if got != tt.want {
				t.Errorf("RealIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRateLimiterAllow(t *testing.T) {
	rl := NewRateLimiter(5, false) // 5 req/min

	// First 5 requests should be allowed
	for i := range 5 {
		if !rl.Allow("127.0.0.1") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	// 6th request should be denied
	if rl.Allow("127.0.0.1") {
		t.Error("6th request should be rate-limited")
	}

	// Different IP should still be allowed
	if !rl.Allow("10.0.0.1") {
		t.Error("different IP should be allowed")
	}
}

func TestRateLimiterStop(t *testing.T) {
	rl := NewRateLimiter(10, false)
	// Should not panic when called once.
	rl.Stop()
}

func TestRateLimitMiddleware(t *testing.T) {
	rl := NewRateLimiter(1, false)

	handler := RateLimit(rl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request passes
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("first request: got %d, want %d", rec.Code, http.StatusOK)
	}

	// Second request should be rate-limited
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("second request: got %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	if rec.Header().Get("Retry-After") != "60" {
		t.Errorf("Retry-After = %q, want %q", rec.Header().Get("Retry-After"), "60")
	}
}

func TestCORSMiddleware(t *testing.T) {
	handler := CORS("http://localhost:5173")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("matching origin", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Origin", "http://localhost:5173")
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("got %d, want %d", rec.Code, http.StatusOK)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
			t.Errorf("ACAO = %q, want %q", got, "http://localhost:5173")
		}
		if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
			t.Errorf("ACAC = %q, want %q", got, "true")
		}
		if got := rec.Header().Get("Vary"); got != "Origin" {
			t.Errorf("Vary = %q, want %q", got, "Origin")
		}
	})

	t.Run("non-matching origin", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Origin", "https://evil.com")
		handler.ServeHTTP(rec, req)
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("ACAO should be empty for non-matching origin, got %q", got)
		}
	})

	t.Run("no origin header", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
		if rec.Code != http.StatusOK {
			t.Errorf("got %d, want %d", rec.Code, http.StatusOK)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("ACAO should be empty without Origin header, got %q", got)
		}
	})

	t.Run("preflight OPTIONS", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("OPTIONS", "/", nil)
		req.Header.Set("Origin", "http://localhost:5173")
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Errorf("got %d, want %d", rec.Code, http.StatusNoContent)
		}
	})
}

func TestCORSEmptyOrigin_NoHeader(t *testing.T) {
	handler := CORS("")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://evil.com")
	handler.ServeHTTP(rec, req)
	if h := rec.Header().Get("Access-Control-Allow-Origin"); h != "" {
		t.Errorf("expected no ACAO header with empty origin config, got %q", h)
	}
}

func TestRecoverMiddleware(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	handler := Recover(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestChain(t *testing.T) {
	var order []string

	mwA := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "A")
			next.ServeHTTP(w, r)
		})
	}
	mwB := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "B")
			next.ServeHTTP(w, r)
		})
	}

	handler := Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
	}), mwA, mwB)

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))

	if len(order) != 3 || order[0] != "A" || order[1] != "B" || order[2] != "handler" {
		t.Errorf("execution order = %v, want [A B handler]", order)
	}
}

func TestCORSWildcard_NoCredentials(t *testing.T) {
	handler := CORS("*")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://any-origin.com")
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("ACAO = %q, want %q", got, "*")
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("ACAC should be empty for wildcard origin, got %q", got)
	}
}

func TestSecurityHeaders(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	tests := []struct {
		header, want string
	}{
		{"X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "DENY"},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
	}
	for _, tt := range tests {
		if got := rec.Header().Get(tt.header); got != tt.want {
			t.Errorf("%s = %q, want %q", tt.header, got, tt.want)
		}
	}
}

func TestRequireAuthWithCookie_ValidSession(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long-enough")
	db := testutil.SetupTestDB(t, "auth", auth.MigrationFiles)

	// Create a session.
	sessionID := "test-session-id"
	emailHash := "test-email-hash"
	expires := time.Now().Add(time.Hour).UTC()
	if err := auth.CreateSession(context.Background(), db, sessionID, emailHash, expires,
		auth.WithSessionUser("user-1", "member")); err != nil {
		t.Fatal(err)
	}

	handler := RequireAuthWithCookie(db, secret, "sid", logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := EmailHashFromContext(r.Context()); got != emailHash {
			t.Errorf("EmailHashFromContext = %q, want %q", got, emailHash)
		}
		if got := SessionIDFromContext(r.Context()); got != sessionID {
			t.Errorf("SessionIDFromContext = %q, want %q", got, sessionID)
		}
		if got := UserIDFromContext(r.Context()); got != "user-1" {
			t.Errorf("UserIDFromContext = %q, want %q", got, "user-1")
		}
		if got := UserTypeFromContext(r.Context()); got != "member" {
			t.Errorf("UserTypeFromContext = %q, want %q", got, "member")
		}
		w.WriteHeader(http.StatusOK)
	}))

	signed := crypto.SignSession(sessionID, secret)
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "sid", Value: signed})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("valid session: got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRequireAuthWithCookie_NoCookie(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long-enough")
	db := testutil.SetupTestDB(t, "auth", auth.MigrationFiles)

	handler := RequireAuthWithCookie(db, secret, "sid", logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no cookie: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireAuthWithCookie_InvalidSignature(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long-enough")
	db := testutil.SetupTestDB(t, "auth", auth.MigrationFiles)

	handler := RequireAuthWithCookie(db, secret, "sid", logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "sid", Value: "fake-session.invalidsig"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("invalid signature: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireAuthWithCookie_ExpiredSession(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long-enough")
	db := testutil.SetupTestDB(t, "auth", auth.MigrationFiles)

	sessionID := "expired-session"
	expires := time.Now().Add(-time.Hour).UTC() // already expired
	if err := auth.CreateSession(context.Background(), db, sessionID, "hash", expires); err != nil {
		t.Fatal(err)
	}

	handler := RequireAuthWithCookie(db, secret, "sid", logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for expired session")
	}))

	signed := crypto.SignSession(sessionID, secret)
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "sid", Value: signed})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expired session: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRateLimiterStop_DoubleCall(t *testing.T) {
	rl := NewRateLimiter(10, false)
	rl.Stop()
	rl.Stop() // must not panic
}

func TestRateLimiterAllow_Cleanup(t *testing.T) {
	rl := NewRateLimiter(10, false)
	defer rl.Stop()
	if !rl.Allow("127.0.0.1") {
		t.Error("first request should be allowed")
	}
}

func TestRealIP_XForwardedFor(t *testing.T) {
	tests := []struct {
		name    string
		xff     string
		xRealIP string
		want    string
	}{
		{"single IP", "203.0.113.50", "", "203.0.113.50"},
		{"multiple IPs", "203.0.113.50, 70.41.3.18, 150.172.238.178", "", "203.0.113.50"},
		{"X-Real-IP takes priority", "203.0.113.50", "10.0.0.1", "10.0.0.1"},
		{"whitespace trimmed", "  203.0.113.50 , 70.41.3.18 ", "", "203.0.113.50"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = "127.0.0.1:1234"
			if tt.xff != "" {
				r.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xRealIP != "" {
				r.Header.Set("X-Real-IP", tt.xRealIP)
			}
			got := RealIP(r, true)
			if got != tt.want {
				t.Errorf("RealIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRealIP_XForwardedFor_NotTrusted(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("X-Forwarded-For", "203.0.113.50")
	got := RealIP(r, false)
	if got != "127.0.0.1" {
		t.Errorf("RealIP() = %q, want %q (should ignore XFF when not trusted)", got, "127.0.0.1")
	}
}

func TestMaintenanceMiddleware(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long-enough")
	db := testutil.SetupTestDB(t, "auth", auth.MigrationFiles)

	// Create the settings table.
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS site_settings (key TEXT PRIMARY KEY, value TEXT)`)
	if err != nil {
		t.Fatal(err)
	}

	// Create an admin session.
	adminEmailHash := "admin-hash-123"
	adminSessionID := "admin-sess-1"
	expires := time.Now().Add(time.Hour).UTC()
	if err := auth.CreateSession(context.Background(), db, adminSessionID, adminEmailHash, expires); err != nil {
		t.Fatal(err)
	}
	adminCookie := crypto.SignSession(adminSessionID, secret)

	mc, err := NewMaintenanceChecker(db, []string{adminEmailHash}, secret)
	if err != nil {
		t.Fatal(err)
	}

	handler := mc.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("disabled - requests pass through", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
		if rec.Code != http.StatusOK {
			t.Errorf("got %d, want %d", rec.Code, http.StatusOK)
		}
	})

	// Enable maintenance mode.
	_, err = db.Exec(`INSERT INTO site_settings (key, value) VALUES ('maintenance_mode', '1')`)
	if err != nil {
		t.Fatal(err)
	}
	// Reset cache by waiting for TTL (we force it by manipulating cachedAt).
	mc.mu.Lock()
	mc.cachedAt = time.Time{}
	mc.mu.Unlock()

	t.Run("enabled - regular requests blocked", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("got %d, want %d", rec.Code, http.StatusServiceUnavailable)
		}
	})

	t.Run("enabled - admin bypasses", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "sid", Value: adminCookie})
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("admin bypass: got %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("enabled - non-admin cookie rejected", func(t *testing.T) {
		// Create a non-admin session.
		nonAdminSessionID := "user-sess-1"
		if err := auth.CreateSession(context.Background(), db, nonAdminSessionID, "user-hash-456", expires); err != nil {
			t.Fatal(err)
		}
		nonAdminCookie := crypto.SignSession(nonAdminSessionID, secret)

		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "sid", Value: nonAdminCookie})
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("non-admin: got %d, want %d", rec.Code, http.StatusServiceUnavailable)
		}
	})

	// Disable maintenance mode.
	_, err = db.Exec(`UPDATE site_settings SET value = '0' WHERE key = 'maintenance_mode'`)
	if err != nil {
		t.Fatal(err)
	}
	mc.mu.Lock()
	mc.cachedAt = time.Time{}
	mc.mu.Unlock()

	t.Run("disabled again - requests pass through", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
		if rec.Code != http.StatusOK {
			t.Errorf("got %d, want %d", rec.Code, http.StatusOK)
		}
	})
}

func TestMaintenanceChecker_CacheTTL(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long-enough")
	db := testutil.SetupTestDB(t, "auth", auth.MigrationFiles)

	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS site_settings (key TEXT PRIMARY KEY, value TEXT)`)
	if err != nil {
		t.Fatal(err)
	}

	mc, err := NewMaintenanceChecker(db, nil, secret)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// First call — not enabled, caches result.
	if mc.isEnabled(ctx) {
		t.Error("should not be enabled initially")
	}

	// Enable in DB but cache should still return false.
	_, err = db.Exec(`INSERT INTO site_settings (key, value) VALUES ('maintenance_mode', '1')`)
	if err != nil {
		t.Fatal(err)
	}

	if mc.isEnabled(ctx) {
		t.Error("should still be cached as disabled")
	}

	// Expire the cache.
	mc.mu.Lock()
	mc.cachedAt = time.Time{}
	mc.mu.Unlock()

	if !mc.isEnabled(ctx) {
		t.Error("should be enabled after cache expiry")
	}
}

func TestRealIP_IPv6(t *testing.T) {
	tests := []struct {
		name         string
		remoteAddr   string
		xRealIP      string
		trustedProxy bool
		want         string
	}{
		{"IPv6 with port", "[::1]:12345", "", false, "::1"},
		{"proxy - X-Real-IP with port", "10.0.0.1:1234", "192.168.1.1:8080", true, "192.168.1.1"},
		{"proxy - X-Real-IP bare IPv6", "10.0.0.1:1234", "::1", true, "::1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = tt.remoteAddr
			if tt.xRealIP != "" {
				r.Header.Set("X-Real-IP", tt.xRealIP)
			}
			got := RealIP(r, tt.trustedProxy)
			if got != tt.want {
				t.Errorf("RealIP() = %q, want %q", got, tt.want)
			}
		})
	}
}
