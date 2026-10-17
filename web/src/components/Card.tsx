import type { ReactNode } from 'react';

export interface CardProps {
  title?: ReactNode;
  actions?: ReactNode;
  selected?: boolean;
  headingLevel?: 2 | 3 | 4;
  children?: ReactNode;
  className?: string;
}

export function Card({ title, actions, selected, headingLevel = 2, children, className }: CardProps) {
  const H = `h${headingLevel}` as 'h2' | 'h3' | 'h4';
  return (
    <section className={`ox-card${selected ? ' ox-card--selected' : ''}${className ? ` ${className}` : ''}`}>
      {(title || actions) && (
        <header className="ox-card__head">
          {title ? <H className="ox-card__title">{title}</H> : <span />}
          {actions}
        </header>
      )}
      {children}
    </section>
  );
}

export interface StatCardProps {
  label: string;
  value: ReactNode;
  detail?: ReactNode;
  icon?: ReactNode;
}

export function StatCard({ label, value, detail, icon }: StatCardProps) {
  return (
    <div className="ox-card ox-stat" role="group" aria-label={label}>
      <div className="ox-card__head" style={{ marginBottom: 0 }}>
        <span className="ox-stat__label">{label}</span>
        {icon}
      </div>
      <span className="ox-stat__value">{value}</span>
      {detail ? <span className="ox-muted">{detail}</span> : null}
    </div>
  );
}
