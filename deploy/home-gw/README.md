# Home GW deployment plan

**State:** Prepared, not deployed. The owner chose Home GW for both Authentik and oxnet.work. The proposed private HTTPS names are `auth.oxnet.work` and `oxnet.work`. No public route is part of this stack.

## Before starting

1. Join the administration machine to the same Tailscale tailnet as Home GW. Confirm Home GW's device name or 100.x address and SSH account. Check its OS, architecture, Docker Compose version, free memory, storage, existing services, reverse proxy and backup arrangement. Authentik's 2026.8 Compose installation guide calls for **at least 2 CPU cores and 2 GB RAM**. Do not launch this stack until those resources and port conflicts are checked.
2. Configure private DNS so tailnet clients resolve `oxnet.work` and `auth.oxnet.work` to Home GW's Tailscale address (`100.71.184.10` at planning time). Obtain trusted certificates for both names using DNS validation; an HTTP challenge would require public ingress. Configure the existing reverse proxy to listen only on the Tailscale interface, serve HTTPS for both names, and forward to `127.0.0.1:8090` and `127.0.0.1:9000` respectively. Host port 8090 avoids Home GW's Guacamole on 8080; the oxnet-work container still listens on 8080. Forward the original `Host`, `X-Forwarded-Proto`, and `X-Forwarded-For` headers; support WebSocket upgrades for Authentik. Overwrite client supplied forwarding headers. Do not create a Pangolin resource or public port forward for either name.
3. Verify from **inside the oxnet-work container** that it can reach `https://auth.oxnet.work/application/o/oxnet-work/.well-known/openid-configuration` with a trusted certificate. The public issuer URL must resolve and reach the same Authentik instance for the browser and the app. Proxy networking may need adjustment to achieve this; do not substitute an internal issuer URL.

### Spaceship DNS and certificates

The owner identified Spaceship as the DNS provider for `oxnet.work`. Spaceship's public DNS zone can supply the `_acme-challenge` TXT records needed for DNS-01 certificates without opening HTTP/HTTPS to the internet. The current `acme.sh` `dns_spaceship` integration uses `SPACESHIP_API_KEY` and `SPACESHIP_API_SECRET`; create these in Spaceship and store them only on Home GW in a protected ACME account/configuration. Do not put them in this repository, the handoff zip or chat. Test automatic renewal and proxy reload before considering HTTPS complete.

Spaceship public DNS records are not private DNS records. For private name resolution, use an existing tailnet DNS resolver with split DNS if available. A public A record pointing at a Tailscale 100.x address does not create an internet route, but it publishes that address; choose that shortcut only if the owner accepts it. The agent reported that `auth.oxnet.work` currently points publicly to `51.89.148.88` with the wrong certificate; remove or correct that stale record as part of the DNS change. Wait for the existing `tailscale serve status` and version before selecting the final 443 listener or DNS target, because Home GW already uses tailnet ports 443 and 8443.

## Prepare secrets on Home GW

The easiest route is to run `bash bootstrap-authentik.sh` from this directory after checking Home GW capacity and existing services. It generates the PostgreSQL and Authentik secrets locally, keeps an existing `.env` intact, and starts only the Authentik services. After configuring the Authentik OIDC application, run `bash finish-oxnet.sh` from an interactive terminal; it prompts for the client ID and secret without echoing the secret, builds oxnet.work, checks secret readability, starts it and calls `/readyz`.

The equivalent manual steps are below.

From this directory on Home GW:

```sh
mkdir -p secrets
chmod 700 secrets
printf 'PG_PASS=%s\n' "$(openssl rand -base64 36 | tr -d '\n')" > .env
printf 'AUTHENTIK_SECRET_KEY=%s\n' "$(openssl rand -base64 60 | tr -d '\n')" >> .env
printf 'OXNET_OIDC_ISSUER_URL=https://auth.oxnet.work/application/o/oxnet-work/\n' >> .env
printf 'OXNET_OIDC_CLIENT_ID=pending\n' >> .env
chmod 600 .env
```

Those commands create new secrets; preserve them in an encrypted backup. The `pending` client ID only allows Compose to parse while Authentik starts; replace it with the real client ID before starting oxnet.work. Do not commit `.env` or `secrets/`.

## Bring up Authentik privately

```sh
docker compose --env-file .env -f compose.yml up -d postgresql authentik-server authentik-worker
docker compose --env-file .env -f compose.yml ps
```

Open the initial Authentik setup flow through private HTTPS or the SSH tunnel below, create the `akadmin` password, then create a normal owner account with MFA. In Authentik, create an OAuth2/OpenID Provider and Application with slug `oxnet-work`. Use a confidential client and authorization code flow. Set the **exact redirect URI** to `https://oxnet.work/auth/callback`, select a signing key, and restrict application access to the intended owner/group. Add the client ID to `.env` as `OXNET_OIDC_CLIENT_ID`; write the client secret to `secrets/oidc_client_secret`. The app runs as UID/GID 10001 and must be able to read that file. For a local Compose file secret, set its owner/group and permissions to `root:10001` and `0640` (for example, `sudo chown root:10001 secrets/oidc_client_secret && sudo chmod 640 secrets/oidc_client_secret`). Confirm the discovery document reports issuer `https://auth.oxnet.work/application/o/oxnet-work/`.

The **initial Authentik account setup can happen before DNS is ready**. From a trusted PC that can SSH to Home GW over Tailscale, open an SSH tunnel with `ssh -L 19000:127.0.0.1:9000 <ssh-user>@100.71.184.10`, then visit `http://127.0.0.1:19000` on that PC and set the `akadmin` password. The traffic to Home GW stays inside SSH; do not publish port 9000. Continue with the final browser login test at the private HTTPS name after DNS and certificates are working.

The worker has no Docker socket mount. Automatic Docker outpost management is therefore unavailable; the OAuth2 provider here does not require an outpost.

## Start oxnet.work and verify

```sh
docker compose --env-file .env -f compose.yml config --quiet
docker compose --env-file .env -f compose.yml build oxnet-work
docker compose --env-file .env -f compose.yml up -d oxnet-work
docker compose --env-file .env -f compose.yml ps
```

Verify the loopback health endpoint on Home GW, then use a tailnet-connected browser at `https://oxnet.work`. An unauthenticated browser must be redirected to Authentik; an unauthenticated API request must receive `401`. Sign in and sign out, inspect the secure session cookie, and confirm both names fail from outside the tailnet. The dashboard is still **simulated read-only data** until live adapters are configured; Authentik does not make that data live.

Back up the Authentik PostgreSQL database, `authentik-data`, the oxnet-work SQLite volume, and the two secret files. Test restore on a separate instance. Never rotate `AUTHENTIK_SECRET_KEY` without following Authentik's key rotation procedure.

This Compose uses Authentik `2026.8.4`, based on the upstream `version/2026.8.4` Compose generator. Review Authentik release notes and update the pinned image deliberately before a later production upgrade.
