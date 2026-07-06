import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { axe } from 'vitest-axe';
// @ts-expect-error vitest-axe/matchers uses export type * but the runtime value exists
import { toHaveNoViolations } from 'vitest-axe/matchers';
import WithdrawalConsentModal from './WithdrawalConsentModal';

// @ts-expect-error see above
expect.extend({ toHaveNoViolations });

const defaultProps = {
  onConfirm: vi.fn(),
  onCancel: vi.fn(),
};

function getDialog(container: HTMLElement): HTMLDialogElement {
  const dialog = container.querySelector('dialog.consent-modal');
  if (!dialog) throw new Error('dialog not found');
  return dialog as HTMLDialogElement;
}

describe('WithdrawalConsentModal', () => {
  it('renders a native <dialog> labelled and described by its content', () => {
    const { container } = render(<WithdrawalConsentModal {...defaultProps} />);
    const dialog = getDialog(container);
    expect(dialog).toHaveAttribute('aria-labelledby', 'consent-modal-title');
    expect(dialog).toHaveAttribute('aria-describedby', 'consent-modal-desc');
  });

  it('dialog is discoverable via role query', () => {
    render(<WithdrawalConsentModal {...defaultProps} />);
    expect(screen.getByRole('dialog')).toBeInTheDocument();
  });

  it('shows heading and description text (German translations)', () => {
    render(<WithdrawalConsentModal {...defaultProps} />);
    expect(screen.getByText('Zustimmung erforderlich')).toBeInTheDocument();
    expect(screen.getByText('Bevor Sie fortfahren, bestätigen Sie bitte Folgendes:')).toBeInTheDocument();
  });

  it('confirm button is disabled until checkbox checked', async () => {
    const user = userEvent.setup();
    render(<WithdrawalConsentModal {...defaultProps} />);
    const confirmBtn = screen.getByText('Weiter zur Zahlung');
    expect(confirmBtn).toBeDisabled();
    await user.click(screen.getByRole('checkbox'));
    expect(confirmBtn).not.toBeDisabled();
  });

  it('checkbox has associated label (wrapping label element)', () => {
    render(<WithdrawalConsentModal {...defaultProps} />);
    const checkbox = screen.getByRole('checkbox');
    expect(checkbox.closest('label')).not.toBeNull();
  });

  it('calls onCancel on the native cancel event (Escape)', () => {
    const onCancel = vi.fn();
    const { container } = render(<WithdrawalConsentModal {...defaultProps} onCancel={onCancel} />);
    fireEvent(getDialog(container), new Event('cancel', { cancelable: true }));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it('calls onConfirm after checkbox checked and confirm clicked', async () => {
    const onConfirm = vi.fn();
    const user = userEvent.setup();
    render(<WithdrawalConsentModal {...defaultProps} onConfirm={onConfirm} />);
    await user.click(screen.getByRole('checkbox'));
    await user.click(screen.getByText('Weiter zur Zahlung'));
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it('cancel button calls onCancel', async () => {
    const onCancel = vi.fn();
    const user = userEvent.setup();
    render(<WithdrawalConsentModal {...defaultProps} onCancel={onCancel} />);
    await user.click(screen.getByText('Abbrechen'));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it('has no axe-core accessibility violations', async () => {
    const { container } = render(<WithdrawalConsentModal {...defaultProps} />);
    const results = await axe(container);
    // @ts-expect-error vitest-axe matcher type
    expect(results).toHaveNoViolations();
  });
});
