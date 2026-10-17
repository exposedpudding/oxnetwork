import type { ReactNode } from 'react';
import { EmptyState } from './States';

export interface Column<T> {
  key: string;
  header: string;
  render: (row: T) => ReactNode;
  align?: 'left' | 'right';
}

export interface DataTableProps<T> {
  caption: string;
  columns: Column<T>[];
  rows: T[];
  rowKey: (row: T) => string;
  selectedKey?: string;
  onRowSelect?: (row: T) => void;
  emptyTitle?: string;
  emptyDescription?: string;
}

/** Semantic table. Rows are keyboard-selectable (Enter/Space) when `onRowSelect` is given. */
export function DataTable<T>({
  caption,
  columns,
  rows,
  rowKey,
  selectedKey,
  onRowSelect,
  emptyTitle = 'Nothing to show',
  emptyDescription,
}: DataTableProps<T>) {
  if (rows.length === 0) {
    return <EmptyState title={emptyTitle} description={emptyDescription ?? `No ${caption.toLowerCase()} yet.`} />;
  }
  return (
    <div className="ox-table-wrap">
      <table className="ox-table">
        <caption className="ox-sr-only">{caption}</caption>
        <thead>
          <tr>
            {columns.map((c) => (
              <th key={c.key} scope="col" style={{ textAlign: c.align }}>
                {c.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => {
            const key = rowKey(row);
            const selectable = !!onRowSelect;
            return (
              <tr
                key={key}
                data-selectable={selectable}
                aria-selected={selectable ? key === selectedKey : undefined}
                tabIndex={selectable ? 0 : undefined}
                onClick={selectable ? () => onRowSelect(row) : undefined}
                onKeyDown={
                  selectable
                    ? (e) => {
                        if (e.key === 'Enter' || e.key === ' ') {
                          e.preventDefault();
                          onRowSelect(row);
                        }
                      }
                    : undefined
                }
              >
                {columns.map((c) => (
                  <td key={c.key} style={{ textAlign: c.align }}>
                    {c.render(row)}
                  </td>
                ))}
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
