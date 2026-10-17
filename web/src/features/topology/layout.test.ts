import { layoutGraph } from './layout';
import type { NetworkGraph } from '../../lib/api/types';

const n = (id: string) => ({ id, kind: 'node', label: id });
const e = (id: string, source: string, target: string) => ({ id, source, target, kind: 'link' });

const tree: NetworkGraph = {
  nodes: [n('root'), n('a'), n('b'), n('c'), n('leaf')],
  edges: [e('e1', 'root', 'a'), e('e2', 'root', 'b'), e('e3', 'a', 'c'), e('e4', 'b', 'c'), e('e5', 'c', 'leaf')],
};

describe('layoutGraph', () => {
  it('assigns longest-path layers', () => {
    const l = layoutGraph(tree);
    const layer = Object.fromEntries(l.nodes.map((p) => [p.node.id, p.layer]));
    expect(layer).toEqual({ root: 0, a: 1, b: 1, c: 2, leaf: 3 });
  });

  it('is stable regardless of input order', () => {
    const shuffled: NetworkGraph = { nodes: [...tree.nodes].reverse(), edges: [...tree.edges].reverse() };
    expect(layoutGraph(shuffled)).toEqual(layoutGraph(tree));
    expect(layoutGraph(tree)).toEqual(layoutGraph(structuredClone(tree)));
  });

  it('does not overlap nodes within a layer and stays inside bounds', () => {
    const l = layoutGraph(tree);
    for (const p of l.nodes) {
      expect(p.x).toBeGreaterThanOrEqual(0);
      expect(p.x + p.width).toBeLessThanOrEqual(l.width);
      expect(p.y + p.height).toBeLessThanOrEqual(l.height);
    }
    const row = l.nodes.filter((p) => p.layer === 1).sort((a, b) => a.x - b.x);
    expect(row[1].x - (row[0].x + row[0].width)).toBeGreaterThan(0);
  });

  it('survives cycles, self-loops, dangling edges and disconnected nodes', () => {
    const g: NetworkGraph = {
      nodes: [n('x'), n('y'), n('z'), n('solo')],
      edges: [e('1', 'x', 'y'), e('2', 'y', 'z'), e('3', 'z', 'x'), e('4', 'x', 'x'), e('5', 'x', 'ghost')],
    };
    const l = layoutGraph(g);
    expect(l.nodes).toHaveLength(4);
    expect(l.edges.map((r) => r.edge.id).sort()).toEqual(['1', '2', '3']);
    expect(l.nodes.find((p) => p.node.id === 'solo')!.layer).toBe(0);
    expect(l.nodes.every((p) => Number.isFinite(p.x) && Number.isFinite(p.y))).toBe(true);
  });

  it('handles an empty graph', () => {
    const l = layoutGraph({ nodes: [], edges: [] });
    expect(l.nodes).toEqual([]);
    expect(l.width).toBeGreaterThan(0);
  });

  it('routes edges with orthogonal segments only', () => {
    for (const r of layoutGraph(tree).edges) expect(r.path).toMatch(/^M[\d.]+,[\d.]+( [VH][\d.]+)+$/);
  });
});
