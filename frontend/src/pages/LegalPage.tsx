import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import PageHeader from '../components/PageHeader';
import { useDocumentHead } from '../hooks/useDocumentHead';
import { getLegalPage } from '../api/client';
import { useLocale } from '../context/LocaleContext';

interface Props {
  type: 'impressum' | 'privacy' | 'refund' | 'terms';
}

const legalConfig = {
  impressum: { slug: 'impressum', titleKey: 'legal.impressum_title' },
  privacy: { slug: 'privacy', titleKey: 'legal.privacy_title' },
  refund: { slug: 'refund-policy', titleKey: 'legal.refund_title' },
  terms: { slug: 'terms', titleKey: 'legal.terms_title' },
} as const;

export default function LegalPage({ type }: Props) {
  const { locale } = useLocale();
  const { t } = useTranslation();
  const { slug, titleKey } = legalConfig[type];

  useDocumentHead({ title: t(titleKey), noindex: true });
  const [html, setHtml] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    getLegalPage(slug)
      .then((res) => setHtml(res.html))
      .catch(() => setHtml(`<p>${t('legal.error')}</p>`))
      .finally(() => setLoading(false));
  }, [slug, t]);

  return (
    <div className="page">
      <PageHeader
        title={t(titleKey)}
        backTo={`/${locale}/`}
        backLabel={t('legal.back')}
      />
      {loading ? (
        <p className="loading-text">{t('legal.loading')}</p>
      ) : (
        <div
          className="legal-content"
          dangerouslySetInnerHTML={{ __html: html }}
        />
      )}
    </div>
  );
}
