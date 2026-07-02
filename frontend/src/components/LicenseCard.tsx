import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { createDownloadToken, type License } from '../api/client';
import { useSiteConfig } from '../context/SiteConfigContext';
import { useLocale } from '../context/LocaleContext';
import { formatPrice } from '../utils/format';
import { isSafeDownloadURL } from '../utils/safeUrl';
import DeviceList from './DeviceList';

interface Props {
  license: License;
  pricePaidCents?: number;
  onUpdate: () => void;
}

export default function LicenseCard({ license, pricePaidCents, onUpdate }: Props) {
  const [downloading, setDownloading] = useState(false);
  const [downloadError, setDownloadError] = useState('');
  const { max_activations, currency_symbol } = useSiteConfig();
  const { locale } = useLocale();
  const { t } = useTranslation();

  async function handleDownload() {
    setDownloading(true);
    setDownloadError('');
    try {
      const { url } = await createDownloadToken(license.id);
      if (!isSafeDownloadURL(url)) {
        throw new Error('unexpected download URL');
      }
      window.location.href = url;
    } catch {
      setDownloadError(t('license.download_error'));
    } finally {
      setDownloading(false);
    }
  }

  return (
    <div className="license-card">
      <div className="license-header">
        <h3>{license.app_name || license.app_id}</h3>
        <code className="license-key">{license.key}</code>
      </div>

      <p className="license-meta">
        {t('license.purchased', { date: new Date(license.created_at).toLocaleDateString(locale) })}
        {pricePaidCents !== undefined && (
          <> &middot; {formatPrice(pricePaidCents, currency_symbol)}</>
        )}
      </p>

      <div className="license-actions">
        <button
          className="btn btn-secondary btn-small"
          onClick={handleDownload}
          disabled={downloading}
        >
          {downloading ? t('license.downloading') : t('license.download')}
        </button>
      </div>

      {downloadError && <p className="error license-download-error">{downloadError}</p>}

      <DeviceList
        activations={license.activations}
        maxDevices={license.max_activations ?? max_activations}
        onUpdate={onUpdate}
      />
    </div>
  );
}
