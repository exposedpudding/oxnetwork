# Home GW deployment plan

**State (last agent report):** Authentik is healthy on Home GW; oxnet.work is built but stopped. Private DNS, HTTPS, and Authentik OIDC sign-in remain to be completed and verified. The owner chose Home GW for both services and chose private Tailscale access for `auth.oxnet.work` and `oxnet.work`.

## Before starting

1. Start with [`AGENT_HANDOFF.md`](AGENT_HANDOFF.md) and run `bash preflight-home-gw.sh` on Home GW. Confirm the reported Home GW state before changing anything. Authentik's 2026.8 Compose guide calls for at least 2 CPU cores and 2 GB RAM.
2. Create a separate Tailscale Service with raw TCP 443 forwarding to Home GW loopback port 8445. Configure private DNS so tailnet clients resolve **both names to that Service's virtual IP**, not Home GW's node IP (`100.71.184.10` at last report). The node's own Tailscale Serve 443 and 8443 remain assigned to Guacamole and Filebrowser.
3. Obtain trusted certificates for both names using Spaceship DNS-01. Run the supplied Caddy edge on Home GW loopback port 8445. It forwards by hostname to Authentik and oxnet.work over the Compose network. Verify from inside the oxnet-work container that it can reach `https://auth.oxnet.work/application/o/oxnet-work/.well-known/openid-configuration` with a trusted certificate and matching issuer URL. Do not substitute an internal HTTP issuer.

### Spaceship DNS and certificates

The owner identified Spaceship as the DNS provider for `oxnet.work`. Spaceship's public DNS zone can supply the `_acme-challenge` TXT records needed for DNS-01 certificates without opening HTTP/HTTPS to the internet. The current `acme.sh` `dns_spaceship` integration uses `SPACESHIP_API_KEY` and `SPACESHIP_API_SECRET`; create these in Spaceship and store them only on Home GW in a protected ACME account/configuration. Do not put them in this repository, the handoff zip or chat. Test automatic renewal and proxy reload before considering HTTPS complete.

Spaceship public DNS records are not private DNS records. The agent reported that `auth.oxnet.work` publicly points to `51.89.148.88` with the wrong certificate; remove that stale A record once private DNS is ready. The owner selected private split DNS, so do not publish the Service VIP in Spaceship A records. If the tailnet has no suitable resolver, the Home GW agent must set up a tailnet-reachable DNS resolver before adding the split DNS rule. That resolver must answer `oxnet.work` and `auth.oxnet.work` with the Service VIP, and handle other names under `oxnet.work` appropriately.

The owner chose **private split DNS**. Use Spaceship only for the DNS-01 TXT records and remove the stale public `auth.oxnet.work` A record. Home GW has Tailscale 1.102.4 with Guacamole on node port 443 and Filebrowser on node port 8443. The proposed solution is a **separate Tailscale Service with its own virtual IP**, serving raw TCP 443 to a local HTTPS proxy on `127.0.0.1:8445`; do not change the existing node Serve routes. The optional [`private-edge.compose.yml`](private-edge.compose.yml) and [`Caddyfile`](Caddyfile) provide that proxy. It routes by HTTPS hostname to Authentik and oxnet.work, and gives the oxnet-work container a Compose DNS alias for its `auth.oxnet.work` issuer. Obtain a certificate covering both names, install it as `certs/fullchain.pem` and `certs/key.pem` with restrictive permissions, then start `private-https` with both Compose files. Configure the Tailscale Service's raw TCP forwarding and the tailnet's split DNS only after checking `tailscale service list`, Service advertisement/ACL permissions, port 8445 availability, and whether an existing tailnet DNS resolver can serve these records. A split DNS rule needs a real reachable DNS server; adding the rule alone is insufficient. Renew certificates automatically and restart `private-https` after renewal.

### Optional Tailscale inventory after sign-in works

Keep the default demo mode until a tailnet client can sign in at `https://oxnet.work`. Then create a Tailscale API key with the least device-read permission available and save it **on Home GW** as `secrets/tailscale_api_key`; keep it out of Git and chat. Set `OXNET_TAILSCALE_TAILNET` in the local `.env` to the tailnet name. The container runs as UID 10001, so make the file readable to that UID (for example `root:10001`, mode `0640`) while keeping the `secrets` directory private. Check the overlay, then recreate just the app:

```bash
docker compose --env-file .env -f compose.yml -f live-tailscale.compose.yml config --quiet
docker compose --env-file .env -f compose.yml -f live-tailscale.compose.yml up -d oxnet-work
```

Confirm `GET /api/v1/system` says `"demo_mode":false` after login, then check that `/api/v1/nodes` lists Tailscale devices with `"simulated":false` and `"state":"unknown"`. Verify the API key stays outside logs and responses. Switching to live mode clears old demo inventory; a failed Tailscale poll leaves an empty list initially, or the last successful list after a later outage, until a retry succeeds. Live inventory reports device identity only, not host health or topology. A future `docker compose` invocation for oxnet-work must include this overlay to keep live mode enabled.

## Prepare secrets on Home GW

Authentik was reported running already; inspect that installation before using bootstrap. For a fresh Home GW installation, `bash bootstrap-authentik.sh` generates PostgreSQL and Authentik secrets locally, keeps an existing `.env` intact, and starts only the Authentik services. After configuring the Authentik OIDC application, run `sudo bash finish-oxnet.sh` from an interactive terminal; it prompts for the client ID and secret without echoing the secret, builds oxnet.work, checks secret readability, starts it and calls `/readyz`.

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
