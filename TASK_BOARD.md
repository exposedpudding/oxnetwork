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

## OpenCode compute providers (Milestone 8)

**Status (19 October 2026): implemented by Claude Code on branch `claude/opencode-adapter`, stacked on `claude/backup-restore` (PR #3), which is stacked on `claude/service-health` (PR #2); awaiting Codex review. Not merged, not deployed.** See ADR-017 and [docs/OPENCODE.md](docs/OPENCODE.md). What exists: `internal/integrations/opencode` (read-only client and poller), provider-neutral types in `internal/compute/provider.go`, `opencode_servers` config, `GET /api/v1/compute/providers`, real `jobs` and `active_jobs`, and the Compute page showing servers and sessions. Home GW opt-in files: `deploy/home-gw/live-opencode.compose.yml` and `opencode-servers.example.yml`.

**Important finding recorded for you:** OpenCode scopes sessions and status to a *project directory*; an adapter that does not visit every project reports nothing running while agents work. The first version had exactly this bug and was fixed after testing against the real server (details in OPENCODE.md). If you replace the adapter, keep a test with a busy session in a non-default project.

**For Codex when you return:** (1) review ADR-017, especially read-only scope, "idle is not a result", and in-memory-only job state; (2) to deploy, the owner must create `secrets/opencode_ox_c_01` (the OpenCode server password, `root:10001`, mode 640) and confirm a Tailscale grant lets Home GW reach `100.69.88.98:4096`; verify with `curl` from Home GW first; (3) job submit/cancel and placement are still open and need the Milestone 9 action framework; (4) a multi-node pool and primary election need more nodes (OX-C-02 is not racked yet). Merge order: #2, then #3, then the PR for this branch (each retargets automatically).

## Integration health (Milestone 6)

**Status (19 October 2026): implemented by Claude Code on branch `claude/integration-health`, stacked on `claude/opencode-adapter` (PR #5); awaiting Codex review. Not merged, not deployed.** What exists: `GET /api/v1/integrations` (`internal/api/integrations.go`) summarising the inventory source, host agents, service probes, OpenCode, automations and sign-in, `Repository.RefreshedAt` (so Tailscale staleness is measured from the last real refresh, not from host-agent writes), and an Integrations page with sidebar entry. `not_configured` is its own state so an absent source is not shown as failing. No ADR was added: it is read-only and derived from existing data, with no new dependency, secret or outbound call.

**For Codex when you return:** (1) review the staleness thresholds (180 s Tailscale, 90 s agents/probes, 45 s OpenCode), which mirror the existing polling intervals; (2) the page does not yet show per-host agent detail or Docker, which are separate milestones. Merge order: #2, #3, #5, then this PR.

## Windows agent on DESKTOP-HHBRUDD (19 October 2026)

Root cause confirmed and fixed: the agent had been installed from inside a packaged app, so its files and Run entry lived in that app's private store and vanished from the real profile. It was reinstalled from the real desktop shell and reports again. **Open:** confirm it starts by itself at the next sign-in. Details in `deploy/agents/README.md`. Lesson: install agents only from an ordinary PowerShell window, never from inside another app's terminal.

## Docker container inventory (Milestone 6)

**Status (19 October 2026): implemented by Claude Code on branch `claude/docker-inventory` at the owner's request; awaiting Codex confirmation. Not merged, not deployed. Nothing runs unless a host opts in.** See ADR-019 (it implements ADR-015, but as one reporter program instead of a bridge plus the collector) and [docs/DOCKER_INVENTORY.md](docs/DOCKER_INVENTORY.md). What exists: `cmd/docker-reporter` (one request on the Docker socket: `GET /containers/json`), `POST /internal/v1/docker-report/{id}` with its own per-node token directory, migration 009, `GET /api/v1/nodes/{id}/containers`, a Containers card on Node Detail, a Docker line on the Integrations page, and the systemd unit, install script, token script and Compose overlay under `deploy/`.

**For Codex when you return:** (1) ADR-015 was Proposed; the owner asked for the work, so review ADR-019 as the accepting decision, especially the choice of one reporter instead of a bridge and the root-equivalent `docker` group it needs; (2) the open question of reporter-as-mode-of-host-agent versus a separate binary is answered here as separate, to keep the metrics agent Docker-free; (3) to deploy: take a backup, add `live-docker-reporter.compose.yml`, rebuild (migration 009 runs on start), create a token per host, then install the reporter only on hosts where Docker socket access is acceptable; (4) container health triggers for workflows and any container action are not built.

## Automation run retention (Milestone 4)

**Status (19 October 2026): implemented by Claude Code on branch `claude/run-retention` (from `main`) at the owner's request; awaiting Codex review. Not merged, not deployed.** See ADR-018. `Store.PruneRuns` and a once-per-6-hours call from the scheduler trim finished runs older than `automation_run_retention_days` (default 90, range 7-3650) while always keeping each workflow's newest 20 finished runs and never touching queued or running rows. The append-only audit log is deliberately not pruned; each trimming adds one `workflow.runs.pruned` audit event.

**For Codex when you return:** (1) confirm the 90 day and newest-20 defaults and that leaving audit unbounded is acceptable until an archive step is designed; (2) first deploy will trim at startup, so take a backup first (`backup-oxnet.sh`); (3) there is no UI or API for retention, by design (fixed configuration).

## Backup and restore (Milestone 11)

**Status (19 October 2026): implemented by Claude Code on branch `claude/backup-restore`, stacked on `claude/service-health` (PR #2); awaiting Codex review. Not merged, not deployed.** See ADR-016 and [docs/BACKUP_RESTORE.md](docs/BACKUP_RESTORE.md). What exists: `internal/backup` and the `backup`, `verify` and `restore` subcommands of the server binary; session-free, integrity-checked, checksummed snapshots taken while the app runs; restore with a safety copy and a refusal of newer-schema backups; `deploy/home-gw/backup-oxnet.sh`, `restore-oxnet.sh` and an optional `oxnetwork-backup.timer`. Verified: `gofmt`, `go vet`, full Go suite with `-race`; the scripts run against a sandbox container; and a **real Home GW database backup restored and read back on OX-C-01** (results in the doc).

**For Codex when you return:** (1) review ADR-016, especially the root-only verify and restore containers and the decision to remove sessions from backups; (2) deploy: the scripts call `/app/oxnet-work backup`, so Home GW needs a rebuilt image first; then run `backup-oxnet.sh` once by hand, test `verify` on the copy, and only then decide whether to install the timer; (3) the standing owner items remain open: a second off-host copy, restoring Authentik, certificates and secrets on a separate host, and `DESKTOP-HHBRUDD` logon reliability. Merge order: PR #2 first, then the PR for this branch (it is based on PR #2's branch and retargets to `main` when that merges).

## Next live-data slice: verified service health

**Status (19 October 2026): implemented by Claude Code on branch `claude/service-health`, awaiting Codex review. Not merged, not deployed.** Design and trade-offs are in ADR-014; the Docker inventory decision still open below is written up as ADR-015 (Proposed). What changed: `internal/probe` (fixed read-only HTTP probes), migration 008 (`endpoint` and `detail` on service observations), `service_probes` config loaded from the file named by `OXNET_SERVICE_PROBES_FILE`, probe results merged into every Tailscale refresh and also published per probe round, `endpoint`/`detail` on `/api/v1/services`, and Overview rows that show them. Home GW opt-in files: `deploy/home-gw/live-service-probes.compose.yml` and `service-probes.example.yml`. Verified: `gofmt`, `go vet` and the full Go suite with `-race`, 64 frontend tests, typecheck, build, and the Compose merge with and without the overlay. The probe endpoints were checked against the live private edge (`https://oxnet.work/readyz` returns 200; Authentik `/-/health/live/` returns 200 on this version, not 204).

**For Codex when you return:** (1) review the PR and ADR-014, in particular that probes run from inside the web container and a failed Tailscale refresh no longer hides service state; (2) to deploy, follow "Optional service probes" in `deploy/home-gw/README.md` (the five-file Compose command) and keep the earlier backup-then-build habit; (3) decide ADR-015 before any Docker work; (4) the standing items in `DEPLOYMENT_STATE.md` (second off-host backup copy, restore test, `DESKTOP-HHBRUDD` logon reliability) are unchanged by this work.

- Add fixed, operator-configured read-only probes for private services, beginning with oxnet.work and Authentik on Home GW. Use short deadlines, verified TLS where applicable, no redirects, and no arbitrary dashboard-supplied URLs. Record the probed endpoint and observation time; a recent host-agent report must not count as service health.
- Merge probe observations with the Tailscale snapshot before `Repository.ReplaceSnapshot` runs. That method deletes `inventory_services` on each refresh, so writing service rows separately without changing reconciliation would lose them within a minute. Test refresh, probe failure, timeout, and stale-result behavior.
- Keep Docker inventory as a separate opt-in provider. Do not mount the unrestricted Docker socket into the web server or infer container health from an HTTP probe. Decide the least-privilege host collector or filtered Docker API path before implementation.
