package app_test

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"

	appsvc "github.com/SteVio89/stevio-home/app"
)

func setupTestApp(t *testing.T) (*appsvc.App, *appsvc.TestMailer) {
	t.Helper()
	return appsvc.SetupTestApp(t, appsvc.Config{
		DefaultRole: "member",
	})
}

func TestPublicRoute(t *testing.T) {
	app, _ := setupTestApp(t)
	app.Public().Handle("GET /health", func(c *appsvc.Ctx) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	client := appsvc.NewTestClient(t, app)
	resp := client.GET("/health")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("want status=ok, got %q", body["status"])
	}
}

func TestAuthenticatedRoute_Unauthenticated(t *testing.T) {
	app, _ := setupTestApp(t)
	app.Authenticated().Handle("GET /me", func(c *appsvc.Ctx) error {
		return c.JSON(200, c.User())
	})

	client := appsvc.NewTestClient(t, app)
	resp := client.GET("/me")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 401 {
		t.Fatalf("want 401, got %d", resp.StatusCode)
	}
}

func TestAuthenticatedRoute_Authenticated(t *testing.T) {
	app, _ := setupTestApp(t)
	app.Authenticated().Handle("GET /me", func(c *appsvc.Ctx) error {
		return c.JSON(200, map[string]string{
			"id":   c.User().ID,
			"role": c.User().Role,
		})
	})

	client := appsvc.NewTestClient(t, app).AsUser("test@example.com", "member")
	resp := client.GET("/me")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["role"] != "member" {
		t.Errorf("want role=member, got %q", body["role"])
	}
	if body["id"] == "" {
		t.Error("want non-empty user ID")
	}
}

func TestRoleRoute_WrongRole(t *testing.T) {
	app, _ := setupTestApp(t)
	app.Role("admin").Handle("GET /admin", func(c *appsvc.Ctx) error {
		return c.JSON(200, map[string]string{"ok": "true"})
	})

	client := appsvc.NewTestClient(t, app).AsUser("user@example.com", "member")
	resp := client.GET("/admin")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 403 {
		t.Fatalf("want 403, got %d", resp.StatusCode)
	}
}

func TestRoleRoute_CorrectRole(t *testing.T) {
	app, _ := setupTestApp(t)
	app.Role("admin").Handle("GET /admin", func(c *appsvc.Ctx) error {
		return c.JSON(200, map[string]string{"ok": "true"})
	})

	client := appsvc.NewTestClient(t, app).AsUser("admin@example.com", "admin")
	resp := client.GET("/admin")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

func TestLoginEndpoint_SendsEmail(t *testing.T) {
	app, mailer := setupTestApp(t)

	client := appsvc.NewTestClient(t, app)
	resp := client.POST("/auth/login", map[string]string{
		"email": "test@example.com",
	})
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("want 200, got %d: %s", resp.StatusCode, body)
	}

	emails := mailer.SentEmails()
	if len(emails) != 1 {
		t.Fatalf("want 1 email sent, got %d", len(emails))
	}
	if emails[0].To != "test@example.com" {
		t.Errorf("want email to test@example.com, got %s", emails[0].To)
	}
}

func TestLoginEndpoint_InvalidEmail(t *testing.T) {
	app, _ := setupTestApp(t)

	client := appsvc.NewTestClient(t, app)
	resp := client.POST("/auth/login", map[string]string{
		"email": "not-an-email",
	})
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 400 {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestLoginEndpoint_DuplicateToken(t *testing.T) {
	app, mailer := setupTestApp(t)

	client := appsvc.NewTestClient(t, app)

	// First login.
	resp := client.POST("/auth/login", map[string]string{
		"email": "dup@example.com",
	})
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("first login: want 200, got %d", resp.StatusCode)
	}

	// Second login — should not send another email.
	resp = client.POST("/auth/login", map[string]string{
		"email": "dup@example.com",
	})
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		t.Fatalf("second login: want 200, got %d", resp.StatusCode)
	}

	if emails := mailer.SentEmails(); len(emails) != 1 {
		t.Errorf("want 1 email total, got %d", len(emails))
	}
}

func TestLogoutEndpoint(t *testing.T) {
	app, _ := setupTestApp(t)

	client := appsvc.NewTestClient(t, app).AsUser("test@example.com", "member")
	resp := client.POST("/auth/logout", nil)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 204 {
		t.Fatalf("want 204, got %d", resp.StatusCode)
	}

	// Verify cookie is cleared.
	for _, c := range resp.Cookies() {
		if c.Name == "sid" && c.MaxAge < 0 {
			return // cookie cleared
		}
	}
	t.Error("want cleared sid cookie")
}

func TestGroupRoutes(t *testing.T) {
	app, _ := setupTestApp(t)

	api := app.Group("/api/v1")
	api.Public().Handle("GET /status", func(c *appsvc.Ctx) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})
	api.Authenticated().Handle("GET /profile", func(c *appsvc.Ctx) error {
		return c.JSON(200, c.User())
	})

	client := appsvc.NewTestClient(t, app)

	// Public route through group.
	resp := client.GET("/api/v1/status")
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("public group route: want 200, got %d", resp.StatusCode)
	}

	// Authenticated route without auth.
	resp = client.GET("/api/v1/profile")
	_ = resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Errorf("auth group route unauthenticated: want 401, got %d", resp.StatusCode)
	}

	// Authenticated route with auth.
	authedClient := appsvc.NewTestClient(t, app).AsUser("test@example.com", "member")
	resp = authedClient.GET("/api/v1/profile")
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("auth group route authenticated: want 200, got %d", resp.StatusCode)
	}
}

func TestHandlerErrorDispatch_APIError(t *testing.T) {
	app, _ := setupTestApp(t)
	app.Public().Handle("GET /not-found", func(c *appsvc.Ctx) error {
		return appsvc.NotFound("item not found")
	})

	client := appsvc.NewTestClient(t, app)
	resp := client.GET("/not-found")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 404 {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}

	var body map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["error"] != "not_found" {
		t.Errorf("want error=not_found, got %q", body["error"])
	}
}

func TestHandlerErrorDispatch_InternalError(t *testing.T) {
	app, _ := setupTestApp(t)
	app.Public().Handle("GET /fail", func(c *appsvc.Ctx) error {
		return http.ErrServerClosed // any generic error
	})

	client := appsvc.NewTestClient(t, app)
	resp := client.GET("/fail")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 500 {
		t.Fatalf("want 500, got %d", resp.StatusCode)
	}
}

func TestPathParams(t *testing.T) {
	app, _ := setupTestApp(t)
	app.Public().Handle("GET /items/{id}", func(c *appsvc.Ctx) error {
		return c.JSON(200, map[string]string{"id": c.Param("id")})
	})

	client := appsvc.NewTestClient(t, app)
	resp := client.GET("/items/abc-123")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["id"] != "abc-123" {
		t.Errorf("want id=abc-123, got %q", body["id"])
	}
}

// tokenFromEmail extracts the token from a magic-link email body.
var tokenRe = regexp.MustCompile(`token=([a-zA-Z0-9_-]+)`)

func extractToken(body string) string {
	m := tokenRe.FindStringSubmatch(body)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func TestFullLoginVerifyFlow(t *testing.T) {
	app, mailer := setupTestApp(t)
	app.Authenticated().Handle("GET /me", func(c *appsvc.Ctx) error {
		return c.JSON(200, map[string]string{
			"id":   c.User().ID,
			"role": c.User().Role,
		})
	})

	client := appsvc.NewTestClient(t, app)

	// Step 1: Login — sends magic link email.
	resp := client.POST("/auth/login", map[string]string{
		"email": "flow@example.com",
	})
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("login: want 200, got %d", resp.StatusCode)
	}
	emails := mailer.SentEmails()
	if len(emails) != 1 {
		t.Fatalf("want 1 email, got %d", len(emails))
	}

	token := extractToken(emails[0].Body)
	if token == "" {
		t.Fatal("could not extract token from email body")
	}

	// Step 2: Verify — exchanges token for session cookie.
	resp = client.POST("/auth/verify", map[string]string{
		"token": token,
	})
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("verify: want 200, got %d: %s", resp.StatusCode, body)
	}

	var verifyBody map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&verifyBody)
	_ = resp.Body.Close()

	if verifyBody["user_id"] == "" {
		t.Error("verify response should include user_id")
	}
	if verifyBody["role"] != "member" {
		t.Errorf("verify role = %v, want member", verifyBody["role"])
	}

	// Extract session cookie from verify response.
	var sessionCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "sid" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("verify response should set sid cookie")
	}

	// Step 3: Use session cookie to access authenticated route.
	// Build a manual request with the cookie.
	req, _ := http.NewRequest("GET", client.Server.URL+"/me", nil)
	req.AddCookie(sessionCookie)
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("GET /me: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET /me: want 200, got %d: %s", resp.StatusCode, body)
	}

	var meBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&meBody)
	if meBody["role"] != "member" {
		t.Errorf("GET /me role = %q, want member", meBody["role"])
	}
	if meBody["id"] == "" {
		t.Error("GET /me should return non-empty user ID")
	}

	// Step 4: Token reuse should fail.
	resp2 := client.POST("/auth/verify", map[string]string{
		"token": token,
	})
	defer func() { _ = resp2.Body.Close() }()
	if resp2.StatusCode != 401 {
		t.Errorf("token reuse: want 401, got %d", resp2.StatusCode)
	}
}

func TestVerifyEndpoint_InvalidToken(t *testing.T) {
	app, _ := setupTestApp(t)
	client := appsvc.NewTestClient(t, app)

	resp := client.POST("/auth/verify", map[string]string{
		"token": "nonexistent-token",
	})
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 401 {
		t.Errorf("want 401, got %d", resp.StatusCode)
	}
}

func TestVerifyEndpoint_EmptyToken(t *testing.T) {
	app, _ := setupTestApp(t)
	client := appsvc.NewTestClient(t, app)

	resp := client.POST("/auth/verify", map[string]string{
		"token": "",
	})
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestDecodeValidate_ValidationError(t *testing.T) {
	app, _ := setupTestApp(t)
	app.Public().Handle("POST /validate-test", func(c *appsvc.Ctx) error {
		var req struct {
			Name string `json:"name"`
		}
		return c.DecodeValidate(&req, func(ve *appsvc.ValidationErrors) {
			if req.Name == "" {
				ve.Add("name", appsvc.ErrRequired)
			}
		})
	})

	client := appsvc.NewTestClient(t, app)

	// Empty name should return 422.
	resp := client.POST("/validate-test", map[string]string{"name": ""})
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 422 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("want 422, got %d: %s", resp.StatusCode, body)
	}

	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["error"] != "validation_failed" {
		t.Errorf("want error=validation_failed, got %v", body["error"])
	}
}

func TestDecode_MissingContentType(t *testing.T) {
	app, _ := setupTestApp(t)
	app.Public().Handle("POST /ct-test", func(c *appsvc.Ctx) error {
		var req struct{ Name string }
		if err := c.Decode(&req); err != nil {
			return err
		}
		return c.JSON(200, req)
	})

	// Build a raw request without Content-Type.
	server := appsvc.NewTestClient(t, app)
	req, _ := http.NewRequest("POST", server.Server.URL+"/ct-test", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 400 {
		t.Errorf("missing Content-Type: want 400, got %d", resp.StatusCode)
	}
}

func TestDecode_OversizedBody(t *testing.T) {
	app, _ := setupTestApp(t)
	app.Public().Handle("POST /big-body", func(c *appsvc.Ctx) error {
		var req struct{ Data string }
		if err := c.Decode(&req); err != nil {
			return err
		}
		return c.JSON(200, req)
	})

	server := appsvc.NewTestClient(t, app)
	// Build a body larger than 1 MB.
	bigBody := make([]byte, 1<<20+1024)
	for i := range bigBody {
		bigBody[i] = 'x'
	}
	body := `{"data":"` + string(bigBody) + `"}`
	req, _ := http.NewRequest("POST", server.Server.URL+"/big-body", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 400 {
		t.Errorf("oversized body: want 400, got %d", resp.StatusCode)
	}
}

func TestDecode_TrailingJSON(t *testing.T) {
	app, _ := setupTestApp(t)
	app.Public().Handle("POST /trailing", func(c *appsvc.Ctx) error {
		var req struct{ Name string }
		if err := c.Decode(&req); err != nil {
			return err
		}
		return c.JSON(200, req)
	})

	server := appsvc.NewTestClient(t, app)
	body := `{"name":"first"}{"name":"second"}`
	req, _ := http.NewRequest("POST", server.Server.URL+"/trailing", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 400 {
		t.Errorf("trailing JSON: want 400, got %d", resp.StatusCode)
	}
}

func TestDecode_WrongContentType(t *testing.T) {
	app, _ := setupTestApp(t)
	app.Public().Handle("POST /ct-test2", func(c *appsvc.Ctx) error {
		var req struct{ Name string }
		if err := c.Decode(&req); err != nil {
			return err
		}
		return c.JSON(200, req)
	})

	server := appsvc.NewTestClient(t, app)
	req, _ := http.NewRequest("POST", server.Server.URL+"/ct-test2", nil)
	req.Header.Set("Content-Type", "text/plain")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 400 {
		t.Errorf("wrong Content-Type: want 400, got %d", resp.StatusCode)
	}
}

func TestGroupRoute_Role(t *testing.T) {
	app, _ := setupTestApp(t)

	api := app.Group("/api/v1")
	api.Role("admin").Handle("GET /admin-only", func(c *appsvc.Ctx) error {
		return c.JSON(200, map[string]string{"ok": "true"})
	})

	client := appsvc.NewTestClient(t, app)

	// Member should be denied.
	memberClient := appsvc.NewTestClient(t, app).AsUser("user@test.com", "member")
	resp := memberClient.GET("/api/v1/admin-only")
	_ = resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Errorf("member: want 403, got %d", resp.StatusCode)
	}

	// Admin should pass.
	adminClient := appsvc.NewTestClient(t, app).AsUser("admin@test.com", "admin")
	resp = adminClient.GET("/api/v1/admin-only")
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("admin: want 200, got %d", resp.StatusCode)
	}

	// Unauthenticated should be 401.
	resp = client.GET("/api/v1/admin-only")
	_ = resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Errorf("unauth: want 401, got %d", resp.StatusCode)
	}
}

func TestUpdateUserRole(t *testing.T) {
	app, _ := setupTestApp(t)

	app.Authenticated().Handle("GET /me", func(c *appsvc.Ctx) error {
		return c.JSON(200, map[string]string{"role": c.User().Role})
	})

	// Create a user with member role.
	client := appsvc.NewTestClient(t, app).AsUser("role-test@example.com", "member")
	resp := client.GET("/me")
	var body map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&body)
	_ = resp.Body.Close()

	if body["role"] != "member" {
		t.Fatalf("initial role: want member, got %q", body["role"])
	}
	userID := ""
	// Get the user ID by re-reading the response (we need to fetch it from a route).
	app.Authenticated().Handle("GET /my-id", func(c *appsvc.Ctx) error {
		return c.JSON(200, map[string]string{"id": c.User().ID})
	})
	client2 := appsvc.NewTestClient(t, app).AsUser("role-test@example.com", "member")
	resp = client2.GET("/my-id")
	_ = json.NewDecoder(resp.Body).Decode(&body)
	_ = resp.Body.Close()
	userID = body["id"]

	// Update role to admin — this should invalidate all sessions.
	if err := app.UpdateUserRole(t.Context(), userID, "admin"); err != nil {
		t.Fatalf("UpdateUserRole: %v", err)
	}

	// Old session should be invalidated — 401.
	resp = client.GET("/me")
	_ = resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Errorf("old session after role change: want 401, got %d", resp.StatusCode)
	}
}
