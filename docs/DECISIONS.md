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
