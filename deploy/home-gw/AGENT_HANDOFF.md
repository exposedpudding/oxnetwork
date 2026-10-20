# Home GW handoff — 20 October 2026

Read [DEPLOYMENT_STATE.md](DEPLOYMENT_STATE.md), especially its latest dated section, before changing the host. The older sections are deployment history. The live source is reviewed GitHub `main` commit `4844d6146f8260e177547a72f67aa9f3b3d2c00e` (PRs #2–#8 merged); `/opt/oxnetwork/SOURCE_COMMIT.txt` records it. The host source directory has no `.git` checkout.

## Live setup

- Home GW (`100.71.184.10`) runs oxnet.work, Authentik 2026.8.4, PostgreSQL and Caddy under `/opt/oxnetwork/deploy/home-gw`. The app and Authentik use trusted private HTTPS through Tailscale `svc:oxnet-work`. Keep them off Pangolin, Funnel and public router ports.
- `liam` signs in through Authentik with MFA. `akadmin` also has MFA and **must be retained**, as the owner requested. App sign-out ends its session; the Authentik SSO session may remain.
- Tailscale OAuth (`devices:core:read`) supplies live device inventory. Per-node host agents report CPU, RAM, disk and capacity on Home GW, OX-C-01, the VPS, OX-01, OX-02 and DESKTOP-HHBRUDD. The Windows 11 agents start at user sign-in, so they can go stale when signed out. Pixel 10 and SHADOW-LULLF0L7 have no host metrics collector.
- Fixed HTTPS probes now verify `https://oxnet.work/readyz` and `https://auth.oxnet.work/-/health/live/`. Both returned HTTP 200 and were healthy after the reviewed deployment. This verifies the service path, unlike an agent heartbeat.
- OpenCode on OX-C-01 is configured as a read-only compute provider. Partial project/session reads must show as degraded with an incomplete session count. No submit or cancel action is enabled.
- Separate read-only Docker reporters run on Home GW and OX-C-01. Each has its own token, independent of the host metrics token. Each reported seven containers at the last check. The Docker socket grants root-equivalent host access to the reporter account; enrol other Docker hosts only with that trade-off understood. No Docker actions are exposed.
- Automations and audit are live. Finished workflow runs are retained for 90 days with a floor of the newest 20 per workflow; queued/running runs and audit events are not pruned. A failed prune is retried at the next scheduler tick. No destructive workflow action is enabled.

## Operating commands and recovery

Use **all seven** Compose files for future app changes: `compose.yml`, `private-edge.compose.yml`, `live-tailscale.compose.yml`, `live-host-agent.compose.yml`, `live-docker-reporter.compose.yml`, `live-opencode.compose.yml`, and `live-service-probes.compose.yml`. Include `--profile private-edge --env-file .env`. The private `.env`, `secrets/`, `certs/`, `opencode-servers.yml` and `service-probes.yml` are on Home GW; never commit or print credentials.

The app was rebuilt from `4844d61` and its `/readyz` returned 200, unauthenticated `/api/v1/system` returned 401, SQLite `quick_check` was `ok` at migration 9, both probes were healthy, and both Docker reports were fresh. The reviewed reporter binary was also installed and restarted on each enrolled host. GitHub CI passed Go, frontend, browser and container checks; targeted Go race tests passed on OX-C-01.

`oxnetwork-backup.timer` is enabled for daily 02:30 with jitter. `backup-oxnet.sh` made and verified `/opt/oxnetwork-backups/oxnet-db/oxnet-20261020T181246Z.db` after this deployment; the backup has sessions scrubbed and mode 600. A pre-change copy is `oxnet-20261020T180509Z.db`. Before this release, the full source snapshot was saved as `/opt/oxnetwork-backups/source-pre-20261020-review.tar.gz` (root-only), and the prior image was tagged `oxnet-work:home-gw-pre-20261020-review`. [BACKUP_RESTORE.md](../../docs/BACKUP_RESTORE.md) explains verification and restore. The app backup covers SQLite only; Authentik PostgreSQL, certificates, `.env` and secrets need separate protection.

If transferring another Git archive from Windows, **normalize CRLF in tracked text files before running shell scripts**. The 20 October archive was normalized on Home GW after extraction; `bash -n` passed for deploy scripts. The source marker should be set only after a successful build on future upgrades.

## Next work

1. Create and verify a current encrypted off-host copy covering the app database and private configuration, and test a complete restore including Authentik, certificates and secrets on a separate host. The older Windows DPAPI copy was captured on 18 October and is tied to its Windows user profile.
2. Confirm DESKTOP-HHBRUDD's reinstalled Windows agent starts by itself at its next normal sign-in. Its missing real-profile Run entry was corrected on 19 October; automatic logon start remains unverified.
3. Recheck provider health through the signed-in Integrations page. Investigate stale agents, service probes, OpenCode partial reads, and Docker reporters separately. Enrol additional hosts only with exact Tailscale node IDs and distinct tokens.
4. Keep mutating features behind the security gates in `AGENTS.md`. The current Docker and OpenCode integrations are read-only. Pangolin exposure, network links, action framework and multi-node compute placement remain future work.
