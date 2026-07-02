package settings

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"
)

var validTableName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// ErrNotFound is returned when no setting exists for a given key.
var ErrNotFound = errors.New("setting not found")

// Store provides settings CRUD scoped to a single SQL table.
type Store struct {
	db    *sql.DB
	table string
}

// NewStore creates a Store that reads/writes from the given table name.
// The table must have (key TEXT PRIMARY KEY, value TEXT) columns.
// Returns an error if the table name contains characters outside [a-zA-Z0-9_].
func NewStore(db *sql.DB, table string) (*Store, error) {
	if !validTableName.MatchString(table) {
		return nil, fmt.Errorf("settings: invalid table name: %q", table)
	}
	return &Store{db: db, table: table}, nil
}

// Get returns the value of a setting by key.
// Returns ErrNotFound if the key does not exist.
func (s *Store) Get(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM `+s.table+` WHERE key = $1`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return value, err
}

// GetInt reads a numeric setting. Returns fallback if the key is missing or
// cannot be parsed as an integer.
func (s *Store) GetInt(ctx context.Context, key string, fallback int) int {
	val, err := s.Get(ctx, key)
	if err != nil {
		return fallback
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return n
}

// GetAll returns all settings as a key→value map.
func (s *Store) GetAll(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM `+s.table+` ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		result[k] = v
	}
	return result, rows.Err()
}

// Upsert creates or updates a setting value by key.
func (s *Store) Upsert(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO `+s.table+` (key, value) VALUES ($1, $2) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	return err
}
