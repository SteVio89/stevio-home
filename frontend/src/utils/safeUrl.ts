// Allowed same-origin path prefixes for provider-returned checkout URLs. The
// mock provider returns /mock-checkout for its local simulated-payment page.
// Paddle Billing no longer returns a redirect URL — it opens an in-page overlay
// via Paddle.js (see utils/paddle.ts) — so no external host is allowed here.
const ALLOWED_SAME_ORIGIN_PREFIXES = [
  '/api/checkout/',
  '/mock-checkout',
];

// PLACEHOLDER_ORIGIN is fed to `new URL(raw, base)` purely to satisfy the
// constructor when validating relative paths. The origin doesn't matter —
// we only inspect the normalised pathname — but a stable base is required so
// the helper works under jsdom test mocks that replace window.location.
const PLACEHOLDER_ORIGIN = 'http://localhost';

// isSafeCheckoutURL accepts only a same-origin path with an allowed prefix (the
// mock provider). Absolute/external URLs are rejected so a misbehaving backend
// response can't redirect the user to an arbitrary external site.
export function isSafeCheckoutURL(raw: string): boolean {
  const samePrefix = ALLOWED_SAME_ORIGIN_PREFIXES.find((p) => raw.startsWith(p));
  if (!samePrefix) return false;
  try {
    // Normalise so "/mock-checkout/../evil" resolves to "/evil" and fails.
    // Accept only an exact match or a proper sub-path (with a "/" separator),
    // never a bare prefix — otherwise "/mock-checkout-evil" would slip through.
    const u = new URL(raw, PLACEHOLDER_ORIGIN);
    const subPathPrefix = samePrefix.endsWith('/') ? samePrefix : samePrefix + '/';
    return u.pathname === samePrefix || u.pathname.startsWith(subPathPrefix);
  } catch {
    return false;
  }
}

// isSafeDownloadURL accepts only the same-origin /api/downloads/file?token=…
// shape returned by createDownloadToken. The URL constructor normalises ".."
// segments, so "/api/downloads/../foo" resolves to "/foo" and is rejected.
export function isSafeDownloadURL(raw: string): boolean {
  if (!raw.startsWith('/api/downloads/')) return false;
  try {
    const u = new URL(raw, PLACEHOLDER_ORIGIN);
    return u.pathname.startsWith('/api/downloads/');
  } catch {
    return false;
  }
}
