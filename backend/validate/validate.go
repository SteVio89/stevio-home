package validate

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	ErrRequired = errors.New("required")
	ErrTooLong  = errors.New("too long")
	ErrInvalid  = errors.New("invalid")
)

var slugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Email performs a structural check: non-empty local part, @, domain with dot,
// no leading/trailing dots in domain. Rejects addresses over 254 characters
// (RFC 5321) and those containing CR/LF (header injection prevention).
func Email(s string) error {
	if len(s) > 254 {
		return fmt.Errorf("email exceeds 254 characters: %w", ErrTooLong)
	}
	if strings.ContainsAny(s, "\r\n") {
		return fmt.Errorf("email contains invalid characters: %w", ErrInvalid)
	}
	at := strings.LastIndex(s, "@")
	if at < 1 {
		return fmt.Errorf("missing or empty local part: %w", ErrInvalid)
	}
	domain := s[at+1:]
	if domain == "" || !strings.Contains(domain, ".") {
		return fmt.Errorf("domain must contain a dot: %w", ErrInvalid)
	}
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return fmt.Errorf("domain has leading or trailing dot: %w", ErrInvalid)
	}
	return nil
}

// safeLinkSchemes are the URL schemes allowed for admin-entered, publicly-rendered
// links (social links, project external URLs). Anything else — notably javascript:,
// data:, vbscript: — is rejected so a stored href cannot execute script when a
// visitor clicks it, independent of the CSP.
var safeLinkSchemes = map[string]bool{"http": true, "https": true, "mailto": true}

// LinkURL validates that s is a well-formed absolute URL using a safe scheme
// (http, https, or mailto). It is the server-side counterpart to the client's
// link-scheme check and must not be bypassable. Rejects strings over 2000 chars.
func LinkURL(s string) error {
	if s == "" {
		return fmt.Errorf("url is empty: %w", ErrRequired)
	}
	if len(s) > 2000 {
		return fmt.Errorf("url exceeds 2000 characters: %w", ErrTooLong)
	}
	if strings.ContainsAny(s, "\r\n\t ") {
		return fmt.Errorf("url contains whitespace: %w", ErrInvalid)
	}
	u, err := url.Parse(s)
	if err != nil {
		return fmt.Errorf("url is malformed: %w", ErrInvalid)
	}
	if !safeLinkSchemes[strings.ToLower(u.Scheme)] {
		return fmt.Errorf("url scheme %q is not allowed: %w", u.Scheme, ErrInvalid)
	}
	return nil
}

// Slug validates that s contains only [a-z0-9-], is non-empty, and has no
// leading or trailing hyphens.
func Slug(s string) error {
	if s == "" {
		return fmt.Errorf("slug is empty: %w", ErrRequired)
	}
	if !slugRe.MatchString(s) {
		return fmt.Errorf("slug %q contains invalid characters or has leading/trailing hyphens: %w", s, ErrInvalid)
	}
	return nil
}
