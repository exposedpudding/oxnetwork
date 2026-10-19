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

## Backup and restore (Milestone 11)

**Status (19 October 2026): implemented by Claude Code on branch `claude/backup-restore`, stacked on `claude/service-health` (PR #2); awaiting Codex review. Not merged, not deployed.** See ADR-016 and [docs/BACKUP_RESTORE.md](docs/BACKUP_RESTORE.md). What exists: `internal/backup` and the `backup`, `verify` and `restore` subcommands of the server binary; session-free, integrity-checked, checksummed snapshots taken while the app runs; restore with a safety copy and a refusal of newer-schema backups; `deploy/home-gw/backup-oxnet.sh`, `restore-oxnet.sh` and an optional `oxnetwork-backup.timer`. Verified: `gofmt`, `go vet`, full Go suite with `-race`; the scripts run against a sandbox container; and a **real Home GW database backup restored and read back on OX-C-01** (results in the doc).

**For Codex when you return:** (1) review ADR-016, especially the root-only verify and restore containers and the decision to remove sessions from backups; (2) deploy: the scripts call `/app/oxnet-work backup`, so Home GW needs a rebuilt image first; then run `backup-oxnet.sh` once by hand, test `verify` on the copy, and only then decide whether to install the timer; (3) the standing owner items remain open: a second off-host copy, restoring Authentik, certificates and secrets on a separate host, and `DESKTOP-HHBRUDD` logon reliability. Merge order: PR #2 first, then the PR for this branch (it is based on PR #2's branch and retargets to `main` when that merges).

## Next live-data slice: verified service health

**Status (19 October 2026): implemented by Claude Code on branch `claude/service-health`, awaiting Codex review. Not merged, not deployed.** Design and trade-offs are in ADR-014; the Docker inventory decision still open below is written up as ADR-015 (Proposed). What changed: `internal/probe` (fixed read-only HTTP probes), migration 008 (`endpoint` and `detail` on service observations), `service_probes` config loaded from the file named by `OXNET_SERVICE_PROBES_FILE`, probe results merged into every Tailscale refresh and also published per probe round, `endpoint`/`detail` on `/api/v1/services`, and Overview rows that show them. Home GW opt-in files: `deploy/home-gw/live-service-probes.compose.yml` and `service-probes.example.yml`. Verified: `gofmt`, `go vet` and the full Go suite with `-race`, 64 frontend tests, typecheck, build, and the Compose merge with and without the overlay. The probe endpoints were checked against the live private edge (`https://oxnet.work/readyz` returns 200; Authentik `/-/health/live/` returns 200 on this version, not 204).

**For Codex when you return:** (1) review the PR and ADR-014, in particular that probes run from inside the web container and a failed Tailscale refresh no longer hides service state; (2) to deploy, follow "Optional service probes" in `deploy/home-gw/README.md` (the five-file Compose command) and keep the earlier backup-then-build habit; (3) decide ADR-015 before any Docker work; (4) the standing items in `DEPLOYMENT_STATE.md` (second off-host backup copy, restore test, `DESKTOP-HHBRUDD` logon reliability) are unchanged by this work.

- Add fixed, operator-configured read-only probes for private services, beginning with oxnet.work and Authentik on Home GW. Use short deadlines, verified TLS where applicable, no redirects, and no arbitrary dashboard-supplied URLs. Record the probed endpoint and observation time; a recent host-agent report must not count as service health.
- Merge probe observations with the Tailscale snapshot before `Repository.ReplaceSnapshot` runs. That method deletes `inventory_services` on each refresh, so writing service rows separately without changing reconciliation would lose them within a minute. Test refresh, probe failure, timeout, and stale-result behavior.
- Keep Docker inventory as a separate opt-in provider. Do not mount the unrestricted Docker socket into the web server or infer container health from an HTTP probe. Decide the least-privilege host collector or filtered Docker API path before implementation.
