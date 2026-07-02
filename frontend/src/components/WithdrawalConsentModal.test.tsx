import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
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

describe('WithdrawalConsentModal', () => {
  it('has correct ARIA structure — role=dialog on inner div, not backdrop', () => {
    const { container } = render(<WithdrawalConsentModal {...defaultProps} />);

    const dialog = container.querySelector('[role="dialog"]');
    expect(dialog).not.toBeNull();
    expect(dialog).toHaveClass('consent-modal');

    const backdrop = container.querySelector('.consent-modal-backdrop');
    expect(backdrop).not.toBeNull();
    expect(backdrop).not.toHaveAttribute('role');
    // Backdrop must NOT have aria-hidden (it wraps the dialog — aria-hidden would hide dialog from AT)
    expect(backdrop).not.toHaveAttribute('aria-hidden');
  });

  it('has aria-modal, aria-labelledby, and aria-describedby on dialog', () => {
    const { container } = render(<WithdrawalConsentModal {...defaultProps} />);
    const dialog = container.querySelector('[role="dialog"]');
    expect(dialog).toHaveAttribute('aria-modal', 'true');
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
    const checkbox = screen.getByRole('checkbox');
    await user.click(checkbox);
    expect(confirmBtn).not.toBeDisabled();
  });

  it('checkbox has associated label (wrapping label element)', () => {
    render(<WithdrawalConsentModal {...defaultProps} />);
    const checkbox = screen.getByRole('checkbox');
    const label = checkbox.closest('label');
    expect(label).not.toBeNull();
  });

  it('calls onCancel when Escape pressed', async () => {
    const onCancel = vi.fn();
    const user = userEvent.setup();
    render(<WithdrawalConsentModal {...defaultProps} onCancel={onCancel} />);
    await user.keyboard('{Escape}');
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

  it('has no axe-core accessibility violations', async () => {
    const { container } = render(<WithdrawalConsentModal {...defaultProps} />);
    const results = await axe(container);
    // @ts-expect-error vitest-axe matcher type
    expect(results).toHaveNoViolations();
  });

  it('cancel button calls onCancel', async () => {
    const onCancel = vi.fn();
    const user = userEvent.setup();
    render(<WithdrawalConsentModal {...defaultProps} onCancel={onCancel} />);
    await user.click(screen.getByText('Abbrechen'));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });
});
