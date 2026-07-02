// Package postgres provides the Postgres connection layer for the application.
//
// Uses pgx/v5 via the stdlib adapter so the rest of the codebase keeps using
// *sql.DB and database/sql. Swapping to pgxpool.Pool directly for native
// Postgres types is a future optimization — not needed for the migration.
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Config holds pool tuning knobs. Zero values use defaults that are sensible
// for a small (2 vCPU / 4 GB) VPS running Postgres + app on the same host.
type Config struct {
	// DSN is the Postgres connection string, e.g.
	// "postgres://user:pass@host:5432/dbname?sslmode=disable".
	// Required.
	DSN string

	// MaxOpenConns is the maximum number of open connections to Postgres.
	// Default: 25. On a shared-host deployment, tune down; on a larger DB
	// instance, tune up.
	MaxOpenConns int

	// MaxIdleConns is the maximum number of idle connections kept in the pool.
	// Default: 5.
	MaxIdleConns int

	// ConnMaxIdleTime is how long an idle connection can live before being
	// recycled. Default: 5 minutes. Short values free resources; long values
	// reduce reconnect overhead.
	ConnMaxIdleTime time.Duration

	// ConnMaxLifetime is the maximum total lifetime of a connection, regardless
	// of activity. Default: 1 hour. Useful behind a connection-recycling proxy
	// (PgBouncer) or when the server might rotate credentials.
	ConnMaxLifetime time.Duration

	// PingTimeout is how long Connect waits for the initial Ping before
	// returning an error. Default: 5 seconds.
	PingTimeout time.Duration
}

// Connect opens a Postgres connection pool via pgx/stdlib and verifies
// connectivity with a Ping.
func Connect(ctx context.Context, cfg Config) (*sql.DB, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("postgres: DSN is required")
	}

	applyDefaults(&cfg)

	db, err := sql.Open("pgx", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("postgres: open: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	pingCtx, cancel := context.WithTimeout(ctx, cfg.PingTimeout)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}

	return db, nil
}

func applyDefaults(cfg *Config) {
	if cfg.MaxOpenConns == 0 {
		cfg.MaxOpenConns = 25
	}
	if cfg.MaxIdleConns == 0 {
		cfg.MaxIdleConns = 5
	}
	if cfg.ConnMaxIdleTime == 0 {
		cfg.ConnMaxIdleTime = 5 * time.Minute
	}
	if cfg.ConnMaxLifetime == 0 {
		cfg.ConnMaxLifetime = 1 * time.Hour
	}
	if cfg.PingTimeout == 0 {
		cfg.PingTimeout = 5 * time.Second
	}
}
