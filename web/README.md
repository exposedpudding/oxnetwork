# oxnet.work web

React + TypeScript + Vite frontend.

- **Production app** (`src/App.tsx`, `src/DetailViews.tsx`, `src/NetworkView.tsx`, `src/main.tsx`, `src/api.ts`, owned by Codex): Go-backed Overview, Compute, Node Detail and Network routes. It uses the delegated typed client, topology renderer and presentation components.
- **Delegated packages CC-01..CC-05** (see `/CLAUDE.md`): reusable components, topology, API client, workflow editor, fixtures. They are not imported by the production entry yet, so they are excluded from the production bundle.
- **Fixture harness** (`fixtures.html` -> `src/dev/`): multi-screen pages using static fixtures. Served only by `vite dev` at `/fixtures.html#/dev/overview`; the production build input is `index.html` only. Controls there change local state only.

```bash
npm ci
npm run dev              # app at :5173 (proxies /api to the Go server); harness at :5173/fixtures.html
npm test                 # vitest (jsdom): unit + component tests
npm run build            # tsc --noEmit + vite build
npm run smoke            # production smoke test against the Go server (needs Go; starts it)
npm run smoke:fixtures   # multi-screen tests against the Vite fixture harness (no backend)
```

If the sandbox already has Chromium, set `CHROMIUM_PATH=/path/to/chrome` for either Playwright run.

## Layout

| Path | Package | Notes |
|---|---|---|
| `src/components/` | CC-01 | AppShell, Sidebar/NavItem, Topbar, HealthPill, Card/StatCard, StatusDot/StatusChip, StaleBadge, MetricBar, DataTable, EmptyState/ErrorState, ConfirmDangerDialog, CommandPalette |
| `src/features/topology/` | CC-02 | `layoutGraph()` (pure, deterministic) + `TopologyView` (SVG) + text fallback |
| `src/lib/api/` | CC-03 | `createApiClient`, `connectEventStream` (SSE, separate), `freshnessOf`, types |
| `src/features/workflows/` | CC-04 | `WorkflowEditor`, `WorkflowDiagram`, validation |
| `src/fixtures/`, `src/dev/`, `tests/fixtures/` | CC-05 | Deterministic fixtures, dev-only harness pages, Playwright tests |
| `src/styles/tokens.css` | | Copy of `config/design-tokens.css`; the only source of colours/radii/spacing |

## Notes for integration

- **Credentials policy** (documented in `client.ts`): same-origin session cookie, `credentials: 'same-origin'`, never `include`; no tokens in JS.
- **CSRF / mutations**: no CSRF scheme exists yet (ADR-008: read-only server until auth). The client is GET-only and has no mutation methods; the auth milestone should add its CSRF scheme and typed mutations together. Mutation-style controls exist only in the dev fixture harness (local state).
- **API types** follow `specs/openapi.yaml`, which now covers only implemented endpoints: `/system`, `/nodes`, `/nodes/{id}/metrics` (returns `Metrics`), `/services`, `/activity`, `/compute/summary`, `/compute/workers`, `/jobs`, `/events`. Anything else is marked **provisional** (job items, `NodeMetrics` extras such as temperature, topology graph input, workflow shapes). Compute `used_vcpu`/job counts are demo placeholders.
- **Topology** takes a `NetworkGraph` and a `kindLabels` map and is decoupled from any provider. Layout depends only on node/edge ids, not input order.
- **Components never hardcode hosts/actions.** Editor actions, targets and channels come from `EditorOptions` supplied by the host.
- `ConfirmDangerDialog` is UI only; server-side confirmation, authorisation and audit remain required by `docs/SECURITY.md`.
