package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	appsvc "github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/auth"
	"github.com/SteVio89/stevio-home/config"
	"github.com/SteVio89/stevio-home/crypto"
	"github.com/SteVio89/stevio-home/db"
	"github.com/SteVio89/stevio-home/dbutil"
	"github.com/SteVio89/stevio-home/handlers"
	"github.com/SteVio89/stevio-home/i18n"
	"github.com/SteVio89/stevio-home/mailer"
	"github.com/SteVio89/stevio-home/middleware"
	"github.com/SteVio89/stevio-home/payment"
	"github.com/SteVio89/stevio-home/payment/mock"
	"github.com/SteVio89/stevio-home/payment/paddle"
)

func main() {
	logger := log.New(os.Stdout, "[store] ", log.LstdFlags|log.Lshortfile)

	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("config: %v", err)
	}

	// The framework reads STEVIO_ENV; the store historically reads ENV.
	if os.Getenv("STEVIO_ENV") == "" && cfg.Env != "" {
		_ = os.Setenv("STEVIO_ENV", cfg.Env)
	}

	// Build the i18n catalog for auth emails.
	catalog := appsvc.NewCatalog("de")
	catalog.Load("de", map[string]string{
		appsvc.KeyLoginSubject: "Dein Login-Link",
		appsvc.KeyLoginBody:    "Klicke den folgenden Link, um dich einzuloggen.\n\nDieser Link läuft in 15 Minuten ab und kann nur einmal verwendet werden.\n\n%s\n\nFalls du dies nicht angefordert hast, ignoriere diese E-Mail.",
	})
	catalog.Load("en", map[string]string{
		appsvc.KeyLoginSubject: "Your login link",
		appsvc.KeyLoginBody:    "Click the link below to log in to your account.\n\nThis link expires in 15 minutes and can only be used once.\n\n%s\n\nIf you didn't request this, ignore this email.",
	})

	mail := mailer.New(mailer.SMTPConfig{
		Host:     cfg.SMTPHost,
		Port:     cfg.SMTPPort,
		Username: cfg.SMTPUser,
		Password: cfg.SMTPPass,
		From:     cfg.SMTPFrom,
		Logger:   logger,
	})

	// Determine CORS origin — only in development.
	allowedOrigin := ""
	if cfg.Env == "development" {
		allowedOrigin = cfg.CORSOrigin
	}

	app := appsvc.New(appsvc.Config{
		DatabaseURL: cfg.DatabaseURL,
		Secret:      cfg.SessionSecretBytes,
		EmailSalt:   cfg.EmailHashSalt,
		BaseURL:     cfg.BaseURL,
		AppMigrations: []appsvc.MigrationSource{
			{Prefix: "i18n", Files: i18n.MigrationFiles},
			{Prefix: "app", Files: db.MigrationFiles},
		},
		SMTP: appsvc.SMTPConfig{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			Username: cfg.SMTPUser,
			Password: cfg.SMTPPass,
			From:     cfg.SMTPFrom,
		},
		Mailer:          mail,
		Translations:    catalog,
		FallbackLang:    "de",
		AllowedOrigin:   allowedOrigin,
		TrustedProxy:    cfg.Env == "production",
		Logger:          logger,
		SettingsTable:   "site_settings",
		DefaultRole:     "member",
		InsecureCookies: cfg.Env != "production",
		// Admins hold destructive powers (uploads, license issue/revoke, settings),
		// so give admin sessions a much shorter lifetime than the 30-day member
		// default to bound the exposure window of a stolen admin cookie. Adjust to
		// taste — a longer value trades security for fewer magic-link re-logins.
		SessionDurationByRole: map[string]time.Duration{
			"admin": 24 * time.Hour,
		},
		I18n: &appsvc.I18nConfig{
			FallbackLocales: []i18n.Locale{
				{Code: "de", Name: "Deutsch", IsDefault: true, Enabled: true, SortOrder: 0},
				{Code: "en", Name: "English", IsDefault: false, Enabled: true, SortOrder: 1},
			},
		},
	})
	defer func() { _ = app.Close() }()

	// Seed admin users from ADMIN_EMAILS env var. ADMIN_EMAILS is the single
	// source of truth for admin access: listed addresses are promoted here, and
	// any admin no longer listed is demoted below. Revocation takes effect on the
	// next deploy/restart, which matches this single-operator deployment model.
	seedCtx := context.Background()
	allowedAdmin := make(map[string]bool, len(cfg.AdminEmailHashes))
	for _, hash := range cfg.AdminEmailHashes {
		allowedAdmin[hash] = true
		_, err := app.DB().ExecContext(seedCtx,
			`INSERT INTO users (id, email_hash, role) VALUES ($1, $2, 'admin')
			 ON CONFLICT (email_hash) DO UPDATE SET role = 'admin'`,
			dbutil.NewID(), hash)
		if err != nil {
			logger.Printf("seed admin: %v", err)
		}
	}

	// Demote any admin whose email is no longer in ADMIN_EMAILS and revoke their
	// live sessions, so removing an address from the env var + redeploying is a
	// real revocation rather than a no-op. Without this, a demoted admin keeps
	// their session-cached admin role and can re-mint a fresh admin session.
	demoteRows, err := app.DB().QueryContext(seedCtx,
		`SELECT id, email_hash FROM users WHERE role = 'admin'`)
	if err != nil {
		logger.Printf("demote scan: %v", err)
	} else {
		var toDemote []string
		for demoteRows.Next() {
			var id, hash string
			if err := demoteRows.Scan(&id, &hash); err != nil {
				logger.Printf("demote scan row: %v", err)
				continue
			}
			if !allowedAdmin[hash] {
				toDemote = append(toDemote, id)
			}
		}
		_ = demoteRows.Close()
		for _, id := range toDemote {
			// UpdateUserRole sets role='member' and deletes the user's sessions.
			if err := app.UpdateUserRole(seedCtx, id, "member"); err != nil {
				logger.Printf("demote admin %s: %v", id, err)
			} else {
				logger.Printf("demoted admin no longer in ADMIN_EMAILS (user %s)", id)
			}
		}
	}

	// Cleanup goroutine for expired tokens and sessions.
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	go runCleanup(cleanupCtx, app, logger)

	signer := crypto.NewDBSigner(app.DB().DB, cfg.SigningKeySecret)

	// Build payment provider registry. Credentials are fetched per-request from
	// site_settings by the checkout handler, so the startup registry only needs
	// credential-less placeholders — its role is to declare which provider
	// names are valid for the admin `payment_provider` setting.
	// The mock provider mints fully-signed licenses without payment. It stays
	// available in every environment (a single-prod deployment uses it to rehearse
	// the checkout flow before going live), and it is kept safe by an invariant:
	// selecting it forces the store into maintenance mode, so while
	// payment_provider=="mock" the public store — and the maintenance-blocked mock
	// trigger route — are walled off from everyone except admins. See
	// middleware.MaintenanceChecker.isEnabled for where that coupling is enforced.
	payments := payment.Registry{
		"paddle": paddle.New("", "", ""),
		"mock":   mock.New(cfg.BaseURL, cfg.SigningKeySecret),
	}

	h := handlers.New(app, cfg, signer, logger, payments, mail)

	// Maintenance middleware for public routes.
	mc, err := middleware.NewMaintenanceChecker(app.DB().DB, cfg.AdminEmailHashes, cfg.SessionSecretBytes)
	if err != nil {
		logger.Fatalf("maintenance checker: %v", err)
	}
	handlers.RegisterRoutes(app, h, mc.Middleware())

	srv := app.Server(":" + cfg.Port)
	srv.WriteTimeout = 300 * time.Second // large binary downloads

	go func() {
		logger.Printf("listening on :%s (env=%s)", cfg.Port, cfg.Env)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Println("shutting down...")
	cleanupCancel()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Printf("shutdown error: %v", err)
	}
	logger.Println("stopped")
}

func runCleanup(ctx context.Context, app *appsvc.App, logger *log.Logger) {
	db := app.DB().DB

	do := func() {
		tokens, err := auth.DeleteExpiredAuthTokens(ctx, db)
		if err != nil {
			logger.Printf("cleanup: auth tokens: %v", err)
		} else if tokens > 0 {
			logger.Printf("cleanup: removed %d expired/used auth token(s)", tokens)
		}

		sessions, err := auth.DeleteExpiredSessions(ctx, db)
		if err != nil {
			logger.Printf("cleanup: sessions: %v", err)
		} else if sessions > 0 {
			logger.Printf("cleanup: removed %d expired session(s)", sessions)
		}
	}

	do()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			do()
		case <-ctx.Done():
			return
		}
	}
}
