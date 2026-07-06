import { useEffect, useState } from 'react';
import { getPublicConfig, setMaintenanceListener } from '../api/client';
import { SiteConfigContext, defaults, type SiteConfig } from './SiteConfigContext';

export function SiteConfigProvider({ children }: { children: React.ReactNode }) {
  const [config, setConfig] = useState<SiteConfig>(defaults);

  // Force maintenance mode on when any API call returns a 503 maintenance response.
  useEffect(() => {
    setMaintenanceListener(() => {
      setConfig(prev => ({ ...prev, maintenance_mode: true }));
    });
    return () => setMaintenanceListener(null);
  }, []);

  useEffect(() => {
    getPublicConfig()
      .then((raw) =>
        setConfig({
          currency_symbol: raw.currency_symbol,
          currency_code: raw.currency_code,
          site_name: raw.site_name,
          maintenance_mode: raw.maintenance_mode,
          payment_enabled: raw.payment_enabled,
          payment_provider: raw.payment_provider ?? '',
          paddle_client_token: raw.paddle_client_token ?? '',
          paddle_environment: raw.paddle_environment ?? 'production',
          max_activations: raw.max_activations,
          base_url: raw.base_url,
          locales: Array.isArray(raw.locales) && raw.locales.length > 0
            ? raw.locales
            : defaults.locales,
        })
      )
      .catch(() => {/* keep defaults on error */});
  }, []);

  return (
    <SiteConfigContext.Provider value={config}>
      {children}
    </SiteConfigContext.Provider>
  );
}
