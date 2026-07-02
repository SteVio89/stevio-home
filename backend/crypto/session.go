package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strings"
)

// SignSession returns the signed cookie value for the given session ID.
// Format: sessionID.base64url(HMAC-SHA256(sessionID, secret))
func SignSession(id string, secret []byte) string {
	mac := sessionMAC(id, secret)
	return id + "." + base64.RawURLEncoding.EncodeToString(mac)
}

// VerifySession parses and validates a signed session cookie value.
// Returns the raw session ID on success, or ("", false) if the format
// is invalid or the HMAC does not match.
func VerifySession(cookie string, secret []byte) (string, bool) {
	dot := strings.LastIndexByte(cookie, '.')
	if dot < 1 {
		return "", false
	}
	id := cookie[:dot]
	got, err := base64.RawURLEncoding.DecodeString(cookie[dot+1:])
	if err != nil {
		return "", false
	}
	if !hmac.Equal(got, sessionMAC(id, secret)) {
		return "", false
	}
	return id, true
}

func sessionMAC(id string, secret []byte) []byte {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(id))
	return h.Sum(nil)
}
