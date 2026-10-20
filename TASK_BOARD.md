# oxnet.work — current task board (20 October 2026)

The implementation contract is [AGENTS.md](AGENTS.md). For the exact live deployment, recovery paths and remaining operator work, start with [deploy/home-gw/AGENT_HANDOFF.md](deploy/home-gw/AGENT_HANDOFF.md) and the latest section of [DEPLOYMENT_STATE.md](deploy/home-gw/DEPLOYMENT_STATE.md). Older deployment sections are history.

## Delivered and live

- Milestone 4 Automations and audit: PR #1, ADR-013. Manual and cron read-only checks were exercised on Home GW. Run retention is now live (PR #7, ADR-018): 90 days with the newest 20 finished runs per workflow retained; audit remains append-only.
- Tailscale inventory and per-node host metrics: Home GW, OX-C-01, ovh-vps, and Windows 11 OX-01, OX-02 and DESKTOP-HHBRUDD report CPU/RAM/disk/capacity. Agent freshness is separate from service health. Keep `akadmin` and `liam` with MFA.
- Verified service probes: PR #2, ADR-014. oxnet.work and Authentik private HTTPS probes were healthy after activation on 20 October. Backup/verify/restore tooling: PR #3, ADR-016; a daily backup timer is enabled and a post-deploy SQLite backup was verified.
- OpenCode read-only compute provider on OX-C-01: PR #5, ADR-017. Integration health API and page: PR #6. Partial OpenCode project reads are degraded and session counts are marked incomplete.
- Docker container inventory: PR #8, ADR-019, deployed on Home GW and OX-C-01 only. The reporter has separate per-node tokens and no Docker actions. Docker socket access is root-equivalent; no other host is enrolled by default. Node Detail and Integrations display current or stale reports.
- Reviewed release source: GitHub `main` `4844d6146f8260e177547a72f67aa9f3b3d2c00e` on Home GW. CI passed Go, frontend, browser and container checks; targeted Go race tests passed on OX-C-01. `/readyz`, private HTTPS, auth boundary, SQLite integrity, probes and both Docker reports were checked after deployment.
- A fresh full-stack backup on 20 October includes Authentik PostgreSQL/data, app SQLite, certificates, configuration and tokens. An encrypted Windows off-host copy passed decrypt-and-hash verification. An isolated restore on OX-C-01 started PostgreSQL, Authentik, Caddy and the app over private HTTPS; readiness, 401 auth boundary, database integrity, TOTP records and TLS key match passed. See [BACKUP_RESTORE.md](docs/BACKUP_RESTORE.md) for test limits.

## Next operational tasks

1. Make a second encrypted off-host copy whose recovery does not depend on Lee's Windows DPAPI profile, and test recovery from that copy. The enabled daily timer covers the app SQLite database only; refresh the full-stack set separately and repeat the isolated restore at least quarterly.
2. Verify DESKTOP-HHBRUDD's reinstalled Windows agent starts on the next ordinary sign-in. Its previous installation landed in a packaged app's private profile; it was reinstalled in the real profile on 19 October.
3. Check the signed-in Integrations page over time for probe failures, stale Windows agents, OpenCode partial reads and Docker reporter freshness. Add additional service probes only as fixed operator configuration. Enrol Docker hosts one at a time, with separate tokens and an explicit acceptance of Docker group privilege.
4. Keep future actions behind `AGENTS.md` security gates. Pangolin exposure, topology links, OpenCode submit/cancel, Docker start/stop/restart and multi-node placement are not yet implemented.

## Working rule

Update this board and the Home GW handoff after meaningful deployment changes. Preserve the private-only edge, keep `akadmin`, and never commit or paste credentials.
