export interface MetricBarProps {
  label: string;
  /** 0-100. `undefined` renders "Not reported" rather than a fabricated value. */
  value: number | undefined;
  warnAt?: number;
  criticalAt?: number;
}

export function MetricBar({ label, value, warnAt = 75, criticalAt = 90 }: MetricBarProps) {
  if (value === undefined || Number.isNaN(value)) {
    return (
      <div className="ox-metric">
        <span className="ox-muted">{label}</span>
        <span className="ox-muted">Not reported</span>
        <span />
      </div>
    );
  }
  const v = Math.max(0, Math.min(100, value));
  const tone = v >= criticalAt ? ' ox-metric__fill--critical' : v >= warnAt ? ' ox-metric__fill--warning' : '';
  return (
    <div className="ox-metric">
      <span className="ox-muted">{label}</span>
      <div
        className="ox-metric__track"
        role="progressbar"
        aria-label={label}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={Math.round(v)}
      >
        <div className={`ox-metric__fill${tone}`} style={{ width: `${v}%` }} />
      </div>
      <span className="ox-metric__value">{Math.round(v)}%</span>
    </div>
  );
}
