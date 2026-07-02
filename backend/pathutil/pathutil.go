package pathutil

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SafePath resolves joined path components and verifies the result stays within
// root. Returns the clean absolute path or an error if traversal is detected.
func SafePath(root string, parts ...string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("safePath: abs root: %w", err)
	}
	joined := filepath.Join(append([]string{absRoot}, parts...)...)
	cleaned := filepath.Clean(joined)
	// Ensure the resolved path is under root (prefix check with separator).
	if !strings.HasPrefix(cleaned, absRoot+string(filepath.Separator)) && cleaned != absRoot {
		return "", fmt.Errorf("safePath: %q escapes root %q", cleaned, absRoot)
	}
	return cleaned, nil
}

// SanitizePathSegment returns filepath.Base of s and rejects values that
// resolve to "." or ".." or contain path separators after cleaning.
func SanitizePathSegment(s string) (string, error) {
	base := filepath.Base(s)
	if base == "." || base == ".." || base == string(filepath.Separator) {
		return "", fmt.Errorf("invalid path segment: %q", s)
	}
	return base, nil
}

// SanitizeFilename strips all characters except [A-Za-z0-9._-] to prevent
// header injection via Content-Disposition filenames.
// Returns "download" if no safe characters remain.
func SanitizeFilename(name string) string {
	var b strings.Builder
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '-' || c == '_' {
			b.WriteRune(c)
		}
	}
	if b.Len() == 0 {
		return "download"
	}
	return b.String()
}
