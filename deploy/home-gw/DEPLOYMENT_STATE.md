# Home GW deployment state — 18 October 2026

This is the verified live state as of the local PC SSH session on 18 October 2026. Authentik OIDC and oxnet.work are running. The owner signed in as `liam` with MFA and reached the demo dashboard. A full restore on a separate host remains.

## Host and source

- Home GW: Ubuntu 26.04 LTS, Tailscale node `home-gw` (`100.71.184.10`, tag `tag:homegw`), 14 GiB RAM, 207 GiB free at preflight.
- Source: `/opt/oxnetwork`, copied from GitHub `main` after normalizing line endings, then patched with the activity alias and sign-out fix from this session. `/opt/oxnetwork/SOURCE_COMMIT.txt` records the synced commit. The host checkout is archive based and has no `.git` directory.
- Compose: `/opt/oxnetwork/deploy/home-gw`. The original `.env` and `secrets/` were retained. The local 8090 port change is included in `main`.
- Pre-update backup: `/opt/oxnetwork-backups/pre-sync-2026-10-18.tar.gz`, root owned mode 600. Authentik PostgreSQL dumps: `/opt/oxnetwork-backups/authentik-2026-10-18.dump` (before setup) and `/opt/oxnetwork-backups/authentik-post-oidc-2026-10-18.dump` (after app creation). The latter restored into a temporary PostgreSQL database and contained one Authentik application; the temporary database was removed. Authentik data archive: `/opt/oxnetwork-backups/authentik-data-2026-10-18.tar.gz`. A root-only archive of the DNS API credentials, ACME state, certificate, Compose `.env`, and app secrets is `/opt/oxnetwork-backups/oxnetwork-private-2026-10-18.tar.gz`. The oxnet SQLite backup is `/opt/oxnetwork-backups/oxnet-2026-10-18.db`; its integrity check returned `ok`.
- After MFA enrollment, a fresh PostgreSQL dump is at `/opt/oxnetwork-backups/authentik-post-mfa-2026-10-18.dump`; its isolated restore contained four users and two confirmed TOTP devices. The matching Authentik data archive, integrity-checked SQLite backup, and combined off-host tar have `post-mfa-2026-10-18` in their names in the same directory. The combined tar also includes the existing private secrets archive.
- An encrypted off-host copy is in this Codex task's Windows `outputs/oxnetwork-offhost-post-mfa-2026-10-18.tar.dpapi`. It was encrypted with Windows DPAPI `CurrentUser` and entropy `oxnetwork-offhost-backup-v1`; decryption and SHA-256 equality with the streamed tar were verified. Recovery requires the same Windows user profile or its DPAPI recovery material. This is not yet a full restore on a separate host; arrange a durable second off-host copy. The earlier `outputs/oxnetwork-offhost-2026-10-18.tar.dpapi` predates MFA enrollment.

## Running services and private path

- Authentik server and worker `ghcr.io/goauthentik/server:2026.8.4` and PostgreSQL `postgres:16-alpine` are healthy. Authentik binds `127.0.0.1:9000`.
- Filebrowser still uses the Home GW node's Tailscale Serve port 8443. The currently observed Serve status did **not** list Guacamole on port 443; the earlier handoff's claim about Guacamole was stale. Do not repurpose existing node Serve routes.
- `svc:oxnet-work` is defined for TCP 443 and Home GW is an approved online host. Its IPv4 VIP is `100.90.240.202`, IPv6 VIP `fd7a:115c:a1e0::f03a:f0cb`, and MagicDNS name `oxnet-work.tail9a48c3.ts.net`. Home GW forwards raw TCP 443 to `127.0.0.1:8445`. The existing tailnet grant permits all members to reach all destinations, so Authentik access policy and MFA remain necessary.
- `oxnetwork-dns.service` runs dnsmasq on `tailscale0:53`, configured by `/etc/oxnetwork-dns.conf`; it answers `oxnet.work` and `auth.oxnet.work` A queries with the Service VIP and forwards other queries to public resolvers. Tailscale split DNS for `oxnet.work` points to `100.71.184.10`. A tailnet PC resolved both private names to the VIP and `pangolin.oxnet.work` to its public VPS address. No AAAA answer was observed for the private names.
- Public Spaceship DNS retains the wildcard and `pangolin` A records for `51.89.148.88`. An explicit `auth.oxnet.work` TXT record (`private-tailscale-only`) suppresses the wildcard A answer for that name. Both authoritative Spaceship nameservers answered `auth.oxnet.work` A with NOERROR and no answer. Cloudflare DNS over HTTPS likewise returned no A answer after the old 30-minute cache expired. Authoritative queries returned no `auth.oxnet.work` AAAA or apex A answer; `pangolin.oxnet.work` still returned `51.89.148.88`.

## Certificate and edge

- `acme.sh` 3.1.2 issued a Let's Encrypt ECDSA certificate for both names through Spaceship DNS-01. The Spaceship API key has **DNS Records read/write only** (Spaceship applies that permission across the account's domains); its values are stored in `/etc/oxnetwork-acme.env`, root owned mode 600. Never copy that file into Git or chat.
- The certificate and key are installed at `certs/fullchain.pem` and `certs/key.pem`, mode 600, expiring 16 January 2027. `acme.sh --install-cert` is configured to restart `private-https` after renewal. `oxnetwork-acme-renew.timer` runs a daily renewal check; a manual check on 18 October exited successfully and skipped renewal because the certificate is fresh.
- Caddy `caddy:2.10.2-alpine` runs as `private-https` with TLS bound only to host loopback port 8445. From a tailnet PC, `https://auth.oxnet.work/` returned 302 and `https://oxnet.work/` returned 303 to Authentik, both with a trusted certificate and Service VIP. Unauthenticated `https://oxnet.work/api/v1/system` returned 401.

## Authentik and oxnet.work

- The owner completed initial `akadmin` setup and set Authentik's base URL to `https://auth.oxnet.work`. A separate internal user `liam` uses the owner's Gmail address. The owner set passwords and enrolled MFA for both accounts, and specifically requested that `akadmin` remain available. PostgreSQL shows one confirmed TOTP device per account; the default authentication flow binds `default-authentication-mfa-validation` between password and login stages.
- The Authentik OAuth2/OIDC application `oxnet-work` uses a confidential client, authorization code flow, explicit consent, a signing certificate, and the strict redirect `https://oxnet.work/auth/callback`. Both `akadmin` and `liam` are bound to the application. The provider discovery issuer is exactly `https://auth.oxnet.work/application/o/oxnet-work/`.
- The generated client ID is in the root-only Compose `.env`; the client secret is in `secrets/oidc_client_secret` owned by `root:10001` mode 640. Do not print or copy either value to Git or chat.
- The oxnet-work image built successfully and the app is running. `/readyz` on `127.0.0.1:8090` returned `{"status":"ready"}`. Its inventory source is `demo`. Startup succeeded after OIDC discovery, which validates the internal issuer path and trusted certificate. Browser sign-in through Authentik consent reached the app; authenticated `/api/v1/system`, `/api/v1/nodes`, and `/api/v1/activity` returned JSON. Overview, Compute, and Network pages rendered demo inventory.
- The initial overview failed because the browser's scripted request to `/api/v1/activity` did not reach Home GW although direct navigation to that URL worked. The app now also serves the same data at `/api/v1/recent-events`, and the overview uses that route. After rebuilding, the overview loaded with concurrent API requests again. The cause of the browser's filtering of the original URL was not identified; retain the alias for compatibility.
- The owner confirmed a fresh `liam` sign-in with MFA and supplied a dashboard screenshot showing Overview, three simulated nodes, three simulated services, and the Demo Mode banner. This verifies the owner path through the private browser route.
- The app's Sign out button now clears its own session and lands on `/auth/signed-out` instead of immediately starting another OIDC login. The signed-out page explains that Authentik SSO may remain active. The auth unit test checks that logout revokes the session; Home GW returned HTTP 200 for the new page after rebuilding. A browser sign-out check by the owner is pending.

## Remaining work

1. Keep `akadmin` as requested. Confirm the owner's browser sign-out check; decide whether Sign out should also end the Authentik SSO session. Both accounts remain app-bound.
2. Inventory remains simulated until the optional Tailscale adapter is enabled. The running app's OIDC startup succeeded using the issuer; an explicit in-container certificate/discovery probe is still useful.
3. Test both private names from a device outside the tailnet; they must be unreachable. Filebrowser Serve returned HTTP 200 with trusted TLS during this session. Test a full restore on a separate host and arrange a second durable off-host copy.

Do not publish either application through Pangolin or a public router port. Do not paste credentials into logs, Git, or chat.
