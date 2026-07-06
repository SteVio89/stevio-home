import { useEffect } from 'react';
import { Outlet, useParams } from 'react-router-dom';
import { useSiteConfig } from './SiteConfigContext';
import { setClientLocale } from '../api/client';
import i18next from 'i18next';
import { DEFAULT_LOCALE, loadDBOverrides } from '../i18n/index';
import { LocaleContext } from './LocaleContext';

/**
 * LocaleProvider is used as a route layout element for /:locale/* routes.
 * It reads :locale from the URL, validates it, syncs the API client locale,
 * and updates the document lang attribute.
 */
export function LocaleProvider() {
  const { locale: rawLocale } = useParams<{ locale: string }>();
  const { locales } = useSiteConfig();
  const codes = locales.map(l => l.code);
  const defaultCode = locales.find(l => l.is_default)?.code ?? DEFAULT_LOCALE;
  const locale = codes.includes(rawLocale ?? '') ? rawLocale! : defaultCode;

  // Set client locale synchronously so child components' API calls use the
  // correct Accept-Language header from the very first render/effect.
  setClientLocale(locale);

  useEffect(() => {
    document.documentElement.lang = locale;
    if (i18next.language !== locale) {
      i18next.changeLanguage(locale);
    }
    loadDBOverrides(locale);
  }, [locale]);

  return (
    <LocaleContext.Provider value={{ locale }}>
      <Outlet />
    </LocaleContext.Provider>
  );
}
