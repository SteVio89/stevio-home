package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/SteVio89/stevio-home/auth"
	"github.com/SteVio89/stevio-home/db/postgres"
	"github.com/SteVio89/stevio-home/i18n"
	"github.com/SteVio89/stevio-home/mailer"
	"github.com/SteVio89/stevio-home/middleware"
	"github.com/SteVio89/stevio-home/migrate"
	"github.com/SteVio89/stevio-home/settings"
	"github.com/SteVio89/stevio-home/users"
)

// SMTPConfig holds SMTP connection parameters.
// Re-exported from the internal mailer package for public use.
type SMTPConfig = mailer.SMTPConfig

// Catalog is the i18n translation catalog type.
// Re-exported from the i18n package for public use.
type Catalog = i18n.Catalog

// Locale is a language entry in the locale registry.
// Re-exported from the i18n package for public use.
type Locale = i18n.Locale

// NewCatalog creates a new translation catalog with the given fallback language.
func NewCatalog(fallback string) *Catalog {
	return i18n.NewCatalog(fallback)
}

// Config is the single initialization struct for a stevio application.
type Config struct {
	// DatabaseURL is the Postgres connection string, e.g.
	// "postgres://user:pass@host:5432/dbname?sslmode=disable".
	// Required.
	DatabaseURL string

	// DBPool optionally overrides Postgres pool tuning. Zero values use the
	// defaults defined in the postgres package (25 max conns, 5 idle, etc.).
	DBPool postgres.Config

	// DefaultMigrations is an optional embedded FS for the app's migrations.
	// Files must be at "migrations/*.sql". Run after framework migrations
	// with prefix "app". Equivalent to adding MigrationSource{Prefix: "app", Files: ...}
	// as the first entry in AppMigrations.
	DefaultMigrations fs.ReadFileFS

	// db is the pre-built DB wrapper injected by New() or test helpers.
	db *DB

	// Secret is the HMAC key for session signing. Must be at least 32 bytes. Required.
	Secret []byte

	// EmailSalt is used for HMAC-SHA256 email hashing.
	// If empty, Secret is used as the salt.
	EmailSalt string

	// BaseURL is the canonical base URL of the application (e.g. "https://myapp.com").
	// Used to construct magic-link URLs. Required when STEVIO_ENV=production.
	// If empty in dev mode, the framework derives the URL from the request Host header.
	BaseURL string

	// CookieName overrides the session cookie name. Default: "sid".
	CookieName string

	// InsecureCookies disables the Secure flag on session cookies.
	// Default: false (cookies are secure). Set to true only for local dev over HTTP.
	InsecureCookies bool

	// SessionDuration controls session lifetime. Default: 30 days.
	// Used as the fallback when SessionDurationByRole has no entry for a role.
	SessionDuration time.Duration

	// SessionDurationByRole overrides SessionDuration for specific roles.
	// Example: map[string]time.Duration{"admin": 8 * time.Hour}
	// Roles not in this map fall back to SessionDuration.
	SessionDurationByRole map[string]time.Duration

	// TokenDuration controls magic-link token lifetime. Default: 15 minutes.
	TokenDuration time.Duration

	// DefaultRole assigned to new users on first login. Default: "member".
	DefaultRole string

	// SMTP configures outbound email. Required for magic-link auth.
	SMTP SMTPConfig

	// Translations is the i18n catalog. Keys used by the framework:
	//   auth.login_subject — email subject for magic-link
	//   auth.login_body    — email body, %s is replaced with the magic link URL
	// If nil, built-in English strings are used.
	Translations *Catalog

	// FallbackLang is used when Accept-Language has no match. Default: "en".
	FallbackLang string

	// AllowedOrigin configures CORS. Empty disables CORS headers.
	AllowedOrigin string

	// RateLimit sets global requests-per-minute per IP. Default: 120.
	RateLimit int

	// TrustedProxy enables reading X-Real-IP for rate limiting.
	TrustedProxy bool

	// Logger for internal errors and panic recovery. Default: log.Default().
	Logger *log.Logger

	// SettingsTable is the SQL table for runtime settings. Default: "app_settings".
	SettingsTable string

	// I18n configures the DB-driven translation engine.
	// If nil, only the static Catalog is used (backward-compatible).
	I18n *I18nConfig

	// Mailer overrides the built-in SMTP mailer. Useful for testing.
	// If non-nil, SMTP config is ignored.
	Mailer Mailer

	// AppMigrations are additional migration sources to run after framework migrations.
	AppMigrations []MigrationSource
}

// I18nConfig configures the DB-driven translation engine.
// If nil in Config, only the static Catalog is used (backward-compatible).
type I18nConfig struct {
	// CacheTTL for the locale registry. Default: 30s.
	CacheTTL time.Duration

	// FallbackLocales used when DB is empty or erroring.
	// Default: [{Code: "en", Name: "English", IsDefault: true, Enabled: true}]
	FallbackLocales []i18n.Locale
}

// Mailer allows swapping the email sender for testing.
type Mailer interface {
	Send(to, subject, body string) error
}

// MigrationSource pairs a migration FS with a namespace prefix.
type MigrationSource struct {
	Prefix string
	Files  fs.ReadFileFS
}

// App is the central framework object. Create with New(Config{}).
type App struct {
	cfg                   Config
	db                    *DB
	mux                   *http.ServeMux
	logger                *log.Logger
	catalog               *i18n.Catalog
	locales               *i18n.LocaleCache
	mailTpl               *i18n.MailTemplateResolver
	mailer                Mailer
	users                 *users.Store
	sets                  *settings.Store
	rl                    *middleware.RateLimiter
	extraRLs              []*middleware.RateLimiter
	secret                []byte
	salt                  string
	baseURL               string
	cookieName            string
	cookieSecure          bool
	defaultRole           string
	sessionDuration       time.Duration
	sessionDurationByRole map[string]time.Duration
	tokenDuration         time.Duration
	globalRate            middleware.Middleware
	globalMW              []middleware.Middleware
}

// New initializes the App, opens the Postgres pool, runs migrations, and
// registers built-in auth routes. Panics if required config (DatabaseURL,
// Secret) is missing, the DB is unreachable, or migrations fail.
//
// App.Close() must be called on shutdown to close the pool and stop rate limiters.
func New(cfg Config) *App {
	if cfg.DatabaseURL == "" {
		panic("stevio: Config.DatabaseURL is required")
	}
	if len(cfg.Secret) < 32 {
		panic("stevio: Config.Secret must be at least 32 bytes")
	}

	pool := cfg.DBPool
	pool.DSN = cfg.DatabaseURL
	sqlDB, err := postgres.Connect(context.Background(), pool)
	if err != nil {
		panic("app: open database: " + err.Error())
	}

	cfg.db = newDB(sqlDB)

	// Run framework migrations first.
	if err := migrate.RunMigrations(sqlDB, "auth", auth.MigrationFiles, cfg.Logger); err != nil {
		panic("app: auth migrations: " + err.Error())
	}
	if err := migrate.RunMigrations(sqlDB, "users", users.MigrationFiles, cfg.Logger); err != nil {
		panic("app: user migrations: " + err.Error())
	}

	// Run default app migrations.
	if cfg.DefaultMigrations != nil {
		if err := migrate.RunMigrations(sqlDB, "app", cfg.DefaultMigrations, cfg.Logger); err != nil {
			panic("app: default migrations: " + err.Error())
		}
	}

	// Run additional app migrations.
	for _, src := range cfg.AppMigrations {
		if err := migrate.RunMigrations(sqlDB, src.Prefix, src.Files, cfg.Logger); err != nil {
			panic("app: migration " + src.Prefix + ": " + err.Error())
		}
	}

	return newApp(cfg)
}

// newApp builds the App with all wiring. Shared by New() and test helpers.
func newApp(cfg Config) *App {
	// Apply defaults.
	if cfg.CookieName == "" {
		cfg.CookieName = "sid"
	}
	cookieSecure := !cfg.InsecureCookies
	if cfg.SessionDuration == 0 {
		cfg.SessionDuration = 30 * 24 * time.Hour
	}
	if cfg.TokenDuration == 0 {
		cfg.TokenDuration = 15 * time.Minute
	}
	if cfg.DefaultRole == "" {
		cfg.DefaultRole = "member"
	}
	if cfg.FallbackLang == "" {
		cfg.FallbackLang = "en"
	}
	if cfg.RateLimit == 0 {
		cfg.RateLimit = 120
	}
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}
	if cfg.SettingsTable == "" {
		cfg.SettingsTable = "app_settings"
	}

	// Validate CORS origin if set.
	if cfg.AllowedOrigin != "" && cfg.AllowedOrigin != "*" {
		if !strings.HasPrefix(cfg.AllowedOrigin, "https://") && !strings.HasPrefix(cfg.AllowedOrigin, "http://") {
			panic("stevio: Config.AllowedOrigin must start with https:// or http://")
		}
		if strings.HasSuffix(cfg.AllowedOrigin, "/") {
			panic("stevio: Config.AllowedOrigin must not have a trailing slash")
		}
	}

	if cfg.BaseURL == "" {
		if os.Getenv("STEVIO_ENV") == "production" {
			panic("stevio: Config.BaseURL is required when STEVIO_ENV=production (prevents Host header injection in magic-link emails)")
		}
		// If a real mailer is configured, BaseURL is required to prevent
		// Host header injection in magic-link emails sent to real addresses.
		if cfg.Mailer != nil || cfg.SMTP.Host != "" {
			panic("stevio: Config.BaseURL is required when a Mailer or SMTP is configured (prevents Host header injection)")
		}
		cfg.Logger.Printf("stevio: WARNING: Config.BaseURL is empty — magic-link URLs will use the request Host header (spoofable)")
	}

	// Derive EmailSalt from Secret using HMAC with a domain-specific label.
	// This avoids reusing the raw Secret for a different cryptographic purpose.
	// If you need backward compat with an older deployment that used Secret
	// directly, set EmailSalt = string(Secret) explicitly in your Config.
	salt := cfg.EmailSalt
	if salt == "" {
		cfg.Logger.Printf("stevio: WARNING: Config.EmailSalt is empty — deriving from Secret; set it explicitly for stable email hashes")
		mac := hmac.New(sha256.New, cfg.Secret)
		mac.Write([]byte("stevio:email-salt"))
		salt = hex.EncodeToString(mac.Sum(nil))
	}

	catalog := cfg.Translations
	if catalog == nil {
		catalog = defaultCatalog(cfg.FallbackLang)
	}

	// Build mailer.
	var m Mailer
	if cfg.Mailer != nil {
		m = cfg.Mailer
	} else if cfg.SMTP.Host != "" {
		m = mailer.New(cfg.SMTP)
	}

	// Build rate limiter.
	rl := middleware.NewRateLimiter(cfg.RateLimit, cfg.TrustedProxy)

	// Build global middleware chain.
	globalMW := []middleware.Middleware{
		middleware.SecurityHeaders,
		middleware.CORS(cfg.AllowedOrigin),
		middleware.Recover(cfg.Logger),
	}

	// Build settings store.
	sets, err := settings.NewStore(cfg.db.DB, cfg.SettingsTable)
	if err != nil {
		panic("stevio: " + err.Error())
	}

	// Build i18n engine if configured.
	var locales *i18n.LocaleCache
	var mailTpl *i18n.MailTemplateResolver
	if cfg.I18n != nil {
		ttl := cfg.I18n.CacheTTL
		if ttl == 0 {
			ttl = 30 * time.Second
		}
		var cacheOpts []i18n.LocaleCacheOption
		cacheOpts = append(cacheOpts, i18n.WithTTL(ttl))
		if len(cfg.I18n.FallbackLocales) > 0 {
			cacheOpts = append(cacheOpts, i18n.WithFallbackLocales(cfg.I18n.FallbackLocales))
		}
		locales = i18n.NewLocaleCache(cfg.db.DB, cacheOpts...)
		mailTpl = i18n.NewMailTemplateResolver(cfg.db.DB, catalog, cfg.FallbackLang, ttl, cfg.Logger)
	}

	a := &App{
		cfg:                   cfg,
		db:                    cfg.db,
		mux:                   http.NewServeMux(),
		logger:                cfg.Logger,
		catalog:               catalog,
		locales:               locales,
		mailTpl:               mailTpl,
		mailer:                m,
		users:                 users.New(cfg.db.DB),
		sets:                  sets,
		rl:                    rl,
		secret:                cfg.Secret,
		salt:                  salt,
		baseURL:               cfg.BaseURL,
		cookieName:            cfg.CookieName,
		cookieSecure:          cookieSecure,
		defaultRole:           cfg.DefaultRole,
		sessionDuration:       cfg.SessionDuration,
		sessionDurationByRole: cfg.SessionDurationByRole,
		tokenDuration:         cfg.TokenDuration,
		globalRate:            middleware.RateLimit(rl),
		globalMW:              globalMW,
	}

	// Register built-in auth routes with stricter rate limits.
	a.Public().Handle("POST /auth/login", a.loginHandler,
		WithRateLimit(a, 10, cfg.TrustedProxy))
	a.Public().Handle("POST /auth/verify", a.verifyHandler,
		WithRateLimit(a, 10, cfg.TrustedProxy))
	a.Authenticated().Handle("POST /auth/logout", a.logoutHandler)

	return a
}

// Public returns an AuthScope for routes that require no authentication.
func (a *App) Public() *AuthScope {
	return &AuthScope{app: a, authKind: "public"}
}

// Authenticated returns an AuthScope for routes that require a valid session.
func (a *App) Authenticated() *AuthScope {
	return &AuthScope{app: a, authKind: "authenticated"}
}

// Role returns an AuthScope for routes that require a specific role.
func (a *App) Role(role string) *AuthScope {
	return &AuthScope{app: a, authKind: "role", role: role}
}

// Group returns a prefix-scoped router group.
func (a *App) Group(prefix string) *RouterGroup {
	return &RouterGroup{app: a, prefix: prefix}
}

// Handler returns the http.Handler with the global middleware chain applied.
// Use this with http.ListenAndServe or httptest.
func (a *App) Handler() http.Handler {
	return middleware.Chain(a.mux, a.globalMW...)
}

// Server returns an *http.Server with the app's handler and sensible defaults.
// Callers can override timeouts before calling srv.ListenAndServe().
func (a *App) Server(addr string) *http.Server {
	return &http.Server{
		Addr:         addr,
		Handler:      a.Handler(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
}

// ListenAndServe starts the HTTP server with default timeouts. Blocks until exit.
// For custom timeouts, use app.Server(addr) and call srv.ListenAndServe() directly.
func (a *App) ListenAndServe(addr string) error {
	return a.Server(addr).ListenAndServe()
}

// Settings returns the runtime settings store.
func (a *App) Settings() *settings.Store {
	return a.sets
}

// DB returns the framework's database wrapper.
// Use DB().DB to access the underlying *sql.DB.
func (a *App) DB() *DB {
	return a.db
}

// Locales returns the locale registry cache, or nil if Config.I18n was not set.
func (a *App) Locales() *i18n.LocaleCache {
	return a.locales
}

// MailTemplates returns the mail template resolver, or nil if Config.I18n was not set.
func (a *App) MailTemplates() *i18n.MailTemplateResolver {
	return a.mailTpl
}

// sessionDurationForRole returns the session duration for the given role.
// Falls back to the global SessionDuration if no per-role override exists.
func (a *App) sessionDurationForRole(role string) time.Duration {
	if d, ok := a.sessionDurationByRole[role]; ok {
		return d
	}
	return a.sessionDuration
}

// UpdateUserRole changes a user's role and invalidates all their existing
// sessions. This forces a re-login so the new role takes effect immediately,
// preventing stale elevated-privilege sessions from lingering.
func (a *App) UpdateUserRole(ctx context.Context, userID, role string) error {
	emailHash, err := a.users.UpdateRole(ctx, userID, role)
	if err != nil {
		return err
	}
	return auth.DeleteSessionsByEmailHash(ctx, a.db.DB, emailHash)
}

// Close stops rate limiters and closes the database pool.
func (a *App) Close() error {
	if a.mailTpl != nil {
		a.mailTpl.Stop()
	}
	a.rl.Stop()
	for _, rl := range a.extraRLs {
		rl.Stop()
	}
	if a.db != nil {
		return a.db.Close()
	}
	return nil
}

// defaultCatalog builds the built-in English catalog.
func defaultCatalog(fallback string) *i18n.Catalog {
	c := i18n.NewCatalog(fallback)
	c.Load("en", map[string]string{
		KeyLoginSubject: "Your login link",
		KeyLoginBody:    "Click the link below to log in:\n\n%s\n\nThis link expires in 15 minutes.",
	})
	return c
}
