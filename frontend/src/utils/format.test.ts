import { describe, it, expect } from 'vitest';
import { formatPrice } from './format';

describe('formatPrice', () => {
  it('renders cents as a two-decimal amount with the currency symbol', () => {
    expect(formatPrice(1999, '€')).toBe('€19.99');
    expect(formatPrice(500, '$')).toBe('$5.00');
  });

  it('handles zero and sub-euro amounts', () => {
    expect(formatPrice(0, '€')).toBe('€0.00');
    expect(formatPrice(9, '€')).toBe('€0.09');
  });

  it('rounds to two decimals', () => {
    expect(formatPrice(12345, '$')).toBe('$123.45');
  });
});
