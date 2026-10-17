# Architecture - oxnet.work

## 1. Architectural goals

The architecture optimizes for:

- low idle resource use;
- easy Docker deployment;
- private-network operation;
- clear security boundaries;
- replaceable external integrations;
- deterministic behavior;
- simple backup/recovery;
- incremental expansion.

## 2. Logical architecture

```text
                                      +-----------------------+
                                      |       Browser         |
                                      | React static frontend |
                                      +-----------+-----------+
                                                  |
                                           HTTPS / Tailnet
                                                  |
                         +------------------------v------------------------+
                         |             oxnet.work control plane           |
                         |                                                 |
                         |  HTTP API   SSE   Auth   Audit   Scheduler      |
                         |  Domain services      Integration manager        |
                         |                     SQLite                       |
                         +-----+----------------------+--------------------+
                               |                      |
                  provider API |                      | private agent API
                               |                      |
      +------------------------+------+        +------v----------------------+
      | External/provider adapters   |        | Node agents                 |
      | Tailscale / Pangolin          |        | OX-01 / OX-02 / Pi / etc. |
      | OpenCode / Uptime / Beszel    |        | metrics / docker / actions |
      | Authentik / backups / future  |        +-----------------------------+
      +-------------------------------+
```

## 3. Runtime components

### 3.1 Control plane server

One Go process owns:

- API routing;
- authentication/session validation;
- domain services;
- adapter polling;
- action dispatch;
- workflow scheduler;
- audit/event recording;
- SSE subscriptions;
- serving compiled frontend assets.

This reduces moving parts and allows a one-container default installation.

### 3.2 SQLite

SQLite stores:

- application configuration that is safe to persist;
- inventory identity/metadata;
- workflow definitions and run history;
- jobs and job run summaries;
- audit events;
- last-known integration snapshots where useful;
- UI/user preferences where appropriate.

Do not use SQLite as a high-frequency time-series store. Keep only bounded/recent metric samples if needed for small spark lines. Deep historical metrics should remain in Beszel/Prometheus/Grafana or a future metric provider.

Recommended settings:

- WAL mode;
- foreign keys enabled;
- busy timeout;
- explicit migrations;
- transaction boundaries at domain operation level.

### 3.3 Frontend

React is compiled by Vite. Production output is embedded into or copied beside the Go binary/container.

State strategy:

- server state fetched through a small typed client;
- SSE updates invalidate/update relevant caches;
- minimal local state;
- no Redux unless complexity proves it necessary.

### 3.4 Node agent

The optional agent exposes host capabilities without granting the control plane broad network-level shell access.

Suggested agent model:

- agent has stable ID and generated keypair/token;
- first enrollment requires an owner-approved bootstrap token;
- token is exchanged for a scoped agent credential;
- agent periodically posts heartbeat/capabilities/metrics;
- agent polls or receives a bounded action queue over private network;
- actions are enumerated (`wake_proxy`, `docker_restart`, `collect_logs`, etc.), not arbitrary shell strings;
- every action returns a structured result.

A later release may use mutually authenticated transport, but v1 must at least use TLS/private tailnet plus scoped credentials and replay-resistant action IDs.

## 4. Domain boundaries

### Inventory
Owns nodes, capabilities, tags, services, service instances, integration ownership.

### Telemetry
Normalizes health/resource samples and freshness.

### Compute
Owns workers, capacity aggregation, jobs, placement policy, drain state.

### Network
Owns graph nodes/edges derived from inventory and integration relationships.

### Automation
Owns workflow definitions, triggers, ordered steps, execution, retries, outcomes.

### Actions
Owns user-requested mutations and their lifecycle.

### Audit
Append-only operational record.

### Integrations
Provider-specific clients and normalization into domain types.

## 5. Integration interface pattern

Do not design one huge provider interface. Use capability-specific interfaces such as:

```go
type NodeSource interface {
    ListNodes(ctx context.Context) ([]DiscoveredNode, error)
}

type ServiceSource interface {
    ListServices(ctx context.Context) ([]DiscoveredService, error)
}

type MetricSource interface {
    NodeMetrics(ctx context.Context, nodeID string) (NodeMetrics, error)
}

type ComputeProvider interface {
    ListWorkers(ctx context.Context) ([]Worker, error)
    ListJobs(ctx context.Context) ([]Job, error)
    SubmitJob(ctx context.Context, req SubmitJobRequest) (Job, error)
}

type ActionProvider interface {
    Capabilities(ctx context.Context, target Target) ([]ActionCapability, error)
    Execute(ctx context.Context, req ActionRequest) (ActionResult, error)
}
```

Providers may implement several interfaces.

## 6. Data flow

### Polling

- Each adapter has an independent poll interval, timeout, and backoff.
- Poller writes normalized snapshots through domain services.
- Poll failure records integration health but does not erase last-known data.
- Last-known records become stale according to source-specific TTL.

### Events

Internal events are published to a simple in-process bus. Important events are also persisted in audit/activity tables. SSE subscribers receive safe user-facing events.

Do not introduce an external broker in v1.

## 7. Scheduling and placement

v1 placement algorithm:

1. Filter workers to online, non-drained, fresh state.
2. Filter by required capabilities/tags/architecture/GPU.
3. Exclude workers below configured free resource thresholds.
4. Apply explicit affinity/exclusion rules.
5. Score remaining candidates by normalized free CPU/RAM/GPU and optional load.
6. Choose highest deterministic score; stable tie-break by worker ID.
7. Persist placement decision/reason.

The UI should be able to explain the placement result.

## 8. Topology derivation

Network topology is represented as:

```json
{
  "nodes": [{"id":"...","kind":"edge|network|compute|service|device","label":"..."}],
  "edges": [{"id":"...","source":"...","target":"...","kind":"tailnet|exposure|hosts|depends_on"}]
}
```

The server derives this graph from inventory and integrations. The frontend computes layout for the currently visible graph.

## 9. Failure behavior

- Adapter timeout -> show stale state with last success timestamp.
- Agent offline -> node remains in inventory, actions disabled.
- SQLite unavailable/corrupt -> readiness fails; health endpoint may remain alive for diagnosis.
- SSE disconnect -> frontend falls back to periodic refresh/backoff.
- Workflow step fails -> record step result; follow configured stop/continue behavior.
- Unknown metrics -> render `Not reported`, not zero.

## 10. Upgrade strategy

- database migrations run before server begins accepting normal traffic;
- create a DB backup before destructive migrations;
- expose version/build metadata;
- rolling HA is not required for v1;
- backwards compatibility within v1 APIs should be maintained.
