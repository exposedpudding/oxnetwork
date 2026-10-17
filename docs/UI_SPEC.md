# UI / UX Specification - oxnet.work

## 1. Approved direction

The UI direction is **Mission Control capability with Linear/Vercel restraint**.

It should feel like a modern operations product, not a game HUD and not a generic tile launcher. The approved mockups are:

- `images/01-overview.png`
- `images/02-compute.png`
- `images/03-node-detail.png`
- `images/04-network.png`
- `images/05-automations.png`

The earlier presentation artwork may be used for branding/owner documentation but is intentionally more dramatic than the production UI.

## 2. Visual rules

### Core palette

Suggested starting tokens:

```css
--bg: #07111f;
--bg-elevated: #0b1726;
--panel: #0c1929;
--panel-hover: #102238;
--border: #1c2c42;
--text: #f4f7fb;
--text-muted: #91a3ba;
--primary: #19c8ff;
--primary-strong: #2f8fff;
--healthy: #38d98b;
--warning: #f3ae45;
--critical: #ff5d68;
--info: #4aa8ff;
```

Final values may be tuned, but keep contrast and hierarchy consistent.

### Typography

Use a system-first modern sans stack or one small self-hosted font if already available in the app assets. Avoid remote font dependencies.

- Page title: 28-34px desktop.
- Card title: 15-18px.
- Body: 14px.
- Dense table metadata: 12-13px.
- Monospace only for logs, IDs, IPs, code, command examples.

### Surfaces

- 8-12px corner radius.
- 1px subtle borders.
- Very mild shadow only for floating menus/dialogs.
- Avoid glowing containers.
- Use color primarily for state and interaction, not decoration.

## 3. Global shell

### Sidebar

Desktop width approximately 220-250px, collapsible to icon rail.

Items:

- Overview
- Compute
- Services
- Network
- Automations
- Activity
- Storage
- Security
- Settings

Do not show dead navigation. Hide/disable incomplete pages behind feature flags during development.

### Top bar

Contains:

- environment/site selector;
- global search/command field;
- global health summary;
- notifications;
- account/session menu.

Keyboard shortcut: `Ctrl+K` / `Cmd+K` opens command palette.

## 4. Overview screen

Reference: `images/01-overview.png`.

Priority order:

1. global state and node state;
2. service health;
3. compute capacity;
4. recent activity;
5. quick actions.

Node cards are compact and consistent. Do not build a gauge for every metric. Use simple bars and numbers.

Quick actions are capability-aware. Dangerous actions use red styling only at point of action/confirmation.

## 5. Compute screen

Reference: `images/02-compute.png`.

Required components:

- summary cards;
- capacity visualization;
- workers table;
- job table;
- scheduler explanation;
- quick actions.

Do not imply that vCPU is truly fungible across machines. Aggregate values are informational. Job placement respects individual-node constraints.

## 6. Node Detail screen

Reference: `images/03-node-detail.png`.

Header shows identity, freshness, OS/platform if reported, role, and actions.

Metric cards show only metrics the node reports. If temperature is unavailable, do not reserve a fake value; use `Not reported` or omit based on density.

Tabs:

- Overview
- Containers
- Jobs
- Network
- Logs

Logs should be bounded previews. Do not attempt to become a full log analytics platform.

## 7. Network screen

Reference: `images/04-network.png`.

v1 topology rules:

- SVG nodes and edges;
- orthogonal or gently curved connectors;
- no physics simulation;
- no animated packets;
- no WebGL;
- selected node receives primary border;
- status is shown with small state dot;
- zoom/reset only if useful;
- textual list/table remains available below.

The topology is an operational map, not a perfect layer-2 network diagram.

## 8. Automations screen

Reference: `images/05-automations.png`.

Use a standard list/table for workflows. The selected workflow uses a **linear** diagram:

`Trigger -> Step 1 -> Step 2 -> Outcome`

Do not build a general node-canvas editor in v1.

Statuses:

- success;
- running;
- queued;
- failed;
- disabled;
- cancelled.

## 9. Command palette

The palette is an accelerator, not an AI dependency.

v1 commands can be deterministic:

- navigate to node/service/page;
- wake node;
- open RDP/SSH launch action;
- run workflow;
- run backup;
- show unhealthy services;
- search logs/events metadata;
- drain/undrain node after confirmation.

Natural-language/AI interpretation is a future optional layer.

## 10. States every component must support

- loading;
- live/fresh;
- stale;
- unavailable;
- empty;
- permission denied;
- partial integration failure.

Skeletons are acceptable for short loads. Long-running operations need explicit progress/state rather than indefinite spinners.

## 11. Responsive behavior

Primary target is desktop. Tablet support is required.

- sidebar collapses;
- 4-card rows wrap to 2x2;
- tables may become horizontally scrollable with frozen identity column;
- topology details pane can become a drawer;
- quick actions wrap.

Phone support may be read-mostly in v1.

## 12. Accessibility

- keyboard navigation for all controls;
- WCAG-conscious contrast;
- color is never the only state indicator;
- semantic tables/headings;
- dialogs trap focus and return focus on close;
- icons have accessible labels/tooltips where needed;
- reduced-motion setting respected.
