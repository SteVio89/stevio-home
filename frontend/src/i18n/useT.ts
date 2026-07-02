/**
 * Dynamic locale helpers — used outside the react-i18next context
 * (Nav, SiteFooterFallback, MaintenanceWall, LocaleRedirect).
 *
 * All translation calls should use useTranslation() from react-i18next.
 * This file only exports path/detection utilities.
 */

import { DEFAULT_LOCALE } from './index';

export interface LocaleInfo {
  code: string;
  name: string;
  is_default: boolean;
}

/**
 * Extracts locale from a URL pathname like /de/... or /en/...
 * Use outside LocaleProvider (Nav, footer, MaintenanceWall).
 */
export function localeFromPath(pathname: string, supportedCodes: string[]): string {
  const match = pathname.match(/^\/([a-z]{2,5})(\/|$)/);
  const code = match?.[1];
  if (code && supportedCodes.includes(code)) return code;
  return supportedCodes.find(c => c === DEFAULT_LOCALE) ?? supportedCodes[0] ?? DEFAULT_LOCALE;
}

/**
 * Detects the best locale: saved preference first, then browser language.
 */
export function detectBrowserLocale(locales: LocaleInfo[]): string {
  const codes = locales.map(l => l.code);
  const defaultLocale = locales.find(l => l.is_default)?.code ?? DEFAULT_LOCALE;

  try {
    const saved = localStorage.getItem('preferred_locale');
    if (saved && codes.includes(saved)) return saved;
  } catch { /* localStorage unavailable */ }

  const lang = navigator.language?.slice(0, 2).toLowerCase();
  if (lang && codes.includes(lang)) return lang;
  return defaultLocale;
}
