# Home GW setup handoff

Give this source snapshot to the agent who can access Home GW through Tailscale. Start with [`deploy/home-gw/AGENT_HANDOFF.md`](deploy/home-gw/AGENT_HANDOFF.md), then follow [`deploy/home-gw/README.md`](deploy/home-gw/README.md).

For the owner-selected private DNS and separate Tailscale Service proposal, see [`deploy/home-gw/PRIVATE_EDGE.md`](deploy/home-gw/PRIVATE_EDGE.md). That edge remains a draft until Home GW's Service permissions, DNS resolver, and Spaceship certificate access are checked.

After inspecting Home GW's capacity, existing services, DNS and reverse proxy, the setup uses two commands from `deploy/home-gw/`:

```sh
bash bootstrap-authentik.sh
# Create the confidential oxnet-work OIDC application in Authentik.
bash finish-oxnet.sh
```

The scripts create required new secrets on Home GW and prompt for the OIDC client credentials there. This archive contains **no passwords, private keys or existing Home GW login credentials**. Keep both services accessible only through Tailscale; no public route is authorized. The dashboard data remains simulated until live adapters are installed.
