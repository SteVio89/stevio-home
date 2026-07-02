import { useEffect, useState, useRef, useCallback } from 'react';
import { useSearchParams, Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { verifyCheckout, type CheckoutVerification } from '../api/client';
import PageHeader from '../components/PageHeader';
import { useDocumentHead } from '../hooks/useDocumentHead';
import { useLocale } from '../context/LocaleContext';
import { useSiteConfig } from '../context/SiteConfigContext';

export default function Success() {
  const { locale } = useLocale();
  const { t } = useTranslation();
  const { payment_provider } = useSiteConfig();
  useDocumentHead({ title: t('success.title'), noindex: true });
  const [params] = useSearchParams();
  const sessionId = params.get('session_id');
  const [result, setResult] = useState<CheckoutVerification | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [copied, setCopied] = useState(false);
  const [refunding, setRefunding] = useState(false);
  const [refunded, setRefunded] = useState(false);
  const isMockDev = payment_provider === 'mock' && import.meta.env.DEV;

  async function handleMockRefund() {
    if (!sessionId) return;
    setRefunding(true);
    try {
      const res = await fetch(
        `/api/checkout/mock/trigger?action=refund&session_id=${encodeURIComponent(sessionId)}`,
      );
      if (!res.ok) throw new Error(`status ${res.status}`);
      setRefunded(true);
    } catch (err) {
      setError(`Mock refund failed: ${err instanceof Error ? err.message : 'unknown'}`);
    } finally {
      setRefunding(false);
    }
  }
  const attemptsRef = useRef(0);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const tryVerify = useCallback(async () => {
    if (!sessionId) return;
    try {
      const data = await verifyCheckout(sessionId);
      setResult(data);
      setLoading(false);
    } catch {
      attemptsRef.current++;
      if (attemptsRef.current < 15) {
        timerRef.current = setTimeout(tryVerify, 2000);
      } else {
        setError(t('success.error'));
        setLoading(false);
      }
    }
  }, [sessionId, t]);

  useEffect(() => {
    if (!sessionId) {
      setLoading(false);
      return;
    }
    tryVerify();
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, [sessionId, tryVerify]);

  function handleCopy() {
    if (!result) return;
    navigator.clipboard.writeText(result.license_key).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  }

  if (!sessionId) {
    return (
      <div className="page">
        <PageHeader title={t('success.title')} />
        <p>{t('success.no_session')}</p>
        <Link to={`/${locale}/account`} className="btn btn-secondary">{t('success.go_to_account')}</Link>
      </div>
    );
  }

  return (
    <div className="page">
      <PageHeader title={t('success.title')} />

      <div className="success-card">
        {loading && (
          <div className="success-loading">
            <p className="loading-text">{t('success.verifying')}</p>
          </div>
        )}

        {error && (
          <div className="success-error">
            <p className="error">{error}</p>
            <Link to={`/${locale}/account`} className="btn btn-secondary">{t('success.check_account')}</Link>
          </div>
        )}

        {result && (
          <>
            <p>{t('success.thank_you', { app_name: result.app_name })}</p>

            <div className="license-key-display">
              <label>{t('success.license_key_label')}</label>
              <div className="license-key-row">
                <code className="license-key-value">{result.license_key}</code>
                <button
                  className="btn btn-secondary btn-small"
                  onClick={handleCopy}
                >
                  {copied ? t('success.copied') : t('success.copy')}
                </button>
              </div>
            </div>

            <h2>{t('success.next_steps')}</h2>
            <ol>
              <li>{t('success.step_1')}</li>
              <li>{t('success.step_2')}</li>
              <li>{t('success.step_3')}</li>
            </ol>

            <div className="actions">
              <Link to={`/${locale}/account`} className="btn btn-secondary">
                {t('success.manage_account')}
              </Link>
              <Link to={`/${locale}/`} className="btn btn-primary">
                {t('success.back_to_store')}
              </Link>
            </div>

            {isMockDev && (
              <div style={{ marginTop: '32px', padding: '12px', border: '1px dashed #999', borderRadius: '6px' }}>
                <p style={{ margin: 0, fontSize: '0.85em', color: '#666' }}>
                  <strong>[DEV]</strong> Trigger a mock refund webhook for this session — the license
                  above will be revoked via the same code path a real Paddle refund webhook hits.
                </p>
                <button
                  className="btn btn-secondary btn-small"
                  onClick={handleMockRefund}
                  disabled={refunding || refunded}
                  style={{ marginTop: '8px' }}
                >
                  {refunded ? 'Refunded' : refunding ? 'Refunding…' : 'Trigger Refund (dev)'}
                </button>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
