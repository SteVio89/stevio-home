import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
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

function getDialog(container: HTMLElement): HTMLDialogElement {
  const dialog = container.querySelector('dialog.confirm-modal');
  if (!dialog) throw new Error('dialog not found');
  return dialog as HTMLDialogElement;
}

describe('ConfirmModal', () => {
  it('renders a native <dialog> labelled by its title', () => {
    const { container } = render(<ConfirmModal {...defaultProps} />);
    const dialog = getDialog(container);
    expect(dialog).toHaveAttribute('aria-labelledby', 'confirm-modal-title');
  });

  it('has id on h3 matching aria-labelledby', () => {
    render(<ConfirmModal {...defaultProps} />);
    const heading = document.getElementById('confirm-modal-title');
    expect(heading?.tagName).toBe('H3');
    expect(heading).toHaveTextContent('Test bestätigen');
  });

  it('dialog is discoverable via role query', () => {
    render(<ConfirmModal {...defaultProps} />);
    expect(screen.getByRole('dialog')).toBeInTheDocument();
  });

  it('calls onCancel on the native cancel event (Escape)', () => {
    const onCancel = vi.fn();
    const { container } = render(<ConfirmModal {...defaultProps} onCancel={onCancel} />);
    fireEvent(getDialog(container), new Event('cancel', { cancelable: true }));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it('calls onCancel when the backdrop (dialog element itself) is clicked', async () => {
    const onCancel = vi.fn();
    const user = userEvent.setup();
    const { container } = render(<ConfirmModal {...defaultProps} onCancel={onCancel} />);
    await user.click(getDialog(container));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it('does not call onCancel when dialog content is clicked', async () => {
    const onCancel = vi.fn();
    const user = userEvent.setup();
    render(<ConfirmModal {...defaultProps} onCancel={onCancel} />);
    await user.click(screen.getByText('Bist du sicher?'));
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
