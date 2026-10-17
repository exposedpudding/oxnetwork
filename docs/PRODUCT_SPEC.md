# Product Specification - oxnet.work

## 1. Product statement

oxnet.work is the owner's private infrastructure command centre: one interface to understand the state of the network, see where compute capacity is available, inspect services and nodes, trigger approved operational actions, and automate repeatable work.

It is not intended to replace every specialist tool. Grafana can still be the deep metrics tool, Pangolin the edge/access tool, Tailscale the private mesh, and OpenCode the coding/compute layer. oxnet.work is the operational **control plane and unifying experience** above them.

## 2. Primary user

The initial product is single-owner / small trusted-operator oriented. Multi-tenant SaaS requirements are explicitly out of scope for v1.

The owner needs to answer, quickly:

- Is everything healthy?
- Which machine/service is unhealthy or stale?
- Which node has spare CPU/RAM/GPU capacity?
- What is OpenCode doing and where are jobs running?
- How are nodes/services connected?
- What changed recently?
- Can I safely wake, drain, restart, deploy, or back up a target?
- What automations exist and did they succeed?

## 3. Product pillars

### Observe

Provide an immediate, trustworthy picture of hosts, services, jobs, connections, and recent events.

### Operate

Provide narrowly scoped operational actions with confirmation, permissions, audit, and clear results.

### Automate

Turn repeated operational steps into simple workflows without becoming a general-purpose enterprise workflow suite.

### Coordinate compute

Treat capable devices as a compute pool. Show available capacity and allow OpenCode/job scheduling adapters to choose eligible nodes.

### Recover

Make backup state, last successful runs, runbooks, and audit history visible.

## 4. Scope for v1

### In scope

- single control-plane deployment;
- responsive web UI;
- node/service inventory;
- current health and freshness;
- host resource metrics where available;
- Docker/container inventory through agents;
- OpenCode worker/job abstraction;
- lightweight placement rules;
- simple network topology;
- activity/audit feed;
- workflow definitions and execution;
- safe node/service actions;
- configuration export/backup docs;
- OIDC-ready authentication architecture;
- dark UI matching approved mockups.

### Out of scope for v1

- Kubernetes management;
- generic SSH terminal embedded in browser;
- arbitrary shell command execution;
- full SIEM/log analytics;
- long-term high-cardinality metrics warehouse;
- multi-organization SaaS tenancy;
- billing/licensing;
- mobile native apps;
- complex drag-and-drop workflow canvas;
- service mesh replacement;
- replacing Grafana/Prometheus where already appropriate.

## 5. User journeys

### Journey A - morning health check

1. Open Overview.
2. See global health and stale/unhealthy items.
3. Inspect recent warnings.
4. Click affected node/service.
5. Review recent metrics/events.
6. Take a supported action or open the specialist tool.

### Journey B - start a compute-heavy job

1. Open Compute.
2. See workers and available resources.
3. Submit or observe an OpenCode job.
4. Scheduler chooses an eligible node based on capacity/rules.
5. Job progress appears in activity stream and node detail.
6. Result/failure is recorded.

### Journey C - node maintenance

1. Open Node Detail.
2. Drain the node so new work is not scheduled there.
3. Wait for active jobs to finish or explicitly handle them.
4. Perform restart/update action.
5. Node returns online and is marked schedulable.
6. Audit history shows who/what performed each step.

### Journey D - network understanding

1. Open Network.
2. View derived topology.
3. Select OX VPS, tailnet, worker, or service.
4. Inspect relationships and exposure status.
5. Open related specialist system if deeper configuration is required.

### Journey E - automation

1. Open Automations.
2. Review scheduled workflows and success rate.
3. Inspect a workflow.
4. Run it manually or enable/disable it.
5. Review step outcomes and run history.

## 6. Functional requirements

### FR-001 Inventory
The system shall maintain a normalized inventory of nodes, services, service instances, capabilities, and integrations.

### FR-002 Freshness
Every externally sourced state shall include `observed_at` and a freshness state: live, fresh, stale, unavailable, or unknown.

### FR-003 Health
Health shall be normalized to: healthy, degraded, unhealthy, unknown, disabled.

### FR-004 Actions
Actions shall be capability-aware. The UI must not show a Wake button for a node that has no configured wake capability, for example.

### FR-005 Audit
All actions and workflow runs shall create immutable audit events.

### FR-006 Compute
The system shall aggregate worker capacities and job state while preserving provider-specific metadata separately.

### FR-007 Scheduling
v1 scheduling shall use deterministic rules rather than AI decision-making. Inputs may include online state, drain state, free CPU/RAM/GPU, tags, capabilities, and explicit affinity/exclusion.

### FR-008 Topology
The system shall expose a graph representation independent of the visual layout.

### FR-009 Automations
Workflows shall support manual, cron, and simple condition/event triggers with ordered steps.

### FR-010 Degraded operation
A failed external integration shall not prevent unrelated pages or integrations from loading.

## 7. Non-functional requirements

- Fast initial render on a normal LAN/WAN connection.
- No required cloud service.
- ARM64 and AMD64 container builds.
- Backward-compatible API within `/api/v1`.
- SQLite backup must be documented and testable.
- Accessible keyboard navigation for core actions.
- UI must work at typical desktop widths and remain usable on a tablet.
- User-visible error messages must state what failed and whether data is stale.

## 8. Success criteria

A successful v1 replaces the need to manually visit multiple tools just to answer routine operational questions. It should reduce context switching while leaving deep configuration in the specialist products where appropriate.
