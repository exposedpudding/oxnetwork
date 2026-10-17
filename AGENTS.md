# oxnet.work - Agent Implementation Contract

This file is the primary implementation contract for coding agents working in this repository. Read it before making changes. The project owner has approved the product direction, UI style, architecture principles, and phased build approach described here and in `/docs`.

## 1. Mission

Build **oxnet.work**, a lightweight self-hosted command centre for a private network / homelab / development fabric. It should bring together:

- secure access and edge exposure through Pangolin;
- private device connectivity through Tailscale;
- direct administration through RDP and SSH launch actions;
- Docker/container visibility and actions;
- OpenCode workers and distributed compute/job placement;
- host/service health and observability;
- automation workflows;
- network topology and connection visibility;
- security/identity integrations;
- backup visibility and operational runbooks.

The product must feel like a purpose-built operations console rather than a generic link dashboard.

## 2. Non-negotiable product principles

1. **Lightweight first.** Do not introduce infrastructure dependencies unless they solve a proven problem. v1 must not require Redis, Kafka, RabbitMQ, Elasticsearch, Kubernetes, or PostgreSQL.
2. **One small control plane.** The default deployment is one `oxnet-work` container serving both API and compiled frontend assets, plus optional small node agents.
3. **Local-first and private by default.** The control plane should be reachable through Tailscale by default. Public exposure through Pangolin is optional and must be explicit.
4. **Adapters, not hard wiring.** Pangolin, Tailscale, Docker, OpenCode, Beszel, Uptime Kuma, Grafana, Authentik, backup systems, and future integrations must sit behind interfaces/adapters.
5. **Read-only before destructive.** Every integration should begin with discovery/status. Mutating actions are added only after read paths are stable, auditable, and permission-gated.
6. **No credentials in source control.** Secrets are supplied through environment variables, mounted secret files, or a future secret provider.
7. **No invented telemetry.** Mock data is permitted in development only. Production UI must clearly distinguish unavailable, stale, simulated, and live data.
8. **Audit every action.** Wake, drain, restart, deploy, backup, shell/remote launch, configuration change, and workflow run must produce an audit event.
9. **UI restraint.** Follow the approved dark SaaS visual language in `docs/UI_SPEC.md`. Avoid heavy glow, WebGL, 3D, video backgrounds, particle effects, or animation libraries.
10. **Incremental delivery.** Each milestone must leave the application runnable and useful.

## 3. Approved technical baseline

### Control plane

- Backend: **Go** (current stable toolchain available to the repository).
- HTTP: standard `net/http` or a small router such as Chi. Avoid heavyweight frameworks.
- Database: **SQLite** with WAL mode and migrations.
- Realtime updates: **Server-Sent Events (SSE)**. Prefer SSE over WebSockets until bidirectional realtime becomes necessary.
- Scheduler: in-process cron/timer subsystem persisted in SQLite.
- Logging: structured JSON with human-readable dev mode.
- Metrics: optional Prometheus-compatible `/metrics` endpoint.

### Frontend

- **React + TypeScript + Vite**.
- Styling: Tailwind CSS or a small token-based CSS layer. Do not add a large design system framework.
- Icons: Lucide or equivalent tree-shakeable icon set.
- Charts: lightweight SVG/canvas library only where justified. Prefer CSS bars/SVG for simple metrics.
- Topology: SVG-based nodes and edges for v1. A small graph library is acceptable only if bundle impact is measured and justified.
- Build output is embedded/served by the Go control plane in production.

### Node agent

- Optional **Go agent** installed only where direct host/container actions are required.
- Agent initiates outbound connection/polling to the control plane; do not require inbound public ports.
- Tailscale identity/address may be used for private reachability.
- Agent capabilities are explicitly declared and permission-scoped.

## 4. Resource targets

Treat these as engineering targets, not marketing claims:

- Control plane idle memory: target **< 150 MB** including frontend server/API process, excluding OS page cache.
- Control plane idle CPU: effectively near zero outside scheduled polling.
- Node agent idle memory: target **< 30 MB**.
- Browser initial JS payload: target **< 350 KB gzip** for the shell; lazy-load topology/charts if needed.
- Fresh install should require **one Docker Compose file** and one writable data volume.
- A Raspberry Pi-class node must not be required to run the control plane, but agents should be suitable for lightweight ARM64 hosts.

If a dependency materially threatens these targets, document the reason before adding it.

## 5. Source-of-truth hierarchy

When instructions conflict, use this order:

1. `AGENTS.md`
2. `docs/PRODUCT_SPEC.md`
3. `docs/ARCHITECTURE.md`
4. `docs/SECURITY.md`
5. `docs/UI_SPEC.md`
6. `docs/API_SPEC.md`
7. `docs/INTEGRATIONS.md`
8. `docs/ROADMAP.md`
9. `CLAUDE.md` for delegated Claude Code work
10. comments/TODOs in code

Do not silently resolve a material contradiction. Add an entry to `docs/DECISIONS.md`.

## 6. Repository layout to create

```text
/
├── AGENTS.md
├── CLAUDE.md
├── README.md
├── cmd/
│   ├── server/
│   └── agent/
├── internal/
│   ├── api/
│   ├── auth/
│   ├── audit/
│   ├── config/
│   ├── db/
│   ├── domain/
│   ├── events/
│   ├── integrations/
│   ├── jobs/
│   ├── scheduler/
│   └── telemetry/
├── web/
│   ├── src/
│   │   ├── app/
│   │   ├── components/
│   │   ├── features/
│   │   ├── lib/
│   │   └── styles/
│   └── ...
├── migrations/
├── docs/
├── deploy/
│   ├── docker/
│   └── systemd/
├── scripts/
└── tests/
```

Keep domain logic out of HTTP handlers and React components.

## 7. Core domain model

Minimum entities:

- `Node`: a machine/device participating in oxnet.work.
- `NodeCapability`: compute, docker, ssh, rdp, wake, backup, monitor, agent, etc.
- `Service`: a logical service such as Pangolin, Grafana, n8n, OpenCode.
- `ServiceInstance`: where a service is actually running.
- `MetricSample`: short-lived or persisted health/resource sample.
- `Connection`: observed network/tunnel/service relationship.
- `Job`: a unit of compute or operational work.
- `JobRun`: a concrete execution attempt.
- `Workflow`: automation definition.
- `WorkflowRun`: workflow execution.
- `Action`: user-requested mutation such as wake/restart/drain.
- `AuditEvent`: immutable record of user/system actions.
- `Integration`: configured external provider and health state.

Use UUIDs or ULIDs for internal IDs. Human labels such as `OX-02` are mutable display names and must not be primary keys.

## 8. MVP screens

Build these first and match the approved mockups in `docs/images`:

1. **Overview** - `01-overview.png`
2. **Compute** - `02-compute.png`
3. **Node Detail** - `03-node-detail.png`
4. **Network** - `04-network.png`
5. **Automations** - `05-automations.png`

The image files are visual direction, not literal data requirements. Do not hardcode their sample IPs, versions, dates, capacities, node counts, or usernames.

## 9. Required behavior by MVP screen

### Overview

- node summary cards;
- aggregate compute capacity;
- services health list;
- recent activity;
- safe quick actions;
- global command palette entry point.

### Compute

- aggregate capacity;
- workers table;
- active/queued/completed jobs;
- scheduler/placement rule summary;
- drain/add/test/log actions with permission checks.

### Node detail

- live/stale state;
- CPU/RAM/storage/temperature/latency where available;
- capability-aware actions;
- containers/jobs/network/logs sections;
- show `not reported` instead of fabricated values.

### Network

- simple SVG topology;
- selected-node detail pane;
- active connections/routes table;
- filters and node categories;
- topology derives from domain data, not coordinates hardcoded to specific node names.

### Automations

- workflows list;
- workflow detail;
- simple trigger -> steps -> outcome representation;
- recent runs;
- manual run/enable/disable/history actions.

## 10. Integration sequence

Implement integrations in this order unless blocked:

1. Internal mock/demo provider behind interfaces.
2. Tailscale node/tailnet discovery.
3. Docker read-only discovery through agent.
4. Beszel or direct agent host metrics.
5. Uptime Kuma service checks/status import where useful.
6. Pangolin resource/exposure inventory.
7. OpenCode worker/job adapter.
8. Wake-on-LAN and node actions.
9. Backup adapter(s).
10. Authentik/OIDC.
11. Optional Grafana deep links and metrics integration.
12. Future providers.

An unavailable integration must degrade gracefully and never prevent the rest of the dashboard loading.

## 11. Security gates

Before enabling any mutating action:

- authentication is active;
- action is authorized by role/capability;
- CSRF/state-changing request protections are in place where relevant;
- target identity is verified;
- action is logged before and after execution;
- dangerous actions require explicit confirmation;
- secrets are redacted from logs;
- integration errors do not leak tokens or private config.

Do not implement a generic arbitrary remote shell endpoint in v1.

## 12. Testing expectations

Minimum test layers:

- Go unit tests for domain, scheduler, placement rules, configuration, and adapters.
- SQLite migration tests.
- HTTP API tests using `httptest`.
- Frontend component tests for state/status rendering and destructive confirmations.
- Playwright smoke tests for the five MVP screens.
- Contract fixtures for each integration adapter so UI development does not depend on live infrastructure.

Every milestone should include:

```bash
go test ./...
npm test
npm run build
```

and a container build smoke test.

## 13. Coding rules

- Prefer explicit, boring code over abstraction for its own sake.
- Keep packages small and cohesive.
- Add interfaces at external system boundaries, not everywhere.
- Avoid global mutable state.
- Use context cancellation/timeouts on all external calls.
- Bound polling concurrency.
- Store timestamps in UTC; present in browser-local time.
- Never assume an IP address, domain subdomain, host OS, or virtualization platform from a display name.
- Use feature flags for unfinished mutating capabilities.
- Add database migrations; never mutate schema implicitly in production.
- Use semantic API errors with stable error codes.

## 14. Definition of done for v1

v1 is complete when a clean machine can run oxnet.work via Docker Compose and the owner can:

- sign in;
- see current nodes and their freshness/health;
- see service state;
- see aggregate compute capacity;
- inspect a node;
- view a derived network topology;
- see jobs/workers;
- create/run basic workflows;
- execute only supported safe actions;
- review an audit trail;
- back up/restore the SQLite data directory using documented steps;
- operate the product without any external SaaS dependency.

## 15. First coding task

Start with `docs/ROADMAP.md` Milestone 0 and Milestone 1. Do not jump straight to live integrations.

Before writing significant code, create the repository skeleton, configuration loader, SQLite migration framework, domain types, demo provider, frontend shell/design tokens, and container build. The first runnable target is the **Overview screen populated through the same API interfaces production integrations will later use**.
