import type { ReactNode } from 'react';
import type { HealthState } from '../lib/api/types';

export type Status =
  | HealthState
  | 'stale'
  | 'success'
  | 'running'
  | 'queued'
  | 'failed'
  | 'cancelled'
  | 'online'
  | 'draining'
  | 'offline';

export const STATUS_LABEL: Record<Status, string> = {
  healthy: 'Healthy',
  degraded: 'Degraded',
  unhealthy: 'Unhealthy',
  unknown: 'Unknown',
  disabled: 'Disabled',
  stale: 'Stale',
  success: 'Success',
  running: 'Running',
  queued: 'Queued',
  failed: 'Failed',
  cancelled: 'Cancelled',
  online: 'Online',
  draining: 'Draining',
  offline: 'Offline',
};

/** Color is never the only indicator: the dot carries hidden text and hollow styles for inactive states. */
export function StatusDot({ status, label }: { status: Status; label?: string }) {
  return (
    <span className={`ox-dot ox-dot--${status}`} role="img" aria-label={label ?? STATUS_LABEL[status]} />
  );
}

export function StatusChip({ status, children }: { status: Status; children?: ReactNode }) {
  return (
    <span className={`ox-chip ox-chip--${status}`}>
      <span className={`ox-dot ox-dot--${status}`} aria-hidden="true" />
      {children ?? STATUS_LABEL[status]}
    </span>
  );
}

export function StaleBadge({ observedAt, title }: { observedAt?: string; title?: string }) {
  return (
    <span className="ox-chip ox-stale" title={title ?? (observedAt ? `Last observed ${observedAt}` : 'Data may be out of date')}>
      Stale{observedAt ? <time dateTime={observedAt} className="ox-sr-only"> since {observedAt}</time> : null}
    </span>
  );
}

export function HealthPill({ status, label }: { status: Status; label: string }) {
  return (
    <span className="ox-pill" role="status">
      <span className={`ox-dot ox-dot--${status}`} aria-hidden="true" />
      {label}
    </span>
  );
}
