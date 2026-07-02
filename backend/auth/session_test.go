package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SteVio89/stevio-home/testutil"
)

func TestCreateGetSession(t *testing.T) {
	db := testutil.SetupTestDB(t, "auth", MigrationFiles)
	ctx := context.Background()

	id := "sess-001"
	emailHash := "hash-abc"
	expires := time.Now().Add(time.Hour).UTC().Truncate(time.Millisecond)

	if err := CreateSession(ctx, db, id, emailHash, expires); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	s, err := GetSession(ctx, db, id)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if s == nil {
		t.Fatal("GetSession returned nil")
	}
	if s.ID != id {
		t.Errorf("ID = %q, want %q", s.ID, id)
	}
	if s.EmailHash != emailHash {
		t.Errorf("EmailHash = %q, want %q", s.EmailHash, emailHash)
	}
	if !s.ExpiresAt.Equal(expires) {
		t.Errorf("ExpiresAt = %v, want %v", s.ExpiresAt, expires)
	}
	if s.UserID != nil {
		t.Errorf("UserID should be nil, got %v", s.UserID)
	}
	if s.UserType != nil {
		t.Errorf("UserType should be nil, got %v", s.UserType)
	}
}

func TestCreateGetSession_WithUserFields(t *testing.T) {
	db := testutil.SetupTestDB(t, "auth", MigrationFiles)
	ctx := context.Background()

	userID := "user-abc"
	userType := "shop"
	expires := time.Now().Add(time.Hour).UTC().Truncate(time.Millisecond)

	if err := CreateSession(ctx, db, "sess-ext", "hash-ext", expires,
		WithSessionUser(userID, userType)); err != nil {
		t.Fatalf("CreateSession with user fields: %v", err)
	}

	s, err := GetSession(ctx, db, "sess-ext")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if s.UserID == nil || *s.UserID != userID {
		t.Errorf("UserID = %v, want %q", s.UserID, userID)
	}
	if s.UserType == nil || *s.UserType != userType {
		t.Errorf("UserType = %v, want %q", s.UserType, userType)
	}
}

func TestGetSession_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t, "auth", MigrationFiles)
	ctx := context.Background()

	s, err := GetSession(ctx, db, "nonexistent")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
	if s != nil {
		t.Errorf("expected nil session, got %+v", s)
	}
}

func TestDeleteSession(t *testing.T) {
	db := testutil.SetupTestDB(t, "auth", MigrationFiles)
	ctx := context.Background()

	id := "sess-del"
	if err := CreateSession(ctx, db, id, "hash", time.Now().Add(time.Hour).UTC()); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if err := DeleteSession(ctx, db, id); err != nil {
		t.Fatal(err)
	}

	_, err := GetSession(ctx, db, id)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("deleted session: want ErrSessionNotFound, got %v", err)
	}
}

func TestDeleteSessionsByEmailHash(t *testing.T) {
	db := testutil.SetupTestDB(t, "auth", MigrationFiles)
	ctx := context.Background()

	emailHash := "hash-email"
	if err := CreateSession(ctx, db, "s1", emailHash, time.Now().Add(time.Hour).UTC()); err != nil {
		t.Fatalf("CreateSession s1: %v", err)
	}
	if err := CreateSession(ctx, db, "s2", emailHash, time.Now().Add(time.Hour).UTC()); err != nil {
		t.Fatalf("CreateSession s2: %v", err)
	}
	if err := CreateSession(ctx, db, "s3", "other-hash", time.Now().Add(time.Hour).UTC()); err != nil {
		t.Fatalf("CreateSession s3: %v", err)
	}

	if err := DeleteSessionsByEmailHash(ctx, db, emailHash); err != nil {
		t.Fatal(err)
	}

	// s1 and s2 should be gone.
	for _, id := range []string{"s1", "s2"} {
		_, err := GetSession(ctx, db, id)
		if !errors.Is(err, ErrSessionNotFound) {
			t.Errorf("session %s: want ErrSessionNotFound, got %v", id, err)
		}
	}
	// s3 should remain.
	s, err := GetSession(ctx, db, "s3")
	if err != nil {
		t.Fatalf("GetSession s3: %v", err)
	}
	if s == nil {
		t.Error("session s3 should still exist")
	}
}

func TestDeleteExpiredSessions(t *testing.T) {
	db := testutil.SetupTestDB(t, "auth", MigrationFiles)
	ctx := context.Background()

	// Create one expired and one valid session.
	if err := CreateSession(ctx, db, "expired", "hash", time.Now().Add(-time.Hour).UTC()); err != nil {
		t.Fatalf("CreateSession expired: %v", err)
	}
	if err := CreateSession(ctx, db, "valid", "hash", time.Now().Add(time.Hour).UTC()); err != nil {
		t.Fatalf("CreateSession valid: %v", err)
	}

	n, err := DeleteExpiredSessions(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("deleted %d sessions, want 1", n)
	}

	_, err = GetSession(ctx, db, "expired")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Error("expired session should be deleted")
	}
	s, err := GetSession(ctx, db, "valid")
	if err != nil {
		t.Fatalf("GetSession valid: %v", err)
	}
	if s == nil {
		t.Error("valid session should still exist")
	}
}
