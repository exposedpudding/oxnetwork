# Home GW agent handoff

The owner authorized hosting **both Authentik and oxnet.work on Home GW**. Home GW is the Tailscale device `home-gw`, last supplied as `100.71.184.10`. The command centre URL is `https://oxnet.work`; `https://auth.oxnet.work` is the proposed Authentik URL. Keep both **Tailscale private**. A DNS name alone does not grant internet access, but do not add public ingress, Pangolin resources, router forwarding, or listeners on the public interface.

The archive containing this file is a clean source snapshot; it contains no credentials or live Home GW configuration. Its Git commit is recorded in the archive's `SOURCE_COMMIT.txt`. Start with [`README.md`](README.md) and [`compose.yml`](compose.yml) in this directory. The repository's [`AGENTS.md`](../../AGENTS.md) and [`docs/SECURITY.md`](../../docs/SECURITY.md) describe the project rules. The application currently displays **simulated, read-only inventory**.

## Assignment

1. Connect to Home GW through Tailscale using your existing authorized access. Inspect OS/architecture, CPU and RAM, free storage, Docker/Compose, existing Authentik/PostgreSQL/proxy services, DNS, open ports, and backups. Authentik needs at least 2 cores and 2 GB RAM. Preserve existing services and data. If Authentik already runs on Home GW, integrate that instance instead of starting a duplicate.
2. Establish private DNS and trusted HTTPS certificates for `oxnet.work` and `auth.oxnet.work`. Use DNS validation for certificates; bind the HTTPS proxy to Home GW's Tailscale interface only. Confirm the public internet cannot reach either service. If the exact domain or DNS control is unavailable, report the blocker rather than exposing an HTTP alternative.
3. Follow the staged instructions in `README.md`: run `bash bootstrap-authentik.sh` on Home GW, help the owner reach the initial Authentik setup through an SSH tunnel to `127.0.0.1:9000`, create the `oxnet-work` confidential OIDC application, then run `bash finish-oxnet.sh` from an interactive terminal after private HTTPS is ready. The scripts generate and write credentials **on Home GW**; the client secret prompt does not echo. Restrict Authentik app access to the intended owner/group and enable MFA. Keep all credentials out of Git, the archive, logs and chat.
4. Check `/readyz` locally. From a tailnet browser, verify redirect to Authentik, login, authenticated API, CSRF protected sign out, and that unauthenticated `/api/v1/system` returns 401. Confirm the OIDC issuer discovery URL works from the oxnet-work container using its trusted certificate. Confirm both HTTPS names are inaccessible from outside the tailnet.
5. Record installed image versions, Compose project path, proxy/DNS configuration, backup locations, test results and any changes to the template. Report remaining blockers clearly. Do not claim the demo data is live infrastructure telemetry.

## Supplied and missing inputs

- Supplied: complete application source, vendored Go dependencies, frontend lockfile, Dockerfile, Home GW Compose template, configuration example, migrations, tests and deployment instructions.
- Create locally: PostgreSQL password, Authentik secret key, Authentik admin/owner account, OIDC client ID and secret. The Compose template deliberately requires these at activation.
- Discover on Home GW: SSH account, capacity, current reverse proxy, available DNS/certificate method and backup scheme. Do not assume the current Tailscale IP is permanent; verify device identity.

The repository snapshot was checked with Go tests, frontend tests/build, a Docker build, a local `/readyz` and `/api/v1/system` smoke request, and `docker compose config` with placeholder values. It has **not** been deployed or tested against a live Authentik instance on Home GW.
