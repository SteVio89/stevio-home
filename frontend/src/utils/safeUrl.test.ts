import { describe, it, expect } from 'vitest';
import { isSafeCheckoutURL, isSafeDownloadURL } from './safeUrl';

describe('isSafeCheckoutURL', () => {
  it('accepts https URLs on paddle.com and its subdomains', () => {
    expect(isSafeCheckoutURL('https://paddle.com')).toBe(true);
    expect(isSafeCheckoutURL('https://checkout.paddle.com/pay/txn_123')).toBe(true);
    expect(isSafeCheckoutURL('https://sandbox-checkout.paddle.com/pay/txn_123')).toBe(true);
  });

  it('rejects arbitrary external hosts', () => {
    expect(isSafeCheckoutURL('https://evil.com/pay')).toBe(false);
    expect(isSafeCheckoutURL('https://example.com')).toBe(false);
  });

  it('rejects suffix-bypass hosts that merely contain paddle.com', () => {
    // Classic allowlist bypass: attacker controls paddle.com.evil.com.
    expect(isSafeCheckoutURL('https://paddle.com.evil.com/pay')).toBe(false);
    expect(isSafeCheckoutURL('https://notpaddle.com/pay')).toBe(false);
  });

  it('rejects non-https schemes on allowed hosts', () => {
    expect(isSafeCheckoutURL('http://checkout.paddle.com/pay')).toBe(false);
    expect(isSafeCheckoutURL('javascript:alert(1)')).toBe(false);
  });

  it('accepts allowed same-origin checkout prefixes', () => {
    expect(isSafeCheckoutURL('/api/checkout/session_abc')).toBe(true);
    expect(isSafeCheckoutURL('/mock-checkout')).toBe(true);
    expect(isSafeCheckoutURL('/mock-checkout/session/1')).toBe(true);
  });

  it('rejects path traversal that escapes an allowed prefix', () => {
    expect(isSafeCheckoutURL('/mock-checkout/../evil')).toBe(false);
    expect(isSafeCheckoutURL('/api/checkout/../../secret')).toBe(false);
  });

  it('rejects protocol-relative and empty inputs', () => {
    expect(isSafeCheckoutURL('//evil.com')).toBe(false);
    expect(isSafeCheckoutURL('')).toBe(false);
  });
});

describe('isSafeDownloadURL', () => {
  it('accepts same-origin download paths', () => {
    expect(isSafeDownloadURL('/api/downloads/file?token=abc')).toBe(true);
  });

  it('rejects traversal, external hosts, and near-miss prefixes', () => {
    expect(isSafeDownloadURL('/api/downloads/../evil')).toBe(false);
    expect(isSafeDownloadURL('https://evil.com/api/downloads/x')).toBe(false);
    expect(isSafeDownloadURL('/api/downloadsX')).toBe(false);
    expect(isSafeDownloadURL('')).toBe(false);
  });
});
