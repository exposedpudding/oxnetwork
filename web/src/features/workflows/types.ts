// provisional: the OpenAPI spec has no Workflow schema yet. Shapes follow docs/PRODUCT_SPEC.md
// (manual, cron and simple condition triggers; ordered steps; no freeform canvas).

export type Trigger =
  | { type: 'manual' }
  | { type: 'cron'; expression: string }
  | { type: 'health_condition'; target_id: string; condition: 'unhealthy' | 'degraded' | 'stale'; for_seconds: number };

export type Step =
  | { id: string; type: 'action'; name: string; action_type: string; target_id: string }
  | { id: string; type: 'notify'; name: string; channel: string; message: string };

export interface FailureHandling {
  behavior: 'stop' | 'continue';
  notify_channel?: string;
}

export interface Workflow {
  id: string;
  name: string;
  description?: string;
  enabled: boolean;
  trigger: Trigger;
  steps: Step[];
  on_success: { notify_channel?: string };
  on_failure: FailureHandling;
}

/** Supplied by the host from live capabilities - the editor never hardcodes actions or hosts. */
export interface EditorOptions {
  actionTypes: { value: string; label: string; dangerous?: boolean }[];
  targets: { id: string; label: string }[];
  channels: { value: string; label: string }[];
}

export type WorkflowErrors = Record<string, string>;
