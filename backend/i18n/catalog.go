package i18n

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// Catalog maps language tags to key-value translation pairs.
// Language tags should be lowercase BCP-47 (e.g. "en", "de", "pt-br").
// All methods are safe for concurrent use.
type Catalog struct {
	mu       sync.RWMutex
	fallback string
	messages map[string]map[string]string
}

// NewCatalog creates a Catalog with the given fallback language.
func NewCatalog(fallback string) *Catalog {
	return &Catalog{
		fallback: strings.ToLower(fallback),
		messages: make(map[string]map[string]string),
	}
}

// Load bulk-registers translations for a language.
func (c *Catalog) Load(lang string, messages map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	tag := strings.ToLower(lang)
	if c.messages[tag] == nil {
		c.messages[tag] = make(map[string]string)
	}
	for k, v := range messages {
		c.messages[tag][k] = v
	}
}

// Set registers a single translation.
func (c *Catalog) Set(lang, key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	tag := strings.ToLower(lang)
	if c.messages[tag] == nil {
		c.messages[tag] = make(map[string]string)
	}
	c.messages[tag][key] = value
}

// Lookup returns the translation for the given language and key.
// Falls back to the catalog's fallback language if not found.
// Returns the key itself if no translation exists in any language.
// If args are provided, fmt.Sprintf is applied to the result.
func (c *Catalog) Lookup(lang, key string, args ...any) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	tag := strings.ToLower(lang)

	if msgs, ok := c.messages[tag]; ok {
		if v, ok := msgs[key]; ok {
			return applyArgs(v, args)
		}
	}

	// Try primary subtag (e.g. "pt" from "pt-br").
	if idx := strings.IndexByte(tag, '-'); idx > 0 {
		if msgs, ok := c.messages[tag[:idx]]; ok {
			if v, ok := msgs[key]; ok {
				return applyArgs(v, args)
			}
		}
	}

	// Fallback language.
	if tag != c.fallback {
		if msgs, ok := c.messages[c.fallback]; ok {
			if v, ok := msgs[key]; ok {
				return applyArgs(v, args)
			}
		}
	}

	return key
}

// Languages returns the list of loaded language tags.
func (c *Catalog) Languages() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	langs := make([]string, 0, len(c.messages))
	for k := range c.messages {
		langs = append(langs, k)
	}
	return langs
}

// Resolve picks the best language from an Accept-Language header value.
// Returns the fallback language if no loaded language matches.
func (c *Catalog) Resolve(acceptLang string) string {
	if acceptLang == "" {
		return c.fallback
	}

	type entry struct {
		tag string
		q   float64
	}

	var entries []entry
	for _, part := range strings.Split(acceptLang, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		tag := part
		q := 1.0
		if idx := strings.Index(part, ";"); idx >= 0 {
			tag = strings.TrimSpace(part[:idx])
			qstr := strings.TrimSpace(part[idx+1:])
			if strings.HasPrefix(qstr, "q=") {
				if v, err := strconv.ParseFloat(qstr[2:], 64); err == nil {
					q = v
				}
			}
		}
		entries = append(entries, entry{tag: strings.ToLower(tag), q: q})
	}

	// Sort by quality descending (stable within original order).
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j].q > entries[j-1].q; j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, e := range entries {
		if _, ok := c.messages[e.tag]; ok {
			return e.tag
		}
		// Try primary subtag.
		if idx := strings.IndexByte(e.tag, '-'); idx > 0 {
			primary := e.tag[:idx]
			if _, ok := c.messages[primary]; ok {
				return primary
			}
		}
	}

	return c.fallback
}

func applyArgs(s string, args []any) string {
	if len(args) == 0 {
		return s
	}
	return fmt.Sprintf(s, args...)
}
