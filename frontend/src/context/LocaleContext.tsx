import { createContext, useContext } from 'react';
import { DEFAULT_LOCALE } from '../i18n/index';

export interface LocaleContextValue {
  locale: string;
}

export const LocaleContext = createContext<LocaleContextValue>({ locale: DEFAULT_LOCALE });

export function useLocale(): LocaleContextValue {
  return useContext(LocaleContext);
}
