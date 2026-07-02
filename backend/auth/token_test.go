package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SteVio89/stevio-home/testutil"
)

func TestInsertConsumeRoundTrip(t *testing.T) {
	db := testutil.SetupTestDB(t, "auth", MigrationFiles)
	ctx := context.Background()

	token := "tok-001"
	emailHash := "hash-abc"
	expires := time.Now().Add(time.Hour).UTC()

	if err := InsertAuthToken(ctx, db, token, emailHash, expires); err != nil {
		t.Fatalf("InsertAuthToken: %v", err)
	}

	got, err := ConsumeAuthToken(ctx, db, token)
	if err != nil {
		t.Fatalf("ConsumeAuthToken: %v", err)
	}
	if got != emailHash {
		t.Errorf("ConsumeAuthToken returned email %q, want %q", got, emailHash)
	}
}

func TestConsumeAuthToken_Used(t *testing.T) {
	db := testutil.SetupTestDB(t, "auth", MigrationFiles)
	ctx := context.Background()

	token := "tok-used"
	_ = InsertAuthToken(ctx, db, token, "hash", time.Now().Add(time.Hour).UTC())
	_, _ = ConsumeAuthToken(ctx, db, token) // first consume

	_, err := ConsumeAuthToken(ctx, db, token)
	if !errors.Is(err, ErrTokenUsed) {
		t.Errorf("double consume error = %v, want ErrTokenUsed", err)
	}
}

func TestConsumeAuthToken_Expired(t *testing.T) {
	db := testutil.SetupTestDB(t, "auth", MigrationFiles)
	ctx := context.Background()

	token := "tok-expired"
	_ = InsertAuthToken(ctx, db, token, "hash", time.Now().Add(-time.Hour).UTC())

	_, err := ConsumeAuthToken(ctx, db, token)
	if !errors.Is(err, ErrTokenExpired) {
		t.Errorf("expired token error = %v, want ErrTokenExpired", err)
	}
}

func TestConsumeAuthToken_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t, "auth", MigrationFiles)
	ctx := context.Background()

	_, err := ConsumeAuthToken(ctx, db, "nonexistent")
	if !errors.Is(err, ErrTokenNotFound) {
		t.Errorf("not found error = %v, want ErrTokenNotFound", err)
	}
}

func TestConsumeAuthToken_WrongToken(t *testing.T) {
	db := testutil.SetupTestDB(t, "auth", MigrationFiles)
	ctx := context.Background()

	_ = InsertAuthToken(ctx, db, "real-token", "hash-abc", time.Now().Add(time.Hour).UTC())

	// A different token value should not match, since they hash differently.
	_, err := ConsumeAuthToken(ctx, db, "wrong-token")
	if !errors.Is(err, ErrTokenNotFound) {
		t.Errorf("wrong token error = %v, want ErrTokenNotFound", err)
	}
}

func TestConsumeAuthTokenFull_WithUserType(t *testing.T) {
	db := testutil.SetupTestDB(t, "auth", MigrationFiles)
	ctx := context.Background()

	token := "tok-typed"
	emailHash := "hash-typed"
	expires := time.Now().Add(time.Hour).UTC()

	if err := InsertAuthToken(ctx, db, token, emailHash, expires, WithTokenUserType("customer")); err != nil {
		t.Fatalf("InsertAuthToken: %v", err)
	}

	gotEmail, gotType, err := ConsumeAuthTokenFull(ctx, db, token)
	if err != nil {
		t.Fatalf("ConsumeAuthTokenFull: %v", err)
	}
	if gotEmail != emailHash {
		t.Errorf("email = %q, want %q", gotEmail, emailHash)
	}
	if gotType == nil || *gotType != "customer" {
		t.Errorf("userType = %v, want %q", gotType, "customer")
	}
}

func TestConsumeAuthTokenFull_WithoutUserType(t *testing.T) {
	db := testutil.SetupTestDB(t, "auth", MigrationFiles)
	ctx := context.Background()

	token := "tok-untyped"
	_ = InsertAuthToken(ctx, db, token, "hash-untyped", time.Now().Add(time.Hour).UTC())

	_, gotType, err := ConsumeAuthTokenFull(ctx, db, token)
	if err != nil {
		t.Fatalf("ConsumeAuthTokenFull: %v", err)
	}
	if gotType != nil {
		t.Errorf("userType should be nil, got %v", gotType)
	}
}

func TestHasValidAuthToken(t *testing.T) {
	db := testutil.SetupTestDB(t, "auth", MigrationFiles)
	ctx := context.Background()

	emailHash := "hash-valid"

	// No token yet.
	has, err := HasValidAuthToken(ctx, db, emailHash)
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Error("expected false before inserting token")
	}

	// Insert valid token.
	_ = InsertAuthToken(ctx, db, "tok-valid", emailHash, time.Now().Add(time.Hour).UTC())
	has, err = HasValidAuthToken(ctx, db, emailHash)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Error("expected true after inserting valid token")
	}

	// Consume it.
	_, _ = ConsumeAuthToken(ctx, db, "tok-valid")
	has, err = HasValidAuthToken(ctx, db, emailHash)
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Error("expected false after consuming token")
	}
}

func TestDeleteExpiredAuthTokens(t *testing.T) {
	db := testutil.SetupTestDB(t, "auth", MigrationFiles)
	ctx := context.Background()

	// Insert an expired token and a used token.
	_ = InsertAuthToken(ctx, db, "tok-exp", "hash", time.Now().Add(-time.Hour).UTC())
	_ = InsertAuthToken(ctx, db, "tok-use", "hash", time.Now().Add(time.Hour).UTC())
	_, _ = ConsumeAuthToken(ctx, db, "tok-use")
	// Insert a valid, unused token.
	_ = InsertAuthToken(ctx, db, "tok-ok", "hash", time.Now().Add(time.Hour).UTC())

	n, err := DeleteExpiredAuthTokens(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("deleted %d tokens, want 2", n)
	}

	// Valid token should remain.
	has, _ := HasValidAuthToken(ctx, db, "hash")
	if !has {
		t.Error("valid token should still exist")
	}
}
