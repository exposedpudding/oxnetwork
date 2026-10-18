# Host metrics agents

The optional Go agent reads CPU usage, physical RAM usage, system/root disk usage and capacity on Linux or Windows 11. It sends a small JSON report every 30 seconds to oxnet.work. It has no listener, Docker socket, action API or remote shell. A report is matched to the existing Tailscale inventory node ID, never to a display hostname. The server marks the report stale after 90 seconds. `healthy` on a host-agent observation means **recent reporting**, not that the host's services are healthy.

As of 18 October 2026, Home GW, `ox-c-01`, `ovh-vps` and Windows 11 `DESKTOP-HHBRUDD` report metrics. The other four discovered devices have no host collector. The Windows installation runs in the owner's interactive session and starts at logon, not at boot before sign-in.

## Server enrollment

On Home GW, keep the `live-tailscale.compose.yml` overlay and add `live-host-agent.compose.yml` to every Compose command. The latter mounts `deploy/home-gw/secrets/host-agents/` read-only in the app. It is a root-owned directory with mode `750`, group `10001`; each token file is root:`10001` mode `640`. Never commit or print these files.

Find the exact `tailscale-<numeric ID>` from the authenticated `/api/v1/nodes` response or the node detail URL. Then run on Home GW:

```sh
sudo bash /opt/oxnetwork/deploy/agents/create-host-token.sh tailscale-NODEID
```

The script refuses to overwrite an existing credential. Copy that one node's token file securely to its host. Treat it as a telemetry-submit credential: possession can forge readings for that node, but it cannot read inventory or execute actions. Revoke a node by stopping its agent and removing its token file from Home GW; the old report then ages to stale.

Agents use `https://oxnet.work/internal/v1/host-report/tailscale-NODEID` over the private Tailscale path. The server accepts only a matching bearer token and a known Tailscale inventory node. Unauthenticated requests return 401. Home GW's agent uses `http://127.0.0.1:8090` instead of the network path.

## Build

From the repository root with Go 1.27 and vendored dependencies:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -o oxnet-host-agent-linux-amd64 ./cmd/host-agent
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -mod=vendor -ldflags=-H=windowsgui -o oxnet-host-agent-windows-amd64.exe ./cmd/host-agent
```

Build for another architecture by changing `GOARCH`. The Windows GUI subsystem flag prevents a console window at logon; use `-log-file` for diagnostics. The two tested amd64 binaries from this deployment are saved as Codex task outputs, not in Git.

## Install on Linux

Copy the Linux binary, the node's token file, `install-linux.sh`, and `oxnet-host-agent@.service` to the host. Use the exact inventory ID:

```sh
sudo bash install-linux.sh tailscale-NODEID ./oxnet-host-agent-linux-amd64 ./node-token
systemctl status oxnet-host-agent@tailscale-NODEID.service
journalctl -u oxnet-host-agent@tailscale-NODEID.service -n 30 --no-pager
```

The systemd unit runs under a dynamic unprivileged user and receives the root-only token through `LoadCredential`. The machine must resolve and trust the private `oxnet.work` HTTPS endpoint. The Home GW variant at `deploy/home-gw/oxnet-host-agent@.service` uses the loopback port.

On `ox-c-01` (`tailscale-2365559827781033`) and `ovh-vps` (`tailscale-8781071780610037`), the generic Linux unit is enabled at boot. A Tailscale grant allows only their tailnet IPs, `100.69.88.98` and `100.123.16.3`, to reach `svc:oxnet-work` on TCP 443. Both hosts have `100.90.240.202 oxnet.work` in `/etc/hosts` because their existing DNS configuration did not resolve the private name; the original file is backed up as `/etc/hosts.oxnetwork-before-2026-10-18`. If the Service VIP changes, update this mapping. The hosts still validate the trusted HTTPS certificate for `oxnet.work`.

Use the host systemd unit for host metrics on Docker-capable Linux machines. Running this collector in a container would require host `/proc`, root filesystem and CPU/memory visibility, weakening isolation and complicating which filesystem the numbers describe. The agent has no Docker dependency or socket access.

## Install on Windows 11

Copy the Windows binary and the node's token file to the target PC. Run in a PowerShell session for the account that should report while logged on:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\install-windows.ps1 -NodeId tailscale-NODEID -Binary .\oxnet-host-agent-windows-amd64.exe -TokenFile .\node-token
```

The script installs under `%LOCALAPPDATA%\oxnetwork\agent`, restricts the token to the current user and SYSTEM, registers an `HKCU\...\Run` logon entry, and starts the agent in a hidden window. Remove the temporary transferred token after installation. Read `%LOCALAPPDATA%\oxnetwork\agent\agent.log` for errors. The agent retries transient network failures every 30 seconds; if the process exits, the logon entry starts it at the next sign-in.

Do not reuse one node's token for another. The Windows agent reads the system drive; the Linux agent reads `/`. Other disks, temperature, service state, jobs, and containers need separate providers. Device entries without an installed agent remain `unknown` and show `Not reported` resource metrics.
