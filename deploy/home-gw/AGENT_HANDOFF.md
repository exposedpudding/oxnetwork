# Home GW handoff

Read [DEPLOYMENT_STATE.md](DEPLOYMENT_STATE.md) before changing the host. It records the verified live deployment on 18 October 2026, exact paths, private network addresses, backups, and remaining work. [README.md](README.md) retains the setup procedure for a fresh installation; most of those steps have already been done on Home GW.

## Current state

- Home GW (`100.71.184.10`) runs Authentik 2026.8.4, PostgreSQL, the oxnet.work app, and Caddy. The Compose project is `/opt/oxnetwork/deploy/home-gw`.
- `svc:oxnet-work` serves private TCP 443 at `100.90.240.202`; split DNS resolves `oxnet.work` and `auth.oxnet.work` to that VIP. The public DNS answers for those names have no A/AAAA records. Do not publish either service through Pangolin, Funnel, or a public router port.
- Spaceship DNS-01 issued a trusted certificate for both names. Its credentials and all other secrets are stored only on Home GW. `oxnetwork-acme-renew.timer` is enabled.
- Authentik initial admin setup and the confidential `oxnet-work` OIDC application are complete. Both `akadmin` and `liam` are bound to the app. The owner enrolled MFA on both accounts and requested that `akadmin` remain available.
- oxnet.work is running in demo mode. Browser sign-in through Authentik and the Overview, Compute, and Network pages worked. The authenticated API returned data; an unauthenticated API request returned 401. The original `/api/v1/activity` frontend request was filtered in the browser, so the frontend now uses `/api/v1/recent-events` for the same data.
- The owner signed in as `liam` with MFA and reached the demo dashboard. Sign out now lands on `/auth/signed-out` after revoking the app session; it leaves the Authentik SSO session available. The owner confirmed the signed-out page and an unauthenticated response from `/api/v1/system`.
- The Home GW source was archive-copied from GitHub `main`, then updated with the activity alias and sign-out fix from this session. The host has no `.git` directory. `/opt/oxnetwork/SOURCE_COMMIT.txt` records the synced GitHub commit. Preserve `.env`, `secrets/`, `certs/`, and `/etc/oxnetwork-acme.env`.

## Remaining work

1. The owner verified a fresh `liam` sign-in through MFA and reached the oxnet.work dashboard. Keep the `akadmin` account as requested. Both accounts are currently app-bound; review whether that access policy is appropriate after the owner test.
2. Decide whether app sign-out should also end the Authentik SSO session. Test both private names from a device outside the tailnet and confirm the Filebrowser node Serve route still works.
3. Test a full restore on a separate host and arrange a durable second off-host copy. The refreshed root-only backup and an isolated PostgreSQL restore check exist at `/opt/oxnetwork-backups`; a Windows DPAPI-encrypted copy exists in this Codex task's `outputs/` directory. DPAPI recovery is tied to the current Windows user profile.
4. Optionally enable the read-only Tailscale inventory adapter only after the owner has reviewed its account-wide API permission. Demo mode accurately marks all current inventory as simulated.

Do not paste credentials into chat, logs, or Git. Do not rerun bootstrap or rotate Authentik's secret key.
