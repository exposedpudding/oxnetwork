# Private HTTPS edge draft for Home GW

**Prepared locally; not applied on Home GW.** The owner chose private Tailscale DNS. Home GW's node currently serves Guacamole on 443 and Filebrowser on 8443. Keep those routes intact.

## Proposed path

```text
tailnet browser -> private DNS: oxnet.work/auth.oxnet.work -> Tailscale Service VIP:443
             -> raw TCP forward -> Home GW 127.0.0.1:8445 -> Caddy TLS
             -> oxnet-work:8080 or authentik-server:9000 (Compose network)

oxnet-work container -> Compose DNS alias auth.oxnet.work -> Caddy TLS
                     -> authentik-server:9000
```

The Service virtual IP separates this 443 listener from Home GW's existing node Serve routes. Caddy uses a DNS-01 certificate for both requested names. The Compose alias keeps the OIDC issuer name and certificate identical for browsers and the application.

## Checks before changing Home GW

1. Run `tailscale service list --json` and inspect any existing Services. Check `tailscale version`, Service advertisement permissions for Home GW's `tagged-devices` tag, client ACL reachability, and whether `127.0.0.1:8445` is free.
2. Identify a tailnet DNS resolver that can answer `oxnet.work` and `auth.oxnet.work` with the **new Service VIP**. A Tailscale split DNS rule must point to a reachable resolver. Check whether port 53 is already in use before adding one. Do not put the Service VIP into public Spaceship A records under this private DNS plan.
3. Remove the stale public Spaceship A record for `auth.oxnet.work` (reported as `51.89.148.88`) once the correct private records are ready. Spaceship still hosts public `_acme-challenge` TXT records for certificate issuance.
4. Create a Spaceship DNS API key and secret with the minimum available DNS rights. Give them to the Home GW certificate client through a protected local secret store, not chat or Git. `acme.sh` supports the `dns_spaceship` provider (`SPACESHIP_API_KEY`, `SPACESHIP_API_SECRET`). Issue one SAN certificate for `oxnet.work` and `auth.oxnet.work`; install full chain and key to `certs/fullchain.pem` and `certs/key.pem`. Restrict file permissions, and arrange automatic renewal followed by a `private-https` restart.
5. Review [`private-edge.compose.yml`](private-edge.compose.yml) and [`Caddyfile`](Caddyfile). Start the proxy with `docker compose --env-file .env -f compose.yml -f private-edge.compose.yml up -d private-https`. It publishes TLS only on Home GW loopback port 8445.
6. Define/advertise a **separate** Tailscale Service with TCP 443, forwarding raw TCP to `127.0.0.1:8445`. Use raw TCP, so Caddy presents the certificate for the custom domains. Check the Home GW CLI's Service syntax and admin settings before changing it; do not use Tailscale's HTTPS termination for these names.
7. Configure split DNS for the two names to the Service VIP. Validate from a tailnet client that both resolve correctly and present the expected certificate. Validate from outside the tailnet that both names are unreachable. Then create the Authentik OIDC app and run `finish-oxnet.sh`.

If Service advertisement or a private DNS resolver is unavailable, stop at that blocker and report it. Do not change the existing node Serve routes, enable Funnel, open a public port, or substitute an HTTP issuer.
