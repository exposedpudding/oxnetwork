// Hand-maintained from specs/openapi.yaml (v0.1.0). Keep in sync with the spec.
// Types marked "provisional" have no schema in the spec yet; they follow docs/API_SPEC.md
// and should be replaced when the spec gains schemas for them.

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
  features?: string[];
}

export interface Node {
  id: string;
  name: string;
  role?: string;
  platform?: string;
  health: Health;
  capabilities: string[];
  addresses?: Record<string, string>;
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

/** provisional: GET /nodes/{id}/metrics - only metrics the node reports are present. */
export interface NodeMetrics {
  observed_at: string;
  cpu_percent?: number;
  memory_percent?: number;
  storage_percent?: number;
  temperature_celsius?: number;
  latency_ms?: number;
}

/** provisional: GET /services */
export interface Service {
  id: string;
  name: string;
  health: Health;
  instance_count?: number;
}

/** provisional: GET /compute/summary. Informational only - vCPU is not fungible across nodes. */
export interface ComputeSummary {
  total_vcpu: number;
  used_vcpu: number;
  reserved_vcpu: number;
  active_jobs: number;
  queued_jobs: number;
  observed_at: string;
}

/** provisional: GET /compute/workers */
export interface Worker {
  id: string;
  node_id: string;
  name: string;
  state: 'online' | 'draining' | 'offline';
  vcpu_total: number;
  vcpu_used: number;
  health: Health;
}

export type JobState = 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled';

/** provisional: GET /jobs */
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
