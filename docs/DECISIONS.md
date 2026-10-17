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
