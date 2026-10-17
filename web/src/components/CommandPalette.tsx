import { useEffect, useId, useMemo, useRef, useState } from 'react';
import { useFocusTrap } from './useFocusTrap';

export interface PaletteCommand {
  id: string;
  label: string;
  hint?: string;
  /** Dangerous commands are styled red; the host app must still route them through a confirmation dialog. */
  danger?: boolean;
  keywords?: string[];
}

export interface CommandPaletteProps {
  open: boolean;
  commands: PaletteCommand[];
  onSelect: (command: PaletteCommand) => void;
  onClose: () => void;
}

/** Ctrl/Cmd+K toggles the palette. Call from the shell. */
export function useCommandPaletteShortcut(toggle: () => void) {
  useEffect(() => {
    const h = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        toggle();
      }
    };
    window.addEventListener('keydown', h);
    return () => window.removeEventListener('keydown', h);
  }, [toggle]);
}

/** Deterministic command filter - no AI dependency. */
export function filterCommands(commands: PaletteCommand[], query: string): PaletteCommand[] {
  const terms = query.toLowerCase().split(/\s+/).filter(Boolean);
  if (terms.length === 0) return commands;
  return commands.filter((c) => {
    const hay = [c.label, c.hint ?? '', ...(c.keywords ?? [])].join(' ').toLowerCase();
    return terms.every((t) => hay.includes(t));
  });
}

export function CommandPalette({ open, ...rest }: CommandPaletteProps) {
  return open ? <Palette {...rest} /> : null;
}

function Palette({ commands, onSelect, onClose }: Omit<CommandPaletteProps, 'open'>) {
  const [query, setQuery] = useState('');
  const [active, setActive] = useState(0);
  const ref = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const listId = useId();
  useFocusTrap(ref, true, inputRef);
  const results = useMemo(() => filterCommands(commands, query), [commands, query]);
  const idx = Math.min(active, Math.max(results.length - 1, 0));

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Escape') onClose();
    else if (e.key === 'ArrowDown') (e.preventDefault(), setActive(Math.min(idx + 1, results.length - 1)));
    else if (e.key === 'ArrowUp') (e.preventDefault(), setActive(Math.max(idx - 1, 0)));
    else if (e.key === 'Enter' && results[idx]) (e.preventDefault(), onSelect(results[idx]));
  };

  return (
    <div className="ox-overlay" onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
      <div ref={ref} className="ox-palette" role="dialog" aria-modal="true" aria-label="Command palette" onKeyDown={onKeyDown}>
        <input
          ref={inputRef}
          className="ox-input ox-palette__input"
          role="combobox"
          aria-expanded="true"
          aria-controls={listId}
          aria-activedescendant={results[idx] ? `${listId}-${results[idx].id}` : undefined}
          aria-label="Search commands"
          placeholder="Type a command or search…"
          value={query}
          onChange={(e) => (setQuery(e.target.value), setActive(0))}
        />
        <ul id={listId} role="listbox" className="ox-palette__list" aria-label="Commands">
          {results.length === 0 ? (
            <li className="ox-muted" style={{ padding: 12 }} role="presentation">
              No matching commands
            </li>
          ) : (
            results.map((c, i) => (
              <li
                key={c.id}
                id={`${listId}-${c.id}`}
                role="option"
                aria-selected={i === idx}
                className={`ox-palette__item${c.danger ? ' ox-palette__item--danger' : ''}`}
                onMouseEnter={() => setActive(i)}
                onClick={() => onSelect(c)}
              >
                <span>{c.label}</span>
                {c.hint ? <span className="ox-muted">{c.hint}</span> : null}
              </li>
            ))
          )}
        </ul>
      </div>
    </div>
  );
}
