import { useCallback, useState, type ReactNode } from 'react';
import { Activity, Boxes, Cpu, LayoutDashboard, Network, Play, Shapes } from 'lucide-react';
import { AppShell, CommandPalette, HealthPill, useCommandPaletteShortcut, type PaletteCommand } from '../components';
import { nodes } from '../fixtures';

export const DEV_ROUTES = [
  { path: '/dev/overview', label: 'Overview', icon: <LayoutDashboard size={18} /> },
  { path: '/dev/compute', label: 'Compute', icon: <Cpu size={18} /> },
  { path: '/dev/network', label: 'Network', icon: <Network size={18} /> },
  { path: '/dev/automations', label: 'Automations', icon: <Play size={18} /> },
  { path: `/dev/node/${nodes[0].id}`, label: 'Node detail', icon: <Boxes size={18} /> },
  { path: '/dev/components', label: 'Components', icon: <Shapes size={18} /> },
];

/** Dev-only harness shell: fixture-backed pages used for visual smoke tests. Not the production app shell. */
export function DevLayout({ path, title, children }: { path: string; title: string; children: ReactNode }) {
  const [paletteOpen, setPaletteOpen] = useState(false);
  const toggle = useCallback(() => setPaletteOpen((o) => !o), []);
  useCommandPaletteShortcut(toggle);
  const commands: PaletteCommand[] = [
    ...DEV_ROUTES.map((r) => ({ id: `go:${r.path}`, label: `Go to ${r.label}`, keywords: ['navigate'] })),
    { id: 'show-unhealthy', label: 'Show unhealthy services', keywords: ['filter'] },
  ];
  return (
    <AppShell
      navItems={DEV_ROUTES.map((r) => ({
        label: r.label,
        icon: r.icon,
        href: `#${r.path}`,
        current: path === r.path || (r.path.startsWith('/dev/node/') && path.startsWith('/dev/node/')),
      }))}
      topbar={{
        onOpenSearch: toggle,
        health: <HealthPill status="degraded" label="1 service degraded" />,
        trailing: <Activity size={18} aria-label="Dev harness" role="img" />,
      }}
    >
      <h1 className="ox-page-title">{title}</h1>
      {children}
      <CommandPalette
        open={paletteOpen}
        commands={commands}
        onClose={() => setPaletteOpen(false)}
        onSelect={(c) => {
          setPaletteOpen(false);
          if (c.id.startsWith('go:')) window.location.hash = c.id.slice(3);
        }}
      />
    </AppShell>
  );
}
