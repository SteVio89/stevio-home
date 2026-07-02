import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

interface Props {
  onConfirm: () => void;
  onCancel: () => void;
}

export default function WithdrawalConsentModal({ onConfirm, onCancel }: Props) {
  const { t } = useTranslation();
  const [agreed, setAgreed] = useState(false);
  const modalRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const modal = modalRef.current;
    if (!modal) return;
    modal.focus();

    function handleTab(e: KeyboardEvent) {
      if (e.key !== 'Tab') return;
      const focusable = modal!.querySelectorAll<HTMLElement>(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
      );
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    }

    function handleEsc(e: KeyboardEvent) {
      if (e.key === 'Escape') onCancel();
    }

    document.addEventListener('keydown', handleTab);
    document.addEventListener('keydown', handleEsc);
    return () => {
      document.removeEventListener('keydown', handleTab);
      document.removeEventListener('keydown', handleEsc);
    };
  }, [onCancel]);

  return (
    <div className="consent-modal-backdrop">
      <div
        ref={modalRef}
        className="consent-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="consent-modal-title"
        aria-describedby="consent-modal-desc"
        tabIndex={-1}
      >
        <h2 id="consent-modal-title">{t('consent.title')}</h2>
        <p id="consent-modal-desc" className="consent-modal-text">
          {t('consent.description')}
        </p>
        <label className="consent-modal-checkbox">
          <input
            type="checkbox"
            checked={agreed}
            onChange={(e) => setAgreed(e.target.checked)}
          />
          <span>{t('consent.waiver_text')}</span>
        </label>
        <div className="consent-modal-actions">
          <button className="btn btn-secondary" onClick={onCancel}>
            {t('consent.cancel')}
          </button>
          <button
            className="btn btn-primary"
            onClick={onConfirm}
            disabled={!agreed}
          >
            {t('consent.confirm')}
          </button>
        </div>
      </div>
    </div>
  );
}
