import type { GraphEdge, GraphNode, NetworkGraph } from '../../lib/api/types';

export interface LayoutOptions {
  nodeWidth?: number;
  nodeHeight?: number;
  hGap?: number;
  vGap?: number;
  padding?: number;
}

export interface PositionedNode {
  node: GraphNode;
  layer: number;
  x: number; // top-left
  y: number;
  width: number;
  height: number;
}

export interface RoutedEdge {
  edge: GraphEdge;
  /** SVG path data using orthogonal segments. */
  path: string;
}

export interface GraphLayout {
  nodes: PositionedNode[];
  edges: RoutedEdge[];
  width: number;
  height: number;
}

const DEFAULTS = { nodeWidth: 176, nodeHeight: 56, hGap: 32, vGap: 64, padding: 24 };

const byId = <T extends { id: string }>(a: T, b: T) => (a.id < b.id ? -1 : a.id > b.id ? 1 : 0);

/**
 * Deterministic layered layout. No physics, no randomness: the output depends only on the set of
 * nodes/edges (input order is irrelevant - everything is sorted by id), so unchanged graphs always
 * get identical coordinates.
 *
 * 1. Edges referencing unknown nodes and self-loops are ignored for layout.
 * 2. Cycles are broken by dropping DFS back-edges (roots visited in id order).
 * 3. Layer = longest path from any root. Unconnected nodes sit on layer 0.
 * 4. Within a layer, nodes are ordered by barycenter of neighbours (a few sweeps, ties by id).
 * 5. Layers are centred horizontally.
 */
export function layoutGraph(graph: NetworkGraph, options: LayoutOptions = {}): GraphLayout {
  const o = { ...DEFAULTS, ...options };
  const nodes = [...graph.nodes].sort(byId);
  const ids = new Set(nodes.map((n) => n.id));
  const edges = [...graph.edges].sort(byId);
  const valid = edges.filter((e) => ids.has(e.source) && ids.has(e.target) && e.source !== e.target);

  const out = new Map<string, string[]>();
  const indeg = new Map<string, number>();
  for (const n of nodes) (out.set(n.id, []), indeg.set(n.id, 0));
  for (const e of valid) {
    out.get(e.source)!.push(e.target);
    indeg.set(e.target, indeg.get(e.target)! + 1);
  }

  // Break cycles: iterative-free recursive DFS, roots (indegree 0) first, then remaining by id.
  const state = new Map<string, 0 | 1 | 2>();
  const dag = new Map<string, string[]>(nodes.map((n) => [n.id, []]));
  const visit = (id: string) => {
    state.set(id, 1);
    for (const t of [...out.get(id)!].sort()) {
      const s = state.get(t) ?? 0;
      if (s === 1) continue; // back-edge
      dag.get(id)!.push(t);
      if (s === 0) visit(t);
    }
    state.set(id, 2);
  };
  for (const n of nodes) if (indeg.get(n.id) === 0 && !state.has(n.id)) visit(n.id);
  for (const n of nodes) if (!state.has(n.id)) visit(n.id);

  // Longest-path layering over the DAG (Kahn order).
  const layer = new Map<string, number>(nodes.map((n) => [n.id, 0]));
  const dIn = new Map<string, number>(nodes.map((n) => [n.id, 0]));
  for (const [, ts] of dag) for (const t of ts) dIn.set(t, dIn.get(t)! + 1);
  const queue = nodes.filter((n) => dIn.get(n.id) === 0).map((n) => n.id);
  for (let i = 0; i < queue.length; i++) {
    const id = queue[i];
    for (const t of dag.get(id)!) {
      layer.set(t, Math.max(layer.get(t)!, layer.get(id)! + 1));
      dIn.set(t, dIn.get(t)! - 1);
      if (dIn.get(t) === 0) queue.push(t);
    }
  }

  const maxLayer = nodes.reduce((m, n) => Math.max(m, layer.get(n.id)!), 0);
  const layers: string[][] = Array.from({ length: maxLayer + 1 }, () => []);
  for (const n of nodes) layers[layer.get(n.id)!].push(n.id);

  const neighbours = new Map<string, string[]>(nodes.map((n) => [n.id, []]));
  for (const e of valid) (neighbours.get(e.source)!.push(e.target), neighbours.get(e.target)!.push(e.source));

  const pos = new Map<string, number>();
  const reindex = () => layers.forEach((l) => l.forEach((id, i) => pos.set(id, i)));
  reindex();
  for (let sweep = 0; sweep < 4; sweep++) {
    const order = sweep % 2 === 0 ? layers.map((_, i) => i) : layers.map((_, i) => layers.length - 1 - i);
    for (const li of order) {
      const bary = (id: string) => {
        const ns = neighbours.get(id)!.filter((n) => layer.get(n) !== li);
        return ns.length ? ns.reduce((s, n) => s + pos.get(n)!, 0) / ns.length : pos.get(id)!;
      };
      layers[li].sort((a, b) => bary(a) - bary(b) || (a < b ? -1 : 1));
      reindex();
    }
  }

  const widest = Math.max(1, ...layers.map((l) => l.length));
  const innerWidth = widest * o.nodeWidth + (widest - 1) * o.hGap;
  const width = innerWidth + o.padding * 2;
  const height = nodes.length === 0 ? o.padding * 2 : layers.length * o.nodeHeight + (layers.length - 1) * o.vGap + o.padding * 2;

  const placed = new Map<string, PositionedNode>();
  layers.forEach((ids, li) => {
    const rowWidth = ids.length * o.nodeWidth + (ids.length - 1) * o.hGap;
    const x0 = o.padding + (innerWidth - rowWidth) / 2;
    ids.forEach((id, i) => {
      placed.set(id, {
        node: nodes.find((n) => n.id === id)!,
        layer: li,
        x: x0 + i * (o.nodeWidth + o.hGap),
        y: o.padding + li * (o.nodeHeight + o.vGap),
        width: o.nodeWidth,
        height: o.nodeHeight,
      });
    });
  });

  const routed: RoutedEdge[] = valid.map((edge) => {
    const a = placed.get(edge.source)!;
    const b = placed.get(edge.target)!;
    return { edge, path: routeEdge(a, b, o.vGap) };
  });

  return { nodes: [...placed.values()].sort((a, b) => byId(a.node, b.node)), edges: routed, width, height };
}

function routeEdge(a: PositionedNode, b: PositionedNode, vGap: number): string {
  const ax = a.x + a.width / 2;
  const bx = b.x + b.width / 2;
  if (b.layer > a.layer) {
    // down: bottom of source -> top of target with a horizontal run at mid-gap
    const y1 = a.y + a.height;
    const y2 = b.y;
    const mid = y1 + vGap / 2;
    return `M${ax},${y1} V${mid} H${bx} V${y2}`;
  }
  if (b.layer < a.layer) {
    const y1 = a.y;
    const y2 = b.y + b.height;
    const mid = y1 - vGap / 2;
    return `M${ax},${y1} V${mid} H${bx} V${y2}`;
  }
  // same layer: route over the top of both nodes
  const top = a.y - vGap / 3;
  return `M${ax},${a.y} V${top} H${bx} V${b.y}`;
}
