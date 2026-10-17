# Home GW completion handoff

This is the work order for an agent with Home GW shell access and access to the Tailscale and Spaceship administration consoles. The owner authorized hosting both Authentik and oxnet.work on Home GW and chose **private split DNS**. Do not open internet ingress, add a Pangolin route, or change the existing Guacamole and Filebrowser Tailscale Serve routes.

## Last reported state; verify before changes

- Home GW is the Tailscale device `home-gw`, last reported at `100.71.184.10`.
- Authentik 2026.8.4 and its PostgreSQL/worker containers are healthy. Authentik listens on Home GW loopback `127.0.0.1:9000`.
- The oxnet.work image was built but the app was stopped. Its loopback host port is **8090** because Guacamole uses 8080.
- Source and Compose project are at `/opt/oxnetwork` and `/opt/oxnetwork/deploy/home-gw`. Previous local changes were recorded in `LOCAL_CHANGES.md`; preserve them when updating Git.
- Tailscale 1.102.4 serves Guacamole on the node's 443 and Filebrowser on its 8443. A separate Tailscale Service virtual IP is the planned ingress for the two new names.
- Public `auth.oxnet.work` previously resolved to `51.89.148.88` with a wrong certificate; `oxnet.work` had no public A record. Check again before editing Spaceship DNS.
- No private DNS resolver, Service registration, certificate, owner MFA, OIDC app, or live browser sign-in has been reported complete. Treat these as unverified.

## Work order

1. On Home GW, run `cd /opt/oxnetwork/deploy/home-gw && bash preflight-home-gw.sh`. Preserve its output without secrets. Check `git status`, `LOCAL_CHANGES.md`, and Compose state. Update source from GitHub `main` to the archive's `SOURCE_COMMIT.txt` or newer without overwriting local `.env`, `secrets/`, `certs/`, or local changes. Never run a blind `git reset --hard`.
2. Check `tailscale service list --json`, node Serve status, Home GW's tag (`tagged-devices` at last report), port 8445 availability, and tailnet Service permissions/ACLs. Define and advertise a **separate Service** with raw TCP 443 targeting `127.0.0.1:8445`. Record its virtual IP. Do not reuse the node's 443 or 8443 and do not enable Funnel. Follow the installed CLI's `tailscale service --help` and the admin console because syntax and permissions depend on version and tailnet policy.
3. Set up a tailnet-reachable DNS resolver if none exists. In Tailscale DNS, configure split DNS for `oxnet.work` to that resolver. Its answers for `oxnet.work` and `auth.oxnet.work` must be the **Service virtual IP**, not `100.71.184.10`. Check A and AAAA results from a tailnet client and do not let a public IPv6 answer bypass the private route. Preserve other records under the zone through forwarding or explicit records. Remove the stale public `auth.oxnet.work` A record in Spaceship once private resolution is ready. Use Spaceship public DNS for `_acme-challenge` TXT records for this deployment.
4. On Home GW, issue a trusted certificate covering both names through Spaceship DNS-01, install its full chain and key at `deploy/home-gw/certs/fullchain.pem` and `certs/key.pem`, restrict access, and arrange renewal plus a `private-https` restart after renewal. Keep Spaceship API credentials on Home GW only. Start the supplied Caddy edge with `docker compose --env-file .env -f compose.yml -f private-edge.compose.yml up -d private-https`. Run `bash verify-private-edge.sh` on Home GW and test HTTPS from a tailnet client.
5. Complete Authentik's initial `akadmin` password setup if unfinished, create the owner's normal account with MFA, and create the confidential OAuth2/OIDC application `oxnet-work`. Its redirect URI is exactly `https://oxnet.work/auth/callback`, the issuer is `https://auth.oxnet.work/application/o/oxnet-work/`, and its access policy should allow only the owner or intended group. Enter credentials locally through `sudo bash finish-oxnet.sh` from a real terminal; it stores the OIDC client secret on Home GW and starts the app. Do not send account or client secrets in chat.
6. Confirm locally that `127.0.0.1:8090/readyz` is ready and that the app's container can resolve and validate the Authentik issuer name and certificate. From a tailnet browser, verify login, authenticated API, sign out, and that an unauthenticated API call receives 401. From outside the tailnet, verify both names are unreachable. Check `tailscale serve status` again to prove Guacamole and Filebrowser remain intact.
7. The dashboard remains simulated by default. After sign-in works, optionally enable [read-only Tailscale inventory](live-tailscale.compose.yml) using [README.md](README.md). It reports device identity and unknown health; it does not provide host metrics.
8. Record the Service VIP/name, private resolver address, certificate renewal method, Compose path, source commit, image versions, backup location, test results, and template changes in a local deployment note. Report any blocker with sanitized command output. Back up Authentik PostgreSQL/data, oxnet SQLite, certificates, and secrets before considering the installation complete.

## Inputs created or supplied on Home GW

- Tailscale Service definition and admin permissions, plus a resolver reachable by tailnet clients.
- Spaceship DNS access for DNS-01 and removal of the stale public A record. Keep API credentials out of this archive and Git.
- Certificate and automatic renewal.
- Authentik owner credentials/MFA and OIDC client ID/secret. Use local prompts and protected secret files.

This handoff is based on the repository and prior Home GW agent reports. The cloud workspace has no Home GW VPN access, so no live host changes or verification are claimed here.
