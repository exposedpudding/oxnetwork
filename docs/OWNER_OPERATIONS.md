# Owner Operations Notes - oxnet.work

This file is the concise operational source used to create the owner PDF.

## What oxnet.work does

oxnet.work provides one view of the private infrastructure while keeping specialist systems in their proper roles. Tailscale remains the private network, Pangolin handles controlled exposure, monitoring tools collect deeper telemetry, and OpenCode provides development compute. oxnet.work unifies their operational state and adds safe actions, audit, and automation.

## Normal access model

Prefer accessing oxnet.work through the tailnet. If it is exposed through Pangolin, authentication and access policy must already be enabled.

## Daily owner checks

- global health;
- stale/offline nodes;
- failed services/workflows;
- compute capacity;
- recent activity;
- backup status.

## High-impact controls

Use drain before maintenance on a compute node. Confirm active jobs before restart/shutdown. Treat backup success as incomplete until periodic restore tests pass. Review the audit trail after unusual behavior.

## Recovery priority

1. Restore access to the control plane/private network.
2. Restore the SQLite database/config.
3. Confirm integration credentials are available through secret sources.
4. Start in read-only/degraded mode if action providers are unavailable.
5. Re-enroll agents only if their credentials cannot be recovered safely.
