package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// HashEmail normalizes email (lowercase, trimmed) and returns its HMAC-SHA256
// hex digest using the provided salt. The salt should be a per-deployment secret
// stored as an environment variable, never in the database.
func HashEmail(email, salt string) string {
	normalized := strings.ToLower(strings.TrimSpace(email))
	mac := hmac.New(sha256.New, []byte(salt))
	mac.Write([]byte(normalized))
	return hex.EncodeToString(mac.Sum(nil))
}
