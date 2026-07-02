import { Navigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useAuth } from '../context/AuthContext';
import { useLocale } from '../context/LocaleContext';

export default function RequireAuth({ children }: { children: React.ReactNode }) {
  const { loading, loggedIn } = useAuth();
  const { locale } = useLocale();
  const { t } = useTranslation();

  if (loading) return <div className="page"><p className="loading-text">{t('auth.loading')}</p></div>;
  if (!loggedIn) return <Navigate to={`/${locale}/login`} replace />;

  return <>{children}</>;
}
