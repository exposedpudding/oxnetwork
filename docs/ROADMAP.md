# Implementation Roadmap - oxnet.work

Status legend: `[ ]` not started, `[-]` in progress, `[x]` complete.

## Milestone 0 - repository and runnable shell

- [x] Create initial repo structure from `AGENTS.md`.
- [x] Go module + server entrypoint.
- [x] React/TypeScript/Vite app.
- [x] Shared design tokens/layout shell.
- [x] SQLite connection + migrations framework.
- [x] Config loader + validation.
- [x] Structured logging + request IDs.
- [x] `/healthz`, `/readyz`, `/api/v1/system`.
- [x] Multi-stage Dockerfile.
- [x] Docker Compose example.
- [x] CI for Go tests + frontend build/tests.

Acceptance: one command starts a blank but branded oxnet.work shell and health endpoints pass.

## Milestone 1 - demo vertical slice / Overview

- [x] Domain types for Node, Service, Health, Metric, Activity.
- [x] SQLite-backed inventory repository layer.
- [x] Deterministic demo provider.
- [x] Nodes/services/activity endpoints.
- [x] SSE event stream.
- [x] Overview UI matching approved direction.
- [x] Fresh/stale/unknown states.
- [x] Playwright Overview smoke test.

The demo provider now reconciles its fixtures into SQLite at startup and every 30 seconds. Inventory API reads use persisted observations and recompute freshness at request time. Action/audit behavior remains for later work. This build is read-only and loopback-bound on the host.

Acceptance: Overview is fully populated from API data, not frontend constants.

## Milestone 2 - Compute and Node Detail

- [ ] Worker/job domain.
- [x] Read-only demo compute aggregation endpoint.
- [x] Pure placement policy package + tests (dispatch and decision persistence await a job provider).
- [x] Read-only demo Compute screen backed by summary, workers and jobs APIs.
- [x] Read-only Node Detail screen backed by node and metrics APIs.
- [x] Persisted node capability list.
- [ ] Read-only container/job/log fixtures.

Acceptance: owner can understand aggregate capacity and inspect a worker/node.

## Milestone 3 - Network topology

- [x] Graph domain/schema with persisted, explicit demo connections.
- [x] Read-only graph and connections APIs.
- [x] SVG topology renderer in the production Network page.
- [x] Node selection/details pane.
- [-] Recorded connections table; route observations are pending.
- [x] Textual accessibility fallback.

Acceptance: topology is generated from graph data and remains usable without animation/WebGL.

## Milestone 4 - Automations and audit

- [ ] Workflow schema/migrations.
- [ ] Manual and cron triggers.
- [ ] Ordered action steps.
- [ ] Workflow runner.
- [ ] Run history.
- [ ] Append-only audit events.
- [ ] Automations screen.

Acceptance: a demo workflow can run end-to-end and produce auditable step results.

## Milestone 5 - Agent foundation

- [ ] Agent binary/service.
- [ ] Enrollment/bootstrap.
- [ ] Heartbeat/capability report.
- [ ] Basic host metrics.
- [ ] Typed action queue/result.
- [ ] Agent offline/stale handling.
- [ ] Linux amd64/arm64 build.
- [ ] Windows service packaging approach.

Acceptance: one real node appears from a live agent and reports metrics without inbound public ports.

## Milestone 6 - Tailscale + Docker read-only

- [x] Opt-in Tailscale read-only device discovery adapter (local tests; Home GW activation pending).
- [ ] Reconcile discovered tailnet nodes with existing inventory.
- [ ] Docker inventory through agent.
- [ ] Real containers on Node Detail.
- [ ] Integration health UI.

Acceptance: live private nodes and containers are visible; provider outage degrades gracefully.

## Milestone 7 - Pangolin + service exposure

- [ ] Pangolin read-only adapter.
- [ ] Exposed-resource inventory.
- [ ] Network graph exposure edges.
- [ ] Deep links.

Acceptance: owner can see which services are externally exposed and through what edge relationship.

## Milestone 8 - OpenCode compute

- [ ] OpenCode adapter contract finalized against deployed environment.
- [ ] Live worker discovery.
- [ ] Live job discovery.
- [ ] Submit/cancel only after security review.
- [ ] Placement integration where appropriate.

Acceptance: Compute page reflects real OpenCode worker/job state.

## Milestone 9 - safe actions

- [ ] Wake-on-LAN via appropriate LAN agent.
- [ ] Drain/undrain.
- [ ] Restart approved services/containers.
- [ ] Backup run.
- [ ] Confirmation UX.
- [ ] Idempotency and action audit.

Acceptance: actions are capability-scoped and every execution is auditable.

## Milestone 10 - authentication/security hardening

- [ ] OIDC/AuthentiK integration.
- [ ] Scopes/roles.
- [ ] Session security.
- [ ] CSRF protections.
- [ ] secret-reference system.
- [ ] rate limits on auth/action endpoints.
- [ ] security review checklist.

Public Pangolin exposure should not be enabled before this milestone is complete.

## Milestone 11 - backup, polish, owner release

- [ ] Application DB backup/restore command.
- [ ] Restore test procedure.
- [ ] settings/integration health views.
- [ ] error/empty/stale state polish.
- [ ] tablet layout pass.
- [ ] documentation pass.
- [ ] release build + checksums.

Acceptance: clean install, backup, restore, upgrade, and normal operating procedures are documented and tested.
