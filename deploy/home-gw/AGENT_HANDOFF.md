# Home GW handoff

Read [DEPLOYMENT_STATE.md](DEPLOYMENT_STATE.md) before changing the host. It records the verified live deployment on 18 October 2026, exact paths, private network addresses, backups, and remaining work. [README.md](README.md) retains the setup procedure for a fresh installation; most of those steps have already been done on Home GW.

## Current state

- Home GW (`100.71.184.10`) runs Authentik 2026.8.4, PostgreSQL, the oxnet.work app, and Caddy. The Compose project is `/opt/oxnetwork/deploy/home-gw`.
- `svc:oxnet-work` serves private TCP 443 at `100.90.240.202`; split DNS resolves `oxnet.work` and `auth.oxnet.work` to that VIP. The public DNS answers for those names have no A/AAAA records. Do not publish either service through Pangolin, Funnel, or a public router port.
- Spaceship DNS-01 issued a trusted certificate for both names. Its credentials and all other secrets are stored only on Home GW. `oxnetwork-acme-renew.timer` is enabled.
- Authentik initial admin setup and the confidential `oxnet-work` OIDC application are complete. Both `akadmin` and `liam` are bound to the app. The owner enrolled MFA on both accounts and requested that `akadmin` remain available.
- oxnet.work uses live Tailscale device inventory. Its OAuth trust credential `oxnetworkHomeGW` has only `devices:core:read`; the secret is mounted from a root-owned file and renewed bearer tokens are held in memory. SQLite showed eight real Tailscale node observations and zero demo nodes after activation. Go agents on Home GW and Windows 11 `DESKTOP-HHBRUDD` now add real CPU, RAM, system/root disk and capacity readings for their exact node IDs. The other six devices remain `unknown` with no resource metrics. Agent `healthy` means recent reporting, not verified service health. Keep both `live-tailscale.compose.yml` and `live-host-agent.compose.yml` in every future oxnet-work Compose invocation.
- The owner signed in as `liam` with MFA and reached the dashboard before the live switch. Sign out lands on `/auth/signed-out` after revoking the app session; it leaves the Authentik SSO session available. The owner confirmed the signed-out page and an unauthenticated response from `/api/v1/system`. After the live switch, the authenticated Edge browser showed eight Tailscale device cards and no Demo Mode banner; the owner supplied a matching screenshot.
- The owner tested both private names from outside the tailnet and confirmed they were unreachable. The original `/api/v1/activity` frontend request was filtered in the browser, so the frontend uses `/api/v1/recent-events` for the same data.
- The Home GW source was archive-copied from GitHub `main`, then patched in this session. The host has no `.git` directory. `/opt/oxnetwork/SOURCE_COMMIT.txt` records the synced GitHub commit. Preserve `.env`, `secrets/`, `certs/`, and `/etc/oxnetwork-acme.env`.
- The Home GW collector runs as `oxnet-host-agent@tailscale-4670134924505329.service`; the Windows collector starts from the current user's `HKCU` Run key and reports while that user is logged on. Their credentials are separate protected files under `deploy/home-gw/secrets/host-agents/`, with the Windows node's copy protected in `%LOCALAPPDATA%\oxnetwork\agent`. The authenticated Overview displayed both live nodes and six unreported nodes. Follow [`deploy/agents/README.md`](../agents/README.md) for further enrollment; Linux and Windows amd64 binaries are saved in this Codex task's outputs. Do not reuse a token.

## Remaining work

1. Keep the `akadmin` account as requested. Both accounts are currently app-bound; review whether that access policy is appropriate.
2. Enroll additional machines one at a time with unique tokens and exact Tailscale IDs. Add separate providers for service checks, Docker, jobs and network links. Tailscale membership and agent reporting do not verify service health.
3. Decide whether app sign-out should also end the Authentik SSO session. Confirm the Filebrowser node Serve route still works after future network changes.
4. Test a full restore on a separate host and arrange a durable second off-host copy. The latest root-only `host-agent-2026-10-18` backup set includes both telemetry tokens and an integrity-checked SQLite database; the earlier isolated Authentik PostgreSQL restore check remains valid. A Windows DPAPI-encrypted copy exists in this Codex task's `outputs/` directory. DPAPI recovery is tied to the current Windows user profile.

Do not paste credentials into chat, logs, or Git. Do not rerun bootstrap or rotate Authentik's secret key.
