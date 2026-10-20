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
- [x] Run retention: finished runs older than 90 days (configurable) are trimmed, keeping each workflow's newest 20; audit is never pruned (ADR-018; merged in PR #7 and live on Home GW).
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
- [x] Verified service health: fixed, operator-configured read-only HTTP probes, merged into the inventory snapshot (ADR-014; oxnet.work and Authentik probes live and healthy on Home GW).
- [ ] Reconcile discovered tailnet nodes with existing inventory.
- [x] Docker inventory through a single-purpose reporter (ADR-019, built on ADR-015; PR #8 merged, live on Home GW and OX-C-01, opt-in elsewhere).
- [x] Real containers on Node Detail (Containers card; shows reported, stale, empty and never-reported states).
- [x] Integration health UI: `GET /api/v1/integrations` and an Integrations page (PR #6 merged and live).

Acceptance: live private nodes and containers are visible; provider outage degrades gracefully.

## Milestone 7 - Pangolin + service exposure

- [ ] Pangolin read-only adapter.
- [ ] Exposed-resource inventory.
- [ ] Network graph exposure edges.
- [ ] Deep links.

Acceptance: owner can see which services are externally exposed and through what edge relationship.

## Milestone 8 - OpenCode compute

- [x] OpenCode adapter contract finalized against deployed environment ([OPENCODE.md](OPENCODE.md), verified against the real server on OX-C-01, OpenCode 1.19.3).
- [x] Live worker discovery (read-only; `internal/integrations/opencode`, ADR-017; PR #5 merged and OX-C-01 provider live).
- [x] Live job discovery (root sessions across every project as jobs `running` or `idle`).
- [ ] Submit/cancel only after security review.
- [ ] Placement integration where appropriate.
- [ ] Multi-node pool, primary election and CPU-pooled local models (design, needs more nodes).

Acceptance: Compute page reflects real OpenCode worker/job state. Home GW has `live-opencode.compose.yml` and a private Tailscale grant to OX-C-01 port 4096. Session counts are marked incomplete when any project read fails.

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

- [x] Application DB backup/restore command (`oxnet-work backup|verify|restore`, ADR-016; PR #3 merged, verified live backup on Home GW, daily timer enabled).
- [x] Restore test procedure ([docs/BACKUP_RESTORE.md](BACKUP_RESTORE.md)): app-only restore on OX-C-01 on 18 October; fresh full-stack set and isolated PostgreSQL, Authentik, TLS and app startup on OX-C-01 on 20 October. The Windows DPAPI encrypted copy passed decrypt-and-hash verification. Still open: a second encrypted copy with independent recovery and a test from that copy.
- [x] Integration health view (Milestone 6); settings view remains a separate future task.
- [ ] error/empty/stale state polish.
- [ ] tablet layout pass.
- [ ] documentation pass.
- [ ] release build + checksums.

Acceptance: clean install, backup, restore, upgrade, and normal operating procedures are documented and tested.
