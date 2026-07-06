import { useEffect, useRef } from 'react';

interface Props {
  title: string;
  message: string;
  confirmLabel?: string;
  cancelLabel?: string;
  danger?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}

export default function ConfirmModal({
  title,
  message,
  confirmLabel = 'Confirm',
  cancelLabel = 'Cancel',
  danger = false,
  onConfirm,
  onCancel,
}: Props) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const cancelRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;
    // showModal() gives us focus trapping, Escape-to-close and ::backdrop for free.
    dialog.showModal();
    cancelRef.current?.focus();
    return () => dialog.close();
  }, []);

  // Native Escape fires `cancel`; prevent the default close so React owns unmounting.
  function handleCancel(e: React.SyntheticEvent<HTMLDialogElement>) {
    e.preventDefault();
    onCancel();
  }

  // A click whose target is the dialog element itself is a click on the backdrop.
  function handleClick(e: React.MouseEvent<HTMLDialogElement>) {
    if (e.target === dialogRef.current) onCancel();
  }

  return (
    <dialog
      ref={dialogRef}
      className="confirm-modal"
      aria-labelledby="confirm-modal-title"
      onCancel={handleCancel}
      onClick={handleClick}
    >
      <h3
        id="confirm-modal-title"
        className={danger ? 'confirm-modal-title-danger' : ''}
      >
        {title}
      </h3>
      <p className="confirm-modal-message">{message}</p>
      <div className="confirm-modal-actions">
        <button ref={cancelRef} className="btn btn-secondary" onClick={onCancel}>
          {cancelLabel}
        </button>
        <button
          className={`btn ${danger ? 'btn-danger' : 'btn-primary'}`}
          onClick={onConfirm}
        >
          {confirmLabel}
        </button>
      </div>
    </dialog>
  );
}
