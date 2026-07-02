package users

import (
	"context"
	"errors"
	"testing"

	"github.com/SteVio89/stevio-home/testutil"
)

func TestFindOrCreate_NewUser(t *testing.T) {
	db := testutil.SetupTestDB(t, "users", MigrationFiles)
	s := New(db)
	ctx := context.Background()

	u, created, err := s.FindOrCreate(ctx, "hash-new", "member")
	if err != nil {
		t.Fatalf("FindOrCreate: %v", err)
	}
	if !created {
		t.Error("expected created=true for new user")
	}
	if u.EmailHash != "hash-new" {
		t.Errorf("EmailHash = %q, want %q", u.EmailHash, "hash-new")
	}
	if u.Role != "member" {
		t.Errorf("Role = %q, want %q", u.Role, "member")
	}
	if u.ID == "" {
		t.Error("ID should not be empty")
	}
}

func TestFindOrCreate_ExistingUser(t *testing.T) {
	db := testutil.SetupTestDB(t, "users", MigrationFiles)
	s := New(db)
	ctx := context.Background()

	u1, _, err := s.FindOrCreate(ctx, "hash-exist", "member")
	if err != nil {
		t.Fatalf("first FindOrCreate: %v", err)
	}

	u2, created, err := s.FindOrCreate(ctx, "hash-exist", "admin")
	if err != nil {
		t.Fatalf("second FindOrCreate: %v", err)
	}
	if created {
		t.Error("expected created=false for existing user")
	}
	if u2.ID != u1.ID {
		t.Errorf("ID changed: %q vs %q", u2.ID, u1.ID)
	}
	if u2.Role != "member" {
		t.Errorf("Role should remain %q, got %q", "member", u2.Role)
	}
}

func TestGetByID(t *testing.T) {
	db := testutil.SetupTestDB(t, "users", MigrationFiles)
	s := New(db)
	ctx := context.Background()

	u, _, err := s.FindOrCreate(ctx, "hash-id", "member")
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.EmailHash != "hash-id" {
		t.Errorf("EmailHash = %q, want %q", got.EmailHash, "hash-id")
	}
}

func TestGetByID_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t, "users", MigrationFiles)
	s := New(db)

	_, err := s.GetByID(context.Background(), "nonexistent")
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestGetByEmailHash_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t, "users", MigrationFiles)
	s := New(db)

	_, err := s.GetByEmailHash(context.Background(), "nonexistent")
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUpdateRole(t *testing.T) {
	db := testutil.SetupTestDB(t, "users", MigrationFiles)
	s := New(db)
	ctx := context.Background()

	u, _, err := s.FindOrCreate(ctx, "hash-role", "member")
	if err != nil {
		t.Fatal(err)
	}

	emailHash, err := s.UpdateRole(ctx, u.ID, "admin")
	if err != nil {
		t.Fatalf("UpdateRole: %v", err)
	}
	if emailHash != "hash-role" {
		t.Errorf("returned emailHash = %q, want %q", emailHash, "hash-role")
	}

	updated, err := s.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Role != "admin" {
		t.Errorf("Role after update = %q, want %q", updated.Role, "admin")
	}
}

func TestUpdateRole_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t, "users", MigrationFiles)
	s := New(db)

	_, err := s.UpdateRole(context.Background(), "nonexistent", "admin")
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}
