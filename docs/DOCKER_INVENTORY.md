# Docker container inventory

Read-only container visibility on Node Detail and on the Integrations page. Design and trade-offs: ADR-019 in `DECISIONS.md` (it builds on the proposal in ADR-015). **Opt-in per host:** reporters currently run on Home GW and OX-C-01; other hosts have no reporter.

## What it shows

For each host that has a reporter: container name, image, state (running, exited, restarting and so on), health result if the image defines a health check, creation time, published ports, and the Docker Compose project and service names. It reads **only** those fields. Environment variables, mounts, command lines, labels (other than the two Compose names), logs and networks are never collected. No container can be started, stopped or restarted from oxnet.work.

A host that has never reported shows "No container report". A report older than 90 seconds is shown as stale, so a stopped reporter cannot leave containers looking current.

If the Docker list has more than 300 entries or contains an entry the reporter cannot represent, the reporter does not submit a partial list. The last complete report goes stale instead. The reporter also rejects HTTP redirects, so its bearer token stays on the configured endpoint.

## How it works

```
Docker host                                     Home GW
oxnet-docker-reporter  --HTTPS, own token-->    POST /internal/v1/docker-report/tailscale-ID
 (GET /containers/json only)                    GET  /api/v1/nodes/tailscale-ID/containers
```

`cmd/docker-reporter` is the only oxnet.work program that touches `/var/run/docker.sock`, and the only request it ever sends there is `GET /containers/json?all=1`. It opens no port. The server stores the latest list per node (`node_containers`, migration 009) and replaces it whole on every report.

## Security properties

- **Own token.** Container reports use `secrets/docker-agents/<node-id>`, separate from the host-metrics token. A leaked metrics token cannot forge container state, and the reverse. Tests assert both directions.
- **Strict body.** Unknown JSON fields, trailing data, oversized bodies (256 KiB), more than 300 containers, duplicate ids, control characters and unrecognised states are rejected, so a modified reporter cannot smuggle extra fields in.
- **Fixed node.** The node comes from the URL and must already exist in the Tailscale inventory.
- **Docker access is root-equivalent.** The reporter's account must be able to read the Docker socket (group `docker`). That is the one real cost of this feature. The systemd unit drops everything else (no capabilities, `ProtectSystem=strict`, `PrivateDevices`, address families limited to Unix and IP, `NoNewPrivileges`). Install it only on hosts where you accept that.

## Install

1. On Home GW, add the overlay to the Compose command: `deploy/home-gw/live-docker-reporter.compose.yml` (alongside the existing overlays). It mounts `secrets/docker-agents/` read-only and sets `OXNET_DOCKER_AGENT_TOKEN_DIR`. Rebuild and restart the app (take a backup first, see `BACKUP_RESTORE.md`).
2. For each Docker host, create a token on Home GW (the exact `tailscale-<id>` is on the node's detail URL):

   ```sh
   sudo bash /opt/oxnetwork/deploy/agents/create-docker-token.sh tailscale-NODEID
   ```

3. Build the reporter and copy it, the token file, `install-docker-reporter.sh` and `oxnet-docker-reporter@.service` to the host:

   ```sh
   CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -o oxnet-docker-reporter-linux-amd64 ./cmd/docker-reporter
   sudo bash install-docker-reporter.sh tailscale-NODEID ./oxnet-docker-reporter-linux-amd64 ./node-token
   journalctl -u oxnet-docker-reporter@tailscale-NODEID.service -n 20 --no-pager
   ```

   On Home GW itself, point it at the loopback port: add `-endpoint http://127.0.0.1:8090/internal/v1/docker-report` to `ExecStart`, as the host agent does.
4. Open the node's detail page; the Containers card fills within a minute. The Integrations page lists "Docker container reporters".

To remove a host: stop its unit and delete its token file on Home GW; its last list goes stale and the card says so.

## Not included

Logs, exec, start/stop/restart (Milestone 9 typed actions), image or volume inventory, resource use per container, and workflow triggers on container health. These need separate decisions.
