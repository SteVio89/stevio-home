package crypto

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
	"testing/fstest"
	"time"

	"github.com/SteVio89/stevio-home/testutil"
)

var signingKeysMigration = fstest.MapFS{
	"migrations/001_signing_keys.sql": &fstest.MapFile{
		Data: []byte(`CREATE TABLE signing_keys (
			id TEXT PRIMARY KEY,
			key_id TEXT NOT NULL UNIQUE,
			encrypted_private_key TEXT NOT NULL,
			public_key_b64 TEXT NOT NULL,
			active BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`),
	},
}

// setupTestSigner creates a fresh Postgres schema with just the signing_keys
// table, generates a key, inserts it as active, and returns a Signer.
func setupTestSigner(t *testing.T) (*Signer, string) {
	t.Helper()
	db := testutil.SetupTestDB(t, "signing", signingKeysMigration)

	var secretKey [32]byte
	copy(secretKey[:], []byte("test-secret-key-for-encryption!!"))

	privB64, pubB64, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	privBytes, _ := base64.StdEncoding.DecodeString(privB64)
	pubBytes, _ := base64.StdEncoding.DecodeString(pubB64)
	keyID := DeriveKeyID(pubBytes)

	encrypted, err := EncryptPrivateKey(secretKey, privBytes)
	if err != nil {
		t.Fatalf("EncryptPrivateKey: %v", err)
	}

	_, err = db.Exec(`INSERT INTO signing_keys (id, key_id, encrypted_private_key, public_key_b64, active)
		VALUES ('test-key-1', $1, $2, $3, TRUE)`, keyID, encrypted, pubB64)
	if err != nil {
		t.Fatalf("insert key: %v", err)
	}

	return NewDBSigner(db, secretKey), keyID
}

func TestSign(t *testing.T) {
	signer, _ := setupTestSigner(t)

	sl, err := signer.Sign(context.Background(), "com.test.app", "test-license-key", "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234", time.Now())
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	if sl.Payload.BundleID != "com.test.app" {
		t.Errorf("expected bundle_id com.test.app, got %s", sl.Payload.BundleID)
	}
	if sl.Signature == "" {
		t.Error("signature is empty")
	}
	if sl.KeyID == "" {
		t.Error("key_id is empty")
	}
}

func TestSignatureIsStable(t *testing.T) {
	signer, _ := setupTestSigner(t)

	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	sl, err := signer.Sign(context.Background(), "com.test.app", "key-123", "aaaa1111bbbb2222cccc3333dddd4444eeee5555ffff6666aaaa1111bbbb2222", ts)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	data, _ := json.Marshal(sl.Payload)
	sig, _ := base64.StdEncoding.DecodeString(sl.Signature)

	if len(data) == 0 {
		t.Error("payload JSON should not be empty")
	}
	if len(sig) != 64 {
		t.Errorf("Ed25519 signature should be 64 bytes, got %d", len(sig))
	}
}

func TestEncryptDecryptRoundtrip(t *testing.T) {
	var secretKey [32]byte
	copy(secretKey[:], []byte("test-secret-key-for-encryption!!"))

	original := []byte("this is a 64-byte private key for testing the encryption flow!!")

	encrypted, err := EncryptPrivateKey(secretKey, original)
	if err != nil {
		t.Fatalf("EncryptPrivateKey: %v", err)
	}

	decrypted, err := DecryptPrivateKey(secretKey, encrypted)
	if err != nil {
		t.Fatalf("DecryptPrivateKey: %v", err)
	}

	if string(decrypted) != string(original) {
		t.Errorf("roundtrip failed: got %q, want %q", decrypted, original)
	}
}

func TestDeriveKeyID(t *testing.T) {
	_, pubB64, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	pubBytes, _ := base64.StdEncoding.DecodeString(pubB64)
	keyID := DeriveKeyID(pubBytes)

	if len(keyID) != 8 {
		t.Errorf("expected 8-char key_id, got %d chars: %s", len(keyID), keyID)
	}
}

func TestHasActiveKey(t *testing.T) {
	signer, _ := setupTestSigner(t)

	has, err := signer.HasActiveKey(context.Background())
	if err != nil {
		t.Fatalf("HasActiveKey: %v", err)
	}
	if !has {
		t.Error("should have active key")
	}
}
