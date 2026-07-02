package handlers_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	appsvc "github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/auth"
	"github.com/SteVio89/stevio-home/config"
	"github.com/SteVio89/stevio-home/crypto"
	"github.com/SteVio89/stevio-home/db"
	"github.com/SteVio89/stevio-home/db/queries"
	"github.com/SteVio89/stevio-home/dbutil"
	"github.com/SteVio89/stevio-home/handlers"
	"github.com/SteVio89/stevio-home/i18n"
	"github.com/SteVio89/stevio-home/payment"
	"github.com/SteVio89/stevio-home/payment/mock"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// newPostgresTestDSN creates a fresh Postgres schema, returns a DSN that pins
// search_path to it, and registers cleanup that drops the schema. Skips the
// test if TEST_DATABASE_URL is unset (e.g. during docker build).
//
// This mirrors backend/testutil's helper but is duplicated here because the
// integration tests bootstrap a full *appsvc.App, which runs its own
// migrations — we hand it a DSN, not a *sql.DB.
func newPostgresTestDSN(t *testing.T) string {
	t.Helper()
	baseDSN := os.Getenv("TEST_DATABASE_URL")
	if baseDSN == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping Postgres integration test")
	}

	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("rand: %v", err)
	}
	schema := "test_" + hex.EncodeToString(b[:])

	admin, err := sql.Open("pgx", baseDSN)
	if err != nil {
		t.Fatalf("open admin: %v", err)
	}
	if _, err := admin.Exec(fmt.Sprintf("CREATE SCHEMA %q", schema)); err != nil {
		_ = admin.Close()
		t.Fatalf("create schema: %v", err)
	}
	_ = admin.Close()

	t.Cleanup(func() {
		a, err := sql.Open("pgx", baseDSN)
		if err == nil {
			_, _ = a.Exec(fmt.Sprintf("DROP SCHEMA IF EXISTS %q CASCADE", schema))
			_ = a.Close()
		}
	})

	u, err := url.Parse(baseDSN)
	if err != nil {
		t.Fatalf("parse base DSN: %v", err)
	}
	q := u.Query()
	q.Set("options", "-c search_path="+schema)
	u.RawQuery = q.Encode()
	return u.String()
}

var testSecret = bytes.Repeat([]byte("t"), 32)

type testEnv struct {
	db      *sql.DB
	app     *appsvc.App
	h       *handlers.Handlers
	cfg     *config.Config
	handler http.Handler
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	dsn := newPostgresTestDSN(t)

	var signingKeySecret [32]byte
	copy(signingKeySecret[:], testSecret)

	emailHash := crypto.HashEmail("admin@test.com", "test-salt")
	cfg := &config.Config{
		Env:                "development",
		BaseURL:            "http://localhost:3000",
		SessionSecretBytes: testSecret,
		SigningKeySecret:   signingKeySecret,
		EmailHashSalt:      "test-salt",
		AdminEmailHashes:   []string{emailHash},
		AssetsDir:          t.TempDir(),
		AppsDir:            t.TempDir(),
	}

	app := appsvc.New(appsvc.Config{
		DatabaseURL: dsn,
		Secret:      testSecret,
		EmailSalt:   "test-salt",
		BaseURL:     "http://localhost:3000",
		AppMigrations: []appsvc.MigrationSource{
			{Prefix: "i18n", Files: i18n.MigrationFiles},
			{Prefix: "app", Files: db.MigrationFiles},
		},
		InsecureCookies: true,
		RateLimit:       999999,
		SettingsTable:   "site_settings",
		DefaultRole:     "member",
		Mailer:          &testMailer{},
		I18n: &appsvc.I18nConfig{
			FallbackLocales: []i18n.Locale{
				{Code: "de", Name: "Deutsch", IsDefault: true, Enabled: true, SortOrder: 0},
				{Code: "en", Name: "English", IsDefault: false, Enabled: true, SortOrder: 1},
			},
		},
	})
	t.Cleanup(func() { _ = app.Close() })

	// Seed admin user into the users table.
	_, _ = app.DB().ExecContext(context.Background(),
		`INSERT INTO users (id, email_hash, role) VALUES ($1, $2, 'admin')
		 ON CONFLICT (email_hash) DO UPDATE SET role = 'admin'`,
		dbutil.NewID(), emailHash)

	signer := crypto.NewDBSigner(app.DB().DB, signingKeySecret)

	// Seed a signing key so activation tests work.
	seedSigningKey(t, app.DB().DB, signingKeySecret)

	logger := log.New(io.Discard, "", 0)
	payments := payment.Registry{
		"mock": mock.New(cfg.BaseURL, cfg.SigningKeySecret),
	}
	h := handlers.New(app, cfg, signer, logger, payments, nil)

	// Register routes (no maintenance middleware for tests).
	handlers.RegisterRoutes(app, h, nil)

	return &testEnv{
		db:      app.DB().DB,
		app:     app,
		h:       h,
		cfg:     cfg,
		handler: app.Handler(),
	}
}

type testMailer struct{}

func (m *testMailer) Send(to, subject, body string) error { return nil }

// createSession inserts a session directly in the DB and returns the signed session cookie.
// The session includes user_id and user_type so the framework's auth middleware works.
func createSession(t *testing.T, database *sql.DB, secret []byte, emailHash, role string) *http.Cookie {
	t.Helper()

	// Ensure user exists.
	userID := dbutil.NewID()
	_, _ = database.ExecContext(context.Background(),
		`INSERT INTO users (id, email_hash, role) VALUES ($1, $2, $3)
		 ON CONFLICT (email_hash) DO UPDATE SET role = excluded.role`,
		userID, emailHash, role)

	// Look up the user ID (might be different if the user already existed).
	row := database.QueryRowContext(context.Background(),
		`SELECT id FROM users WHERE email_hash = $1`, emailHash)
	_ = row.Scan(&userID)

	sessionID := dbutil.NewID()
	expiresAt := time.Now().Add(24 * time.Hour)
	if err := auth.CreateSession(context.Background(), database, sessionID, emailHash, expiresAt,
		auth.WithSessionUser(userID, role)); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	return &http.Cookie{Name: "sid", Value: crypto.SignSession(sessionID, secret)}
}

// seedApp creates a project + commerce attachment and returns the app's ID.
// Post-refactor, apps are 1:1 with projects via project_id; the project owns
// the display title, while the app carries pricing + bundle id.
func seedApp(t *testing.T, database *sql.DB) string {
	t.Helper()
	appID, _ := seedAppWithProject(t, database)
	return appID
}

// seedAppWithProject is the variant that also returns the parent project ID.
// Used by tests that need to hit the public project endpoints.
func seedAppWithProject(t *testing.T, database *sql.DB) (appID, projectID string) {
	t.Helper()
	ctx := context.Background()
	// Generate a random slug suffix so multiple seedApp calls in one test don't
	// collide on the partial unique index.
	slugSuffix := dbutil.NewID()[:8]
	slug := "test-app-" + slugSuffix
	err := queries.WithTx(ctx, database, func(tx *sql.Tx) error {
		project, perr := queries.InsertProjectTx(ctx, tx, slug, "/media/icon.png", nil, 0, true)
		if perr != nil {
			return perr
		}
		projectID = project.ID
		// Default-locale title so any title-overlay queries in tests get a value.
		if perr := queries.UpsertTranslationTx(ctx, tx, "project", project.ID, "de", "title", "Test App"); perr != nil {
			return perr
		}
		app, aerr := queries.InsertAppTx(ctx, tx, project.ID, "com.test.app."+slugSuffix, 999, "always_new_license", "digital-goods")
		if aerr != nil {
			return aerr
		}
		appID = app.ID
		return nil
	})
	if err != nil {
		t.Fatalf("seedApp: %v", err)
	}
	return appID, projectID
}

// seedLicense creates an order + license and returns the license key and ID.
func seedLicense(t *testing.T, database *sql.DB, appID, emailHash string) (licenseKey, licenseID string) {
	t.Helper()
	ctx := context.Background()
	err := queries.WithTx(ctx, database, func(tx *sql.Tx) error {
		order, err := queries.InsertOrder(ctx, tx, "pay-"+appID[:8], emailHash, appID, 0, nil, nil, queries.OrderDiscountSnapshot{}, "")
		if err != nil {
			return err
		}
		lic, err := queries.InsertLicense(ctx, tx, "lic-"+appID[:8], order.ID, appID, nil)
		if err != nil {
			return err
		}
		licenseKey = lic.Key
		licenseID = lic.ID
		return nil
	})
	if err != nil {
		t.Fatalf("seedLicense: %v", err)
	}
	return
}

// seedSigningKey generates a signing key, encrypts it, and inserts it as active.
func seedSigningKey(t *testing.T, database *sql.DB, secret [32]byte) {
	t.Helper()
	privB64, pubB64, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	privBytes, _ := base64.StdEncoding.DecodeString(privB64)
	pubBytes, _ := base64.StdEncoding.DecodeString(pubB64)
	keyID := crypto.DeriveKeyID(pubBytes)
	encrypted, err := crypto.EncryptPrivateKey(secret, privBytes)
	if err != nil {
		t.Fatalf("EncryptPrivateKey: %v", err)
	}
	_, err = database.ExecContext(context.Background(),
		`INSERT INTO signing_keys (id, key_id, encrypted_private_key, public_key_b64, active)
		 VALUES ($1, $2, $3, $4, TRUE)`,
		dbutil.NewID(), keyID, encrypted, pubB64)
	if err != nil {
		t.Fatalf("seed signing key: %v", err)
	}
}

func doJSON(t *testing.T, handler http.Handler, method, path string, body any, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// doJSONWithHeaders is like doJSON but takes a raw body (already marshalled)
// and a header map. Used by tests that need to exercise specific headers such
// as X-Mock-Signature.
func doJSONWithHeaders(t *testing.T, handler http.Handler, method, path string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reqBody)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestListAppsEmpty(t *testing.T) {
	env := setupTestEnv(t)
	rec := doJSON(t, env.handler, "GET", "/api/projects", nil)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var projects []json.RawMessage
	_ = json.NewDecoder(rec.Body).Decode(&projects)
	if len(projects) != 0 {
		t.Errorf("expected empty list, got %d", len(projects))
	}
}

func TestGetRefundPolicy(t *testing.T) {
	env := setupTestEnv(t)
	rec := doJSON(t, env.handler, "GET", "/api/legal/refund-policy", nil)
	if rec.Code != 200 {
		t.Fatalf("GET /api/legal/refund-policy status = %d, want 200", rec.Code)
	}
	var body map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if _, ok := body["html"]; !ok {
		t.Error("response missing 'html' field")
	}
}

func TestListAppsWithData(t *testing.T) {
	env := setupTestEnv(t)
	seedApp(t, env.db)
	rec := doJSON(t, env.handler, "GET", "/api/projects", nil)
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	var projects []json.RawMessage
	_ = json.NewDecoder(rec.Body).Decode(&projects)
	if len(projects) != 1 {
		t.Errorf("got %d projects, want 1", len(projects))
	}
}

func TestGetAppDetailNotFound(t *testing.T) {
	env := setupTestEnv(t)
	rec := doJSON(t, env.handler, "GET", "/api/projects/nonexistent", nil)
	if rec.Code != 404 {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestGetAppDetailFound(t *testing.T) {
	env := setupTestEnv(t)
	_, projectID := seedAppWithProject(t, env.db)
	project, _ := queries.GetProjectByID(context.Background(), env.db, projectID)
	if project == nil || project.Slug == "" {
		t.Fatalf("seeded project missing slug")
	}
	rec := doJSON(t, env.handler, "GET", "/api/projects/"+project.Slug, nil)
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	var detail struct {
		ID    string `json:"id"`
		Slug  string `json:"slug"`
		Title string `json:"title"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&detail)
	if detail.ID != project.ID {
		t.Errorf("id = %q, want %q", detail.ID, project.ID)
	}
}

func TestSendMagicLinkSuccess(t *testing.T) {
	env := setupTestEnv(t)
	// Framework uses POST /auth/login instead of POST /api/auth/magic-link.
	rec := doJSON(t, env.handler, "POST", "/auth/login", map[string]string{"email": "user@test.com"})
	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestSendMagicLinkBadRequest(t *testing.T) {
	env := setupTestEnv(t)

	// Missing email
	rec := doJSON(t, env.handler, "POST", "/auth/login", map[string]string{"email": ""})
	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}

	// Invalid JSON
	req := httptest.NewRequest("POST", "/auth/login", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	env.handler.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Errorf("invalid json: status = %d, want 400", rec.Code)
	}
}

func TestSendMagicLinkPendingToken(t *testing.T) {
	env := setupTestEnv(t)

	// First request succeeds
	rec := doJSON(t, env.handler, "POST", "/auth/login", map[string]string{"email": "user@test.com"})
	if rec.Code != 200 {
		t.Fatalf("first request: status = %d", rec.Code)
	}

	// Second request — the framework skips sending a second email (anti-spam) but
	// returns the same 200 "sent" response as the first, so the endpoint doesn't
	// leak that this address recently requested a login (and it is not 429'd).
	rec = doJSON(t, env.handler, "POST", "/auth/login", map[string]string{"email": "user@test.com"})
	if rec.Code != 200 {
		t.Errorf("second request: status = %d, want 200", rec.Code)
	}
	var resp map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp["status"] != "sent" {
		t.Errorf("status = %q, want sent", resp["status"])
	}
}

func TestAuthMeUnauthenticated(t *testing.T) {
	env := setupTestEnv(t)
	rec := doJSON(t, env.handler, "GET", "/api/auth/me", nil)
	if rec.Code != 401 {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestAuthMeAuthenticated(t *testing.T) {
	env := setupTestEnv(t)
	emailHash := crypto.HashEmail("user@test.com", "test-salt")
	cookie := createSession(t, env.db, testSecret, emailHash, "member")

	rec := doJSON(t, env.handler, "GET", "/api/auth/me", nil, cookie)
	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var me struct {
		IsAdmin bool `json:"is_admin"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&me)
	if me.IsAdmin {
		t.Error("regular user should not be admin")
	}
}

func TestAuthMeAdmin(t *testing.T) {
	env := setupTestEnv(t)
	emailHash := crypto.HashEmail("admin@test.com", "test-salt")
	cookie := createSession(t, env.db, testSecret, emailHash, "admin")

	rec := doJSON(t, env.handler, "GET", "/api/auth/me", nil, cookie)
	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var me struct {
		IsAdmin bool `json:"is_admin"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&me)
	if !me.IsAdmin {
		t.Error("admin user should be admin")
	}
}

func TestLogout(t *testing.T) {
	env := setupTestEnv(t)
	emailHash := crypto.HashEmail("user@test.com", "test-salt")
	cookie := createSession(t, env.db, testSecret, emailHash, "member")

	rec := doJSON(t, env.handler, "POST", "/auth/logout", nil, cookie)
	if rec.Code != 204 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	// Session should be gone — /me should 401
	rec = doJSON(t, env.handler, "GET", "/api/auth/me", nil, cookie)
	if rec.Code != 401 {
		t.Errorf("after logout: status = %d, want 401", rec.Code)
	}
}

func TestActivateLicense(t *testing.T) {
	env := setupTestEnv(t)
	appID := seedApp(t, env.db)
	licenseKey, _ := seedLicense(t, env.db, appID, "emailhash")

	machineHash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	rec := doJSON(t, env.handler, "POST", "/api/license/activate", map[string]string{
		"license_key":  licenseKey,
		"machine_hash": machineHash,
		"device_label": "My Mac",
	})
	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	// Response should contain signed license
	var resp struct {
		Payload   json.RawMessage `json:"payload"`
		Signature string          `json:"signature"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Signature == "" {
		t.Error("expected signature in response")
	}
}

func TestActivateLicenseInvalidKey(t *testing.T) {
	env := setupTestEnv(t)
	rec := doJSON(t, env.handler, "POST", "/api/license/activate", map[string]string{
		"license_key":  "invalid-key",
		"machine_hash": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	if rec.Code != 422 {
		t.Errorf("status = %d, want 422", rec.Code)
	}
}

func TestActivateLicenseBadMachineHash(t *testing.T) {
	env := setupTestEnv(t)

	// Too short
	rec := doJSON(t, env.handler, "POST", "/api/license/activate", map[string]string{
		"license_key":  "key",
		"machine_hash": "short",
	})
	if rec.Code != 400 {
		t.Errorf("short hash: status = %d, want 400", rec.Code)
	}

	// Not hex
	rec = doJSON(t, env.handler, "POST", "/api/license/activate", map[string]string{
		"license_key":  "key",
		"machine_hash": "gggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg",
	})
	if rec.Code != 400 {
		t.Errorf("non-hex: status = %d, want 400", rec.Code)
	}
}

func TestActivateLicenseSlotsFull(t *testing.T) {
	env := setupTestEnv(t)
	appID := seedApp(t, env.db)
	licenseKey, _ := seedLicense(t, env.db, appID, "emailhash")

	// Fill all 3 slots
	for i := 0; i < 3; i++ {
		hash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		hash = hash[:63] + string(rune('a'+i))
		rec := doJSON(t, env.handler, "POST", "/api/license/activate", map[string]string{
			"license_key":  licenseKey,
			"machine_hash": hash,
		})
		if rec.Code != 200 {
			t.Fatalf("activate %d: status = %d", i, rec.Code)
		}
	}

	// 4th device should fail
	rec := doJSON(t, env.handler, "POST", "/api/license/activate", map[string]string{
		"license_key":  licenseKey,
		"machine_hash": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	})
	if rec.Code != 422 {
		t.Errorf("status = %d, want 422 (slots full)", rec.Code)
	}
}

func TestGetLicensesAuthenticated(t *testing.T) {
	env := setupTestEnv(t)
	emailHash := crypto.HashEmail("user@test.com", "test-salt")
	cookie := createSession(t, env.db, testSecret, emailHash, "member")
	appID := seedApp(t, env.db)
	seedLicense(t, env.db, appID, emailHash)

	rec := doJSON(t, env.handler, "GET", "/api/account/licenses", nil, cookie)
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	var licenses []json.RawMessage
	_ = json.NewDecoder(rec.Body).Decode(&licenses)
	if len(licenses) != 1 {
		t.Errorf("got %d licenses, want 1", len(licenses))
	}
}

func TestGetOrdersAuthenticated(t *testing.T) {
	env := setupTestEnv(t)
	emailHash := crypto.HashEmail("user@test.com", "test-salt")
	cookie := createSession(t, env.db, testSecret, emailHash, "member")
	appID := seedApp(t, env.db)
	seedLicense(t, env.db, appID, emailHash)

	rec := doJSON(t, env.handler, "GET", "/api/account/orders", nil, cookie)
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	var orders []json.RawMessage
	_ = json.NewDecoder(rec.Body).Decode(&orders)
	if len(orders) != 1 {
		t.Errorf("got %d orders, want 1", len(orders))
	}
}

func TestGetOrdersUnauthenticated(t *testing.T) {
	env := setupTestEnv(t)
	rec := doJSON(t, env.handler, "GET", "/api/account/orders", nil)
	if rec.Code != 401 {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestRenameDevice(t *testing.T) {
	env := setupTestEnv(t)
	emailHash := crypto.HashEmail("user@test.com", "test-salt")
	cookie := createSession(t, env.db, testSecret, emailHash, "member")
	appID := seedApp(t, env.db)
	_, licenseID := seedLicense(t, env.db, appID, emailHash)

	// Activate a device
	machineHash := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	ctx := context.Background()
	var activationID string
	if err := queries.WithTx(ctx, env.db, func(tx *sql.Tx) error {
		act, err := queries.UpsertActivation(ctx, tx, licenseID, machineHash, "Old Name", nil)
		if err != nil {
			return err
		}
		activationID = act.ID
		return nil
	}); err != nil {
		t.Fatalf("UpsertActivation: %v", err)
	}

	// Rename
	rec := doJSON(t, env.handler, "PATCH", "/api/account/activations/"+activationID, map[string]string{
		"device_label": "New Name",
	}, cookie)
	if rec.Code != 200 {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestRenameDeviceNotOwned(t *testing.T) {
	env := setupTestEnv(t)
	otherHash := crypto.HashEmail("other@test.com", "test-salt")
	cookie := createSession(t, env.db, testSecret, otherHash, "member")

	emailHash := crypto.HashEmail("owner@test.com", "test-salt")
	appID := seedApp(t, env.db)
	_, licenseID := seedLicense(t, env.db, appID, emailHash)

	machineHash := "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"[:64]
	ctx := context.Background()
	var activationID string
	if err := queries.WithTx(ctx, env.db, func(tx *sql.Tx) error {
		act, err := queries.UpsertActivation(ctx, tx, licenseID, machineHash, "Name", nil)
		if err != nil {
			return err
		}
		activationID = act.ID
		return nil
	}); err != nil {
		t.Fatalf("UpsertActivation: %v", err)
	}

	// Other user tries to rename — should 404
	rec := doJSON(t, env.handler, "PATCH", "/api/account/activations/"+activationID, map[string]string{
		"device_label": "Hacked",
	}, cookie)
	if rec.Code != 404 {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestDeactivateLicense(t *testing.T) {
	env := setupTestEnv(t)
	emailHash := crypto.HashEmail("user@test.com", "test-salt")
	appID := seedApp(t, env.db)
	licenseKey, licenseID := seedLicense(t, env.db, appID, emailHash)

	machineHash := "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	ctx := context.Background()
	if err := queries.WithTx(ctx, env.db, func(tx *sql.Tx) error {
		_, err := queries.UpsertActivation(ctx, tx, licenseID, machineHash, "Name", nil)
		return err
	}); err != nil {
		t.Fatalf("UpsertActivation: %v", err)
	}

	// Deactivate via SDK endpoint
	rec := doJSON(t, env.handler, "POST", "/api/license/deactivate", map[string]any{
		"license_key":  licenseKey,
		"machine_hash": machineHash,
	})
	if rec.Code != 204 {
		t.Errorf("status = %d, want 204, body = %s", rec.Code, rec.Body.String())
	}

	// Should not be in the list anymore
	count, _ := queries.CountActivations(ctx, env.db, licenseID)
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}

	// Deactivating again should return invalid (same error as bad key)
	rec = doJSON(t, env.handler, "POST", "/api/license/deactivate", map[string]any{
		"license_key":  licenseKey,
		"machine_hash": machineHash,
	})
	if rec.Code != 422 {
		t.Errorf("double-deactivate status = %d, want 422", rec.Code)
	}
}

func TestAdminCreateApp(t *testing.T) {
	env := setupTestEnv(t)
	adminHash := crypto.HashEmail("admin@test.com", "test-salt")
	cookie := createSession(t, env.db, testSecret, adminHash, "admin")

	// Create a project first — apps now require an existing project_id.
	ctx := context.Background()
	var projectID string
	if err := queries.WithTx(ctx, env.db, func(tx *sql.Tx) error {
		p, perr := queries.InsertProjectTx(ctx, tx, "", "/media/icon.png", nil, 0, true)
		if perr != nil {
			return perr
		}
		projectID = p.ID
		return nil
	}); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	rec := doJSON(t, env.handler, "POST", "/api/admin/apps", map[string]any{
		"project_id":  projectID,
		"bundle_id":   "com.test.newapp",
		"price_cents": 1999,
	}, cookie)
	if rec.Code != 201 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var got struct {
		ID         string `json:"id"`
		ProjectID  string `json:"project_id"`
		BundleID   string `json:"bundle_id"`
		PriceCents int    `json:"price_cents"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&got)
	if got.ProjectID != projectID {
		t.Errorf("project_id = %q, want %q", got.ProjectID, projectID)
	}
	if got.BundleID != "com.test.newapp" {
		t.Errorf("bundle_id = %q", got.BundleID)
	}
}

func TestAdminCreateAppForbidden(t *testing.T) {
	env := setupTestEnv(t)
	userHash := crypto.HashEmail("regular@test.com", "test-salt")
	cookie := createSession(t, env.db, testSecret, userHash, "member")

	rec := doJSON(t, env.handler, "POST", "/api/admin/apps", map[string]any{
		"project_id": "irrelevant",
		"bundle_id":  "com.evil.app",
	}, cookie)
	if rec.Code != 403 {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestAdminCreateAppUnauthenticated(t *testing.T) {
	env := setupTestEnv(t)
	rec := doJSON(t, env.handler, "POST", "/api/admin/apps", map[string]any{
		"project_id": "irrelevant",
		"bundle_id":  "com.evil.app",
	})
	if rec.Code != 401 {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// --- Security integration tests ---

func TestSessionTampered_Returns401(t *testing.T) {
	env := setupTestEnv(t)
	emailHash := crypto.HashEmail("user@test.com", "test-salt")

	// Create a valid session and sign it.
	cookie := createSession(t, env.db, testSecret, emailHash, "member")

	// Tamper with the cookie value by appending extra characters.
	tampered := &http.Cookie{Name: "sid", Value: cookie.Value + "tampered"}

	rec := doJSON(t, env.handler, "GET", "/api/auth/me", nil, tampered)
	if rec.Code != 401 {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestSessionExpired_Returns401(t *testing.T) {
	env := setupTestEnv(t)
	emailHash := crypto.HashEmail("user@test.com", "test-salt")

	// Create a session that is already expired (1 hour in the past).
	userID := dbutil.NewID()
	_, _ = env.db.ExecContext(context.Background(),
		`INSERT INTO users (id, email_hash, role) VALUES ($1, $2, 'member')
		 ON CONFLICT (email_hash) DO NOTHING`, userID, emailHash)

	sessionID := dbutil.NewID()
	expiresAt := time.Now().Add(-1 * time.Hour)
	if err := auth.CreateSession(context.Background(), env.db, sessionID, emailHash, expiresAt,
		auth.WithSessionUser(userID, "member")); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Sign correctly — the signature is valid, but the session is expired.
	cookie := &http.Cookie{Name: "sid", Value: crypto.SignSession(sessionID, testSecret)}

	rec := doJSON(t, env.handler, "GET", "/api/auth/me", nil, cookie)
	if rec.Code != 401 {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestGDPRErasure_TombstoneOrders(t *testing.T) {
	env := setupTestEnv(t)
	emailHash := crypto.HashEmail("user@test.com", "test-salt")
	cookie := createSession(t, env.db, testSecret, emailHash, "member")
	appID := seedApp(t, env.db)
	seedLicense(t, env.db, appID, emailHash)

	// Verify the user can see their orders before erasure.
	rec := doJSON(t, env.handler, "GET", "/api/account/orders", nil, cookie)
	if rec.Code != 200 {
		t.Fatalf("pre-erase orders: status = %d", rec.Code)
	}

	// Request data erasure — handler returns 204.
	rec = doJSON(t, env.handler, "DELETE", "/api/account/data", map[string]string{
		"email": "user@test.com",
	}, cookie)
	if rec.Code != 204 {
		t.Fatalf("delete data: status = %d, body = %s", rec.Code, rec.Body.String())
	}

	// Verify the order row now has email = 'DELETED' (GDPR tombstone).
	var orderEmail string
	err := env.db.QueryRowContext(context.Background(),
		`SELECT email FROM orders WHERE app_id = $1`, appID).Scan(&orderEmail)
	if err != nil {
		t.Fatalf("query order email: %v", err)
	}
	if orderEmail != "DELETED" {
		t.Errorf("order email = %q, want 'DELETED'", orderEmail)
	}

	// Verify the old session is now invalid (sessions were deleted by erasure).
	rec = doJSON(t, env.handler, "GET", "/api/auth/me", nil, cookie)
	if rec.Code != 401 {
		t.Errorf("post-erase auth: status = %d, want 401", rec.Code)
	}
}

func TestBodySizeLimit_MagicLink(t *testing.T) {
	env := setupTestEnv(t)

	// Create a body just over 1MB (maxRequestBody = 1 << 20).
	oversized := make([]byte, 1<<20+1)
	for i := range oversized {
		oversized[i] = 'a'
	}

	req := httptest.NewRequest("POST", "/auth/login", bytes.NewReader(oversized))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.handler.ServeHTTP(rec, req)

	// The server should reject the oversized body — it should NOT return 200.
	if rec.Code == 200 {
		t.Errorf("oversized body accepted with 200, expected rejection")
	}
}

func TestStringLengthCap_DeviceLabel(t *testing.T) {
	env := setupTestEnv(t)
	emailHash := crypto.HashEmail("user@test.com", "test-salt")
	cookie := createSession(t, env.db, testSecret, emailHash, "member")
	appID := seedApp(t, env.db)
	_, licenseID := seedLicense(t, env.db, appID, emailHash)

	// Create an activation.
	machineHash := "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	ctx := context.Background()
	var activationID string
	if err := queries.WithTx(ctx, env.db, func(tx *sql.Tx) error {
		act, err := queries.UpsertActivation(ctx, tx, licenseID, machineHash, "Valid Label", nil)
		if err != nil {
			return err
		}
		activationID = act.ID
		return nil
	}); err != nil {
		t.Fatalf("UpsertActivation: %v", err)
	}

	// Try to rename with a 256-character label (over the 255 cap).
	longLabel := strings.Repeat("X", 256)
	rec := doJSON(t, env.handler, "PATCH", "/api/account/activations/"+activationID, map[string]string{
		"device_label": longLabel,
	}, cookie)
	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestDiscountValidation_NonexistentAppSameError(t *testing.T) {
	env := setupTestEnv(t)
	appID := seedApp(t, env.db)
	ctx := context.Background()

	// Create a discount code scoped to the seeded app.
	_, err := queries.InsertDiscountCode(ctx, env.db, queries.InsertDiscountCodeParams{
		Code:          "TESTCODE",
		Label:         "Test Discount",
		DiscountType:  "percent",
		DiscountValue: 10,
		AppID:         &appID,
	})
	if err != nil {
		t.Fatalf("InsertDiscountCode: %v", err)
	}

	// Case 1: Valid app, invalid code.
	rec1 := doJSON(t, env.handler, "POST", "/api/discounts/validate", map[string]string{
		"code":   "INVALIDCODE",
		"app_id": appID,
	})

	// Case 2: Nonexistent app, valid code — should return the same error
	// to prevent app ID enumeration.
	rec2 := doJSON(t, env.handler, "POST", "/api/discounts/validate", map[string]string{
		"code":   "TESTCODE",
		"app_id": "nonexistent-app-id",
	})

	// Parse both error responses — APIError serialises the code as "error".
	var err1, err2 struct {
		Error string `json:"error"`
	}
	_ = json.NewDecoder(rec1.Body).Decode(&err1)
	_ = json.NewDecoder(rec2.Body).Decode(&err2)

	if err1.Error == "" {
		t.Fatal("expected error code in response for invalid code")
	}
	if err2.Error == "" {
		t.Fatal("expected error code in response for nonexistent app")
	}
	if err1.Error != err2.Error {
		t.Errorf("error codes differ: invalid code = %q, nonexistent app = %q — leaks app existence", err1.Error, err2.Error)
	}
}

// ─── Chat Tests ─────────────────────────────────────────────────────────────

func TestChatCreateAndGet(t *testing.T) {
	env := setupTestEnv(t)
	userHash := crypto.HashEmail("chatuser@test.com", "test-salt")
	cookie := createSession(t, env.db, testSecret, userHash, "member")

	// No chat yet — should 404
	rec := doJSON(t, env.handler, "GET", "/api/chat", nil, cookie)
	if rec.Code != 404 {
		t.Fatalf("GET /api/chat before create: status = %d, want 404", rec.Code)
	}

	// Create chat
	rec = doJSON(t, env.handler, "POST", "/api/chat", nil, cookie)
	if rec.Code != 201 {
		t.Fatalf("POST /api/chat: status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
	var conv struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&conv)
	if conv.ID == "" || conv.DisplayName == "" {
		t.Fatal("expected non-empty conversation ID and display name")
	}

	// Duplicate create — should 409
	rec = doJSON(t, env.handler, "POST", "/api/chat", nil, cookie)
	if rec.Code != 409 {
		t.Fatalf("duplicate POST /api/chat: status = %d, want 409", rec.Code)
	}

	// GET chat — should succeed
	rec = doJSON(t, env.handler, "GET", "/api/chat", nil, cookie)
	if rec.Code != 200 {
		t.Fatalf("GET /api/chat: status = %d, want 200", rec.Code)
	}
}

func TestChatSendMessage(t *testing.T) {
	env := setupTestEnv(t)
	userHash := crypto.HashEmail("chatmsg@test.com", "test-salt")
	cookie := createSession(t, env.db, testSecret, userHash, "member")

	// Create conversation
	doJSON(t, env.handler, "POST", "/api/chat", nil, cookie)

	// Send message
	rec := doJSON(t, env.handler, "POST", "/api/chat/messages",
		map[string]string{"body": "Hello, I need help"}, cookie)
	if rec.Code != 201 {
		t.Fatalf("POST /api/chat/messages: status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}

	// Empty body — should 400
	rec = doJSON(t, env.handler, "POST", "/api/chat/messages",
		map[string]string{"body": ""}, cookie)
	if rec.Code != 400 {
		t.Fatalf("empty body: status = %d, want 400", rec.Code)
	}
}

func TestChatEmailFilter(t *testing.T) {
	env := setupTestEnv(t)
	userHash := crypto.HashEmail("emailfilter@test.com", "test-salt")
	cookie := createSession(t, env.db, testSecret, userHash, "member")

	doJSON(t, env.handler, "POST", "/api/chat", nil, cookie)

	rec := doJSON(t, env.handler, "POST", "/api/chat/messages",
		map[string]string{"body": "My email is user@example.com please help"}, cookie)
	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201", rec.Code)
	}

	var msg struct {
		Body string `json:"body"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&msg)
	if strings.Contains(msg.Body, "user@example.com") {
		t.Errorf("email not filtered: body = %q", msg.Body)
	}
	if !strings.Contains(msg.Body, "[email removed") {
		t.Errorf("expected replacement text, got: %q", msg.Body)
	}
}

func TestChatDeleteCascade(t *testing.T) {
	env := setupTestEnv(t)
	userHash := crypto.HashEmail("chatdel@test.com", "test-salt")
	cookie := createSession(t, env.db, testSecret, userHash, "member")

	doJSON(t, env.handler, "POST", "/api/chat", nil, cookie)
	doJSON(t, env.handler, "POST", "/api/chat/messages",
		map[string]string{"body": "test message"}, cookie)

	// Delete
	rec := doJSON(t, env.handler, "DELETE", "/api/chat", nil, cookie)
	if rec.Code != 204 {
		t.Fatalf("DELETE /api/chat: status = %d, want 204", rec.Code)
	}

	// Should be gone
	rec = doJSON(t, env.handler, "GET", "/api/chat", nil, cookie)
	if rec.Code != 404 {
		t.Fatalf("GET after delete: status = %d, want 404", rec.Code)
	}
}

func TestChatAdminListAndDetail(t *testing.T) {
	env := setupTestEnv(t)
	adminHash := crypto.HashEmail("admin@test.com", "test-salt")
	adminCookie := createSession(t, env.db, testSecret, adminHash, "admin")
	userHash := crypto.HashEmail("chatadmin@test.com", "test-salt")
	userCookie := createSession(t, env.db, testSecret, userHash, "member")

	// Create conversation + message as user
	doJSON(t, env.handler, "POST", "/api/chat", nil, userCookie)
	doJSON(t, env.handler, "POST", "/api/chat/messages",
		map[string]string{"body": "Need help with license"}, userCookie)

	// Admin list
	rec := doJSON(t, env.handler, "GET", "/api/admin/chats?page=1&per_page=20", nil, adminCookie)
	if rec.Code != 200 {
		t.Fatalf("admin list: status = %d, want 200", rec.Code)
	}
	var list struct {
		Items []json.RawMessage `json:"items"`
		Total int               `json:"total"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&list)
	if list.Total != 1 {
		t.Fatalf("expected 1 conversation, got %d", list.Total)
	}

	// Unread count
	rec = doJSON(t, env.handler, "GET", "/api/admin/chats/unread-count", nil, adminCookie)
	if rec.Code != 200 {
		t.Fatalf("unread count: status = %d, want 200", rec.Code)
	}
	var unread struct{ Count int }
	_ = json.NewDecoder(rec.Body).Decode(&unread)
	if unread.Count != 1 {
		t.Errorf("expected 1 unread conversation, got %d", unread.Count)
	}

	// Admin get detail (parse conv id from list)
	var item struct{ ID string }
	_ = json.Unmarshal(list.Items[0], &item)

	rec = doJSON(t, env.handler, "GET", "/api/admin/chats/"+item.ID, nil, adminCookie)
	if rec.Code != 200 {
		t.Fatalf("admin detail: status = %d, want 200", rec.Code)
	}

	// Admin reply
	rec = doJSON(t, env.handler, "POST", "/api/admin/chats/"+item.ID+"/messages",
		map[string]string{"body": "Hello, how can I help?"}, adminCookie)
	if rec.Code != 201 {
		t.Fatalf("admin reply: status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
}

func TestChatBanUser(t *testing.T) {
	env := setupTestEnv(t)
	adminHash := crypto.HashEmail("admin@test.com", "test-salt")
	adminCookie := createSession(t, env.db, testSecret, adminHash, "admin")
	userHash := crypto.HashEmail("banneduser@test.com", "test-salt")
	userCookie := createSession(t, env.db, testSecret, userHash, "member")

	// User creates chat
	doJSON(t, env.handler, "POST", "/api/chat", nil, userCookie)

	// Get conv ID
	rec := doJSON(t, env.handler, "GET", "/api/admin/chats?page=1&per_page=20", nil, adminCookie)
	var list struct {
		Items []struct{ ID string } `json:"items"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&list)
	convID := list.Items[0].ID

	// Ban user
	rec = doJSON(t, env.handler, "POST", "/api/admin/chats/"+convID+"/ban",
		map[string]string{"reason": "spam"}, adminCookie)
	if rec.Code != 200 {
		t.Fatalf("ban: status = %d, want 200", rec.Code)
	}

	// User tries to send message — should be forbidden
	rec = doJSON(t, env.handler, "POST", "/api/chat/messages",
		map[string]string{"body": "more spam"}, userCookie)
	if rec.Code != 403 {
		t.Fatalf("banned message: status = %d, want 403", rec.Code)
	}

	// Unban
	rec = doJSON(t, env.handler, "POST", "/api/admin/chats/"+convID+"/unban", nil, adminCookie)
	if rec.Code != 200 {
		t.Fatalf("unban: status = %d, want 200", rec.Code)
	}

	// User can send again
	rec = doJSON(t, env.handler, "POST", "/api/chat/messages",
		map[string]string{"body": "sorry about that"}, userCookie)
	if rec.Code != 201 {
		t.Fatalf("unbanned message: status = %d, want 201", rec.Code)
	}
}

func TestChatAdminDelete(t *testing.T) {
	env := setupTestEnv(t)
	adminHash := crypto.HashEmail("admin@test.com", "test-salt")
	adminCookie := createSession(t, env.db, testSecret, adminHash, "admin")
	userHash := crypto.HashEmail("admindel@test.com", "test-salt")
	userCookie := createSession(t, env.db, testSecret, userHash, "member")

	doJSON(t, env.handler, "POST", "/api/chat", nil, userCookie)
	doJSON(t, env.handler, "POST", "/api/chat/messages",
		map[string]string{"body": "test"}, userCookie)

	rec := doJSON(t, env.handler, "GET", "/api/admin/chats?page=1&per_page=20", nil, adminCookie)
	var list struct {
		Items []struct{ ID string } `json:"items"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&list)

	// Admin deletes
	rec = doJSON(t, env.handler, "DELETE", "/api/admin/chats/"+list.Items[0].ID, nil, adminCookie)
	if rec.Code != 204 {
		t.Fatalf("admin delete: status = %d, want 204", rec.Code)
	}

	// User sees no chat
	rec = doJSON(t, env.handler, "GET", "/api/chat", nil, userCookie)
	if rec.Code != 404 {
		t.Fatalf("user GET after admin delete: status = %d, want 404", rec.Code)
	}
}

func TestGetAppDetailSystemRequirements(t *testing.T) {
	env := setupTestEnv(t)
	appID, projectID := seedAppWithProject(t, env.db)
	project, _ := queries.GetProjectByID(context.Background(), env.db, projectID)
	if project == nil {
		t.Fatalf("project missing")
	}
	// Seed system_requirements translation on the app entity (commerce-only field).
	ctx := context.Background()
	_ = i18n.UpsertEntityTranslation(ctx, env.db, "app", appID, "de", "system_requirements", "- macOS 14.0+\n- Apple Silicon")
	rec := doJSON(t, env.handler, "GET", "/api/projects/"+project.Slug, nil)
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	var detail struct {
		Commerce *struct {
			SystemRequirements string `json:"system_requirements"`
		} `json:"commerce"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&detail)
	if detail.Commerce == nil {
		t.Fatal("commerce missing in detail response")
	}
	if detail.Commerce.SystemRequirements == "" {
		t.Error("system_requirements is empty, want rendered HTML")
	}
	if !strings.Contains(detail.Commerce.SystemRequirements, "macOS 14.0+") {
		t.Errorf("system_requirements = %q, want to contain 'macOS 14.0+'", detail.Commerce.SystemRequirements)
	}
	if !strings.Contains(detail.Commerce.SystemRequirements, "<") {
		t.Errorf("system_requirements should be HTML, got %q", detail.Commerce.SystemRequirements)
	}
}

func TestGetAppDetailNoSystemRequirements(t *testing.T) {
	env := setupTestEnv(t)
	_, projectID := seedAppWithProject(t, env.db)
	project, _ := queries.GetProjectByID(context.Background(), env.db, projectID)
	if project == nil {
		t.Fatalf("project missing")
	}
	rec := doJSON(t, env.handler, "GET", "/api/projects/"+project.Slug, nil)
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	var detail struct {
		Commerce *struct {
			SystemRequirements string `json:"system_requirements"`
		} `json:"commerce"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&detail)
	if detail.Commerce != nil && detail.Commerce.SystemRequirements != "" {
		t.Errorf("system_requirements = %q, want empty", detail.Commerce.SystemRequirements)
	}
}

func TestGetAppVersions(t *testing.T) {
	env := setupTestEnv(t)
	appID, projectID := seedAppWithProject(t, env.db)
	project, _ := queries.GetProjectByID(context.Background(), env.db, projectID)
	if project == nil {
		t.Fatalf("project missing")
	}
	ctx := context.Background()
	// Seed 2 versions.
	v1, err := queries.InsertAppVersion(ctx, env.db, appID, "1.0.0", "", "/apps/v1.zip")
	if err != nil {
		t.Fatalf("InsertAppVersion v1: %v", err)
	}
	v2, err := queries.InsertAppVersion(ctx, env.db, appID, "1.1.0", "", "/apps/v2.zip")
	if err != nil {
		t.Fatalf("InsertAppVersion v2: %v", err)
	}
	// Seed release notes translation for v2.
	_ = i18n.UpsertEntityTranslation(ctx, env.db, "version", v2.ID, "de", "release_notes", "**Bug fixes**")
	_ = v1 // v1 has no release notes — should still appear.

	rec := doJSON(t, env.handler, "GET", "/api/projects/"+project.Slug+"/versions", nil)
	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var versions []struct {
		ID           string `json:"id"`
		Version      string `json:"version"`
		ReleaseNotes string `json:"release_notes"`
		FilePath     string `json:"file_path"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&versions)
	if len(versions) != 2 {
		t.Fatalf("got %d versions, want 2", len(versions))
	}
	// Verify file_path is stripped from public response.
	for _, v := range versions {
		if v.FilePath != "" {
			t.Errorf("file_path should be empty in public response, got %q", v.FilePath)
		}
	}
	// Find v2 and verify release notes are rendered HTML.
	found := false
	for _, v := range versions {
		if v.Version == "1.1.0" {
			found = true
			if !strings.Contains(v.ReleaseNotes, "Bug fixes") {
				t.Errorf("release_notes = %q, want to contain 'Bug fixes'", v.ReleaseNotes)
			}
			if !strings.Contains(v.ReleaseNotes, "<") {
				t.Errorf("release_notes should be HTML, got %q", v.ReleaseNotes)
			}
		}
	}
	if !found {
		t.Error("version 1.1.0 not found in response")
	}
}

func TestGetAppVersionsEmpty(t *testing.T) {
	env := setupTestEnv(t)
	_, projectID := seedAppWithProject(t, env.db)
	project, _ := queries.GetProjectByID(context.Background(), env.db, projectID)
	if project == nil {
		t.Fatalf("project missing")
	}
	rec := doJSON(t, env.handler, "GET", "/api/projects/"+project.Slug+"/versions", nil)
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	var versions []json.RawMessage
	_ = json.NewDecoder(rec.Body).Decode(&versions)
	if len(versions) != 0 {
		t.Errorf("got %d versions, want 0", len(versions))
	}
}

// ─── Phase 10 Page Translation Integration Tests (TEST-01) ───────────────────

func TestAdminPageTranslationsCRUD(t *testing.T) {
	env := setupTestEnv(t)
	adminCookie := createSession(t, env.db, testSecret, crypto.HashEmail("admin@test.com", "test-salt"), "admin")

	t.Run("upsert hero/de", func(t *testing.T) {
		rec := doJSON(t, env.handler, "PUT", "/api/admin/page-translations/hero/de", map[string]any{
			"fields": map[string]string{
				"headline": "Hallo",
				"tagline":  "Welt",
				"bio":      "Test",
			},
		}, adminCookie)
		if rec.Code != 204 {
			t.Fatalf("status = %d, want 204; body = %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("get hero/de", func(t *testing.T) {
		rec := doJSON(t, env.handler, "GET", "/api/admin/page-translations/hero/de", nil, adminCookie)
		if rec.Code != 200 {
			t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
		}
		var fields map[string]string
		_ = json.NewDecoder(rec.Body).Decode(&fields)
		if fields["headline"] != "Hallo" {
			t.Errorf("headline = %q, want %q", fields["headline"], "Hallo")
		}
		if fields["tagline"] != "Welt" {
			t.Errorf("tagline = %q, want %q", fields["tagline"], "Welt")
		}
		if fields["bio"] != "Test" {
			t.Errorf("bio = %q, want %q", fields["bio"], "Test")
		}
	})

	t.Run("list all includes hero/de", func(t *testing.T) {
		rec := doJSON(t, env.handler, "GET", "/api/admin/page-translations", nil, adminCookie)
		if rec.Code != 200 {
			t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
		}
		var result map[string]map[string]map[string]string
		_ = json.NewDecoder(rec.Body).Decode(&result)
		if result["hero"]["de"]["headline"] != "Hallo" {
			t.Errorf("result[hero][de][headline] = %q, want %q", result["hero"]["de"]["headline"], "Hallo")
		}
	})

	t.Run("delete hero/de", func(t *testing.T) {
		rec := doJSON(t, env.handler, "DELETE", "/api/admin/page-translations/hero/de", nil, adminCookie)
		if rec.Code != 204 {
			t.Fatalf("status = %d, want 204; body = %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("get hero/de after delete is empty", func(t *testing.T) {
		rec := doJSON(t, env.handler, "GET", "/api/admin/page-translations/hero/de", nil, adminCookie)
		if rec.Code != 200 {
			t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
		}
		var fields map[string]string
		_ = json.NewDecoder(rec.Body).Decode(&fields)
		if len(fields) != 0 {
			t.Errorf("expected empty map after delete, got %v", fields)
		}
	})
}

func TestAdminPageTranslationsValidation(t *testing.T) {
	env := setupTestEnv(t)
	adminCookie := createSession(t, env.db, testSecret, crypto.HashEmail("admin@test.com", "test-salt"), "admin")

	t.Run("invalid page key returns 404", func(t *testing.T) {
		rec := doJSON(t, env.handler, "PUT", "/api/admin/page-translations/nonexistent/de", map[string]any{
			"fields": map[string]string{},
		}, adminCookie)
		if rec.Code != 404 {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("unsupported locale returns 400", func(t *testing.T) {
		rec := doJSON(t, env.handler, "PUT", "/api/admin/page-translations/hero/zz", map[string]any{
			"fields": map[string]string{},
		}, adminCookie)
		if rec.Code != 400 {
			t.Errorf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("hero field value over 2000 chars returns 400", func(t *testing.T) {
		rec := doJSON(t, env.handler, "PUT", "/api/admin/page-translations/hero/de", map[string]any{
			"fields": map[string]string{
				"headline": strings.Repeat("a", 2001),
			},
		}, adminCookie)
		if rec.Code != 400 {
			t.Errorf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("legal field value over 100000 chars returns 400", func(t *testing.T) {
		rec := doJSON(t, env.handler, "PUT", "/api/admin/page-translations/impressum/de", map[string]any{
			"fields": map[string]string{
				"content": strings.Repeat("a", 100001),
			},
		}, adminCookie)
		if rec.Code != 400 {
			t.Errorf("status = %d, want 400", rec.Code)
		}
	})
}

func TestAdminPageTranslationsUnauthenticated(t *testing.T) {
	env := setupTestEnv(t)
	rec := doJSON(t, env.handler, "GET", "/api/admin/page-translations", nil)
	if rec.Code != 401 {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// ─── Phase 10 Hero / Legal Integration Tests (TEST-02, TEST-03) ──────────────

func TestGetHeroFromPageTranslations(t *testing.T) {
	env := setupTestEnv(t)

	// Seed page translations directly for German (default locale in test env).
	err := i18n.UpsertPageTranslationFields(context.Background(), env.db, "hero", "de", map[string]string{
		"headline": "Willkommen",
		"tagline":  "Mein Store",
		"bio":      "Entwickler",
	})
	if err != nil {
		t.Fatalf("UpsertPageTranslationFields: %v", err)
	}

	rec := doJSON(t, env.handler, "GET", "/api/hero", nil)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var result map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&result)
	if result["headline"] != "Willkommen" {
		t.Errorf("headline = %q, want %q", result["headline"], "Willkommen")
	}
	if result["tagline"] != "Mein Store" {
		t.Errorf("tagline = %q, want %q", result["tagline"], "Mein Store")
	}
	if result["bio"] != "Entwickler" {
		t.Errorf("bio = %q, want %q", result["bio"], "Entwickler")
	}
}

func TestGetHeroLocaleOverlay(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	// Seed German as base.
	if err := i18n.UpsertPageTranslationFields(ctx, env.db, "hero", "de", map[string]string{
		"headline": "DE Headline",
		"tagline":  "DE Tagline",
		"bio":      "DE Bio",
	}); err != nil {
		t.Fatalf("seed de: %v", err)
	}

	// Seed English with empty tagline — should fall back to German tagline.
	if err := i18n.UpsertPageTranslationFields(ctx, env.db, "hero", "en", map[string]string{
		"headline": "EN Headline",
		"tagline":  "",
		"bio":      "EN Bio",
	}); err != nil {
		t.Fatalf("seed en: %v", err)
	}

	// Request with Accept-Language: en to trigger overlay.
	req := httptest.NewRequest("GET", "/api/hero", nil)
	req.Header.Set("Accept-Language", "en")
	w := httptest.NewRecorder()
	env.handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	var result map[string]string
	_ = json.NewDecoder(w.Body).Decode(&result)

	if result["headline"] != "EN Headline" {
		t.Errorf("headline = %q, want %q", result["headline"], "EN Headline")
	}
	// Empty EN tagline should fall back to DE tagline.
	if result["tagline"] != "DE Tagline" {
		t.Errorf("tagline = %q, want %q (expected DE fallback for empty EN)", result["tagline"], "DE Tagline")
	}
	if result["bio"] != "EN Bio" {
		t.Errorf("bio = %q, want %q", result["bio"], "EN Bio")
	}
}

func TestLegalLocaleFallback(t *testing.T) {
	env := setupTestEnv(t)

	t.Run("falls back to default locale when requested locale has no content", func(t *testing.T) {
		// Seed impressum for German only.
		if err := i18n.UpsertPageTranslationFields(context.Background(), env.db, "impressum", "de", map[string]string{
			"content": "# Impressum\n\nTest content",
		}); err != nil {
			t.Fatalf("seed impressum de: %v", err)
		}

		// Request with Accept-Language: en — English has no content, should fall back to German.
		req := httptest.NewRequest("GET", "/api/legal/impressum", nil)
		req.Header.Set("Accept-Language", "en")
		w := httptest.NewRecorder()
		env.handler.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
		}
		var result map[string]string
		_ = json.NewDecoder(w.Body).Decode(&result)
		if !strings.Contains(result["html"], "Impressum") {
			t.Errorf("html = %q, expected to contain 'Impressum'", result["html"])
		}
		if !strings.Contains(result["html"], "Test content") {
			t.Errorf("html = %q, expected to contain 'Test content'", result["html"])
		}
	})

	t.Run("empty DB returns placeholder", func(t *testing.T) {
		// A fresh env with no seeded data.
		freshEnv := setupTestEnv(t)
		rec := doJSON(t, freshEnv.handler, "GET", "/api/legal/impressum", nil)
		if rec.Code != 200 {
			t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
		}
		var result map[string]string
		_ = json.NewDecoder(rec.Body).Decode(&result)
		if !strings.Contains(result["html"], "Please configure your legal pages") {
			t.Errorf("html = %q, expected placeholder text", result["html"])
		}
	})
}

// ─── Phase 10 Settings / Locale Tests (MIG-05, MIG-06) ───────────────────────

func TestSettingsRejectsRemovedKeys(t *testing.T) {
	env := setupTestEnv(t)
	adminCookie := createSession(t, env.db, testSecret, crypto.HashEmail("admin@test.com", "test-salt"), "admin")

	removedKeys := []string{
		"impressum_content.de",
		"hero_headline.de",
		"impressum_content",
	}
	for _, key := range removedKeys {
		rec := doJSON(t, env.handler, "PATCH", "/api/admin/settings", map[string]string{
			"key":   key,
			"value": "test",
		}, adminCookie)
		if rec.Code != 400 {
			t.Errorf("key %q: status = %d, want 400", key, rec.Code)
		}
	}
}

func TestCreateLocaleNoSettingsSeed(t *testing.T) {
	env := setupTestEnv(t)
	adminCookie := createSession(t, env.db, testSecret, crypto.HashEmail("admin@test.com", "test-salt"), "admin")

	// Create a new locale.
	rec := doJSON(t, env.handler, "POST", "/api/admin/locales", map[string]any{
		"code":       "fr",
		"name":       "French",
		"sort_order": 2,
	}, adminCookie)
	if rec.Code != 201 {
		t.Fatalf("create locale: status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}

	// GET all settings and verify no impressum_content.fr or privacy_policy_content.fr were seeded.
	rec = doJSON(t, env.handler, "GET", "/api/admin/settings", nil, adminCookie)
	if rec.Code != 200 {
		t.Fatalf("get settings: status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var settings map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&settings)

	if _, ok := settings["impressum_content.fr"]; ok {
		t.Error("settings should not contain impressum_content.fr after locale creation")
	}
	if _, ok := settings["privacy_policy_content.fr"]; ok {
		t.Error("settings should not contain privacy_policy_content.fr after locale creation")
	}
}
