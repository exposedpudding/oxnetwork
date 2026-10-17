// Hand-maintained from specs/openapi.yaml, which now describes only what the Go server implements
// (/system, /nodes, /nodes/{id}/metrics, /services, /activity, /compute/*, /jobs, /events).
// Types marked "provisional" are NOT in the spec/server yet (docs/API_SPEC.md planned work) and
// must be replaced when the spec gains schemas for them.

export type HealthState = 'healthy' | 'degraded' | 'unhealthy' | 'unknown' | 'disabled';

export interface Health {
  state: HealthState;
  observed_at: string;
  stale_after_seconds?: number;
  source?: string;
}

export interface SystemInfo {
  name: 'oxnet.work';
  version: string;
  server_time: string;
  features: string[];
  site_name: string;
  /** True while the simulated provider is serving data. */
  demo_mode: boolean;
}

/** Server-computed freshness. Prefer this over deriving it client-side when present. */
export type NodeFreshness = 'fresh' | 'stale' | 'unknown';

export interface Metrics {
  cpu_percent?: number;
  ram_percent?: number;
  vcpu?: number;
  ram_gb?: number;
}

export interface Node {
  id: string;
  name: string;
  role?: string;
  platform?: string;
  health: Health;
  capabilities: string[];
  addresses?: Record<string, string>;
  freshness?: NodeFreshness;
  /** True when the record comes from the demo provider, not live infrastructure. */
  simulated?: boolean;
  metrics?: Metrics;
}

export interface ActionRequest {
  type: string;
  idempotency_key: string;
  parameters?: Record<string, unknown>;
}

export type ActionState =
  | 'pending'
  | 'dispatched'
  | 'running'
  | 'succeeded'
  | 'failed'
  | 'cancelled'
  | 'expired';

export interface Action {
  id: string;
  type: string;
  state: ActionState;
  target_id: string;
  created_at: string;
}

/** provisional: the network graph endpoint was removed from the spec until implemented; these are the topology renderer's input shapes. */
export interface GraphNode {
  id: string;
  kind: string;
  label: string;
  health?: Health;
}

export interface GraphEdge {
  id: string;
  source: string;
  target: string;
  kind: string;
}

export interface NetworkGraph {
  nodes: GraphNode[];
  edges: GraphEdge[];
}

/**
 * provisional extras: the implemented GET /nodes/{id}/metrics returns `Metrics` only. These extra
 * fields (storage, temperature, latency) are not in the spec yet; the fixture harness uses them to
 * exercise "Not reported" rendering.
 */
export interface NodeMetrics extends Metrics {
  observed_at?: string;
  storage_percent?: number;
  temperature_celsius?: number;
  latency_ms?: number;
}

export interface Service {
  id: string;
  name: string;
  health: Health;
  freshness?: NodeFreshness;
  simulated?: boolean;
}

export interface Activity {
  id: string;
  title: string;
  detail: string;
  state: string;
  occurred_at: string;
  simulated?: boolean;
}

export interface ComputeSummary {
  total_vcpu: number;
  /** Demo placeholder (0) until a job provider exists; not a live scheduler reading. */
  used_vcpu: number;
  reserved_vcpu: number;
  active_jobs: number;
  queued_jobs: number;
  observed_at: string;
  freshness: NodeFreshness;
  simulated: boolean;
}

export interface Worker {
  id: string;
  node_id: string;
  name: string;
  state: 'online' | 'draining' | 'offline';
  vcpu_total: number;
  vcpu_used: number;
  health: Health;
  freshness: NodeFreshness;
  simulated: boolean;
}

export type JobState = 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled';

/** provisional shape: the spec only says `GET /jobs` returns `{items: object[]}` (empty in demo mode). */
export interface Job {
  id: string;
  name: string;
  state: JobState;
  worker_id?: string;
  started_at?: string;
}

export interface ApiError {
  code: string;
  message: string;
  request_id?: string;
  /** HTTP status, or 0 when no response was received. */
  status: number;
}

export interface ListResponse<T> {
  items: T[];
  next_cursor?: string;
}
