import { Link, Navigate } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import { useSiteConfig } from '../context/SiteConfigContext';
import { detectBrowserLocale } from '../i18n/useT';

export default function RequireAdmin({ children }: { children: React.ReactNode }) {
  const { loading, loggedIn, isAdmin } = useAuth();
  const { locales } = useSiteConfig();
  const locale = detectBrowserLocale(locales);

  if (loading) return <div className="page"><p className="loading-text">Loading...</p></div>;
  if (!loggedIn) return <Navigate to={`/${locale}/login`} replace />;
  if (!isAdmin) {
    return (
      <div className="page">
        <h1>Access Denied</h1>
        <p>You are not authorized to view this page.</p>
        <Link to={`/${locale}/`} className="btn btn-secondary">Back to Store</Link>
      </div>
    );
  }

  return <>{children}</>;
}
