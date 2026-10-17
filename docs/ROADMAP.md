# Implementation Roadmap - oxnet.work

Status legend: `[ ]` not started, `[-]` in progress, `[x]` complete.

## Milestone 0 - repository and runnable shell

- [ ] Create repo structure from `AGENTS.md`.
- [ ] Go module + server entrypoint.
- [ ] React/TypeScript/Vite app.
- [ ] Shared design tokens/layout shell.
- [ ] SQLite connection + migrations framework.
- [ ] Config loader + validation.
- [ ] Structured logging + request IDs.
- [ ] `/healthz`, `/readyz`, `/api/v1/system`.
- [ ] Multi-stage Dockerfile.
- [ ] Docker Compose example.
- [ ] CI for Go tests + frontend build/tests.

Acceptance: one command starts a blank but branded oxnet.work shell and health endpoints pass.

## Milestone 1 - demo vertical slice / Overview

- [ ] Domain types for Node, Service, Health, Metric, Activity.
- [ ] Repository layer.
- [ ] Deterministic demo provider.
- [ ] Nodes/services/activity endpoints.
- [ ] SSE event stream.
- [ ] Overview UI matching approved direction.
- [ ] Fresh/stale/unknown states.
- [ ] Playwright Overview smoke test.

Acceptance: Overview is fully populated from API data, not frontend constants.

## Milestone 2 - Compute and Node Detail

- [ ] Worker/job domain.
- [ ] Compute aggregation endpoint.
- [ ] Placement policy package + tests.
- [ ] Compute screen.
- [ ] Node detail endpoints/screens.
- [ ] Capability model.
- [ ] Read-only container/job/log fixtures.

Acceptance: owner can understand aggregate capacity and inspect a worker/node.

## Milestone 3 - Network topology

- [ ] Graph domain/schema.
- [ ] Graph API.
- [ ] SVG topology renderer.
- [ ] Node selection/details pane.
- [ ] Connections/routes table.
- [ ] Textual accessibility fallback.

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

- [ ] Tailscale discovery adapter.
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
