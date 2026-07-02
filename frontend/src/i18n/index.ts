import i18next from 'i18next';
import { initReactI18next } from 'react-i18next';
import de from './locales/de.json';
import en from './locales/en.json';

// Single source of truth for the locale fallback. Used wherever code needs to
// pick a locale before the dynamic locales/* response arrives. Changing the
// site default means changing this and seeding the matching locales row.
export const DEFAULT_LOCALE = 'de';

const bundled: Record<string, Record<string, string>> = { de, en };

i18next
  .use(initReactI18next)
  .init({
    fallbackLng: DEFAULT_LOCALE,
    resources: { de: { translation: de }, en: { translation: en } },
    interpolation: { escapeValue: false },
    react: { useSuspense: false },
  });

/**
 * Load DB overrides for a locale and merge them on top of bundled translations.
 * Called by LocaleProvider on locale change and once on init.
 */
export async function loadDBOverrides(locale: string): Promise<void> {
  try {
    const res = await fetch(`/api/i18n/${locale}`);
    if (!res.ok) return;
    const overrides: Record<string, string> = await res.json();
    if (Object.keys(overrides).length === 0) return;
    // Start from bundled defaults (if any), then overlay DB overrides.
    const base = bundled[locale] ?? {};
    const merged = { ...base, ...overrides };
    i18next.addResourceBundle(locale, 'translation', merged, true, true);
  } catch {
    // API unavailable — keep bundled translations as-is.
  }
}

// Load overrides for the initial language.
loadDBOverrides(i18next.language);

export default i18next;
