package commands

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"

	"github.com/SteVio89/stevio-home/auth"
	"github.com/SteVio89/stevio-home/config"
	"github.com/SteVio89/stevio-home/db"
	"github.com/SteVio89/stevio-home/db/postgres"
	"github.com/SteVio89/stevio-home/db/queries"
	"github.com/SteVio89/stevio-home/i18n"
	"github.com/SteVio89/stevio-home/migrate"
	"github.com/spf13/cobra"
)

// openDB creates a minimal DB connection using only DATABASE_URL from the
// environment. Does not require all config vars.
func openDB(ctx context.Context) (*sql.DB, error) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return nil, fmt.Errorf("DATABASE_URL env var is not set")
	}
	db, err := postgres.Connect(ctx, postgres.Config{DSN: url})
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	return db, nil
}

func CheckConfigCmd() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "check-config",
		Short: "Validate all required environment variables",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			if jsonOutput {
				return printJSON(map[string]any{
					"valid":        true,
					"env":          cfg.Env,
					"port":         cfg.Port,
					"base_url":     cfg.BaseURL,
					"smtp":         cfg.SMTPHost != "",
					"admin_emails": len(cfg.AdminEmails),
				})
			}

			fmt.Println("Config OK")
			fmt.Println()
			fmt.Printf("  %-22s %s\n", "DATABASE_URL:", mask(cfg.DatabaseURL))
			fmt.Printf("  %-22s %s\n", "SIGNING_KEY_SECRET:", "set")
			fmt.Printf("  %-22s %s\n", "SESSION_SECRET:", "set")
			fmt.Printf("  %-22s %s\n", "EMAIL_HASH_SALT:", "set")
			fmt.Printf("  %-22s %s\n", "ENV:", cfg.Env)
			fmt.Printf("  %-22s %s\n", "PORT:", cfg.Port)
			fmt.Printf("  %-22s %s\n", "BASE_URL:", cfg.BaseURL)
			fmt.Printf("  %-22s %s\n", "SMTP:", setOrNot(cfg.SMTPHost))
			fmt.Printf("  %-22s %s\n", "ADMIN_EMAILS:", countLabel(len(cfg.AdminEmails), "address"))
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func MigrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations",
		Long:  "Runs auth, i18n, and app migrations. User table migrations are framework-internal and run on server start.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			sqlDB, err := openDB(ctx)
			if err != nil {
				return err
			}
			defer func() { _ = sqlDB.Close() }()

			logger := log.New(os.Stdout, "[migrate] ", log.LstdFlags)

			type migrationSource struct {
				prefix string
				files  fs.ReadFileFS
			}
			sources := []migrationSource{
				{"auth", auth.MigrationFiles},
				{"i18n", i18n.MigrationFiles},
				{"app", db.MigrationFiles},
			}

			for _, src := range sources {
				if err := migrate.RunMigrations(sqlDB, src.prefix, src.files, logger); err != nil {
					return fmt.Errorf("%s migrations: %w", src.prefix, err)
				}
			}

			fmt.Println("Migrations complete.")
			return nil
		},
	}
}

func DBStatsCmd() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "db-stats",
		Short: "Show database entity counts and revenue",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			sqlDB, err := openDB(ctx)
			if err != nil {
				return err
			}
			defer func() { _ = sqlDB.Close() }()

			stats, err := queries.GetAdminStats(ctx, sqlDB)
			if err != nil {
				return fmt.Errorf("get stats: %w", err)
			}

			var appCount, userCount int
			_ = sqlDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM apps WHERE deleted_at IS NULL`).Scan(&appCount)
			_ = sqlDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&userCount)

			if jsonOutput {
				return printJSON(map[string]any{
					"apps":                appCount,
					"users":               userCount,
					"orders":              stats.TotalOrders,
					"licenses":            stats.TotalLicenses,
					"activations":         stats.TotalActivations,
					"total_revenue_cents": stats.TotalRevenueCents,
					"revenue_30d_cents":   stats.Revenue30dCents,
				})
			}

			fmt.Println("Database Stats")
			fmt.Println()
			fmt.Printf("  %-22s %d\n", "Apps:", appCount)
			fmt.Printf("  %-22s %d\n", "Users:", userCount)
			fmt.Printf("  %-22s %d\n", "Orders:", stats.TotalOrders)
			fmt.Printf("  %-22s %d\n", "Licenses:", stats.TotalLicenses)
			fmt.Printf("  %-22s %d\n", "Activations:", stats.TotalActivations)
			fmt.Println()
			fmt.Printf("  %-22s %s\n", "Revenue (all time):", formatCents(stats.TotalRevenueCents))
			fmt.Printf("  %-22s %s\n", "Revenue (30d):", formatCents(stats.Revenue30dCents))
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func CleanupCmd() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Remove expired auth tokens and sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			sqlDB, err := openDB(ctx)
			if err != nil {
				return err
			}
			defer func() { _ = sqlDB.Close() }()

			tokens, err := auth.DeleteExpiredAuthTokens(ctx, sqlDB)
			if err != nil {
				return fmt.Errorf("cleanup tokens: %w", err)
			}

			sessions, err := auth.DeleteExpiredSessions(ctx, sqlDB)
			if err != nil {
				return fmt.Errorf("cleanup sessions: %w", err)
			}

			if jsonOutput {
				return printJSON(map[string]any{
					"tokens_removed":   tokens,
					"sessions_removed": sessions,
				})
			}

			fmt.Printf("Removed %d expired/used auth token(s)\n", tokens)
			fmt.Printf("Removed %d expired session(s)\n", sessions)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func SigningKeysCmd() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "signing-keys",
		Short: "Show the active signing key",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			sqlDB, err := openDB(ctx)
			if err != nil {
				return err
			}
			defer func() { _ = sqlDB.Close() }()

			key, err := queries.GetActiveSigningKey(ctx, sqlDB)
			if err != nil {
				return fmt.Errorf("get active signing key: %w", err)
			}

			if key == nil {
				if jsonOutput {
					return printJSON(map[string]any{"key": nil})
				}
				fmt.Println("No active signing key.")
				fmt.Println("Generate one via the admin UI.")
				return nil
			}

			if jsonOutput {
				return printJSON(map[string]any{"key": map[string]any{
					"id":         key.ID,
					"key_id":     key.KeyID,
					"active":     key.Active,
					"created_at": key.CreatedAt.Format("2006-01-02 15:04:05"),
				}})
			}

			fmt.Printf("Active Signing Key\n\n")
			fmt.Printf("  ID:         %s\n", key.ID)
			fmt.Printf("  Key ID:     %s\n", key.KeyID)
			fmt.Printf("  Created:    %s\n", key.CreatedAt.Format("2006-01-02"))
			fmt.Printf("  Public Key: %s\n", key.PublicKeyB64)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func mask(s string) string {
	if len(s) <= 8 {
		return "***"
	}
	return s[:8] + "***"
}

func setOrNot(s string) string {
	if s != "" {
		return "configured"
	}
	return "not configured"
}

func countLabel(n int, singular string) string {
	if n == 0 {
		return "none"
	}
	if n == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %ses", n, singular)
}

func formatCents(cents int) string {
	return fmt.Sprintf("%d.%02d", cents/100, cents%100)
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
