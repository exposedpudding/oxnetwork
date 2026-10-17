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
