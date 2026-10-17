# Deployment Guide - oxnet.work

## 1. Default deployment

The default target is a single Docker container plus a persistent data directory.

```text
oxnet-work
  /data/oxnet.db
  /data/backups/
  mounted config/secrets
```

The container serves both the API and the compiled frontend.

## 2. Network exposure

Recommended initial deployment:

- bind service to a host/private interface;
- access via Tailscale;
- do not publish directly to the public internet.

If public/remote web access is required, place Pangolin/access policy in front of it and enable oxnet.work authentication.

## 3. Docker Compose baseline

See `config/docker-compose.example.yml`.

Do not mount `/var/run/docker.sock` into the control plane by default. Docker access belongs on node agents.

## 4. Configuration

Configuration sources, lowest to highest precedence:

1. application defaults;
2. YAML config file;
3. environment variables;
4. explicit CLI arguments.

Secrets should come from environment or secret files, not committed YAML.

## 5. Persistent data

Required:

- SQLite DB;
- optional generated agent enrollment state;
- local backup snapshots.

The application should be able to recreate frontend assets from the image and configuration from documented source values; avoid mutable application binaries in the data directory.

## 6. Backup

For SQLite use a safe online backup mechanism rather than copying a live DB file blindly. Implement an application backup endpoint/CLI later or document `sqlite3 .backup` / VACUUM INTO equivalent.

Minimum backup set:

- SQLite database;
- config YAML without external secret values if those are separately managed;
- secret files if used, encrypted/protected separately;
- owner runbook.

## 7. Restore test

Document a restore flow to a clean temporary instance. A backup is not considered proven until a test restore successfully boots and passes readiness checks.

## 8. Agent deployment

Agent delivery options:

- Docker container on Linux hosts;
- systemd binary on Linux;
- Windows service on Windows nodes.

The agent config contains control-plane URL, agent identity, and credential reference. Keep the footprint small and support auto-reconnect with jittered backoff.

## 9. Releases

Recommended release artifacts:

- multi-arch container image (`amd64`, `arm64`);
- checksums;
- optional standalone server/agent binaries;
- migration notes.

Use semantic versions once the project leaves prototype stage.

## 10. Observability of oxnet.work itself

Expose:

- `/healthz`;
- `/readyz`;
- structured logs;
- optional `/metrics`.

Self-monitor the service with Uptime Kuma/Beszel where convenient.
