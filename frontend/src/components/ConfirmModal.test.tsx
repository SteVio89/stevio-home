import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { axe } from 'vitest-axe';
// @ts-expect-error vitest-axe/matchers uses export type * but the runtime value exists
import { toHaveNoViolations } from 'vitest-axe/matchers';
import ConfirmModal from './ConfirmModal';

// @ts-expect-error see above
expect.extend({ toHaveNoViolations });

const defaultProps = {
  title: 'Test bestätigen',
  message: 'Bist du sicher?',
  onConfirm: vi.fn(),
  onCancel: vi.fn(),
};

describe('ConfirmModal', () => {
  it('has correct ARIA structure — role=dialog on inner div, not backdrop', () => {
    const { container } = render(<ConfirmModal {...defaultProps} />);

    // Inner dialog div must have role="dialog"
    const dialog = container.querySelector('[role="dialog"]');
    expect(dialog).not.toBeNull();
    expect(dialog).toHaveClass('confirm-modal');

    // Backdrop must NOT have role="dialog"
    const backdrop = container.querySelector('.confirm-modal-backdrop');
    expect(backdrop).not.toBeNull();
    expect(backdrop).not.toHaveAttribute('role', 'dialog');
    // Backdrop must NOT have aria-hidden (it wraps the dialog — aria-hidden would hide dialog from AT)
    expect(backdrop).not.toHaveAttribute('aria-hidden');
  });

  it('has aria-modal and aria-labelledby on dialog', () => {
    const { container } = render(<ConfirmModal {...defaultProps} />);
    const dialog = container.querySelector('[role="dialog"]');
    expect(dialog).toHaveAttribute('aria-modal', 'true');
    expect(dialog).toHaveAttribute('aria-labelledby', 'confirm-modal-title');
  });

  it('has id on h3 matching aria-labelledby', () => {
    render(<ConfirmModal {...defaultProps} />);
    const heading = document.getElementById('confirm-modal-title');
    expect(heading).not.toBeNull();
    expect(heading?.tagName).toBe('H3');
    expect(heading).toHaveTextContent('Test bestätigen');
  });

  it('dialog is discoverable via role query', () => {
    render(<ConfirmModal {...defaultProps} />);
    // screen.getByRole('dialog') must work — proves role is on correct element
    expect(screen.getByRole('dialog')).toBeInTheDocument();
  });

  it('calls onCancel when Escape pressed', async () => {
    const onCancel = vi.fn();
    const user = userEvent.setup();
    render(<ConfirmModal {...defaultProps} onCancel={onCancel} />);
    await user.keyboard('{Escape}');
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it('calls onCancel when backdrop clicked', async () => {
    const onCancel = vi.fn();
    const user = userEvent.setup();
    const { container } = render(<ConfirmModal {...defaultProps} onCancel={onCancel} />);
    const backdrop = container.querySelector('.confirm-modal-backdrop') as HTMLElement;
    await user.click(backdrop);
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it('does not call onCancel when inner modal clicked', async () => {
    const onCancel = vi.fn();
    const user = userEvent.setup();
    render(<ConfirmModal {...defaultProps} onCancel={onCancel} />);
    const dialog = screen.getByRole('dialog');
    await user.click(dialog);
    expect(onCancel).not.toHaveBeenCalled();
  });

  it('calls onConfirm when confirm button clicked', async () => {
    const onConfirm = vi.fn();
    const user = userEvent.setup();
    render(<ConfirmModal {...defaultProps} onConfirm={onConfirm} confirmLabel="Ja" />);
    await user.click(screen.getByText('Ja'));
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it('has no axe-core accessibility violations', async () => {
    const { container } = render(<ConfirmModal {...defaultProps} />);
    const results = await axe(container);
    // @ts-expect-error vitest-axe matcher type
    expect(results).toHaveNoViolations();
  });

  it('applies danger styles to title and confirm button when danger=true', () => {
    const { container } = render(<ConfirmModal {...defaultProps} danger confirmLabel="Löschen" />);
    const heading = document.getElementById('confirm-modal-title');
    expect(heading).toHaveClass('confirm-modal-title-danger');
    const confirmBtn = container.querySelector('.btn-danger');
    expect(confirmBtn).not.toBeNull();
  });
});
