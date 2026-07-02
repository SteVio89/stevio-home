package dbutil

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// NewID returns a new UUID v4 string generated from crypto/rand.
func NewID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand.Read failed: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// NewSecureToken returns a 256-bit cryptographically random token,
// base64url-encoded without padding (43 characters).
// Use this for security-sensitive tokens (magic links, API keys) instead
// of NewID, which is designed for database primary keys.
func NewSecureToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand.Read failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
