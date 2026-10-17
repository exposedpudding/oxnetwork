# oxnet.work

**oxnet.work** is a lightweight, self-hosted command centre for a private network and distributed development/compute environment.

It is designed to unify the operational view of Pangolin, Tailscale, remote access, Docker, OpenCode workers, monitoring, automation, backups, and related infrastructure without turning the homelab into a heavyweight platform.

## Repository handoff pack

This package is an implementation specification for Codex and supporting agents. It contains no production secrets.

Start here:

1. `AGENTS.md` - authoritative implementation contract.
2. `CODEX.md` - primary agent handoff.
3. `CLAUDE.md` - bounded work Claude Code can take on.
4. `docs/PRODUCT_SPEC.md` - product requirements.
5. `docs/ARCHITECTURE.md` - system architecture.
6. `docs/UI_SPEC.md` - approved interface/design system.
7. `docs/API_SPEC.md` and `specs/openapi.yaml` - API contract.
8. `docs/SECURITY.md` - security boundaries.
9. `docs/INTEGRATIONS.md` - external systems/adapters.
10. `docs/DEPLOYMENT.md` - deployment model.
11. `docs/ROADMAP.md` - implementation milestones.
12. `docs/DECISIONS.md` - architectural decision log.

## Approved UI references

- `docs/images/01-overview.png`
- `docs/images/02-compute.png`
- `docs/images/03-node-detail.png`
- `docs/images/04-network.png`
- `docs/images/05-automations.png`

The images are design references, not sources of live infrastructure facts.

## Intended production shape

```text
Browser
  |
  | HTTPS / Tailnet
  v
oxnet.work control plane
  |- React static UI
  |- Go API
  |- SQLite
  |- scheduler / audit / SSE
  |
  +---- provider adapters ---- Pangolin / Tailscale / OpenCode / monitors
  |
  +---- private agent API ---- OX nodes / Docker hosts / lightweight devices
```

The default deployment should remain small enough to run comfortably as a single Docker service with one persistent data volume.

## Current build

Milestones 0 and 1 are complete, and Milestone 2 has read-only Compute and Node Detail screens. The app uses SQLite-backed observations from a deterministic demo provider. The production routes are `/`, `/compute`, and `/nodes/{id}`. All displayed infrastructure and activity are marked as simulated. Authentication, live integrations, node actions, and deployment to a home gateway are not implemented. Do not expose this build publicly.

### Run locally

Requires Go 1.27, Node 24, npm, and a C compiler for SQLite.

```bash
cd web
npm ci
npm test
npm run build
cd ..
go test ./...
go run ./cmd/server
```

The app binds to `127.0.0.1:8080` by default. Check `http://127.0.0.1:8080/readyz`, then open the local UI. SQLite state lives in `./data/oxnet.db`. For a config file, run `go run ./cmd/server -config config/config.example.yaml`. The supported environment overrides are `OXNET_LISTEN`, `OXNET_DATA_DIR`, `OXNET_WEB_DIR`, and `OXNET_SITE_NAME`.

Run the browser smoke tests from `web` with `npm run smoke`. The delegated multi-screen fixture harness has separate tests under `npm run smoke:fixtures`. Set `CHROMIUM_PATH` to a system Chromium executable if Playwright's managed browser is not installed.

### Run in Docker

```bash
docker compose -f deploy/docker-compose.yml up --build -d
curl -f http://127.0.0.1:8080/readyz
```

The Compose port is bound to host loopback and persists SQLite in a named volume. The container listens on all of its own interfaces solely to receive that loopback-published port. This unauthenticated demo should stay private. Stop it with `docker compose -f deploy/docker-compose.yml down` (leave the volume intact).

Cloud builds behind a custom TLS proxy may need `docker build --secret id=proxy_ca,src="$CODEX_PROXY_CERT" -t oxnet-work:dev .` before `docker compose -f deploy/docker-compose.yml up -d`. The certificate is mounted only in dependency download steps and is not copied into the image.
