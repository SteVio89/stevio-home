// Whitelisted hosts for external checkout redirects. Add new payment-provider
// hosts here as we wire them up. Empty list means same-origin paths only.
// Paddle's hosted checkout URL (tx.Checkout.URL) lives under *.paddle.com in
// production and under a sandbox subdomain in the sandbox environment; both
// match the `paddle.com` suffix rule.
const ALLOWED_CHECKOUT_HOSTS = [
  'paddle.com',
];

// Allowed same-origin path prefixes for provider-returned URLs. The mock
// provider returns /mock-checkout for its local simulated-payment page; no
// other same-origin path should be treated as a valid checkout redirect.
const ALLOWED_SAME_ORIGIN_PREFIXES = [
  '/api/checkout/',
  '/mock-checkout',
];

function hostIsAllowed(host: string, allowed: string[]): boolean {
  return allowed.some((suffix) => host === suffix || host.endsWith('.' + suffix));
}

// PLACEHOLDER_ORIGIN is fed to `new URL(raw, base)` purely to satisfy the
// constructor when validating relative paths. The origin doesn't matter —
// we only inspect the normalised pathname — but a stable base is required so
// the helper works under jsdom test mocks that replace window.location.
const PLACEHOLDER_ORIGIN = 'http://localhost';

// isSafeCheckoutURL accepts either a same-origin path with an allowed prefix
// (mock provider) or an https:// URL on a whitelisted payment host (Paddle).
// Anything else is rejected so a misbehaving backend response can't redirect
// the user to an arbitrary external site.
export function isSafeCheckoutURL(raw: string): boolean {
  const samePrefix = ALLOWED_SAME_ORIGIN_PREFIXES.find((p) => raw.startsWith(p));
  if (samePrefix) {
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
  try {
    const u = new URL(raw);
    return u.protocol === 'https:' && hostIsAllowed(u.hostname, ALLOWED_CHECKOUT_HOSTS);
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
