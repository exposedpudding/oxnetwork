import { useId, useRef, useState, type ReactNode } from 'react';
import { useFocusTrap } from './useFocusTrap';

export interface ConfirmDangerDialogProps {
  open: boolean;
  title: string;
  description: ReactNode;
  confirmLabel: string;
  /** When set, the user must type this exact text to enable confirmation (e.g. the target's name). */
  requireText?: string;
  /** `danger` (default) uses red styling at the point of action; `neutral` is for routine confirmations. */
  tone?: 'danger' | 'neutral';
  busy?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}

/** Modal: traps focus, closes on Escape, returns focus to the opener. Starts focus on Cancel. */
export function ConfirmDangerDialog(props: ConfirmDangerDialogProps) {
  if (!props.open) return null;
  return <Dialog {...props} />;
}

function Dialog({
  title,
  description,
  confirmLabel,
  requireText,
  tone = 'danger',
  busy,
  onConfirm,
  onCancel,
}: ConfirmDangerDialogProps) {
  const ref = useRef<HTMLDivElement>(null);
  const cancelRef = useRef<HTMLButtonElement>(null);
  const [typed, setTyped] = useState('');
  const titleId = useId();
  const descId = useId();
  const inputId = useId();
  useFocusTrap(ref, true, cancelRef);
  const allowed = !busy && (!requireText || typed === requireText);

  return (
    <div className="ox-overlay" onMouseDown={(e) => e.target === e.currentTarget && onCancel()}>
      <div
        ref={ref}
        role="alertdialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={descId}
        className={`ox-dialog${tone === 'danger' ? ' ox-dialog--danger' : ''}`}
        onKeyDown={(e) => e.key === 'Escape' && (e.stopPropagation(), onCancel())}
      >
        <h2 id={titleId}>{title}</h2>
        <div id={descId} className="ox-muted">
          {description}
        </div>
        {requireText ? (
          <div className="ox-field" style={{ marginTop: 'var(--ox-space-4)' }}>
            <label htmlFor={inputId}>
              Type <span className="ox-mono">{requireText}</span> to confirm
            </label>
            <input id={inputId} className="ox-input" value={typed} onChange={(e) => setTyped(e.target.value)} autoComplete="off" />
          </div>
        ) : null}
        <div className="ox-dialog__actions">
          <button ref={cancelRef} type="button" className="ox-btn" onClick={onCancel}>
            Cancel
          </button>
          <button
            type="button"
            className={`ox-btn ${tone === 'danger' ? 'ox-btn--danger' : 'ox-btn--primary'}`}
            disabled={!allowed}
            onClick={onConfirm}
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
