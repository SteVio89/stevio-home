package handlers_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/SteVio89/stevio-home/crypto"
)

func TestProjectsCRUD(t *testing.T) {
	env := setupTestEnv(t)
	adminCookie := createSession(t, env.db, testSecret, crypto.HashEmail("admin@test.com", "test-salt"), "admin")

	var projectID string
	var commerceProjectID string

	t.Run("create project with external URL", func(t *testing.T) {
		rec := doJSON(t, env.handler, "POST", "/api/admin/projects", map[string]any{
			"external_url": "https://github.com/test",
			"image_url":    "/media/test.png",
			"title":        "Test Project",
			"tagline":      "A tagline",
			"description":  "A description",
		}, adminCookie)
		if rec.Code != 201 {
			t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
		}
		var body map[string]any
		_ = json.NewDecoder(rec.Body).Decode(&body)
		id, ok := body["id"].(string)
		if !ok || id == "" {
			t.Fatal("response missing id")
		}
		projectID = id
		if pos, ok := body["position"].(float64); !ok || pos < 0 {
			t.Errorf("position = %v, want >= 0", body["position"])
		}
	})

	t.Run("create project with commerce", func(t *testing.T) {
		rec := doJSON(t, env.handler, "POST", "/api/admin/projects", map[string]any{
			"slug":        "store-app",
			"image_url":   "/media/test.png",
			"title":       "Store App",
			"tagline":     "Linked",
			"description": "Linked project",
			"commerce": map[string]any{
				"bundle_id":     "com.test.commerce",
				"price_cents":   1499,
				"purchase_mode": "always_new_license",
			},
		}, adminCookie)
		if rec.Code != 201 {
			t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
		}
		var body map[string]any
		_ = json.NewDecoder(rec.Body).Decode(&body)
		commerceProjectID = body["id"].(string)
	})

	t.Run("reject commerce + external_url", func(t *testing.T) {
		rec := doJSON(t, env.handler, "POST", "/api/admin/projects", map[string]any{
			"external_url": "https://example.com",
			"image_url":    "/media/test.png",
			"title":        "Bad",
			"commerce": map[string]any{
				"bundle_id":   "com.test.bad",
				"price_cents": 100,
			},
		}, adminCookie)
		if rec.Code != 400 {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("plain showcase is allowed", func(t *testing.T) {
		// Neither commerce nor external_url is now valid (plain showcase).
		rec := doJSON(t, env.handler, "POST", "/api/admin/projects", map[string]any{
			"image_url":   "/media/test.png",
			"title":       "Plain Showcase",
			"tagline":     "Just a card",
			"description": "Informational only",
		}, adminCookie)
		if rec.Code != 201 {
			t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("reject invalid image_url", func(t *testing.T) {
		rec := doJSON(t, env.handler, "POST", "/api/admin/projects", map[string]any{
			"external_url": "https://example.com",
			"image_url":    "https://evil.com/image.png",
			"title":        "Bad",
			"tagline":      "Bad",
			"description":  "Bad",
		}, adminCookie)
		if rec.Code != 400 {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("list projects admin", func(t *testing.T) {
		rec := doJSON(t, env.handler, "GET", "/api/admin/projects", nil, adminCookie)
		if rec.Code != 200 {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var items []json.RawMessage
		_ = json.NewDecoder(rec.Body).Decode(&items)
		if len(items) < 2 {
			t.Errorf("got %d items, want >= 2", len(items))
		}
	})

	t.Run("update project", func(t *testing.T) {
		rec := doJSON(t, env.handler, "PATCH", "/api/admin/projects/"+projectID, map[string]any{
			"external_url": "https://github.com/updated",
			"image_url":    "/media/test.png",
			"title":        "Updated",
			"tagline":      "Updated tag",
			"description":  "Updated desc",
		}, adminCookie)
		if rec.Code != 204 {
			t.Fatalf("status = %d, want 204; body = %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("soft-delete project", func(t *testing.T) {
		rec := doJSON(t, env.handler, "DELETE", "/api/admin/projects/"+projectID, nil, adminCookie)
		if rec.Code != 204 {
			t.Fatalf("status = %d, want 204", rec.Code)
		}
	})

	t.Run("list public excludes deleted", func(t *testing.T) {
		rec := doJSON(t, env.handler, "GET", "/api/projects", nil)
		if rec.Code != 200 {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		body := rec.Body.String()
		if strings.Contains(body, projectID) {
			t.Error("deleted project should not appear in public listing")
		}
	})

	t.Run("restore project", func(t *testing.T) {
		rec := doJSON(t, env.handler, "POST", "/api/admin/projects/"+projectID+"/restore", nil, adminCookie)
		if rec.Code != 204 {
			t.Fatalf("status = %d, want 204", rec.Code)
		}
	})

	t.Run("list public includes restored", func(t *testing.T) {
		rec := doJSON(t, env.handler, "GET", "/api/projects", nil)
		if rec.Code != 200 {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, projectID) {
			t.Error("restored project should appear in public listing")
		}
	})

	t.Run("commerce project carries commerce block", func(t *testing.T) {
		rec := doJSON(t, env.handler, "GET", "/api/projects", nil)
		if rec.Code != 200 {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var items []map[string]any
		_ = json.NewDecoder(rec.Body).Decode(&items)
		found := false
		for _, item := range items {
			if item["id"] == commerceProjectID {
				found = true
				if item["commerce"] == nil {
					t.Error("commerce project missing commerce block in listing")
				}
			}
		}
		if !found {
			t.Error("commerce project not found in listing")
		}
	})

	t.Run("reorder projects", func(t *testing.T) {
		rec := doJSON(t, env.handler, "PATCH", "/api/admin/projects/reorder", map[string]any{
			"positions": map[string]int{
				projectID:         1,
				commerceProjectID: 0,
			},
		}, adminCookie)
		if rec.Code != 204 {
			t.Fatalf("status = %d, want 204; body = %s", rec.Code, rec.Body.String())
		}

		// Verify order
		rec = doJSON(t, env.handler, "GET", "/api/projects", nil)
		var items []map[string]any
		_ = json.NewDecoder(rec.Body).Decode(&items)
		if len(items) >= 2 && items[0]["id"] != commerceProjectID {
			t.Errorf("first project should be %s (position 0), got %s", commerceProjectID, items[0]["id"])
		}
	})
}

func TestProjectTranslations(t *testing.T) {
	env := setupTestEnv(t)
	adminCookie := createSession(t, env.db, testSecret, crypto.HashEmail("admin@test.com", "test-salt"), "admin")

	// Create a project first.
	rec := doJSON(t, env.handler, "POST", "/api/admin/projects", map[string]any{
		"external_url": "https://github.com/trans-test",
		"image_url":    "/media/test.png",
		"title":        "Deutsches Projekt",
		"tagline":      "Ein Tagline",
		"description":  "Eine Beschreibung",
	}, adminCookie)
	if rec.Code != 201 {
		t.Fatalf("create status = %d; body = %s", rec.Code, rec.Body.String())
	}
	var created map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&created)
	projID := created["id"].(string)

	t.Run("get default locale translations", func(t *testing.T) {
		rec := doJSON(t, env.handler, "GET", "/api/admin/projects/"+projID+"/translations/de", nil, adminCookie)
		if rec.Code != 200 {
			t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
		}
		var fields map[string]string
		_ = json.NewDecoder(rec.Body).Decode(&fields)
		if fields["title"] != "Deutsches Projekt" {
			t.Errorf("title = %q, want %q", fields["title"], "Deutsches Projekt")
		}
	})

	t.Run("upsert English translations", func(t *testing.T) {
		rec := doJSON(t, env.handler, "PUT", "/api/admin/projects/"+projID+"/translations/en", map[string]string{
			"title":       "English Title",
			"tagline":     "English Tagline",
			"description": "English Desc",
		}, adminCookie)
		if rec.Code != 204 {
			t.Fatalf("status = %d, want 204; body = %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("public endpoint returns translated content", func(t *testing.T) {
		// English translations were already upserted via the admin PUT endpoint above.
		// The public endpoint reads Accept-Language header via c.Lang().
		// In test the default locale is "de", so requesting with no language header returns German.
		rec := doJSON(t, env.handler, "GET", "/api/projects", nil)
		if rec.Code != 200 {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var items []map[string]any
		_ = json.NewDecoder(rec.Body).Decode(&items)
		for _, item := range items {
			if item["id"] == projID {
				if item["title"] != "Deutsches Projekt" {
					t.Errorf("default locale title = %q, want %q", item["title"], "Deutsches Projekt")
				}
			}
		}
	})
}

func TestSocialLinksCRUD(t *testing.T) {
	env := setupTestEnv(t)
	adminCookie := createSession(t, env.db, testSecret, crypto.HashEmail("admin@test.com", "test-salt"), "admin")

	var linkID string
	var link2ID string

	t.Run("create social link", func(t *testing.T) {
		rec := doJSON(t, env.handler, "POST", "/api/admin/social-links", map[string]string{
			"platform": "github",
			"url":      "https://github.com/stefantest",
		}, adminCookie)
		if rec.Code != 201 {
			t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
		}
		var body map[string]any
		_ = json.NewDecoder(rec.Body).Decode(&body)
		linkID = body["id"].(string)
		if body["platform"] != "github" {
			t.Errorf("platform = %v, want github", body["platform"])
		}
	})

	t.Run("create second link", func(t *testing.T) {
		rec := doJSON(t, env.handler, "POST", "/api/admin/social-links", map[string]string{
			"platform": "linkedin",
			"url":      "https://linkedin.com/in/test",
		}, adminCookie)
		if rec.Code != 201 {
			t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
		}
		var body map[string]any
		_ = json.NewDecoder(rec.Body).Decode(&body)
		link2ID = body["id"].(string)
	})

	t.Run("reject invalid platform", func(t *testing.T) {
		rec := doJSON(t, env.handler, "POST", "/api/admin/social-links", map[string]string{
			"platform": "tiktok",
			"url":      "https://tiktok.com",
		}, adminCookie)
		if rec.Code != 400 {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("list social links admin", func(t *testing.T) {
		rec := doJSON(t, env.handler, "GET", "/api/admin/social-links", nil, adminCookie)
		if rec.Code != 200 {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var items []json.RawMessage
		_ = json.NewDecoder(rec.Body).Decode(&items)
		if len(items) != 2 {
			t.Errorf("got %d items, want 2", len(items))
		}
	})

	t.Run("list social links public", func(t *testing.T) {
		rec := doJSON(t, env.handler, "GET", "/api/social-links", nil)
		if rec.Code != 200 {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var items []json.RawMessage
		_ = json.NewDecoder(rec.Body).Decode(&items)
		if len(items) != 2 {
			t.Errorf("got %d items, want 2", len(items))
		}
	})

	t.Run("update social link", func(t *testing.T) {
		rec := doJSON(t, env.handler, "PATCH", "/api/admin/social-links/"+linkID, map[string]string{
			"platform": "github",
			"url":      "https://github.com/updated",
		}, adminCookie)
		if rec.Code != 204 {
			t.Fatalf("status = %d, want 204; body = %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("delete social link", func(t *testing.T) {
		rec := doJSON(t, env.handler, "DELETE", "/api/admin/social-links/"+linkID, nil, adminCookie)
		if rec.Code != 204 {
			t.Fatalf("status = %d, want 204", rec.Code)
		}
	})

	t.Run("list after delete", func(t *testing.T) {
		rec := doJSON(t, env.handler, "GET", "/api/social-links", nil)
		if rec.Code != 200 {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var items []json.RawMessage
		_ = json.NewDecoder(rec.Body).Decode(&items)
		if len(items) != 1 {
			t.Errorf("got %d items, want 1 (hard-deleted link should be gone)", len(items))
		}
	})

	t.Run("reorder social links", func(t *testing.T) {
		// Create a third link.
		rec := doJSON(t, env.handler, "POST", "/api/admin/social-links", map[string]string{
			"platform": "steam",
			"url":      "https://store.steampowered.com/test",
		}, adminCookie)
		if rec.Code != 201 {
			t.Fatalf("create third: status = %d", rec.Code)
		}
		var body map[string]any
		_ = json.NewDecoder(rec.Body).Decode(&body)
		link3ID := body["id"].(string)

		rec = doJSON(t, env.handler, "PATCH", "/api/admin/social-links/reorder", map[string]any{
			"positions": map[string]int{
				link2ID: 1,
				link3ID: 0,
			},
		}, adminCookie)
		if rec.Code != 204 {
			t.Fatalf("reorder: status = %d; body = %s", rec.Code, rec.Body.String())
		}

		rec = doJSON(t, env.handler, "GET", "/api/social-links", nil)
		var items []map[string]any
		_ = json.NewDecoder(rec.Body).Decode(&items)
		if len(items) >= 2 && items[0]["id"] != link3ID {
			t.Errorf("first link should be %s (position 0), got %s", link3ID, items[0]["id"])
		}
	})
}

func TestHeroContent(t *testing.T) {
	env := setupTestEnv(t)
	adminCookie := createSession(t, env.db, testSecret, crypto.HashEmail("admin@test.com", "test-salt"), "admin")

	t.Run("set hero fields for German via page translations", func(t *testing.T) {
		rec := doJSON(t, env.handler, "PUT", "/api/admin/page-translations/hero/de", map[string]any{
			"fields": map[string]string{
				"headline": "Willkommen",
				"tagline":  "Entwickler",
				"bio":      "Hallo Welt",
			},
		}, adminCookie)
		if rec.Code != 204 {
			t.Fatalf("status = %d, want 204; body = %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("get hero public", func(t *testing.T) {
		rec := doJSON(t, env.handler, "GET", "/api/hero", nil)
		if rec.Code != 200 {
			t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
		}
		var body map[string]string
		_ = json.NewDecoder(rec.Body).Decode(&body)
		if body["headline"] != "Willkommen" {
			t.Errorf("headline = %q, want %q", body["headline"], "Willkommen")
		}
		if body["tagline"] != "Entwickler" {
			t.Errorf("tagline = %q, want %q", body["tagline"], "Entwickler")
		}
		if body["bio"] != "Hallo Welt" {
			t.Errorf("bio = %q, want %q", body["bio"], "Hallo Welt")
		}
	})

	t.Run("reject hero field value too long", func(t *testing.T) {
		rec := doJSON(t, env.handler, "PUT", "/api/admin/page-translations/hero/de", map[string]any{
			"fields": map[string]string{
				"headline": strings.Repeat("a", 2001),
			},
		}, adminCookie)
		if rec.Code != 400 {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("reject unsupported locale", func(t *testing.T) {
		rec := doJSON(t, env.handler, "PUT", "/api/admin/page-translations/hero/zz", map[string]any{
			"fields": map[string]string{
				"headline": "test",
			},
		}, adminCookie)
		if rec.Code != 400 {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("reject invalid page key", func(t *testing.T) {
		rec := doJSON(t, env.handler, "PUT", "/api/admin/page-translations/invalid_page/de", map[string]any{
			"fields": map[string]string{
				"content": "test",
			},
		}, adminCookie)
		if rec.Code != 404 {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
	})
}
