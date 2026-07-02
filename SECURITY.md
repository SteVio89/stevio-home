# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability, please report it responsibly:

1. **Do NOT** open a public GitHub issue
2. Email the maintainer directly (see the project's GitHub profile for contact info)
3. Include a clear description of the vulnerability, steps to reproduce, and potential impact

You should receive an acknowledgment within 48 hours. We will work with you to understand and address the issue before any public disclosure.

## Security Practices

Stevio Store is designed with privacy and security as first-class concerns:

- **Email hashing**: All email addresses are stored as keyed hashes (HMAC-SHA256 with a per-deployment secret salt) — plaintext emails are never persisted
- **Ed25519 signing**: License keys are cryptographically signed
- **Input validation**: All user input is validated and length-capped before database writes
- **Path safety**: File operations use `safePath()` to prevent directory traversal
- **Rate limiting**: All public endpoints are rate-limited (IP-based)
- **Session security**: HttpOnly, SameSite=Lax cookies backed by server-side sessions (CSRF protection via SameSite plus JSON-only API endpoints); Secure flag in production
- **CSP headers**: Strict Content Security Policy with no external script sources
- **No tracking**: No analytics, no third-party cookies, self-hosted fonts

## Supported Versions

Security fixes are applied to the latest release on `main`. There is no backporting to older versions.
