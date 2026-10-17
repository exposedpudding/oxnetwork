/**
 * Deterministic fixtures mirroring the domain shapes in lib/api/types.ts.
 * Values are synthetic (documentation address ranges, invented names) - not copied from mockups.
 * Everything is anchored to FIXTURE_NOW so output never depends on the wall clock.
 */
import type { Activity, ComputeSummary, Job, NetworkGraph, Node, NodeMetrics, Service, Worker } from '../lib/api/types';
import { freshnessOf } from '../lib/api/freshness';
import type { EditorOptions, Workflow } from '../features/workflows/types';

export const FIXTURE_NOW = Date.parse('2026-03-02T12:00:00Z');
const t = (secondsAgo: number) => new Date(FIXTURE_NOW - secondsAgo * 1000).toISOString();
const health = (state: Node['health']['state'], secondsAgo = 10) => ({
  state, observed_at: t(secondsAgo), stale_after_seconds: 60, source: 'fixture',
});

/** Mirrors the server's `freshness` field, derived from health age so fixtures stay self-consistent. */
const fresh = (h: Node['health']): NonNullable<Node['freshness']> => (freshnessOf(h, FIXTURE_NOW) === 'live' ? 'fresh' : 'stale');
const withFreshness = <T extends { health: Node['health'] }>(x: T) => ({ ...x, freshness: fresh(x.health), simulated: true });

const nodeSeed: Omit<Node, 'freshness' | 'simulated'>[] = [
  { id: 'nd-0001', name: 'forge', role: 'Primary compute', platform: 'Debian 12', health: health('healthy'), capabilities: ['compute', 'docker', 'ssh', 'monitor'], addresses: { lan: '192.0.2.10' }, metrics: { cpu_percent: 21, ram_percent: 47, vcpu: 12, ram_gb: 32 } },
  { id: 'nd-0002', name: 'anvil', role: 'Secondary compute', platform: 'Debian 12', health: health('healthy'), capabilities: ['compute', 'docker', 'ssh', 'wake'], addresses: { lan: '192.0.2.11' }, metrics: { cpu_percent: 9, ram_percent: 31, vcpu: 12, ram_gb: 32 } },
  { id: 'nd-0003', name: 'beacon', role: 'Edge gateway', platform: 'Ubuntu 24.04', health: health('degraded'), capabilities: ['ssh', 'monitor'], addresses: { public: '198.51.100.7' }, metrics: { cpu_percent: 78, ram_percent: 92, vcpu: 4, ram_gb: 8 } },
  // Stale: last observed 10 minutes ago against a 60s TTL, and reports no metrics.
  { id: 'nd-0004', name: 'sparrow', role: 'Edge device', health: health('healthy', 600), capabilities: ['agent'], metrics: {} },
];

export const nodes: Node[] = nodeSeed.map(withFreshness);

/**
 * provisional: per-node detail metrics (GET /nodes/{id}/metrics is not implemented yet).
 * `nd-0003` omits temperature; `nd-0004` reports nothing.
 */
export const metrics: Record<string, NodeMetrics | undefined> = {
  'nd-0001': { observed_at: t(10), cpu_percent: 21, ram_percent: 47, storage_percent: 63, temperature_celsius: 54, latency_ms: 2 },
  'nd-0002': { observed_at: t(10), cpu_percent: 9, ram_percent: 31, storage_percent: 40, temperature_celsius: 48, latency_ms: 3 },
  'nd-0003': { observed_at: t(10), cpu_percent: 78, ram_percent: 92, storage_percent: 55, latency_ms: 24 },
  'nd-0004': undefined,
};

const serviceSeed: Omit<Service, 'freshness' | 'simulated'>[] = [
  { id: 'sv-01', name: 'Gateway proxy', health: health('healthy') },
  { id: 'sv-02', name: 'Mesh VPN', health: health('healthy') },
  { id: 'sv-03', name: 'Job runner', health: health('degraded') },
  { id: 'sv-04', name: 'Metrics store', health: health('unknown', 900) },
];

export const services: Service[] = serviceSeed.map(withFreshness);

export const computeSummary: ComputeSummary = {
  total_vcpu: 24, used_vcpu: 9, reserved_vcpu: 3, active_jobs: 2, queued_jobs: 1, observed_at: t(10),
};

export const workers: Worker[] = [
  { id: 'wk-01', node_id: 'nd-0001', name: 'forge-worker', state: 'online', vcpu_total: 12, vcpu_used: 6, health: health('healthy') },
  { id: 'wk-02', node_id: 'nd-0002', name: 'anvil-worker', state: 'draining', vcpu_total: 12, vcpu_used: 3, health: health('healthy') },
];

export const jobs: Job[] = [
  { id: 'jb-01', name: 'index-docs', state: 'running', worker_id: 'wk-01', started_at: t(300) },
  { id: 'jb-02', name: 'build-image', state: 'running', worker_id: 'wk-02', started_at: t(120) },
  { id: 'jb-03', name: 'nightly-lint', state: 'queued' },
  { id: 'jb-04', name: 'sync-assets', state: 'succeeded', worker_id: 'wk-01', started_at: t(3600) },
  { id: 'jb-05', name: 'export-report', state: 'failed', worker_id: 'wk-02', started_at: t(7200) },
];

export const graph: NetworkGraph = {
  nodes: [
    { id: 'g-net', kind: 'internet', label: 'Public internet' },
    { id: 'g-edge', kind: 'edge', label: 'beacon', health: health('degraded') },
    { id: 'g-mesh', kind: 'mesh', label: 'Private mesh', health: health('healthy') },
    { id: 'g-a', kind: 'compute', label: 'forge', health: health('healthy') },
    { id: 'g-b', kind: 'compute', label: 'anvil', health: health('healthy') },
    { id: 'g-c', kind: 'device', label: 'sparrow', health: health('healthy', 600) },
  ],
  edges: [
    { id: 'ge1', source: 'g-net', target: 'g-edge', kind: 'exposure' },
    { id: 'ge2', source: 'g-edge', target: 'g-mesh', kind: 'tunnel' },
    { id: 'ge3', source: 'g-mesh', target: 'g-a', kind: 'mesh' },
    { id: 'ge4', source: 'g-mesh', target: 'g-b', kind: 'mesh' },
    { id: 'ge5', source: 'g-mesh', target: 'g-c', kind: 'mesh' },
  ],
};

export const graphDetails: Record<string, { title: string; rows: [string, string][] }> = {
  'g-net': { title: 'Public internet', rows: [['Type', 'External']] },
  'g-edge': { title: 'beacon', rows: [['Role', 'Edge gateway'], ['Reverse proxy', 'Active'], ['Last seen', 'moments ago']] },
  'g-mesh': { title: 'Private mesh', rows: [['Peers', '3'], ['Type', 'Overlay network']] },
  'g-a': { title: 'forge', rows: [['Role', 'Primary compute'], ['Address', '192.0.2.10']] },
  'g-b': { title: 'anvil', rows: [['Role', 'Secondary compute'], ['Address', '192.0.2.11']] },
  'g-c': { title: 'sparrow', rows: [['Role', 'Edge device'], ['Freshness', 'Stale']] },
};

export const editorOptions: EditorOptions = {
  actionTypes: [
    { value: 'wake', label: 'Wake node' },
    { value: 'backup', label: 'Run backup' },
    { value: 'drain', label: 'Drain node', dangerous: true },
  ],
  targets: nodes.map((n) => ({ id: n.id, label: n.name })),
  channels: [{ value: 'ops-chat', label: 'Ops chat' }, { value: 'email', label: 'Email' }],
};

export const workflows: (Workflow & { last_run_status: 'success' | 'failed' | 'running' | 'disabled' })[] = [
  {
    id: 'wf-01', name: 'Nightly backup', description: 'Back up primary data', enabled: true, last_run_status: 'success',
    trigger: { type: 'cron', expression: '0 2 * * *' },
    steps: [
      { id: 's1', type: 'action', name: 'Run backup', action_type: 'backup', target_id: 'nd-0001' },
      { id: 's2', type: 'notify', name: 'Report result', channel: 'ops-chat', message: 'Backup finished' },
    ],
    on_success: { notify_channel: 'ops-chat' }, on_failure: { behavior: 'stop', notify_channel: 'email' },
  },
  {
    id: 'wf-02', name: 'Wake secondary on demand', enabled: true, last_run_status: 'success',
    trigger: { type: 'manual' },
    steps: [{ id: 's1', type: 'action', name: 'Wake secondary', action_type: 'wake', target_id: 'nd-0002' }],
    on_success: {}, on_failure: { behavior: 'stop' },
  },
  {
    id: 'wf-03', name: 'Edge degraded response', enabled: false, last_run_status: 'disabled',
    trigger: { type: 'health_condition', target_id: 'nd-0003', condition: 'degraded', for_seconds: 120 },
    steps: [{ id: 's1', type: 'notify', name: 'Alert', channel: 'ops-chat', message: 'Edge gateway degraded' }],
    on_success: {}, on_failure: { behavior: 'continue' },
  },
];

export const workflowRuns = [
  { id: 'wr-1', workflow_id: 'wf-01', status: 'success' as const, started_at: t(36000), duration_seconds: 680 },
  { id: 'wr-2', workflow_id: 'wf-01', status: 'failed' as const, started_at: t(122400), duration_seconds: 134 },
  { id: 'wr-3', workflow_id: 'wf-02', status: 'success' as const, started_at: t(5000), duration_seconds: 42 },
];

export const activity: Activity[] = [
  { id: 'ac-1', title: 'Backup completed', detail: 'forge', occurred_at: t(1200), state: 'success', simulated: true },
  { id: 'ac-2', title: 'Edge gateway degraded', detail: 'beacon', occurred_at: t(2400), state: 'degraded', simulated: true },
  { id: 'ac-3', title: 'Worker draining', detail: 'anvil-worker', occurred_at: t(9000), state: 'running', simulated: true },
];
