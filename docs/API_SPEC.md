# API Specification - oxnet.work

## 1. Principles

- Base path: `/api/v1`.
- JSON for normal requests/responses.
- SSE endpoint for server-to-browser events.
- Stable resource IDs separate from display names.
- Every response that represents observed external state includes freshness metadata where relevant.
- Mutations return action/run resources rather than pretending long-running work is synchronous.

## 2. Common response concepts

### Health state

```json
{
  "state": "healthy",
  "observed_at": "2026-10-17T12:00:00Z",
  "stale_after_seconds": 60,
  "source": "tailscale"
}
```

Allowed `state`: `healthy`, `degraded`, `unhealthy`, `unknown`, `disabled`.

### Error

```json
{
  "error": {
    "code": "ACTION_NOT_SUPPORTED",
    "message": "This node does not provide a wake capability.",
    "request_id": "..."
  }
}
```

## 3. Initial endpoints

Implemented in the current demo: system and health, SSE, nodes (including detail and metrics), services, activity, compute summary/workers, an empty jobs list, the network graph/connections, and the Automations endpoints (workflows, workflow runs, audit; see "Workflows" below). Other endpoints below describe planned work and currently return `404`. All inventory observations are persisted in SQLite; the demo provider refreshes them every 30 seconds. Compute job counts and used/reserved vCPU are demo placeholders, not live scheduler readings. Network edges are explicit simulated observations; the server does not infer links from inventory membership.

### System

- `GET /healthz` - process liveness, no auth-sensitive detail.
- `GET /readyz` - DB/migration readiness.
- `GET /api/v1/system` - version, enabled features, server time.
- `GET /api/v1/events` - SSE stream.

### Nodes

- `GET /api/v1/nodes`
- `GET /api/v1/nodes/{id}`
- `GET /api/v1/nodes/{id}/metrics`
- `GET /api/v1/nodes/{id}/containers`
- `GET /api/v1/nodes/{id}/jobs`
- `GET /api/v1/nodes/{id}/connections`
- `GET /api/v1/nodes/{id}/logs?limit=100`
- `GET /api/v1/nodes/{id}/actions`
- `POST /api/v1/nodes/{id}/actions`

Example action request:

```json
{
  "type": "drain",
  "parameters": {},
  "idempotency_key": "client-generated-value"
}
```

Response is `202 Accepted` with action resource.

### Services

- `GET /api/v1/services`
- `GET /api/v1/services/{id}`
- `GET /api/v1/services/{id}/instances`

### Compute

- `GET /api/v1/compute/summary`
- `GET /api/v1/compute/workers`
- `GET /api/v1/jobs`
- `POST /api/v1/jobs` (only after provider/action security is complete)
- `GET /api/v1/jobs/{id}`
- `POST /api/v1/jobs/{id}/cancel`

### Network

- `GET /api/v1/network/graph`
- `GET /api/v1/network/connections`

### Workflows (implemented, Milestone 4; see ADR-013)

- `GET /api/v1/automations/summary` - counts for the Automations page (enabled, scheduled, 7-day success rate, failed in 24h).
- `GET /api/v1/workflows/options` - what the runner can execute now (`action_types`, `targets`, `channels`) and `planned_actions` for later milestones.
- `GET /api/v1/workflows` - items include `last_run` and `next_run_at`.
- `POST /api/v1/workflows` - `201` with the created workflow.
- `GET /api/v1/workflows/{id}`
- `PATCH /api/v1/workflows/{id}` - merges onto the stored definition; a supplied `steps` array replaces all steps.
- `POST /api/v1/workflows/{id}/run` - `202` with a queued run and a `Location` header; execution is asynchronous.
- `POST /api/v1/workflows/{id}/enable`
- `POST /api/v1/workflows/{id}/disable`
- `GET /api/v1/workflows/{id}/runs` - newest first, without steps.
- `GET /api/v1/workflow-runs` - recent runs across all workflows.
- `GET /api/v1/workflow-runs/{id}` - includes step results.

Definition shape: `{name, description?, enabled, trigger, steps[], on_success{notify_channel?}, on_failure{behavior: "stop"|"continue", notify_channel?}}`.
Triggers: `{type:"manual"}`, `{type:"cron", expression}` (5 fields, UTC), `{type:"health_condition", target_id, condition:"unhealthy"|"degraded"|"stale", for_seconds}`.
Steps: `{id, type:"action", name, action_type, target_id}` or `{id, type:"notify", name, channel, message}`. Limits: 100 workflows, 20 steps each.

Run states: `queued -> running -> succeeded | failed | cancelled` (`cancelled` is reserved; there is no cancel endpoint yet). Step states: `pending | running | succeeded | failed | skipped`. Failed steps carry an `error_code` such as `CHECK_FAILED`, `NODE_NOT_FOUND`, `ACTION_NOT_SUPPORTED`, `TIMEOUT` or `INTERRUPTED`.

Executable actions in v1 are read-only inventory checks (`check_node_health`, `check_node_freshness`). Mutating actions (`wake_node`, `drain_node`, `restart_service`, `run_backup`) are listed under `planned_actions`; a definition that uses one is rejected with `ACTION_NOT_SUPPORTED`. The only notification channel is `activity` (recorded in the audit trail).

Errors specific to this area: `INVALID_WORKFLOW` and `ACTION_NOT_SUPPORTED` (`400`, with per-field `error.details`), `INVALID_REQUEST` (`400`), `UNSUPPORTED_MEDIA_TYPE` (`415`), `MUTATIONS_DISABLED`, `CROSS_ORIGIN` and `CSRF_INVALID` (`403`), `WORKFLOW_DISABLED`, `RUN_IN_PROGRESS` and `WORKFLOW_LIMIT` (`409`).

Mutations require `Content-Type: application/json`, a same-origin `Origin` when the browser sends one, and `mutations_enabled: true` in `/api/v1/system` (signed in, or a loopback-bound server). With sign-in enabled the `X-CSRF-Token` header from `GET /auth/session` is required on every non-GET `/api/` request.

### Services (ADR-014)

`GET /api/v1/services` and `GET /api/v1/services/{id}` return `{id, name, health, freshness, simulated}` plus, for probed services, `endpoint` (the address checked, query string removed) and `detail` (the short result). Probed services have ids `probe-<probe id>` and `health.source` `http-probe`. `health.state` is `healthy` only when the endpoint returned an expected status within its deadline, otherwise `unhealthy`; a result older than three probe rounds reports `freshness: "stale"`. Probes are fixed in server configuration and cannot be created or changed through the API.

### Activity/Audit

- `GET /api/v1/activity`
- `GET /api/v1/audit` (implemented) - append-only, newest first. Query: `limit` (default 50, max 200), `cursor` (pass the previous `next_cursor`), `target_type`, `target_id`. Events: `{id, occurred_at, actor, action, target_type, target_id, outcome, detail}`.

### Integrations

- `GET /api/v1/integrations`
- `GET /api/v1/integrations/{id}`
- `POST /api/v1/integrations/{id}/test`

Configuration mutations may initially remain file/env driven rather than API driven.

## 4. Pagination

List endpoints that can grow use cursor pagination:

```json
{
  "items": [],
  "next_cursor": "..."
}
```

Default limits should be conservative.

## 5. SSE events

Event types may include:

- `node.updated`
- `node.health_changed`
- `service.updated`
- `service.health_changed`
- `job.updated`
- `workflow_run.updated`
- `action.updated`
- `activity.created`
- `integration.health_changed`

SSE payloads should be compact. They may contain the changed resource summary or an instruction to refetch a resource.

## 6. Agent endpoints

Keep separate namespace, e.g. `/agent/v1`.

- `POST /agent/v1/enroll`
- `POST /agent/v1/heartbeat`
- `POST /agent/v1/report`
- `GET /agent/v1/actions/next`
- `POST /agent/v1/actions/{id}/result`

Agent endpoints use scoped agent authentication, not browser sessions.

## 7. Action semantics

Actions have states:

`pending -> dispatched -> running -> succeeded|failed|cancelled|expired`

Actions are idempotent where possible. The server records:

- actor;
- target;
- type;
- sanitized parameters;
- timestamps;
- provider/agent;
- result summary;
- error code.

## 8. OpenAPI

`/specs/openapi.yaml` is the machine-readable starting contract. Keep it updated with implemented API behavior.
