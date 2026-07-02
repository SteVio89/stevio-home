package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/SteVio89/stevio-home/dbutil"
)

// User represents a row in the users table.
type User struct {
	ID        string    `json:"id"`
	EmailHash string    `json:"email_hash"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// ErrUserNotFound is returned when no user exists for a given identifier.
var ErrUserNotFound = errors.New("user not found")

// Store provides user CRUD backed by the users table.
type Store struct {
	db *sql.DB
}

// New creates a Store.
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// FindOrCreate returns the existing user for emailHash, or creates one
// with the given defaultRole. Returns the user and whether it was newly created.
func (s *Store) FindOrCreate(ctx context.Context, emailHash, defaultRole string) (*User, bool, error) {
	u, err := s.GetByEmailHash(ctx, emailHash)
	if err == nil {
		return u, false, nil
	}
	if !errors.Is(err, ErrUserNotFound) {
		return nil, false, err
	}

	// User does not exist — create one.
	id := dbutil.NewID()
	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO users (id, email_hash, role, created_at) VALUES ($1, $2, $3, $4)`,
		id, emailHash, defaultRole, now)
	if err != nil {
		// Race condition: concurrent first-login may have created the user.
		if dbutil.IsUniqueViolation(err) {
			u, err2 := s.GetByEmailHash(ctx, emailHash)
			if err2 != nil {
				return nil, false, fmt.Errorf("users: retry after conflict: %w", err2)
			}
			return u, false, nil
		}
		return nil, false, fmt.Errorf("users: insert: %w", err)
	}

	return &User{
		ID:        id,
		EmailHash: emailHash,
		Role:      defaultRole,
		CreatedAt: now,
	}, true, nil
}

// GetByEmailHash returns the user for the given email hash.
func (s *Store) GetByEmailHash(ctx context.Context, emailHash string) (*User, error) {
	return s.scanOne(s.db.QueryRowContext(ctx,
		`SELECT id, email_hash, role, created_at FROM users WHERE email_hash = $1`, emailHash))
}

// GetByID returns the user for the given ID.
func (s *Store) GetByID(ctx context.Context, id string) (*User, error) {
	return s.scanOne(s.db.QueryRowContext(ctx,
		`SELECT id, email_hash, role, created_at FROM users WHERE id = $1`, id))
}

// UpdateRole changes the user's role and returns the user's email hash.
// The caller should invalidate existing sessions for this email hash,
// since sessions cache the role at creation time.
func (s *Store) UpdateRole(ctx context.Context, userID, role string) (emailHash string, err error) {
	// Look up the user first to get the email hash for session invalidation.
	u, err := s.GetByID(ctx, userID)
	if err != nil {
		return "", err
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET role = $1 WHERE id = $2`, role, userID)
	if err != nil {
		return "", err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return "", ErrUserNotFound
	}
	return u.EmailHash, nil
}

func (s *Store) scanOne(row *sql.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.EmailHash, &u.Role, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}
