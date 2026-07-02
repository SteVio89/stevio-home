import { useEffect, useRef, useState } from 'react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { verifyToken } from '../api/client';
import { useAuth } from '../context/AuthContext';
import { useLocale } from '../context/LocaleContext';

export default function VerifyToken() {
  const { locale } = useLocale();
  const { t } = useTranslation();
  const { refreshAuth } = useAuth();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const [error, setError] = useState('');
  const attempted = useRef(false);

  // Run-once-on-mount: token verification must fire exactly once per page load
  // (the token is single-use; re-running would always 401). The `attempted`
  // ref guards against React 18 strict-mode double invocation.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => {
    if (attempted.current) return;
    attempted.current = true;

    const token = searchParams.get('token');
    if (!token) {
      navigate(`/${locale}/login?error=token_invalid`, { replace: true });
      return;
    }

    verifyToken(token)
      .then(() => refreshAuth())
      .then(() => {
        navigate(`/${locale}/account`, { replace: true });
      })
      .catch(() => {
        setError(t('login.error_token_invalid'));
      });
  }, []);

  if (error) {
    return (
      <div className="page">
        <p className="error">{error}</p>
        <a href={`/${locale}/login`} className="btn btn-primary">{t('login.title')}</a>
      </div>
    );
  }

  return (
    <div className="page">
      <p>{t('login.verifying')}</p>
    </div>
  );
}
