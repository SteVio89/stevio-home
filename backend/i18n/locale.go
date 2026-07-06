package i18n

import (
	"context"
	"database/sql"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
)

// Locale represents a language entry in the locale registry.
type Locale struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
	Enabled   bool   `json:"enabled"`
	SortOrder int    `json:"sort_order"`
	CreatedAt string `json:"created_at,omitempty"`
}

// LocaleCache is a thread-safe, TTL-based cache over the locale registry.
type LocaleCache struct {
	db        *sql.DB
	mu        sync.RWMutex
	cached    []Locale
	codes     []string // derived from cached
	defCode   string   // derived from cached
	fetchedAt time.Time
	ttl       time.Duration
	fallback  []Locale
}

// LocaleCacheOption configures the LocaleCache.
type LocaleCacheOption func(*LocaleCache)

// WithTTL sets the cache time-to-live. Default: 30s.
func WithTTL(d time.Duration) LocaleCacheOption {
	return func(c *LocaleCache) {
		c.ttl = d
	}
}

// WithFallbackLocales sets the locales used when the DB is empty or erroring.
func WithFallbackLocales(locales []Locale) LocaleCacheOption {
	return func(c *LocaleCache) {
		c.fallback = locales
	}
}

// NewLocaleCache creates a new LocaleCache.
func NewLocaleCache(db *sql.DB, opts ...LocaleCacheOption) *LocaleCache {
	c := &LocaleCache{
		db:  db,
		ttl: 30 * time.Second,
		fallback: []Locale{
			{Code: "en", Name: "English", IsDefault: true, Enabled: true},
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// EnabledLocales returns all enabled locales, reloading from DB if the cache
// has expired.
func (c *LocaleCache) EnabledLocales(ctx context.Context) []Locale {
	c.ensureFresh(ctx)
	c.mu.RLock()
	defer c.mu.RUnlock()
	cp := make([]Locale, len(c.cached))
	copy(cp, c.cached)
	return cp
}

// Supported returns the enabled locale codes.
func (c *LocaleCache) Supported(ctx context.Context) []string {
	c.ensureFresh(ctx)
	c.mu.RLock()
	defer c.mu.RUnlock()
	cp := make([]string, len(c.codes))
	copy(cp, c.codes)
	return cp
}

// Default returns the default locale code.
func (c *LocaleCache) Default(ctx context.Context) string {
	c.ensureFresh(ctx)
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.defCode
}

// IsSupported checks whether a locale code is enabled.
func (c *LocaleCache) IsSupported(ctx context.Context, code string) bool {
	c.ensureFresh(ctx)
	code = strings.ToLower(code)
	c.mu.RLock()
	defer c.mu.RUnlock()
	return slices.Contains(c.codes, code)
}

// FromRequest resolves the best locale from the request's Accept-Language header.
func (c *LocaleCache) FromRequest(r *http.Request) string {
	ctx := r.Context()
	c.ensureFresh(ctx)
	c.mu.RLock()
	supported := c.codes
	def := c.defCode
	c.mu.RUnlock()
	return resolveAcceptLang(r.Header.Get("Accept-Language"), supported, def)
}

// Invalidate forces a reload on the next access.
func (c *LocaleCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fetchedAt = time.Time{}
}

func (c *LocaleCache) ensureFresh(ctx context.Context) {
	c.mu.RLock()
	fresh := !c.fetchedAt.IsZero() && time.Since(c.fetchedAt) < c.ttl
	c.mu.RUnlock()
	if fresh {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	// Double-check after acquiring write lock.
	if !c.fetchedAt.IsZero() && time.Since(c.fetchedAt) < c.ttl {
		return
	}

	locales, err := GetEnabledLocales(ctx, c.db)
	if err != nil || len(locales) == 0 {
		if len(c.cached) == 0 {
			c.setCachedLocked(c.fallback)
		}
		// On error, keep existing cache. Retry after TTL expires.
		c.fetchedAt = time.Now()
		return
	}
	c.setCachedLocked(locales)
	c.fetchedAt = time.Now()
}

func (c *LocaleCache) setCachedLocked(locales []Locale) {
	c.cached = locales
	c.codes = make([]string, len(locales))
	c.defCode = ""
	for i, l := range locales {
		c.codes[i] = l.Code
		if l.IsDefault {
			c.defCode = l.Code
		}
	}
	if c.defCode == "" && len(c.codes) > 0 {
		c.defCode = c.codes[0]
	}
	if c.defCode == "" {
		c.defCode = "en"
	}
}

// resolveAcceptLang picks the best locale from an Accept-Language header.
func resolveAcceptLang(header string, supported []string, defaultCode string) string {
	if header == "" || len(supported) == 0 {
		return defaultCode
	}

	// Build a set for O(1) lookup.
	set := make(map[string]bool, len(supported))
	for _, s := range supported {
		set[s] = true
	}

	for _, tag := range parseAcceptLanguage(header) {
		if set[tag] {
			return tag
		}
		// Try primary subtag (e.g. "pt" from "pt-br").
		if idx := strings.IndexByte(tag, '-'); idx > 0 {
			if set[tag[:idx]] {
				return tag[:idx]
			}
		}
	}

	return defaultCode
}
