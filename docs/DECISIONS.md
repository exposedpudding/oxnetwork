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

## ADR-014 - Verified service health through fixed HTTP probes

**Status:** Implemented by Claude Code on branch `claude/service-health` on 19 October 2026, following the slice Codex scoped in `TASK_BOARD.md`. Awaiting Codex review; not yet deployed.

**Decision:**

- **What a probe is.** One `GET` from inside the oxnet-work container to an operator-configured URL. No credentials are sent, redirects are not followed, TLS is always verified (there is no insecure option), proxy environment variables are ignored, and at most 1 KiB of the response body is read and discarded. The default deadline is 3 seconds (1 to 10 allowed).
- **What counts as healthy.** The endpoint returned one of `expect_status` (default 200 or 204) before the deadline. Anything else is `unhealthy` with a short `detail` (`HTTP 502 (expected 200/204)`, `no response within 3s`, `connection refused`, `name did not resolve`, `TLS certificate not trusted or does not match the name`). A redirect is unhealthy unless the operator lists that status. A recent host-agent report never makes a service healthy.
- **Fixed configuration only.** Probes come from `service_probes` in the config or from the YAML file named by `OXNET_SERVICE_PROBES_FILE`; they cannot be created or changed through the API or UI. Validation caps them at 20, requires lowercase ids, http or https URLs without credentials, query strings or fragments, and rejects unknown keys. Probes require live inventory (not demo mode).
- **Recorded data.** Each service observation stores `endpoint` (the address checked, query string removed) and `detail` next to the existing health, source (`http-probe`) and observation time. Migration 008 adds the two columns. Service ids are `probe-<id>`. Observation time is when the answer arrived.
- **Staleness.** A result is stale after three missed probe rounds (default 30 second rounds, so 90 seconds), computed when read like every other observation, so a stopped prober cannot leave a service showing healthy.
- **Reconciliation.** `Repository.ReplaceSnapshot` deletes every service row on each Tailscale refresh. The refresh therefore appends the prober's latest results to the snapshot before replacing it. Separately, each probe round is published with `UpsertSnapshot`, which never deletes other rows, so service state keeps updating while the Tailscale API is down and a failed Tailscale refresh no longer hides it. Services removed from configuration disappear at the next successful Tailscale refresh.
- **Not decided here.** Probing other hosts' services from the web container, TCP-only checks, and probe-driven workflows (a `health_condition` trigger on a service). Docker inventory is a separate provider (ADR-015).

**Reason:** Service health had to be separate from agent heartbeats, and the web server should only make outbound requests to endpoints the owner chose. Putting probes in the existing refresh path alone would have lost service rows during any Tailscale outage, and writing them separately without changing reconciliation would have lost them within a minute.

**Home GW use:** probe the private edge names (`https://oxnet.work/readyz`, `https://auth.oxnet.work/-/health/live/`). `deploy/home-gw/live-service-probes.compose.yml` adds the `oxnet.work` alias to the Caddy edge so the container resolves both names to it, which exercises the proxy, its routing and the trusted certificate. Opt-in; without the overlay nothing changes.

## ADR-015 - Docker container inventory (proposal)

**Status:** Proposed by Claude Code on 19 October 2026 for Codex to accept, change or reject. No code written.

**Constraint:** ADR-006 and ADR-012 rule out mounting a Docker socket into the web server. Container health must not be inferred from an HTTP probe (`TASK_BOARD.md`).

**Options considered:**

1. *Mount `docker.sock` into oxnet-work.* Rejected. It is root on the host, and the container is the most exposed process.
2. *Give the existing host collector Docker access.* Rejected. It runs as a `DynamicUser`; adding the `docker` group would make a telemetry-only unit root-equivalent.
3. *Run `tecnativa/docker-socket-proxy` with `CONTAINERS=1`.* Better, but `CONTAINERS=1` also allows `GET /containers/{id}/json`, which returns each container's environment variables and so can expose secrets.
4. **Recommended: a small allow-list bridge.** A new restricted systemd unit (or a container with the socket mounted read-only) listens on a Unix socket or loopback port and forwards exactly one request, `GET /containers/json`, rejecting every other path and method. The existing collector reads that bridge and posts a container list to a new `POST /internal/v1/docker-report/{node-id}` using a token of its own (separate file, so a leaked host-metrics token cannot forge container state).

**Data to record, if accepted:** container name, image reference, state, status text, health status if the image defines one, creation time, and published ports. Never environment variables, mounts, command lines or labels except `com.docker.compose.project` and `com.docker.compose.service`. Store per node in a new table with the report time; stale after 90 seconds like host reports; show on Node Detail ("Containers") and add `GET /api/v1/nodes/{id}/containers`.

**Out of scope:** any container action (restart, stop, logs). Those belong to Milestone 9 and need typed actions, confirmation and idempotency.

**Open questions for Codex:** whether the bridge should be a separate Go binary in `cmd/` or a mode of `cmd/host-agent`; whether container health should feed `health_condition` workflow triggers.
