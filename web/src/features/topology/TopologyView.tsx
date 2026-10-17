import { useMemo, useState } from 'react';
import { Maximize2, ZoomIn, ZoomOut } from 'lucide-react';
import type { GraphNode, NetworkGraph } from '../../lib/api/types';
import { freshnessOf } from '../../lib/api/freshness';
import { StatusDot, type Status } from '../../components/Status';
import { layoutGraph, type LayoutOptions } from './layout';

export interface TopologyViewProps {
  graph: NetworkGraph;
  selectedId?: string;
  onSelect?: (id: string) => void;
  /** Maps a node `kind` to a display label; falls back to the raw kind. */
  kindLabels?: Record<string, string>;
  layoutOptions?: LayoutOptions;
  /** Injectable for deterministic freshness in tests. */
  now?: number;
}

const ZOOM_STEPS = [0.5, 0.75, 1, 1.25, 1.5, 2];

function nodeStatus(n: GraphNode, now?: number): Status {
  if (!n.health) return 'unknown';
  if (freshnessOf(n.health, now) === 'stale') return 'stale';
  return n.health.state;
}

/**
 * SVG topology. `zoom === null` is "fit" (scales to container width); otherwise it is a
 * multiple of natural size inside a scrollable container. A textual list of the same graph
 * is always rendered below the map as the accessible fallback.
 */
export function TopologyView({ graph, selectedId, onSelect, kindLabels = {}, layoutOptions, now }: TopologyViewProps) {
  const layout = useMemo(() => layoutGraph(graph, layoutOptions), [graph, layoutOptions]);
  const [zoom, setZoom] = useState<number | null>(null);
  const kind = (k: string) => kindLabels[k] ?? k;

  const step = (dir: 1 | -1) => {
    const cur = zoom ?? 1;
    const i = ZOOM_STEPS.findIndex((z) => z >= cur - 1e-9);
    const next = ZOOM_STEPS[Math.min(Math.max((i < 0 ? ZOOM_STEPS.length - 1 : i) + dir, 0), ZOOM_STEPS.length - 1)];
    setZoom(next);
  };

  const selectedEdges = new Set(
    layout.edges.filter((e) => e.edge.source === selectedId || e.edge.target === selectedId).map((e) => e.edge.id),
  );

  return (
    <div className="ox-topo">
      <div className="ox-topo__bar" role="toolbar" aria-label="Topology zoom">
        <button type="button" className="ox-btn" aria-label="Zoom out" onClick={() => step(-1)} style={{ padding: 6 }}>
          <ZoomOut size={16} aria-hidden="true" />
        </button>
        <span aria-live="polite" className="ox-muted" style={{ minWidth: 48, textAlign: 'center' }}>
          {zoom === null ? 'Fit' : `${Math.round(zoom * 100)}%`}
        </span>
        <button type="button" className="ox-btn" aria-label="Zoom in" onClick={() => step(1)} style={{ padding: 6 }}>
          <ZoomIn size={16} aria-hidden="true" />
        </button>
        <button type="button" className="ox-btn" aria-label="Zoom to fit" onClick={() => setZoom(null)} style={{ padding: 6 }}>
          <Maximize2 size={16} aria-hidden="true" />
        </button>
        <button type="button" className="ox-btn" onClick={() => setZoom(1)}>
          Reset
        </button>
      </div>

      <div className="ox-topo__scroll" tabIndex={0} aria-label="Network topology map, scrollable">
        {graph.nodes.length === 0 ? (
          <p className="ox-muted" style={{ padding: 24, margin: 0 }}>No nodes in the network graph.</p>
        ) : (
          <svg
            role="group"
            aria-label="Network topology"
            viewBox={`0 0 ${layout.width} ${layout.height}`}
            width={zoom === null ? '100%' : layout.width * zoom}
            height={zoom === null ? undefined : layout.height * zoom}
            style={zoom === null ? { maxHeight: 560 } : undefined}
          >
            <g aria-hidden="true">
              {layout.edges.map((e) => (
                <path key={e.edge.id} d={e.path} className={`ox-topo__edge${selectedEdges.has(e.edge.id) ? ' ox-topo__edge--active' : ''}`} />
              ))}
            </g>
            {layout.nodes.map((p) => {
              const status = nodeStatus(p.node, now);
              const selected = p.node.id === selectedId;
              return (
                <g
                  key={p.node.id}
                  className="ox-topo__node"
                  transform={`translate(${p.x},${p.y})`}
                  role="button"
                  tabIndex={0}
                  aria-pressed={selected}
                  aria-label={`${p.node.label}, ${kind(p.node.kind)}, ${status}`}
                  data-node-id={p.node.id}
                  onClick={() => onSelect?.(p.node.id)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      onSelect?.(p.node.id);
                    }
                  }}
                >
                  <rect width={p.width} height={p.height} rx={10} />
                  <text className="ox-topo__label" x={14} y={25}>{truncate(p.node.label, 20)}</text>
                  <text className="ox-topo__kind" x={14} y={43}>{truncate(kind(p.node.kind), 24)}</text>
                  <circle className={`ox-dot--${status}`} cx={p.width - 16} cy={16} r={5} style={dotStyle(status)} />
                </g>
              );
            })}
          </svg>
        )}
      </div>

      <TopologyList graph={graph} selectedId={selectedId} onSelect={onSelect} kindLabels={kindLabels} now={now} />
    </div>
  );
}

// SVG circles can't use the .ox-dot box styles; mirror them with token variables.
function dotStyle(status: Status): React.CSSProperties {
  const solid: Record<string, string> = {
    healthy: 'var(--ox-healthy)', success: 'var(--ox-healthy)', online: 'var(--ox-healthy)',
    degraded: 'var(--ox-warning)', draining: 'var(--ox-warning)', queued: 'var(--ox-warning)',
    unhealthy: 'var(--ox-critical)', failed: 'var(--ox-critical)', offline: 'var(--ox-critical)',
    running: 'var(--ox-info)',
  };
  if (solid[status]) return { fill: solid[status], stroke: 'none' };
  // hollow ring for stale/unknown/disabled so state is not colour-only
  return { fill: 'none', stroke: status === 'stale' ? 'var(--ox-warning)' : 'var(--ox-text-muted)', strokeWidth: 2 };
}

function truncate(s: string, n: number) {
  return s.length > n ? `${s.slice(0, n - 1)}…` : s;
}

export function TopologyList({ graph, selectedId, onSelect, kindLabels = {}, now }: Omit<TopologyViewProps, 'layoutOptions'>) {
  const label = new Map(graph.nodes.map((n) => [n.id, n.label]));
  const nodes = [...graph.nodes].sort((a, b) => a.label.localeCompare(b.label) || (a.id < b.id ? -1 : 1));
  return (
    <details open style={{ padding: 'var(--ox-space-3) var(--ox-space-4)' }}>
      <summary style={{ cursor: 'pointer' }}>Nodes and connections (text view)</summary>
      <ul style={{ listStyle: 'none', padding: 0, margin: 'var(--ox-space-3) 0 0', display: 'grid', gap: 8 }} aria-label="Network nodes">
        {nodes.map((n) => {
          const links = graph.edges
            .filter((e) => e.source === n.id || e.target === n.id)
            .map((e) => {
              const outgoing = e.source === n.id;
              const other = label.get(outgoing ? e.target : e.source) ?? (outgoing ? e.target : e.source);
              return `${outgoing ? 'to' : 'from'} ${other} (${e.kind})`;
            });
          const status = nodeStatus(n, now);
          return (
            <li key={n.id}>
              <button
                type="button"
                className="ox-btn"
                aria-pressed={n.id === selectedId}
                onClick={() => onSelect?.(n.id)}
                style={{ width: '100%', justifyContent: 'space-between', borderColor: n.id === selectedId ? 'var(--ox-primary)' : undefined }}
              >
                <span style={{ display: 'inline-flex', gap: 8, alignItems: 'center' }}>
                  <StatusDot status={status} />
                  {n.label}
                  <span className="ox-muted">{kindLabels[n.kind] ?? n.kind}</span>
                </span>
                <span className="ox-muted" style={{ fontSize: 12 }}>{links.length ? links.join('; ') : 'no connections'}</span>
              </button>
            </li>
          );
        })}
      </ul>
    </details>
  );
}
