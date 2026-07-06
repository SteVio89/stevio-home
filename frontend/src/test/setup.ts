import '@testing-library/jest-dom/vitest';
import i18next from 'i18next';
import { initReactI18next } from 'react-i18next';
import de from '../i18n/locales/de.json';

// jsdom (through v28) doesn't implement the HTMLDialogElement modal methods.
// Minimal shim so components using native <dialog>.showModal() can be rendered
// and their close/cancel wiring exercised in tests.
if (!HTMLDialogElement.prototype.showModal) {
  HTMLDialogElement.prototype.showModal = function (this: HTMLDialogElement) {
    this.open = true;
  };
  HTMLDialogElement.prototype.close = function (this: HTMLDialogElement) {
    this.open = false;
    this.dispatchEvent(new Event('close'));
  };
}

// Initialize i18next for tests with bundled German translations (default locale).
// This avoids HTTP backend calls and allows tests to assert on real translated strings.
i18next.use(initReactI18next).init({
  lng: 'de',
  fallbackLng: 'de',
  resources: { de: { translation: de } },
  interpolation: { escapeValue: false },
  react: { useSuspense: false },
});
