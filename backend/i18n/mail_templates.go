package i18n

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"sync"
	"time"
)

// MailTemplate represents a per-locale email template.
type MailTemplate struct {
	Locale    string `json:"locale"`
	Template  string `json:"template"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// GetMailTemplate returns a mail template by locale and template name.
// Returns (nil, nil) when not found.
func GetMailTemplate(ctx context.Context, db *sql.DB, locale, template string) (*MailTemplate, error) {
	var mt MailTemplate
	err := db.QueryRowContext(ctx,
		`SELECT locale, template, subject, body, updated_at
		 FROM mail_templates WHERE locale = $1 AND template = $2`,
		locale, template).
		Scan(&mt.Locale, &mt.Template, &mt.Subject, &mt.Body, &mt.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &mt, nil
}

// UpsertMailTemplate creates or updates a mail template.
func UpsertMailTemplate(ctx context.Context, db *sql.DB, locale, template, subject, body string) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO mail_templates (locale, template, subject, body)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT(locale, template) DO UPDATE SET
		   subject = excluded.subject,
		   body = excluded.body,
		   updated_at = NOW()`,
		locale, template, subject, body)
	return err
}

// ListMailTemplates returns all locales for a template name.
func ListMailTemplates(ctx context.Context, db *sql.DB, template string) ([]MailTemplate, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT locale, template, subject, body, updated_at
		 FROM mail_templates WHERE template = $1 ORDER BY locale`, template)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var templates []MailTemplate
	for rows.Next() {
		var mt MailTemplate
		if err := rows.Scan(&mt.Locale, &mt.Template, &mt.Subject, &mt.Body, &mt.UpdatedAt); err != nil {
			return nil, err
		}
		templates = append(templates, mt)
	}
	return templates, rows.Err()
}

// cachedTemplate holds a subject/body pair in the resolver cache.
type cachedTemplate struct {
	subject string
	body    string
}

// MailTemplateResolver loads templates from DB with fallback to the Catalog.
// It auto-refreshes from the DB on a TTL interval.
type MailTemplateResolver struct {
	db           *sql.DB
	catalog      *Catalog
	fallbackLang string
	mu           sync.RWMutex
	cache        map[string]map[string]cachedTemplate // locale → template → {subject, body}
	ttl          time.Duration
	logger       *log.Logger
	stopOnce     sync.Once
	done         chan struct{}
}

// NewMailTemplateResolver creates a resolver with auto-refresh.
// Pass nil for logger to use the default logger.
func NewMailTemplateResolver(db *sql.DB, catalog *Catalog, fallbackLang string, ttl time.Duration, logger *log.Logger) *MailTemplateResolver {
	if ttl == 0 {
		ttl = 30 * time.Second
	}
	if logger == nil {
		logger = log.Default()
	}
	r := &MailTemplateResolver{
		db:           db,
		catalog:      catalog,
		fallbackLang: fallbackLang,
		cache:        make(map[string]map[string]cachedTemplate),
		ttl:          ttl,
		logger:       logger,
		done:         make(chan struct{}),
	}

	// Initial load.
	if err := r.LoadFromDB(context.Background()); err != nil {
		logger.Printf("i18n: initial mail template load: %v", err)
	}

	// Start auto-refresh goroutine.
	go r.refreshLoop()

	return r
}

// Resolve returns subject + body for a template in the given locale.
// Fallback chain: cache(locale) → cache(fallbackLang) → catalog(locale) → catalog(fallbackLang).
func (r *MailTemplateResolver) Resolve(ctx context.Context, locale, template string) (subject, body string, err error) {
	if ct, ok := r.lookupCache(locale, template); ok {
		return ct.subject, ct.body, nil
	}

	// Fall through to Catalog.
	if r.catalog != nil {
		subjectKey := template + ".subject"
		bodyKey := template + ".body"
		s := r.catalog.Lookup(locale, subjectKey)
		b := r.catalog.Lookup(locale, bodyKey)
		// Catalog returns the key itself when missing — detect that.
		if s != subjectKey || b != bodyKey {
			return s, b, nil
		}
	}

	return "", "", nil
}

// lookupCache checks the in-memory cache for a template, trying the requested
// locale first, then the fallback locale.
func (r *MailTemplateResolver) lookupCache(locale, template string) (cachedTemplate, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if templates, ok := r.cache[locale]; ok {
		if ct, ok := templates[template]; ok {
			return ct, true
		}
	}
	if locale != r.fallbackLang {
		if templates, ok := r.cache[r.fallbackLang]; ok {
			if ct, ok := templates[template]; ok {
				return ct, true
			}
		}
	}
	return cachedTemplate{}, false
}

// SetTemplate updates the in-memory cache for a template.
func (r *MailTemplateResolver) SetTemplate(locale, template, subject, body string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cache[locale] == nil {
		r.cache[locale] = make(map[string]cachedTemplate)
	}
	r.cache[locale][template] = cachedTemplate{subject: subject, body: body}
}

// LoadFromDB bulk-loads all templates into memory.
func (r *MailTemplateResolver) LoadFromDB(ctx context.Context) error {
	rows, err := r.db.QueryContext(ctx,
		`SELECT locale, template, subject, body FROM mail_templates`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	cache := make(map[string]map[string]cachedTemplate)
	for rows.Next() {
		var locale, template, subject, body string
		if err := rows.Scan(&locale, &template, &subject, &body); err != nil {
			return err
		}
		if cache[locale] == nil {
			cache[locale] = make(map[string]cachedTemplate)
		}
		cache[locale][template] = cachedTemplate{subject: subject, body: body}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	r.cache = cache
	r.mu.Unlock()
	return nil
}

// Stop stops the auto-refresh goroutine. Safe to call multiple times.
func (r *MailTemplateResolver) Stop() {
	r.stopOnce.Do(func() {
		close(r.done)
	})
}

func (r *MailTemplateResolver) refreshLoop() {
	ticker := time.NewTicker(r.ttl)
	defer ticker.Stop()
	for {
		select {
		case <-r.done:
			return
		case <-ticker.C:
			if err := r.LoadFromDB(context.Background()); err != nil {
				r.logger.Printf("i18n: mail template refresh: %v", err)
			}
		}
	}
}
