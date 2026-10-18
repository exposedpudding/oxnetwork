# Home GW deployment plan

**Current live status:** See [DEPLOYMENT_STATE.md](DEPLOYMENT_STATE.md), updated 18 October 2026. Private networking, TLS, Authentik OIDC, and oxnet.work are running. The owner signed in as `liam` with MFA; `akadmin` remains available with MFA at the owner's request. The dashboard imports real Tailscale device identity and receives host CPU, RAM, disk and capacity from agents on Home GW and `DESKTOP-HHBRUDD`. Other devices and service health remain unverified. A full restore on a separate host remains. Preserve the certificate, DNS, Tailscale Service, `.env`, and secrets when continuing.

**Historical state (prior handoff):** Authentik was healthy on Home GW while oxnet.work was stopped. The owner chose Home GW for both services and private Tailscale access for `auth.oxnet.work` and `oxnet.work`. Use the live state document above before running any setup commands below.

## Before starting

1. Start with [`AGENT_HANDOFF.md`](AGENT_HANDOFF.md) and run `bash preflight-home-gw.sh` on Home GW. Confirm the reported Home GW state before changing anything. Authentik's 2026.8 Compose guide calls for at least 2 CPU cores and 2 GB RAM.
2. A separate Tailscale Service forwards raw TCP 443 to Home GW loopback port 8445. Private DNS resolves **both names to that Service's virtual IP**, not Home GW's node IP (`100.71.184.10`). The observed node Serve status lists Filebrowser on 8443; it did not list the previously reported Guacamole route.
3. Obtain trusted certificates for both names using Spaceship DNS-01. Run the supplied Caddy edge on Home GW loopback port 8445. It forwards by hostname to Authentik and oxnet.work over the Compose network. Verify from inside the oxnet-work container that it can reach `https://auth.oxnet.work/application/o/oxnet-work/.well-known/openid-configuration` with a trusted certificate and matching issuer URL. Do not substitute an internal HTTP issuer.

### Spaceship DNS and certificates

The owner identified Spaceship as the DNS provider for `oxnet.work`. Spaceship's public DNS zone can supply the `_acme-challenge` TXT records needed for DNS-01 certificates without opening HTTP/HTTPS to the internet. The current `acme.sh` `dns_spaceship` integration uses `SPACESHIP_API_KEY` and `SPACESHIP_API_SECRET`; create these in Spaceship and store them only on Home GW in a protected ACME account/configuration. Do not put them in this repository, the handoff zip or chat. Test automatic renewal and proxy reload before considering HTTPS complete.

Spaceship public DNS records are not private DNS records. The former public `auth.oxnet.work` A answer came from the wildcard record; an explicit TXT record now suppresses that wildcard answer at both authoritative nameservers. Do not publish the Service VIP in Spaceship A records. Home GW runs a tailnet-reachable resolver for the split DNS rule; its configuration is based on [`oxnetwork-dns.conf.example`](oxnetwork-dns.conf.example).

The owner chose **private split DNS**. Home GW has Tailscale 1.102.4, and `svc:oxnet-work` has its own virtual IP with raw TCP 443 forwarded to the local HTTPS proxy on `127.0.0.1:8445`. The [`private-edge.compose.yml`](private-edge.compose.yml) and [`Caddyfile`](Caddyfile) provide that proxy. It routes by HTTPS hostname to Authentik and oxnet.work, and gives the oxnet-work container a Compose DNS alias for its `auth.oxnet.work` issuer. The certificate covers both names and is installed as `certs/fullchain.pem` and `certs/key.pem` with restrictive permissions. The [`oxnetwork-acme-renew.timer`](oxnetwork-acme-renew.timer) runs daily renewal checks and restarts `private-https` after renewal. Do not change existing node Serve routes.

### Optional Tailscale inventory after sign-in works

After verifying private OIDC sign-in, create a Tailscale OAuth trust credential with only **Devices → Core → Read** (`devices:core:read`). A traditional Tailscale API token has full API access, so do not use it for this deployment. Add the OAuth client ID as `OXNET_TAILSCALE_OAUTH_CLIENT_ID` in the root-only local `.env`; save the client secret **on Home GW** as `secrets/tailscale_oauth_client_secret`, never in Git or chat. The container runs as UID 10001, so make the secret file readable to that UID (`root:10001`, mode `0640`) while keeping the `secrets` directory private. The overlay uses `-` as Tailscale's shorthand for the credential's tailnet. Check the overlay, then recreate just the app:

```bash
docker compose --env-file .env -f compose.yml -f private-edge.compose.yml -f live-tailscale.compose.yml config --quiet
docker compose --env-file .env -f compose.yml -f private-edge.compose.yml -f live-tailscale.compose.yml up -d oxnet-work
```

Confirm `GET /api/v1/system` says `"demo_mode":false` after login, then check that `/api/v1/nodes` lists Tailscale devices with `"simulated":false`. Devices without agents remain `unknown` with no resource metrics. Verify the OAuth secret and access token stay outside logs and responses. Switching to live mode clears old demo inventory; a failed Tailscale poll leaves an empty list initially, or the last successful list after a later outage, until a retry succeeds. Tailscale reports device identity only, not host health or topology. A future `docker compose` invocation for oxnet-work must include this overlay to keep live mode enabled.

### Host metrics agents

The Home GW deployment also uses `live-host-agent.compose.yml`, which mounts the per-node report token directory read-only. Include **both** live overlays in every future oxnet-work Compose command:

```bash
docker compose -f compose.yml -f private-edge.compose.yml -f live-tailscale.compose.yml -f live-host-agent.compose.yml up -d --build oxnet-work
```

The Home GW collector runs as `oxnet-host-agent@tailscale-4670134924505329.service` and posts to the loopback app port. The Windows 11 `DESKTOP-HHBRUDD` collector starts from its current user's logon entry and posts through private HTTPS. Each has a different protected token. See [agent enrollment](../agents/README.md) before adding another host. A fresh sample means the agent is reporting; it does not verify other services. The agent endpoint requires its bearer token and returns 401 without one.

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

Verify the loopback health endpoint on Home GW, then use a tailnet-connected browser at `https://oxnet.work`. An unauthenticated browser must be redirected to Authentik; an unauthenticated API request must receive `401`. Sign in and sign out, inspect the secure session cookie, and confirm both names fail from outside the tailnet. Without the live Tailscale overlay the dashboard uses **simulated read-only data**; Authentik alone does not make that data live.

Back up the Authentik PostgreSQL database, `authentik-data`, the oxnet-work SQLite volume, and the two secret files. Test restore on a separate instance. Never rotate `AUTHENTIK_SECRET_KEY` without following Authentik's key rotation procedure.

This Compose uses Authentik `2026.8.4`, based on the upstream `version/2026.8.4` Compose generator. Review Authentik release notes and update the pinned image deliberately before a later production upgrade.
