import { useState, type ReactNode } from 'react';
import { ChevronsLeft, ChevronsRight, Search as SearchIcon } from 'lucide-react';

export interface NavItemProps {
  label: string;
  icon?: ReactNode;
  href?: string;
  current?: boolean;
  onClick?: () => void;
}

export function NavItem({ label, icon, href, current, onClick }: NavItemProps) {
  const content = (
    <>
      <span aria-hidden="true" style={{ display: 'inline-flex' }}>{icon}</span>
      <span className="ox-navitem__label">{label}</span>
    </>
  );
  const aria = { 'aria-current': current ? ('page' as const) : undefined, 'aria-label': label };
  return href ? (
    <a className="ox-navitem" href={href} onClick={onClick} {...aria}>{content}</a>
  ) : (
    <button type="button" className="ox-navitem" onClick={onClick} {...aria}>{content}</button>
  );
}

export interface SidebarProps {
  items: NavItemProps[];
  brand?: ReactNode;
  collapsed: boolean;
  onToggleCollapsed: () => void;
}

export function Sidebar({ items, brand, collapsed, onToggleCollapsed }: SidebarProps) {
  return (
    <aside className="ox-sidebar">
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div className="ox-brand">{brand ?? <>oxnet<b>.work</b></>}</div>
        <button
          type="button"
          className="ox-btn"
          aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          aria-expanded={!collapsed}
          onClick={onToggleCollapsed}
          style={{ padding: 6 }}
        >
          {collapsed ? <ChevronsRight size={16} aria-hidden="true" /> : <ChevronsLeft size={16} aria-hidden="true" />}
        </button>
      </div>
      <nav className="ox-nav" aria-label="Primary">
        {items.map((i) => <NavItem key={i.label} {...i} />)}
      </nav>
    </aside>
  );
}

export interface TopbarProps {
  /** Opens the command palette. The field is a button, not a text input, so there is one search surface. */
  onOpenSearch: () => void;
  searchPlaceholder?: string;
  health?: ReactNode;
  trailing?: ReactNode;
  leading?: ReactNode;
}

export function Topbar({ onOpenSearch, searchPlaceholder = 'Search or run a command…', health, trailing, leading }: TopbarProps) {
  const isMac = typeof navigator !== 'undefined' && /mac/i.test(navigator.platform);
  return (
    <header className="ox-topbar">
      {leading}
      <button type="button" className="ox-search" onClick={onOpenSearch} aria-label="Open command palette" aria-keyshortcuts="Control+K Meta+K">
        <SearchIcon size={16} aria-hidden="true" />
        <span>{searchPlaceholder}</span>
        <kbd>{isMac ? '⌘ K' : 'Ctrl K'}</kbd>
      </button>
      {health}
      {trailing}
    </header>
  );
}

export interface AppShellProps {
  navItems: NavItemProps[];
  topbar: Omit<TopbarProps, 'leading'> & { leading?: ReactNode };
  children: ReactNode;
}

export function AppShell({ navItems, topbar, children }: AppShellProps) {
  const [collapsed, setCollapsed] = useState(false);
  return (
    <div className={`ox-shell${collapsed ? ' ox-shell--collapsed' : ''}`}>
      <Sidebar items={navItems} collapsed={collapsed} onToggleCollapsed={() => setCollapsed((c) => !c)} />
      <div className="ox-main">
        <Topbar {...topbar} />
        <main className="ox-content">{children}</main>
      </div>
    </div>
  );
}
