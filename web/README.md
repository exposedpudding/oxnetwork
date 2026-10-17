# oxnet.work web

React + TypeScript + Vite frontend. This directory currently holds the delegated Claude Code packages CC-01..CC-05 (see `/CLAUDE.md`); the production app shell and screen integration remain with the primary agent.

```bash
npm install
npm run dev        # dev harness at http://localhost:5173/#/dev/overview (fixture data, dev builds only)
npm test           # vitest: 42 unit/component tests
npm run build      # tsc + vite build
npm run test:e2e   # Playwright smoke tests against the dev harness
```

If the sandbox already has Chromium, run e2e with `PLAYWRIGHT_CHROMIUM_PATH=/path/to/chrome`.

## Layout

| Path | Package | Notes |
|---|---|---|
| `src/components/` | CC-01 | AppShell, Sidebar/NavItem, Topbar, HealthPill, Card/StatCard, StatusDot/StatusChip, StaleBadge, MetricBar, DataTable, EmptyState/ErrorState, ConfirmDangerDialog, CommandPalette |
| `src/features/topology/` | CC-02 | `layoutGraph()` (pure, deterministic) + `TopologyView` (SVG) + text fallback |
| `src/lib/api/` | CC-03 | `createApiClient`, `connectEventStream` (SSE, separate), `freshnessOf`, types |
| `src/features/workflows/` | CC-04 | `WorkflowEditor`, `WorkflowDiagram`, validation |
| `src/fixtures/`, `src/dev/`, `tests/e2e/` | CC-05 | Deterministic fixtures, dev-only harness pages, Playwright tests |
| `src/styles/tokens.css` | | Copy of `config/design-tokens.css`; the only source of colours/radii/spacing |

## Notes for integration

- **Credentials policy** (documented in `client.ts`): same-origin session cookie, `credentials: 'same-origin'`, never `include`; no tokens in JS. Mutations send `X-Requested-With: oxnet-web` and an optional `X-CSRF-Token` via the `csrfToken` option once the server defines its scheme.
- **Provisional types**: `specs/openapi.yaml` has no schemas for services, compute, jobs, metrics or workflows. The types in `lib/api/types.ts` and `features/workflows/types.ts` follow `docs/API_SPEC.md` and are marked provisional. Please add schemas to the spec and replace these.
- **Topology** is decoupled from any provider: it takes `NetworkGraph` and a `kindLabels` map. Layout depends only on node/edge ids, not input order.
- **Components never hardcode hosts/actions.** Editor actions, targets and channels come from `EditorOptions` supplied by the host (capability-driven).
- The dev harness (`src/dev`) is loaded only when `import.meta.env.DEV`; the production bundle excludes it (~69 KB gzip JS including React).
- Dialogs: `ConfirmDangerDialog` is UI only. Server-side confirmation, authorisation and audit remain required by `docs/SECURITY.md`.
