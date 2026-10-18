# Integration Plan - oxnet.work

## General rules

Each integration has:

- an adapter package;
- configuration schema;
- health state;
- last successful sync;
- read capabilities;
- optional action capabilities;
- fixture implementation for tests/demo;
- strict timeouts/backoff.

No integration is allowed to block the global dashboard indefinitely.

## 1. Tailscale

Purpose:

- node discovery and private identity;
- tailnet addresses;
- online/last-seen information where available;
- relationship context for network graph;
- private reachability for agents/control plane.

Initial mode: read-only discovery.

Do not assume MagicDNS names are permanent IDs.

The opt-in adapter reads `GET /api/v2/tailnet/{tailnet}/devices` with a Tailscale OAuth client limited to `devices:core:read`. It obtains short-lived access tokens and renews them automatically. Legacy API-key configuration remains available for compatibility, but Tailscale's traditional API tokens have broad access and should not be used on Home GW. The adapter stores Tailscale's device ID as stable identity, display hostname, OS, addresses and last-seen timestamp. Last seen does not prove that a host is currently reachable. Every imported node has `unknown` health and no fabricated metrics, services or graph edges. A failed poll leaves the previous successful inventory in SQLite so its observation freshness ages; switching from demo to live clears simulated records before the first poll. Live mode requires complete OIDC configuration. See `deploy/home-gw/live-tailscale.compose.yml` for the Home GW overlay.

## 2. Pangolin

Purpose:

- inventory exposed resources/routes/tunnels;
- show which services are externally reachable;
- deep links to Pangolin administration;
- later, bounded create/update actions if API support and security are satisfactory.

Initial mode: read-only.

## 3. Docker

Preferred architecture: local oxnet agent -> local Docker Engine.

Read:

- containers;
- image name;
- state;
- uptime/start time;
- lightweight CPU/RAM summary where practical.

Actions later:

- restart known container;
- start/stop with confirmation;
- deploy approved Compose stack through explicit stack definitions.

Never provide arbitrary Docker API proxying to the browser.

## 4. OpenCode

Treat OpenCode as a provider behind a compute/job adapter. Exact implementation may evolve with the deployed OpenCode architecture.

The owner reports that Gemini is deploying OX-C-01 as the main OpenCode host. Treat its API and availability as unverified until that work is complete; do not show OX-C-01 as a live worker based on this note alone.

Normalize:

- worker identity;
- worker status/capabilities;
- job state;
- requested resources/tags;
- placement;
- timestamps;
- output/result reference.

Do not make the rest of oxnet.work dependent on OpenCode being installed.

## 5. Beszel

Use for host/container monitoring if it provides the required API in the deployed environment. Otherwise the oxnet agent may provide basic live metrics directly.

oxnet.work should display operational summaries and link to deeper monitoring rather than duplicate full historical analysis.

## 6. Uptime Kuma

Potential uses:

- import endpoint/service status;
- deep-link to checks;
- consume notification/status data where API integration is suitable.

Do not duplicate every Uptime Kuma configuration feature.

## 7. Grafana / Prometheus

Potential uses:

- deep links from node/service detail;
- optional query adapter for selected metrics;
- dashboard links.

Grafana remains the deep visualization surface.

## 8. Authentik

Target role: OIDC provider for owner/operator authentication.

Requirements:

- issuer/client IDs configurable;
- scopes minimal;
- group/claim mapping optional;
- no hard dependency for demo mode.

## 9. Backups

Design a generic backup provider interface. Initial implementation may wrap a configured script/tool only through a typed agent action, e.g. restic.

Expose:

- target/repository label;
- last run;
- last success;
- duration;
- size where available;
- verify/check status where available.

Never expose repository passwords to the frontend.

## 10. RDP / SSH

v1 should provide launch/deep-link actions and connection metadata, not browser-embedded terminals.

Examples:

- RDP file/download or OS-supported launch link where feasible;
- copy hostname/Tailscale IP;
- SSH URI/command copy.

Avoid storing plaintext remote account passwords.

## 11. Wake-on-LAN

Wake may be executed by a node/agent on the relevant LAN. The control plane should not assume that sending broadcast WOL from the VPS can reach home/work LAN segments.

Model a `wake_proxy` capability with target MAC/interface/network metadata stored securely/configurably.
