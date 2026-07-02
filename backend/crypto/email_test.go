package crypto

import "testing"

const testSalt = "test-salt"

func TestHashEmail(t *testing.T) {
	t.Parallel()
	// Pre-computed HMAC-SHA256 of "user@example.com" with key "test-salt".
	const wantUser = "89f16b1b400734838e972e993c41028446f771037d10b20f4937fc3393644731"

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"lowercase", "user@example.com", wantUser},
		{"mixed case normalized", "User@Example.COM", wantUser},
		{"leading/trailing spaces trimmed", "  user@example.com  ", wantUser},
		{"mixed case + spaces", "  User@Example.COM  ", wantUser},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HashEmail(tt.input, testSalt)
			if got != tt.want {
				t.Errorf("HashEmail(%q, %q) = %q, want %q", tt.input, testSalt, got, tt.want)
			}
		})
	}

	// Verify output is 64 hex chars (HMAC-SHA256 = 32 bytes).
	h := HashEmail("test@test.com", testSalt)
	if len(h) != 64 {
		t.Errorf("expected 64 hex chars, got %d", len(h))
	}

	// Verify different emails produce different hashes.
	h1 := HashEmail("a@b.com", testSalt)
	h2 := HashEmail("c@d.com", testSalt)
	if h1 == h2 {
		t.Error("different emails should produce different hashes")
	}

	// Verify different salts produce different hashes.
	h3 := HashEmail("a@b.com", "other-salt")
	if h1 == h3 {
		t.Error("different salts should produce different hashes")
	}
}
