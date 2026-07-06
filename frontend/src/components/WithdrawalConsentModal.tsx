import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

interface Props {
  onConfirm: () => void;
  onCancel: () => void;
}

export default function WithdrawalConsentModal({ onConfirm, onCancel }: Props) {
  const { t } = useTranslation();
  const [agreed, setAgreed] = useState(false);
  const dialogRef = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;
    // showModal() gives focus trapping and Escape-to-close for free.
    // Deliberately no backdrop-click dismissal — this is a legal waiver.
    dialog.showModal();
    return () => dialog.close();
  }, []);

  // Native Escape fires `cancel`; prevent the default close so React owns unmounting.
  function handleCancel(e: React.SyntheticEvent<HTMLDialogElement>) {
    e.preventDefault();
    onCancel();
  }

  return (
    <dialog
      ref={dialogRef}
      className="consent-modal"
      aria-labelledby="consent-modal-title"
      aria-describedby="consent-modal-desc"
      onCancel={handleCancel}
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
    </dialog>
  );
}
