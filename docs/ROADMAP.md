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

- [x] Workflow schema/migrations (migration 007; renames the unused 001 audit placeholder).
- [x] Manual and cron triggers; health-condition triggers (edge-triggered) also implemented.
- [x] Ordered action steps; only read-only checks execute until Milestone 9 (ADR-013).
- [x] Workflow runner (per-step and per-run timeouts, one run per workflow, interrupted-run recovery).
- [x] Run history with step results.
- [x] Append-only audit events (enforced by SQLite triggers).
- [x] Automations screen, including create/edit, enable/disable and confirmed manual runs.
- [ ] Not yet: run cancellation, retries, external notification channels (need secret storage) and role and scope checks (Milestone 10).

Acceptance: a demo workflow can run end-to-end and produce auditable step results. Met: the simulated demo workflows run against demo inventory and record step outcomes and audit events.

## Milestone 5 - Agent foundation

- [x] Go agent binary for Linux and Windows 11; Home GW systemd service and Windows user-logon installer.
- [x] Manual per-node telemetry token enrollment; automated bootstrap remains.
- [x] Periodic report and monitor capability for enrolled nodes; richer capabilities remain.
- [x] Basic host CPU, physical RAM, system/root disk and capacity metrics.
- [ ] Typed action queue/result.
- [x] Agent report stale after 90 seconds; explicit offline state remains.
- [x] Linux amd64 build; arm64 build and on-device test remain.
- [x] Windows 11 per-user logon packaging; a boot-time service remains optional.

Acceptance met for Home GW, OX-C-01, the OVH VPS and three Windows 11 PCs (DESKTOP-HHBRUDD, OX-01 and OX-02): all six report metrics without inbound public ports. Service health, jobs and Docker data remain separate integration work.

## Milestone 6 - Tailscale + Docker read-only

- [x] Opt-in Tailscale read-only device discovery adapter (Home GW active with OAuth `devices:core:read`).
- [x] Verified service health: fixed, operator-configured read-only HTTP probes, merged into the inventory snapshot (ADR-014; branch `claude/service-health`, awaiting Codex review and deployment).
- [ ] Reconcile discovered tailnet nodes with existing inventory.
- [ ] Docker inventory through agent (design options and a recommendation are in ADR-015, status Proposed).
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

- [x] OIDC/Authentik integration (verified on private Home GW with owner MFA; see `deploy/home-gw/DEPLOYMENT_STATE.md`).
- [ ] Scopes/roles.
- [ ] Session security.
- [ ] CSRF protections.
- [ ] secret-reference system.
- [ ] rate limits on auth/action endpoints.
- [ ] security review checklist.

Public Pangolin exposure should not be enabled before this milestone is complete.

## Milestone 11 - backup, polish, owner release

- [x] Application DB backup/restore command (`oxnet-work backup|verify|restore`, ADR-016; branch `claude/backup-restore`, awaiting Codex review; Home GW scripts and an optional systemd timer are shipped but not deployed or installed).
- [x] Restore test procedure ([docs/BACKUP_RESTORE.md](BACKUP_RESTORE.md)), executed once on 18 October 2026: a real Home GW backup restored on OX-C-01 and read back through the app. Still open from the standing list: restoring Authentik, certificates and secrets on a separate host, a second off-host copy, and repeating the app restore on Home GW itself after deployment.
- [ ] settings/integration health views.
- [ ] error/empty/stale state polish.
- [ ] tablet layout pass.
- [ ] documentation pass.
- [ ] release build + checksums.

Acceptance: clean install, backup, restore, upgrade, and normal operating procedures are documented and tested.
