package crypto

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type LicensePayload struct {
	BundleID    string `json:"bundle_id"`
	LicenseKey  string `json:"license_key"`
	MachineHash string `json:"machine_hash"`
	ActivatedAt string `json:"activated_at"`
}

type SignedLicense struct {
	Payload   LicensePayload `json:"payload"`
	Signature string         `json:"signature"`
	KeyID     string         `json:"key_id"`
}

// Signer is a DB-backed license signer that loads the active signing key on each operation.
type Signer struct {
	db        *sql.DB
	secretKey [32]byte
}

// NewDBSigner creates a DB-backed signer. It does not touch the DB at construction time,
// so the server can start even with zero keys in the DB.
func NewDBSigner(db *sql.DB, secretKey [32]byte) *Signer {
	return &Signer{db: db, secretKey: secretKey}
}

// GenerateKey creates a new Ed25519 keypair and returns base64-encoded private and public keys.
func GenerateKey() (privateB64, publicB64 string, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(priv),
		base64.StdEncoding.EncodeToString(pub),
		nil
}

// DeriveKeyID returns the first 8 hex characters of the SHA-256 hash of the raw public key bytes.
func DeriveKeyID(pubKeyBytes []byte) string {
	h := sha256.Sum256(pubKeyBytes)
	return hex.EncodeToString(h[:4])
}

// EncryptPrivateKey encrypts raw private key bytes with AES-256-GCM.
// Returns base64(nonce || ciphertext).
func EncryptPrivateKey(secretKey [32]byte, privateKeyBytes []byte) (string, error) {
	block, err := aes.NewCipher(secretKey[:])
	if err != nil {
		return "", fmt.Errorf("crypto: aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("crypto: nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, privateKeyBytes, nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// DecryptPrivateKey decrypts a base64-encoded AES-256-GCM ciphertext (nonce prepended).
func DecryptPrivateKey(secretKey [32]byte, ciphertext string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("crypto: decode ciphertext: %w", err)
	}
	block, err := aes.NewCipher(secretKey[:])
	if err != nil {
		return nil, fmt.Errorf("crypto: aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: gcm: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return nil, fmt.Errorf("crypto: ciphertext too short")
	}
	plaintext, err := gcm.Open(nil, raw[:nonceSize], raw[nonceSize:], nil)
	if err != nil {
		return nil, fmt.Errorf("crypto: decrypt: %w", err)
	}
	return plaintext, nil
}

// HasActiveKey returns true if there is an active signing key in the DB.
func (s *Signer) HasActiveKey(ctx context.Context) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM signing_keys WHERE active = TRUE)`).Scan(&exists)
	return exists, err
}

// ActiveKeyID returns the key_id of the currently active signing key, or "" if none.
func (s *Signer) ActiveKeyID(ctx context.Context) (string, error) {
	var keyID string
	err := s.db.QueryRowContext(ctx,
		`SELECT key_id FROM signing_keys WHERE active = TRUE`).Scan(&keyID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return keyID, err
}

// Sign loads the active signing key from the DB, decrypts it, and signs the license payload.
func (s *Signer) Sign(ctx context.Context, bundleID, licenseKey, machineHash string, activatedAt time.Time) (*SignedLicense, error) {

	var keyID, encryptedPrivKey string
	err := s.db.QueryRowContext(ctx,
		`SELECT key_id, encrypted_private_key FROM signing_keys WHERE active = TRUE`).
		Scan(&keyID, &encryptedPrivKey)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("crypto: no active signing key")
	}
	if err != nil {
		return nil, fmt.Errorf("crypto: load active key: %w", err)
	}

	privBytes, err := DecryptPrivateKey(s.secretKey, encryptedPrivKey)
	if err != nil {
		return nil, fmt.Errorf("crypto: decrypt active key: %w", err)
	}

	payload := LicensePayload{
		BundleID:    bundleID,
		LicenseKey:  licenseKey,
		MachineHash: machineHash,
		ActivatedAt: activatedAt.UTC().Format(time.RFC3339),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("crypto: marshal payload: %w", err)
	}

	sig := ed25519.Sign(ed25519.PrivateKey(privBytes), data)

	return &SignedLicense{
		Payload:   payload,
		Signature: base64.StdEncoding.EncodeToString(sig),
		KeyID:     keyID,
	}, nil
}
