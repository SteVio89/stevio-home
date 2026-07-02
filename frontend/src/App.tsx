import { BrowserRouter, Routes, Route, Link, Navigate, useLocation } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { AuthProvider } from './context/AuthContext';
import { useSiteConfig, SiteConfigProvider } from './context/SiteConfigContext';
import { ToastProvider } from './context/ToastContext';
import { LocaleProvider, useLocale } from './context/LocaleContext';
import { useAuth } from './context/AuthContext';
import { localeFromPath } from './i18n/useT';
import { DEFAULT_LOCALE } from './i18n';
import ToastContainer from './components/ToastContainer';
import Nav from './components/Nav';
import RequireAuth from './components/RequireAuth';
import RequireAdmin from './components/RequireAdmin';
import AdminLayout from './components/AdminLayout';
import Landing from './pages/Landing';
import ProjectDetail from './pages/ProjectDetail';
import Success from './pages/Success';
import MockCheckout from './pages/MockCheckout';
import Login from './pages/Login';
import Account from './pages/Account';
import LegalPage from './pages/LegalPage';
import MaintenanceWall from './pages/MaintenanceWall';
import LocaleRedirect from './pages/LocaleRedirect';
import VerifyToken from './pages/VerifyToken';
import AdminDashboard from './pages/admin/AdminDashboard';
import AdminVersions from './pages/admin/AdminVersions';
import AdminProjectImages from './pages/admin/AdminProjectImages';
import AdminUsers from './pages/admin/AdminUsers';
import AdminSales from './pages/admin/AdminSales';
import AdminDiscountCodes from './pages/admin/AdminDiscountCodes';
import AdminSettings from './pages/admin/AdminSettings';
import AdminPayment from './pages/admin/AdminPayment';
import AdminLegalPages from './pages/admin/AdminLegalPages';
import AdminLanguages from './pages/admin/AdminLanguages';
import AdminUITranslations from './pages/admin/AdminUITranslations';
import AdminMailTemplates from './pages/admin/AdminMailTemplates';
import AdminOrders from './pages/admin/AdminOrders';
import AdminLicenses from './pages/admin/AdminLicenses';
import AdminChat from './pages/admin/AdminChat';
import AdminChatDetail from './pages/admin/AdminChatDetail';
import AdminSigningKeys from './pages/admin/AdminSigningKeys';
import AdminHero from './pages/admin/AdminHero';
import AdminProjects from './pages/admin/AdminProjects';
import AdminProjectForm from './pages/admin/AdminProjectForm';
import AdminSocialLinks from './pages/admin/AdminSocialLinks';
import AdminSocialLinkForm from './pages/admin/AdminSocialLinkForm';
import Chat from './pages/Chat';

function NotFound() {
  const { locale } = useLocale();
  const { t } = useTranslation();
  return (
    <div className="not-found">
      <h1>{t('not_found.title')}</h1>
      <p>{t('not_found.message')}</p>
      <Link to={`/${locale}/`} className="btn btn-primary">{t('not_found.back')}</Link>
    </div>
  );
}

function AppShell() {
  const { pathname } = useLocation();
  const isAdmin = pathname.startsWith('/admin');
  const { maintenance_mode, site_name, locales } = useSiteConfig();
  const { isAdmin: userIsAdmin } = useAuth();
  const defaultLocale = locales.find(l => l.is_default)?.code ?? DEFAULT_LOCALE;

  // Show maintenance wall immediately when maintenance mode is detected.
  // Admins bypass: once auth resolves and userIsAdmin is true, the wall drops.
  // Legal pages stay accessible (German/EU law requires it).
  if (maintenance_mode && !userIsAdmin) {
    return (
      <>
        <Nav />
        <main className="site-main">
          <div className="container">
            <Routes>
              <Route path="/:locale" element={<LocaleProvider />}>
                <Route index element={<MaintenanceWall />} />
                <Route path="impressum" element={<LegalPage type="impressum" />} />
                <Route path="imprint" element={<LegalPage type="impressum" />} />
                <Route path="datenschutz" element={<LegalPage type="privacy" />} />
                <Route path="privacy" element={<LegalPage type="privacy" />} />
                <Route path="widerruf" element={<LegalPage type="refund" />} />
                <Route path="refund-policy" element={<LegalPage type="refund" />} />
                <Route path="login" element={<Login />} />
                <Route path="auth/verify" element={<VerifyToken />} />
                <Route path="*" element={<MaintenanceWall />} />
              </Route>
              {/* Legacy redirects */}
              <Route path="/impressum" element={<Navigate to={`/${defaultLocale}/impressum`} replace />} />
              <Route path="/datenschutz" element={<Navigate to={`/${defaultLocale}/datenschutz`} replace />} />
              <Route path="/auth/verify" element={<LocaleRedirect />} />
              <Route path="*" element={<LocaleRedirect />} />
            </Routes>
          </div>
        </main>
        <SiteFooterFallback shopName={site_name} />
      </>
    );
  }

  // Admin area: full-width layout (AdminLayout provides its own sidebar + content grid)
  if (isAdmin) {
    return (
      <>
        <Nav />
        <main className="site-main">
          <Routes>
            <Route element={<RequireAdmin><AdminLayout /></RequireAdmin>}>
              <Route path="/admin" element={<AdminDashboard />} />
              <Route path="/admin/users" element={<AdminUsers />} />
              <Route path="/admin/sales" element={<AdminSales />} />
              <Route path="/admin/orders" element={<AdminOrders />} />
              <Route path="/admin/licenses" element={<AdminLicenses />} />
              <Route path="/admin/discount-codes" element={<AdminDiscountCodes />} />
              <Route path="/admin/settings" element={<AdminSettings />} />
              <Route path="/admin/payment" element={<AdminPayment />} />
              <Route path="/admin/legal-pages" element={<AdminLegalPages />} />
              <Route path="/admin/languages" element={<AdminLanguages />} />
              <Route path="/admin/ui-translations" element={<AdminUITranslations />} />
              <Route path="/admin/mail-templates" element={<AdminMailTemplates />} />
              <Route path="/admin/signing-keys" element={<AdminSigningKeys />} />
              <Route path="/admin/chats" element={<AdminChat />} />
              <Route path="/admin/chats/:id" element={<AdminChatDetail />} />
              <Route path="/admin/hero" element={<AdminHero />} />
              <Route path="/admin/projects" element={<AdminProjects />} />
              <Route path="/admin/projects/new" element={<AdminProjectForm />} />
              <Route path="/admin/projects/:id/edit" element={<AdminProjectForm />} />
              <Route path="/admin/projects/:id/images" element={<AdminProjectImages />} />
              <Route path="/admin/projects/:id/versions" element={<AdminVersions />} />
              <Route path="/admin/social-links" element={<AdminSocialLinks />} />
              <Route path="/admin/social-links/new" element={<AdminSocialLinkForm />} />
              <Route path="/admin/social-links/:id/edit" element={<AdminSocialLinkForm />} />
            </Route>
          </Routes>
        </main>
      </>
    );
  }

  return (
    <>
      <Nav />
      <main className="site-main">
        <div className="container">
          <Routes>
            {/* Locale-prefixed public routes */}
            <Route path="/:locale" element={<LocaleProvider />}>
              <Route index element={<Landing />} />
              <Route path="project/:slug" element={<ProjectDetail />} />
              <Route path="success" element={<Success />} />
              <Route path="mock-checkout" element={<MockCheckout />} />
              <Route path="login" element={<Login />} />
              <Route path="auth/verify" element={<VerifyToken />} />
              <Route path="account" element={<RequireAuth><Account /></RequireAuth>} />
              <Route path="chat" element={<RequireAuth><Chat /></RequireAuth>} />
              <Route path="impressum" element={<LegalPage type="impressum" />} />
              <Route path="imprint" element={<LegalPage type="impressum" />} />
              <Route path="datenschutz" element={<LegalPage type="privacy" />} />
              <Route path="privacy" element={<LegalPage type="privacy" />} />
              <Route path="widerruf" element={<LegalPage type="refund" />} />
              <Route path="refund-policy" element={<LegalPage type="refund" />} />
              <Route path="*" element={<NotFound />} />
            </Route>

            {/* Legacy redirects for old paths */}
            <Route path="/impressum" element={<Navigate to={`/${defaultLocale}/impressum`} replace />} />
            <Route path="/datenschutz" element={<Navigate to={`/${defaultLocale}/datenschutz`} replace />} />
            <Route path="/auth/verify" element={<LocaleRedirect />} />
            <Route path="/login" element={<LocaleRedirect />} />
            <Route path="/account" element={<LocaleRedirect />} />
            <Route path="/success" element={<LocaleRedirect />} />
            <Route path="/mock-checkout" element={<LocaleRedirect />} />

            {/* Root: auto-detect locale and redirect */}
            <Route path="/" element={<LocaleRedirect />} />
            <Route path="*" element={<LocaleRedirect />} />
          </Routes>
        </div>
      </main>
      <SiteFooterFallback shopName={site_name} />
    </>
  );
}

/**
 * SiteFooterFallback is used outside LocaleProvider (maintenance wall, main layout).
 * It wraps SiteFooter with a minimal LocaleProvider that reads locale from the URL.
 */
function SiteFooterFallback({ shopName }: { shopName: string }) {
  const { pathname } = useLocation();
  const { locales } = useSiteConfig();
  const { t } = useTranslation();
  const localeCodes = locales.map(l => l.code);
  const locale = localeFromPath(pathname, localeCodes);

  return (
    <footer className="site-footer">
      <div className="site-footer-inner">
        <Link to={`/${locale}/impressum`}>{t('footer.impressum')}</Link>
        <Link to={`/${locale}/datenschutz`}>{t('footer.privacy')}</Link>
        <Link to={`/${locale}/widerruf`}>{t('footer.refund_policy')}</Link>
        <Link to={`/${locale}/`} aria-label={`${shopName} home`}>
          <img src="/stevio-logo.svg" alt="" className="site-footer-logo" />
        </Link>
      </div>
      <div className="site-footer-copy">&copy; {new Date().getFullYear()} {shopName}</div>
    </footer>
  );
}

export default function App() {
  return (
    <BrowserRouter>
      <SiteConfigProvider>
        <ToastProvider>
          <AuthProvider>
            <AppShell />
          </AuthProvider>
          <ToastContainer />
        </ToastProvider>
      </SiteConfigProvider>
    </BrowserRouter>
  );
}
