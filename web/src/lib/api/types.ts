// Hand-maintained from specs/openapi.yaml, which now describes only what the Go server implements
// (/system, /nodes, /nodes/{id}/metrics, /services, /activity, /compute/*, /jobs, /network/*, /events).
// Types marked "provisional" are NOT in the spec/server yet (docs/API_SPEC.md planned work) and
// must be replaced when the spec gains schemas for them.

import type { Workflow } from '../../features/workflows/types';

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
  inventory_source?: 'demo' | 'tailscale';
  auth_required: boolean;
  /** True when this session may change workflows (signed in, or a loopback-bound server). */
  mutations_enabled?: boolean;
}

/** Server-computed freshness. Prefer this over deriving it client-side when present. */
export type NodeFreshness = 'fresh' | 'stale' | 'unknown';

export interface Metrics {
  cpu_percent?: number;
  disk_percent?: number;
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
  addresses?: string[];
  last_seen?: string;
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

export interface GraphNode {
  id: string;
  resource_id?: string;
  kind: string;
  label: string;
  health?: Health;
  freshness?: NodeFreshness;
  simulated?: boolean;
}

export interface GraphEdge {
  id: string;
  source: string;
  target: string;
  kind: string;
  observed_at?: string;
  stale_after_seconds?: number;
  source_system?: string;
  freshness?: NodeFreshness;
  simulated?: boolean;
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
  /** Per-field validation messages (workflow definitions), keyed like the editor's errors. */
  details?: Record<string, string>;
  /** HTTP status, or 0 when no response was received. */
  status: number;
}

// ---- Automations (docs/DECISIONS.md ADR-013) ----

export type RunState = 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled';
export type RunStepState = 'pending' | 'running' | 'succeeded' | 'failed' | 'skipped';

export interface RunSummary {
  id: string;
  state: RunState;
  trigger: string;
  started_at: string;
  finished_at: string | null;
  duration_ms: number | null;
}

/** A stored workflow: the editable definition plus server-managed fields. */
export interface WorkflowRecord extends Workflow {
  simulated: boolean;
  created_at: string;
  updated_at: string;
  last_run: RunSummary | null;
  next_run_at: string | null;
}

/** The writable part of a workflow, as accepted by POST and PATCH. */
export type WorkflowInput = Omit<Workflow, 'id'>;

export interface RunStep {
  position: number;
  step_id: string;
  kind: 'action' | 'notify';
  name: string;
  state: RunStepState;
  started_at: string | null;
  finished_at: string | null;
  output: string;
  error_code?: string;
}

export interface WorkflowRun {
  id: string;
  workflow_id: string;
  workflow_name: string;
  trigger: string;
  actor: string;
  state: RunState;
  started_at: string;
  finished_at: string | null;
  duration_ms: number | null;
  error_code?: string;
  summary: string;
  simulated: boolean;
  /** Present on GET /workflow-runs/{id} and the POST .../run response; omitted in lists. */
  steps?: RunStep[];
}

export interface AutomationSummary {
  workflows: number;
  enabled: number;
  disabled: number;
  scheduled_count: number;
  next_scheduled_at: string | null;
  runs_last_7_days: number;
  succeeded_last_7_days: number;
  /** Percentage 0-100, or null when no runs have finished in the last 7 days. */
  success_rate: number | null;
  failed_last_24h: number;
  active_runs: number;
  generated_at: string;
  simulated: boolean;
}

export interface AutomationOptions {
  action_types: { value: string; label: string; dangerous?: boolean }[];
  targets: { id: string; label: string }[];
  channels: { value: string; label: string }[];
  planned_actions: { value: string; label: string; milestone: string }[];
}

export interface AuditEvent {
  id: number;
  occurred_at: string;
  actor: string;
  action: string;
  target_type: string;
  target_id: string;
  outcome: string;
  detail: Record<string, unknown>;
}

export interface ListResponse<T> {
  items: T[];
  next_cursor?: string;
}
