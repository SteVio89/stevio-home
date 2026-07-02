import { Navigate, useLocation } from 'react-router-dom';
import { useSiteConfig } from '../context/SiteConfigContext';
import { detectBrowserLocale } from '../i18n/useT';

/**
 * Redirects bare paths (e.g., /, /project/foo) to their locale-prefixed equivalents.
 * Auto-detects locale from browser language on first visit.
 */
export default function LocaleRedirect() {
  const { pathname, search, hash } = useLocation();
  const { locales } = useSiteConfig();
  const locale = detectBrowserLocale(locales);
  const target = `/${locale}${pathname === '/' ? '/' : pathname}${search}${hash}`;
  return <Navigate to={target} replace />;
}
