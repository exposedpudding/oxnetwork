# Codex Handoff - oxnet.work

## Objective

You are the primary implementation agent for **oxnet.work**, a lightweight self-hosted command centre for the owner's private network, remote access, container estate, shared OpenCode compute, monitoring, and automation.

Read `AGENTS.md` first, then all files in `/docs` before implementing Milestone 0. The agreed UI mockups are in `/docs/images`.

## What you own

Codex owns:

- repository architecture and build system;
- Go control-plane service;
- SQLite schema/migrations;
- REST/SSE API;
- React/TypeScript frontend shell and primary screen integration;
- node-agent protocol and agent implementation;
- integration interfaces/adapters;
- orchestration/job scheduling;
- audit/event system;
- Docker packaging and release workflow;
- integration of delegated Claude Code work after review.

## What Claude Code may own

`CLAUDE.md` defines bounded tasks that can be delegated. Treat Claude output as a contribution, not as authoritative. Review it against `AGENTS.md`, run tests, and integrate selectively.

## Implementation strategy

### Stage A - foundation

1. Create the monorepo skeleton.
2. Implement config loading and validation.
3. Add SQLite + migrations.
4. Define domain models and repository interfaces.
5. Implement a demo provider returning deterministic fixtures.
6. Build the React shell and shared components.
7. Serve compiled frontend assets from Go.
8. Add `/healthz`, `/readyz`, `/api/v1/system`, and SSE event stream.
9. Add Docker multi-stage build.

### Stage B - usable dashboard

1. Overview screen.
2. Nodes and node detail API.
3. Compute aggregation and workers/jobs API.
4. Network graph API using fixture relationships.
5. Workflows model and basic in-process runner.
6. Audit log.

### Stage C - live infrastructure

Integrate real providers one at a time. Each adapter must expose health and last successful sync time. Keep fixture/demo mode available for development and screenshots.

## Required quality bar

- No hardcoded mockup data in production paths.
- No direct Docker socket mount into the internet-facing control plane by default.
- No credentials rendered to the browser.
- No arbitrary command execution endpoint.
- All dangerous actions are capability-gated, confirmation-gated, and audited.
- UI must indicate stale data.
- External provider failures must be isolated.

## Working method

For each milestone:

1. Restate scope in the PR/commit description.
2. Implement the smallest coherent vertical slice.
3. Add/update tests.
4. Run lint/test/build.
5. Update `docs/DECISIONS.md` for material architectural choices.
6. Update `docs/ROADMAP.md` status markers.
7. Do not silently expand scope.

## Initial deliverables

A first implementation should produce:

- `docker compose up -d` starts the app;
- `http://localhost:8080` serves the UI;
- demo mode renders Overview with deterministic fixture data;
- `/api/v1/nodes` and `/api/v1/services` work;
- SSE emits health/activity changes;
- SQLite DB persists configuration/audit/workflows;
- basic auth bootstrap is implemented securely or the app is hard-bound to private interfaces until OIDC milestone.

See `/docs/ROADMAP.md` for detailed acceptance criteria.
