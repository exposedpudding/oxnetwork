# OpenCode compute providers (Milestone 8, ADR-017)

oxnet.work does not run agents. It sits above OpenCode, which does the coding work, and shows where that work runs. This document is the contract between oxnet.work and an OpenCode server (`opencode serve`), verified against OpenCode **1.19.3** on OX-C-01. It covers what is read, how it maps onto oxnet.work's compute model, how to configure it, and what is deliberately not done yet.

## What is read (and what never is)

The adapter (`internal/integrations/opencode`) is strictly read-only. It makes only `GET` requests, to four stable endpoints:

| Endpoint | Used for |
|---|---|
| `GET /global/health` | reachability, `healthy`, server `version` |
| `GET /project` | the working directory of every project the server knows |
| `GET /session?roots=true&limit=50&directory=<dir>` | root sessions of one project (title, agent, model, tokens, cost, times, archived flag) |
| `GET /session/status?directory=<dir>` | which sessions are `busy` or `retry`; idle sessions are absent |

It never requests messages, parts, file contents, diffs, permissions, configuration, credentials, or the event stream, and it never sends a prompt, abort, delete or any other write. A test records every request a poller makes and fails on anything outside the table above. Responses are capped at 2 MB, requests time out after 5 seconds, redirects are not followed, TLS is verified, proxy environment variables are ignored, and basic-auth credentials go only to the configured server. Session **titles** can contain the first words of a prompt, so they are whitespace-collapsed and truncated to 80 characters, and they are visible only to signed-in users of the dashboard.

## The one thing that is easy to get wrong: projects

OpenCode scopes the session list **and** the status map to a *project directory*. A request without `directory` only sees the server's default project. This was found by testing against the real server: a session that was visibly busy in the `scratch` project did not appear in `/session/status` at all until the request named that project's directory, and the unscoped list did not contain it either. An adapter that skips this reports "0 active" while agents work. So each poll reads `/project`, then reads the list and status once for the default project and once for every project directory (at most 20), and merges the results by session id. If the project list or any project cannot be read, the provider is `degraded`; the last complete session list is retained as stale, and active-job totals are labeled incomplete rather than presenting a partial read as a complete count.

## Mapping onto oxnet.work

| OpenCode | oxnet.work | Notes |
|---|---|---|
| a configured `opencode serve` instance | **provider** (`/api/v1/compute/providers`) | `online` only if `/global/health` answers `healthy: true` and the session inventory is complete; `degraded` means the server answered but a project read failed; `offline` carries reachability or health failures |
| the machine it runs on | `node_id` (optional, `tailscale-<id>`) | links a provider to an inventory node; host CPU and RAM still come from the host agent |
| a root session | **job** (`/api/v1/jobs`) | id is `<provider id>:<session id>`; archived sessions are hidden |
| status `busy` or `retry` | job state `running` | counted in `active_jobs` |
| any other session | job state `idle` | an idle session is not a success or failure; there is no queue, so `queued_jobs` stays 0 |

Freshness: a provider is stale if it has not been read for three polling rounds (default 45 seconds). If a server goes offline or a session read is incomplete, its last complete list is kept and shown as **stale** rather than disappearing. Jobs on an offline, degraded or stale provider do not count as active. `used_vcpu` stays a placeholder: OpenCode does not report CPU use per session, and oxnet.work does not guess.

## Configuration

Providers are fixed in server configuration, live mode only, and cannot be added through the API or UI. In YAML (`opencode_servers`) or in the file named by `OXNET_OPENCODE_SERVERS_FILE`:

```yaml
opencode_poll_interval_seconds: 15        # 5-300
opencode_servers:
  - id: ox-c-01                           # lowercase letters, digits, dashes
    name: OX-C-01 OpenCode
    url: http://100.69.88.98:4096         # http or https; no credentials, query or fragment
    username: opencode                    # default "opencode"
    password_file: /run/secrets/opencode_ox_c_01   # absolute path; read once at startup, never logged or returned
    node_id: tailscale-2365559827781033   # optional inventory node
```

At most 10 servers. A missing or empty password file stops startup with a clear error (as for the other secrets), rather than silently running unauthenticated. Unknown keys are rejected.

### Home GW

`deploy/home-gw/live-opencode.compose.yml` and `opencode-servers.example.yml` add the file and the password secret. Steps: copy the example to `opencode-servers.yml`; create `secrets/opencode_ox_c_01` containing the OpenCode server password (owner action; `root:10001`, mode `640`, never in Git or chat); add the overlay to every Compose command for the app. **Check network reachability first**: from Home GW, `curl -u opencode:... http://100.69.88.98:4096/global/health` must work, and the container reaches it through the host's Tailscale connection, so a Tailscale grant must allow Home GW to reach OX-C-01 on port 4096. The OX-C-01 server listens on its Tailscale address only.

## Verification performed

- Unit tests with a fake server that enforces the same directory scoping as the real one: busy sessions in a non-default project, duplicate sessions across projects, a failing project, a failing project list, offline then stale, authentication failure, oversized, non-JSON and redirecting responses, timeouts, and the read-only request allow-list.
- The adapter was run against the **real** server on OX-C-01: it read 11 sessions (all idle); then a throwaway session was created in the `scratch` project, sent a prompt for the local model, and the adapter reported `running`, `active = 1`, `total = 12` with model `ollama/qwen2.5:3b`. The test session was then aborted and deleted (0 left, status empty).
- A first version that only read the default project missed that busy session entirely; that is the reason for the project handling above.

## Not done (remaining Milestone 8 items)

- **Submitting and cancelling work** through oxnet.work. This needs the security review the roadmap requires, typed actions with confirmation and audit (Milestone 9), and a decision on credentials the web server would then hold. Nothing here sends a prompt.
- **Placement integration.** The placement policy (`internal/compute`) is unchanged; it does not yet consider OpenCode servers.
- **A multi-node pool.** Today there is one server (OX-C-01). Pooling OX-C-02 and others, electing a primary, and running local models across nodes' CPUs (for example llama.cpp RPC) are design work for when those nodes exist. The adapter already supports several servers.
- **Per-session CPU or memory accounting**, which OpenCode does not expose.
- **The v2 API** (`/api/session`, `/api/session/active`) exists in 1.19.3 but was not adopted: it is newer, and the v1 endpoints above are the documented stable ones. `GET /api/session/active` returned empty during the busy-session test, so it should not be relied on without checking.
