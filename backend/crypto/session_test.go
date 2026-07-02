package crypto

import (
	"strings"
	"testing"
)

func TestSignVerifySession_RoundTrip(t *testing.T) {
	t.Parallel()
	secret := []byte("test-secret-key-32-bytes-long!!!")
	id := "session-abc-123"

	cookie := SignSession(id, secret)
	gotID, ok := VerifySession(cookie, secret)
	if !ok {
		t.Fatal("VerifySession returned false for a validly signed cookie")
	}
	if gotID != id {
		t.Fatalf("VerifySession returned ID %q, want %q", gotID, id)
	}
}

func TestVerifySession_WrongSecret(t *testing.T) {
	t.Parallel()
	secret1 := []byte("secret-one-AAAAAAAAAAAAAAAAAAAAAA")
	secret2 := []byte("secret-two-BBBBBBBBBBBBBBBBBBBBBB")
	id := "session-456"

	cookie := SignSession(id, secret1)
	_, ok := VerifySession(cookie, secret2)
	if ok {
		t.Fatal("VerifySession should fail when verified with a different secret")
	}
}

func TestVerifySession_EmptyCookie(t *testing.T) {
	t.Parallel()
	secret := []byte("any-secret")

	gotID, ok := VerifySession("", secret)
	if ok {
		t.Fatal("VerifySession should return false for an empty cookie")
	}
	if gotID != "" {
		t.Fatalf("VerifySession returned ID %q for empty cookie, want empty string", gotID)
	}
}

func TestVerifySession_NoDot(t *testing.T) {
	t.Parallel()
	secret := []byte("any-secret")

	gotID, ok := VerifySession("no-dot-in-this-string", secret)
	if ok {
		t.Fatal("VerifySession should return false when cookie has no dot separator")
	}
	if gotID != "" {
		t.Fatalf("VerifySession returned ID %q for dotless cookie, want empty string", gotID)
	}
}

func TestVerifySession_MalformedBase64(t *testing.T) {
	t.Parallel()
	secret := []byte("any-secret")

	gotID, ok := VerifySession("session.!!!invalid!!!", secret)
	if ok {
		t.Fatal("VerifySession should return false for malformed base64 HMAC")
	}
	if gotID != "" {
		t.Fatalf("VerifySession returned ID %q for malformed cookie, want empty string", gotID)
	}
}

func TestVerifySession_TamperedID(t *testing.T) {
	t.Parallel()
	secret := []byte("test-secret-key-32-bytes-long!!!")
	originalID := "session-123"

	cookie := SignSession(originalID, secret)

	// Extract the HMAC suffix (everything after the last dot)
	dot := strings.LastIndexByte(cookie, '.')
	hmacSuffix := cookie[dot+1:]

	// Build a tampered cookie with a different ID but the original HMAC
	tampered := "evil-session." + hmacSuffix

	_, ok := VerifySession(tampered, secret)
	if ok {
		t.Fatal("VerifySession should fail when the session ID has been tampered with")
	}
}

func TestSignSession_ContainsDot(t *testing.T) {
	t.Parallel()
	secret := []byte("test-secret")
	id := "session-abc"

	cookie := SignSession(id, secret)
	if !strings.Contains(cookie, ".") {
		t.Fatal("signed cookie should contain a dot separator")
	}
	// The portion before the last dot should be the original ID
	dot := strings.LastIndexByte(cookie, '.')
	if cookie[:dot] != id {
		t.Fatalf("cookie prefix is %q, want %q", cookie[:dot], id)
	}
}

func TestSignVerifySession_IDWithDots(t *testing.T) {
	t.Parallel()
	// Session IDs containing dots should still work because VerifySession
	// splits on the *last* dot.
	secret := []byte("test-secret-key-32-bytes-long!!!")
	id := "uuid.with.dots.in.it"

	cookie := SignSession(id, secret)
	gotID, ok := VerifySession(cookie, secret)
	if !ok {
		t.Fatal("VerifySession should succeed for IDs containing dots")
	}
	if gotID != id {
		t.Fatalf("VerifySession returned ID %q, want %q", gotID, id)
	}
}
