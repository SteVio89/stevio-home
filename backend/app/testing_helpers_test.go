package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/SteVio89/stevio-home/auth"
	"github.com/SteVio89/stevio-home/crypto"
	"github.com/SteVio89/stevio-home/dbutil"
	"github.com/SteVio89/stevio-home/migrate"
	"github.com/SteVio89/stevio-home/testutil"
	"github.com/SteVio89/stevio-home/users"
)

// TestMailer records emails instead of sending them via SMTP.
// Safe for concurrent use.
type TestMailer struct {
	mu   sync.Mutex
	sent []TestEmail
}

// TestEmail represents a recorded email.
type TestEmail struct {
	To      string
	Subject string
	Body    string
}

// Send records the email instead of sending it.
func (m *TestMailer) Send(to, subject, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, TestEmail{To: to, Subject: subject, Body: body})
	return nil
}

// SentEmails returns a copy of the sent emails (thread-safe).
func (m *TestMailer) SentEmails() []TestEmail {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]TestEmail, len(m.sent))
	copy(cp, m.sent)
	return cp
}

// SetupTestApp creates a fully-wired App backed by a per-test Postgres schema.
// Rate limiting is effectively disabled. The returned TestMailer captures sent emails.
func SetupTestApp(t *testing.T, cfg Config) (*App, *TestMailer) {
	t.Helper()

	// Set up test database with auth migrations.
	db := testutil.SetupTestDB(t, "auth", auth.MigrationFiles)

	// Run userstore migrations.
	if err := migrate.RunMigrations(db, "users", users.MigrationFiles, nil); err != nil {
		t.Fatalf("users test migrations: %v", err)
	}

	// Run default app migrations.
	if cfg.DefaultMigrations != nil {
		if err := migrate.RunMigrations(db, "app", cfg.DefaultMigrations, nil); err != nil {
			t.Fatalf("app test default migration: %v", err)
		}
	}

	// Run additional app migrations.
	for _, src := range cfg.AppMigrations {
		if err := migrate.RunMigrations(db, src.Prefix, src.Files, nil); err != nil {
			t.Fatalf("app test migration %s: %v", src.Prefix, err)
		}
	}

	tm := &TestMailer{}

	// Override config for testing.
	cfg.db = newDB(db)
	if cfg.DatabaseURL == "" {
		cfg.DatabaseURL = "test://"
	}
	if len(cfg.Secret) == 0 {
		cfg.Secret = bytes.Repeat([]byte("t"), 32)
	}
	if cfg.EmailSalt == "" {
		cfg.EmailSalt = "test-salt"
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:8080"
	}
	cfg.Mailer = tm
	cfg.RateLimit = 999999 // effectively disable rate limiting in tests
	cfg.InsecureCookies = true

	// Build the app without running migrations again (already done above).
	app := newApp(cfg)

	t.Cleanup(func() { _ = app.Close() })

	return app, tm
}

// TestClient wraps httptest and provides helpers for making requests.
type TestClient struct {
	Server  *httptest.Server
	t       *testing.T
	app     *App
	cookies []*http.Cookie
}

// NewTestClient starts a test server for the given app.
func NewTestClient(t *testing.T, app *App) *TestClient {
	t.Helper()
	server := httptest.NewServer(app.Handler())
	t.Cleanup(server.Close)
	return &TestClient{Server: server, t: t, app: app}
}

// AsUser creates a real session for a user with the given role and sets the cookie.
// If no user with this role exists, one is created.
func (tc *TestClient) AsUser(email, role string) *TestClient {
	tc.t.Helper()

	emailHash := crypto.HashEmail(email, tc.app.salt)

	// Find or create user with the desired role.
	user, created, err := tc.app.users.FindOrCreate(
		context.Background(), emailHash, role)
	if err != nil {
		tc.t.Fatalf("TestClient.AsUser: create user: %v", err)
	}
	if !created && user.Role != role {
		if _, err := tc.app.users.UpdateRole(
			context.Background(), user.ID, role); err != nil {
			tc.t.Fatalf("TestClient.AsUser: update role: %v", err)
		}
		user.Role = role
	}

	// Create session with role-specific duration.
	sessionID := dbutil.NewID()
	expiresAt := time.Now().Add(tc.app.sessionDurationForRole(role))
	if err := auth.CreateSession(context.Background(), tc.app.db.DB, sessionID, emailHash, expiresAt,
		auth.WithSessionUser(user.ID, user.Role)); err != nil {
		tc.t.Fatalf("TestClient.AsUser: create session: %v", err)
	}

	signed := crypto.SignSession(sessionID, tc.app.secret)
	tc.cookies = []*http.Cookie{{
		Name:  tc.app.cookieName,
		Value: signed,
	}}

	return tc
}

// GET sends a GET request.
func (tc *TestClient) GET(path string) *http.Response {
	return tc.do("GET", path, nil)
}

// POST sends a POST request with a JSON body.
func (tc *TestClient) POST(path string, body any) *http.Response {
	return tc.do("POST", path, body)
}

// PATCH sends a PATCH request with a JSON body.
func (tc *TestClient) PATCH(path string, body any) *http.Response {
	return tc.do("PATCH", path, body)
}

// DELETE sends a DELETE request.
func (tc *TestClient) DELETE(path string) *http.Response {
	return tc.do("DELETE", path, nil)
}

func (tc *TestClient) do(method, path string, body any) *http.Response {
	tc.t.Helper()

	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			tc.t.Fatalf("TestClient: encode body: %v", err)
		}
	}

	req, err := http.NewRequest(method, tc.Server.URL+path, &buf)
	if err != nil {
		tc.t.Fatalf("TestClient: new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, c := range tc.cookies {
		req.AddCookie(c)
	}

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Do(req)
	if err != nil {
		tc.t.Fatalf("TestClient: do request: %v", err)
	}
	return resp
}
