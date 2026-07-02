export function formatPrice(cents: number, currencySymbol: string): string {
  return `${currencySymbol}${(cents / 100).toFixed(2)}`;
}
