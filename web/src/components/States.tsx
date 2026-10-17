import type { ReactNode } from 'react';

export function EmptyState({ title, description, action }: { title: string; description?: string; action?: ReactNode }) {
  return (
    <div className="ox-state" role="status">
      <p className="ox-state__title">{title}</p>
      {description ? <p style={{ margin: 0 }}>{description}</p> : null}
      {action}
    </div>
  );
}

export interface ErrorStateProps {
  title?: string;
  /** What failed. Per FR error rules, also say whether data shown is stale. */
  message: string;
  stale?: boolean;
  requestId?: string;
  onRetry?: () => void;
}

export function ErrorState({ title = 'Something went wrong', message, stale, requestId, onRetry }: ErrorStateProps) {
  return (
    <div className="ox-state ox-state--error" role="alert">
      <p className="ox-state__title">{title}</p>
      <p style={{ margin: 0 }}>{message}</p>
      {stale ? <p style={{ margin: 0 }}>Showing last known data, which may be out of date.</p> : null}
      {requestId ? <p className="ox-mono" style={{ margin: 0 }}>Request {requestId}</p> : null}
      {onRetry ? (
        <button type="button" className="ox-btn" onClick={onRetry}>
          Retry
        </button>
      ) : null}
    </div>
  );
}
