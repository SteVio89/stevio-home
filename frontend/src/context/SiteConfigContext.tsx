import { createContext, useContext, useEffect, useState } from 'react';
import { getPublicConfig, setMaintenanceListener } from '../api/client';
import type { LocaleInfo } from '../i18n/useT';

export interface SiteConfig {
  currency_symbol: string;
  currency_code: string;
  site_name: string;
  maintenance_mode: boolean;
  payment_enabled: boolean;
  payment_provider: string;
  max_activations: number;
  base_url: string;
  locales: LocaleInfo[];
}

const defaultLocales: LocaleInfo[] = [
  { code: 'de', name: 'Deutsch', is_default: true },
  { code: 'en', name: 'English', is_default: false },
];

const defaults: SiteConfig = {
  currency_symbol: '€',
  currency_code: 'EUR',
  site_name: 'My Store',
  maintenance_mode: false,
  payment_enabled: true,
  payment_provider: '',
  max_activations: 3,
  base_url: '',
  locales: defaultLocales,
};

export const SiteConfigContext = createContext<SiteConfig>(defaults);

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
          max_activations: raw.max_activations,
          base_url: raw.base_url,
          locales: Array.isArray(raw.locales) && raw.locales.length > 0
            ? raw.locales
            : defaultLocales,
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

export function useSiteConfig(): SiteConfig {
  return useContext(SiteConfigContext);
}
