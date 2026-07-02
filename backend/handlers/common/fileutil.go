package common

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"os"
	"regexp"
	"strings"
)

// bundleIDRe matches relaxed bundle identifiers: letters, digits, dots, hyphens, underscores.
var bundleIDRe = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// IsValidBundleID checks that the bundle ID is non-empty, at most 255 chars,
// and contains only letters, digits, dots, hyphens, and underscores.
func IsValidBundleID(id string) bool {
	return len(id) > 0 && len(id) <= 255 && bundleIDRe.MatchString(id)
}

// GenerateToken returns a 64-character hex token from 32 random bytes.
func GenerateToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

// AllowedImageExt returns true if ext is a permitted image extension.
func AllowedImageExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif":
		return true
	}
	return false
}

// AllowedBinaryExt returns true if ext is a permitted app binary extension.
func AllowedBinaryExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".dmg", ".pkg", ".zip":
		return true
	}
	return false
}

// IsValidIconURL restricts icon URLs to same-origin paths only.
func IsValidIconURL(u string) bool {
	return strings.HasPrefix(u, "/media/") || strings.HasPrefix(u, "/assets/")
}

// WriteFile writes r to the file at dst, creating or truncating it.
func WriteFile(dst string, r io.Reader) error {
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = io.Copy(f, r)
	return err
}
