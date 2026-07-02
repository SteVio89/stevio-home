package i18n

import (
	"sync"
	"testing"
)

func TestLookup_ExactMatch(t *testing.T) {
	t.Parallel()
	c := NewCatalog("en")
	c.Load("en", map[string]string{"greeting": "Hello"})
	c.Load("de", map[string]string{"greeting": "Hallo"})

	if got := c.Lookup("de", "greeting"); got != "Hallo" {
		t.Errorf("Lookup(de, greeting) = %q, want %q", got, "Hallo")
	}
}

func TestLookup_FallbackLanguage(t *testing.T) {
	t.Parallel()
	c := NewCatalog("en")
	c.Load("en", map[string]string{"greeting": "Hello"})

	if got := c.Lookup("fr", "greeting"); got != "Hello" {
		t.Errorf("Lookup(fr, greeting) should fall back to en, got %q", got)
	}
}

func TestLookup_SubtagFallback(t *testing.T) {
	t.Parallel()
	c := NewCatalog("en")
	c.Load("pt", map[string]string{"greeting": "Olá"})

	if got := c.Lookup("pt-br", "greeting"); got != "Olá" {
		t.Errorf("Lookup(pt-br, greeting) should match pt, got %q", got)
	}
}

func TestLookup_ReturnsKeyWhenMissing(t *testing.T) {
	t.Parallel()
	c := NewCatalog("en")
	c.Load("en", map[string]string{})

	if got := c.Lookup("en", "missing.key"); got != "missing.key" {
		t.Errorf("Lookup for missing key = %q, want %q", got, "missing.key")
	}
}

func TestLookup_WithArgs(t *testing.T) {
	t.Parallel()
	c := NewCatalog("en")
	c.Load("en", map[string]string{"welcome": "Hello, %s!"})

	if got := c.Lookup("en", "welcome", "World"); got != "Hello, World!" {
		t.Errorf("Lookup with args = %q, want %q", got, "Hello, World!")
	}
}

func TestLookup_CaseInsensitive(t *testing.T) {
	t.Parallel()
	c := NewCatalog("en")
	c.Load("EN", map[string]string{"greeting": "Hello"})

	if got := c.Lookup("en", "greeting"); got != "Hello" {
		t.Errorf("Lookup should be case-insensitive, got %q", got)
	}
}

func TestSet(t *testing.T) {
	t.Parallel()
	c := NewCatalog("en")
	c.Set("en", "key", "value")

	if got := c.Lookup("en", "key"); got != "value" {
		t.Errorf("Set then Lookup = %q, want %q", got, "value")
	}
}

func TestLanguages(t *testing.T) {
	t.Parallel()
	c := NewCatalog("en")
	c.Load("en", map[string]string{"a": "1"})
	c.Load("de", map[string]string{"a": "2"})

	langs := c.Languages()
	if len(langs) != 2 {
		t.Fatalf("Languages() returned %d, want 2", len(langs))
	}
}

func TestConcurrentAccess(t *testing.T) {
	c := NewCatalog("en")
	c.Load("en", map[string]string{"key": "value"})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			c.Lookup("en", "key")
		}()
		go func() {
			defer wg.Done()
			c.Set("en", "key", "updated")
		}()
	}
	wg.Wait()

	// No panic or data race = pass.
	// Run with -race to verify.
	got := c.Lookup("en", "key")
	if got != "value" && got != "updated" {
		t.Errorf("unexpected value after concurrent access: %q", got)
	}
}

func TestResolve(t *testing.T) {
	t.Parallel()
	c := NewCatalog("en")
	c.Load("en", map[string]string{"a": "1"})
	c.Load("de", map[string]string{"a": "2"})

	tests := []struct {
		name       string
		acceptLang string
		want       string
	}{
		{"exact match", "de", "de"},
		{"quality preference", "fr;q=0.5, de;q=0.9", "de"},
		{"subtag match", "de-AT", "de"},
		{"empty header", "", "en"},
		{"no match falls back", "ja", "en"},
		{"wildcard with loaded", "de, *;q=0.1", "de"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.Resolve(tt.acceptLang)
			if got != tt.want {
				t.Errorf("Resolve(%q) = %q, want %q", tt.acceptLang, got, tt.want)
			}
		})
	}
}
