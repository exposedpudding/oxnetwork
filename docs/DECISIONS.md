# Architectural Decision Log - oxnet.work

Record material decisions here. Do not rewrite history; append superseding decisions.

## ADR-001 - Go control plane

**Status:** Accepted

**Decision:** Use Go for the control plane and node agent.

**Reason:** Small static binaries, low idle resource usage, straightforward concurrency/networking, strong cross-platform support, and simple containerization.

## ADR-002 - React + TypeScript + Vite frontend

**Status:** Accepted

**Decision:** Use React/TypeScript compiled by Vite and serve production assets from the Go control plane.

**Reason:** Rich enough for the approved UI while keeping the production deployment to one service.

## ADR-003 - SQLite for v1

**Status:** Accepted

**Decision:** Use SQLite with WAL and migrations. No external database requirement for v1.

**Reason:** The project is owner/small-operator scale and benefits more from low operational overhead than from distributed DB features.

## ADR-004 - SSE before WebSockets

**Status:** Accepted

**Decision:** Use Server-Sent Events for dashboard updates.

**Reason:** Most realtime traffic is server -> browser. SSE is simpler, proxy-friendly, and adequate for v1.

## ADR-005 - No arbitrary shell execution

**Status:** Accepted

**Decision:** Agent actions are typed and enumerated. v1 will not expose a generic shell command API.

**Reason:** Greatly reduces command injection and privilege risk.

## ADR-006 - Docker via node agent

**Status:** Accepted

**Decision:** Prefer agent-local Docker access rather than mounting remote Docker sockets into the control plane.

**Reason:** Narrows privilege and network exposure.

## ADR-007 - Lightweight SVG topology

**Status:** Accepted

**Decision:** Build v1 topology with SVG nodes/edges and deterministic layout, not WebGL/physics.

**Reason:** Lower resource use, predictable UX, easier accessibility and testing.

## ADR-008 - Local demo before authentication

**Status:** Accepted for the initial implementation

**Decision:** Run the first Overview build with a simulated, read-only provider. Bind the native server to loopback by default and publish the container port only on host loopback. Keep action endpoints and live integrations absent until authentication and authorization are built.

**Reason:** This makes Milestones 0–1 testable without suggesting that fictional inventory is live or exposing an unauthenticated operations console.

## ADR-009 - Home GW identity and command centre placement

**Status:** Accepted by owner for deployment; activation pending host access and validation

**Decision:** Host both the Authentik identity server and oxnet.work on Home GW. Use private Tailscale reachability and HTTPS names `auth.oxnet.work` (proposed) and `oxnet.work` (owner specified). Do not create public ingress by default. Authentik is a separate stack service with its own PostgreSQL database; oxnet.work remains a single Go/SQLite control-plane container.

**Reason:** The owner explicitly chose co-location. Authentik requires PostgreSQL, which is an exception for the identity service to the project's lightweight, SQLite-only **control-plane** baseline. Home GW capacity and existing services must be checked before activation.

## ADR-010 - Private Home GW names through Tailscale Service

**Status:** Implemented and verified on 18 October 2026

**Decision:** Serve `oxnet.work` and `auth.oxnet.work` at a dedicated Tailscale Service VIP through a loopback-bound Caddy edge on Home GW. Resolve both names to that VIP through tailnet split DNS. Use a trusted certificate issued through Spaceship DNS-01 and keep both names without public A/AAAA answers. Run Authentik and its PostgreSQL database alongside the Go/SQLite app. Keep all inventory simulated until an owner-approved live adapter is enabled.

**Reason:** The owner requested private access with familiar HTTPS names while preserving the existing node Serve routes and avoiding public ingress. The verified live state and remaining checks are in `deploy/home-gw/DEPLOYMENT_STATE.md`.

## ADR-011 - Scoped live Tailscale device inventory

**Status:** Implemented and verified on 18 October 2026

**Decision:** Activate the read-only Tailscale adapter on Home GW using an OAuth trust credential scoped only to `devices:core:read`. The app exchanges the client credential for short-lived access tokens and renews them automatically. Keep the OAuth secret in a root-owned Compose secret file on Home GW. Show live device identity, tailnet addresses and Tailscale last-seen values without presenting them as host health or resource metrics.

**Reason:** The owner requested live dashboard data after verifying private access. Traditional Tailscale API tokens grant broad API access; the scoped OAuth client limits the app to the device list endpoint. Separate adapters are needed for CPU, RAM, services, jobs and network connections.

## ADR-012 - Per-node host metrics agents

**Status:** Implemented for Home GW and one Windows 11 PC on 18 October 2026

**Decision:** Use an outbound-only Go collector on Linux or Windows for CPU, physical RAM, system/root disk and capacity. Each Tailscale inventory node has its own telemetry-submit token; the server fixes report identity from the URL and a matching root-owned token file, then timestamps the receipt. Host reports are stored separately from Tailscale inventory so a device refresh cannot erase them. After 90 seconds without a report, the node becomes stale. Keep service health, jobs, containers, and network links unavailable until their own adapters report them.

**Reason:** Tailscale device discovery cannot measure host resources. A small local agent avoids broad Docker socket access and inbound ports, while per-node tokens limit the damage of one compromised collector. `healthy` labels recent reporting only, so the UI states that service health remains unverified.

## ADR-013 - Automations: workflows, runner and audit (Milestone 4)

**Status:** Accepted by owner, reviewed and merged by Codex in PR #1 on 18 October 2026. Manual and scheduled read-only runs verified on Home GW.

**Decision:**

- **Definition shape.** A workflow is `{id, name, description, enabled, trigger, steps[], on_success{notify_channel}, on_failure{behavior, notify_channel}}`, exactly the shape of the provisional frontend types. It is stored as JSON in one SQLite row; ordered steps only, no graph.
- **Triggers.** `manual`, `cron` (standard 5-field expression, evaluated in UTC once per minute, no catch-up of missed minutes after a restart), and `health_condition` (node is `unhealthy`, `degraded` or `stale` continuously for `for_seconds`; edge-triggered, so it fires once and re-arms only after the condition clears).
- **Steps.** `action` (typed, enumerated, executed by the in-process runner) and `notify`. There is no shell, script or HTTP step type (ADR-005).
- **Actions in v1.** Only read-only inventory checks execute: `check_node_health` and `check_node_freshness`. Mutating actions (Wake-on-LAN, drain, service restart, backup) are not offered and workflows referencing them are rejected with `ACTION_NOT_SUPPORTED` until Milestone 9 delivers capability checks, confirmation UX and idempotency.
- **Notifications.** The only channel is `activity` (recorded on the run and in the audit log). External channels need secret storage, which remains a separate decision.
- **Run states.** `queued -> running -> succeeded | failed | cancelled`. Step states: `pending | running | succeeded | failed | skipped`. `cancelled` is reserved; there is no cancel endpoint yet. Runs left `queued` or `running` by a restart are marked `failed` with code `INTERRUPTED`.
- **Failure handling.** `stop` marks remaining steps `skipped` and the run `failed`. `continue` runs the remaining steps and the run still ends `failed` if any step failed. No automatic retries. Each step has a 30 second timeout and each run a 5 minute timeout. The runner applies the step deadline to inventory reads as well as action execution (review fix in PR #1).
- **Concurrency.** One active run per workflow. A second start returns `409 RUN_IN_PROGRESS`; scheduled triggers that collide are skipped and audited.
- **Audit.** `audit_events` is append-only, enforced by SQLite triggers that abort `UPDATE` and `DELETE`. Workflow create, update, enable, disable, run start and run finish each append an event with actor, target, outcome and sanitised detail.
- **Mutation gate.** Mutating endpoints require either an authenticated session or a loopback-bound server (`mutations_enabled` in `/api/v1/system`). They additionally require a JSON content type and a same-origin `Origin` header when one is sent. With OIDC enabled the auth middleware now enforces `X-CSRF-Token` on every non-safe `/api/` request. This resolves the CSRF gap noted in ADR-008 for these endpoints; role and scope checks remain Milestone 10.
- **Demo data.** In demo mode three clearly simulated workflows are seeded when the table is empty, and they are removed when the server switches to a live inventory source.

**Reason:** Milestone 4 needs a demo workflow that runs end to end with auditable step results while staying inside the project's security gates. Limiting executable actions to read-only checks delivers that without pre-empting the authorisation, confirmation and idempotency work in Milestones 9 and 10.
