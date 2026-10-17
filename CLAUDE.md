# Claude Code Delegation Brief - oxnet.work

You are a supporting implementation agent. **Codex is the primary integrator and architecture owner.** Read `AGENTS.md` and the relevant `/docs` file before starting any task.

## Collaboration rules

- Work only on a clearly bounded task assigned by Codex or the owner.
- Do not change architecture, runtime stack, database choice, authentication model, or API versioning without an explicit decision recorded in `docs/DECISIONS.md`.
- Prefer small PR-sized changes.
- Do not duplicate a feature Codex is actively building.
- Every change must include tests or a concise explanation of why tests are not applicable.
- Never add a heavy dependency merely to reduce implementation effort.
- Never place infrastructure credentials in code, fixtures, screenshots, or docs.

## Recommended delegated work packages

### CC-01 - Frontend component library

Build reusable components matching `docs/UI_SPEC.md`:

- AppShell
- Sidebar/NavItem
- Topbar/Search/HealthPill
- Card/StatCard
- StatusDot/StatusChip
- MetricBar
- DataTable
- EmptyState/ErrorState/StaleBadge
- ConfirmDangerDialog
- CommandPalette shell

Deliver Storybook only if it does not materially increase runtime/dependency weight; otherwise create a `/dev/components` route available in development mode.

Acceptance:
- keyboard accessible;
- visible focus states;
- no hardcoded host names in components;
- components consume typed props;
- dark design tokens only come from the shared theme.

### CC-02 - SVG topology renderer

Implement the v1 topology renderer described in `docs/UI_SPEC.md`.

Requirements:
- input is a typed graph JSON structure;
- layered/hierarchical layout for small networks;
- zoom-to-fit and reset;
- selectable nodes;
- no canvas/WebGL requirement;
- no physics simulation;
- stable coordinates for unchanged graph input;
- accessible textual fallback/list.

Do not couple it directly to Tailscale/Pangolin APIs.

### CC-03 - OpenAPI client/types

From `specs/openapi.yaml`, generate or hand-maintain a minimal typed frontend client. Avoid bloated generators if a small fetch wrapper is sufficient.

Requirements:
- typed success/error handling;
- AbortSignal support;
- auth credentials policy documented;
- SSE helper separate from normal REST client.

### CC-04 - Workflow editor UI

Implement the lightweight trigger -> steps -> outcome editor/viewer.

Do not build a full freeform node editor. v1 uses ordered steps, simple cards, and arrows. Support create/edit of:
- manual trigger;
- cron trigger;
- health-condition trigger;
- action steps;
- notify step;
- success/failure handling.

### CC-05 - Test fixtures and visual smoke tests

Create deterministic fixtures mirroring domain shapes without copying exact mockup values. Add Playwright tests for:
- Overview loads;
- Compute workers/jobs render;
- Node Detail handles missing metric;
- Network node selection changes details;
- Automations manual run confirmation;
- destructive action confirmation.

## Tasks Claude Code should NOT own unless explicitly reassigned

- authentication/security architecture;
- database migration strategy;
- remote execution design;
- secret storage;
- agent trust/bootstrap protocol;
- scheduler placement semantics;
- destructive backend actions;
- release signing/update mechanism.

These remain with Codex because they cross multiple security boundaries.

## Handoff format back to Codex

For every delegated task, return:

1. files changed;
2. behavior added;
3. dependencies added/removed;
4. tests run and results;
5. unresolved questions;
6. any deviation from the brief;
7. screenshots for UI work where practical.
