import { useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useAuth } from '../context/AuthContext';
import { useSiteConfig } from '../context/SiteConfigContext';
import { localeFromPath } from '../i18n/useT';
import LanguageSwitcher from './LanguageSwitcher';

export default function Nav() {
  const { loading, loggedIn, email, isAdmin, logout } = useAuth();
  const { site_name, locales } = useSiteConfig();
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const { t } = useTranslation();

  const [theme, setTheme] = useState<'light' | 'dark'>(() => {
    return (document.documentElement.getAttribute('data-theme') as 'light' | 'dark') || 'light';
  });

  function toggleTheme() {
    const next = theme === 'light' ? 'dark' : 'light';
    setTheme(next);
    document.documentElement.setAttribute('data-theme', next);
    localStorage.setItem('theme', next);
  }

  const localeCodes = locales.map(l => l.code);
  const locale = localeFromPath(pathname, localeCodes);
  const isLocaleRoute = new RegExp(`^\\/(${localeCodes.join('|')})(\\\/|$)`).test(pathname);

  function switchLocale(newCode: string) {
    localStorage.setItem('preferred_locale', newCode);
    const rest = pathname.replace(new RegExp(`^\\/(${localeCodes.join('|')})`), '');
    navigate(`/${newCode}${rest || '/'}`, { replace: true });
  }

  return (
    <nav className="site-nav">
      <div className="site-nav-inner">
        <Link to={isLocaleRoute ? `/${locale}/` : '/'} className="site-nav-brand">
          <img
            src="/stevio-logo.svg"
            alt=""
            className="site-nav-logo"
          />
          <span className="site-nav-wordmark">{site_name}</span>
        </Link>
        {!loading && (
          <div className="site-nav-links">
            <button
              className="site-nav-theme-toggle"
              onClick={toggleTheme}
              aria-label={theme === 'light' ? t('landing.theme_dark_label') : t('landing.theme_light_label')}
              type="button"
            >
              {theme === 'light' ? '\u263E' : '\u2600'}
            </button>
            {isLocaleRoute && (
              <LanguageSwitcher
                locales={locales}
                currentCode={locale}
                onSwitch={switchLocale}
              />
            )}
            {loggedIn ? (
              <>
                {email && <span className="site-nav-email">{email}</span>}
                {isAdmin && <Link to="/admin">{t('nav.admin')}</Link>}
                {!isAdmin && <Link to={`/${locale}/chat`}>{t('chat.nav')}</Link>}
                <Link to={`/${locale}/account`}>{t('nav.account')}</Link>
                <button className="site-nav-logout" onClick={logout}>{t('nav.logout')}</button>
              </>
            ) : (
              <Link to={isLocaleRoute ? `/${locale}/login` : '/login'}>{t('nav.login')}</Link>
            )}
          </div>
        )}
      </div>
    </nav>
  );
}
