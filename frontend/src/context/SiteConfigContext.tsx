import { createContext, useContext } from 'react';
import type { LocaleInfo } from '../i18n/useT';

export interface SiteConfig {
  currency_symbol: string;
  currency_code: string;
  site_name: string;
  maintenance_mode: boolean;
  payment_enabled: boolean;
  payment_provider: string;
  paddle_client_token: string;
  paddle_environment: string;
  max_activations: number;
  base_url: string;
  locales: LocaleInfo[];
}

const defaultLocales: LocaleInfo[] = [
  { code: 'de', name: 'Deutsch', is_default: true },
  { code: 'en', name: 'English', is_default: false },
];

export const defaults: SiteConfig = {
  currency_symbol: '€',
  currency_code: 'EUR',
  site_name: 'My Store',
  maintenance_mode: false,
  payment_enabled: true,
  payment_provider: '',
  paddle_client_token: '',
  paddle_environment: 'production',
  max_activations: 3,
  base_url: '',
  locales: defaultLocales,
};

export const SiteConfigContext = createContext<SiteConfig>(defaults);

export function useSiteConfig(): SiteConfig {
  return useContext(SiteConfigContext);
}
