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

Implemented in the current demo: system and health, SSE, nodes (including detail and metrics), services, activity, compute summary/workers, an empty jobs list, and the network graph/connections. Other endpoints below describe planned work and currently return `404`. All inventory observations are persisted in SQLite; the demo provider refreshes them every 30 seconds. Compute job counts and used/reserved vCPU are demo placeholders, not live scheduler readings. Network edges are explicit simulated observations; the server does not infer links from inventory membership.

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

### Workflows

- `GET /api/v1/workflows`
- `POST /api/v1/workflows`
- `GET /api/v1/workflows/{id}`
- `PATCH /api/v1/workflows/{id}`
- `POST /api/v1/workflows/{id}/run`
- `POST /api/v1/workflows/{id}/enable`
- `POST /api/v1/workflows/{id}/disable`
- `GET /api/v1/workflows/{id}/runs`
- `GET /api/v1/workflow-runs/{id}`

### Activity/Audit

- `GET /api/v1/activity`
- `GET /api/v1/audit`

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
