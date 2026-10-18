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

## Delivered by Claude Code at the owner's direction; reviewed and merged by Codex

- M4 Automations and audit (18 October 2026): `internal/workflow` (model, cron, store, runner, scheduler, audit), migration 007, REST/SSE endpoints, CSRF enforcement for `/api/` mutations, and the Automations page. Design is recorded in ADR-013. Codex reviewed the schema, mutation gate, read-only action boundary, and one-run-per-workflow rule in PR #1; fixed step timeout propagation and added a regression test. CI passed, PR #1 merged to `main`, and the exact merge tree was deployed on Home GW. The live manual and cron runs succeeded. Next: service health and Docker inventory providers, then backup/restore hardening.
- CC-04 (workflow viewer/editor UI): the editor and diagram already existed in `web/src/features/workflows`; they are now wired to real data on the Automations page.
- CC-05: `web/tests/automations.spec.ts` covers the Automations page (summary, confirmed manual run with step results, creating a workflow). The production smoke suite (8 tests) and the fixture harness (8 tests) pass in headless Chromium against the real container image. `main` was already failing CI at the browser smoke step before this work because `compute-detail.spec.ts` asserted a node-actions string the UI no longer shows; that assertion is updated here.

## Coordination

Codex should assign one Claude work package at a time and integrate only after tests and architecture review. Do not let two agents edit the same files concurrently unless explicitly coordinated.

## Next live-data slice: verified service health

- Add fixed, operator-configured read-only probes for private services, beginning with oxnet.work and Authentik on Home GW. Use short deadlines, verified TLS where applicable, no redirects, and no arbitrary dashboard-supplied URLs. Record the probed endpoint and observation time; a recent host-agent report must not count as service health.
- Merge probe observations with the Tailscale snapshot before `Repository.ReplaceSnapshot` runs. That method deletes `inventory_services` on each refresh, so writing service rows separately without changing reconciliation would lose them within a minute. Test refresh, probe failure, timeout, and stale-result behavior.
- Keep Docker inventory as a separate opt-in provider. Do not mount the unrestricted Docker socket into the web server or infer container health from an HTTP probe. Decide the least-privilege host collector or filtered Docker API path before implementation.
