# oxnet.work - Initial Agent Task Board

## Codex - primary

- C0: repository scaffold, build, Docker, CI
- C1: Go config + SQLite + migrations
- C2: domain types/repositories/demo provider
- C3: REST + SSE API foundation
- C4: Overview API integration
- C5: compute/job domain and placement tests
- C6: agent enrollment/protocol/security foundation
- C7: provider adapters and integration manager
- C8: authentication/OIDC/security hardening
- C9: release/backup/restore tooling

## Claude Code - safe delegation candidates

- CC-01: reusable frontend component library
- CC-02: SVG topology renderer
- CC-03: typed frontend API client
- CC-04: workflow viewer/editor UI
- CC-05: deterministic UI fixtures and Playwright coverage

## Delivered by Claude Code at the owner's direction (needs Codex architecture review)

- M4 Automations and audit (18 October 2026, while Codex was paused): `internal/workflow` (model, cron, store, runner, scheduler, audit), migration 007, REST/SSE endpoints, CSRF enforcement for `/api/` mutations, and the Automations page. Design is recorded in ADR-013. This went beyond CC-04: it includes backend scope that CLAUDE.md normally reserves for Codex (schema, scheduler semantics, mutation gate). Please review those decisions before building on them, particularly ADR-013's mutation gate and the one-run-per-workflow rule.
- CC-04 (workflow viewer/editor UI): the editor and diagram already existed in `web/src/features/workflows`; they are now wired to real data on the Automations page.
- CC-05 gap: `web/tests/automations.spec.ts` was added but has not been run (no Chromium available in the authoring environment). Run `npm run smoke` once in CI.

## Coordination

Codex should assign one Claude work package at a time and integrate only after tests and architecture review. Do not let two agents edit the same files concurrently unless explicitly coordinated.
