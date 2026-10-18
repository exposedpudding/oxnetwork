import { createApiClient } from './client';
import { connectEventStream } from './sse';
import { freshnessOf } from './freshness';

const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });

describe('api client', () => {
  it('returns typed data and uses same-origin credentials', async () => {
    const f = vi.fn().mockResolvedValue(json({ items: [] }));
    const c = createApiClient({ fetch: f });
    const r = await c.listNodes();
    expect(r).toEqual({ ok: true, data: { items: [] }, status: 200 });
    expect(f.mock.calls[0][0]).toBe('/api/v1/nodes');
    expect(f.mock.calls[0][1].credentials).toBe('same-origin');
      });

  it('parses semantic errors', async () => {
    const f = vi.fn().mockResolvedValue(
      json({ error: { code: 'ACTION_NOT_SUPPORTED', message: 'nope', request_id: 'r1' } }, 400),
    );
    const r = await createApiClient({ fetch: f }).getNode('a/b');
    expect(f.mock.calls[0][0]).toBe('/api/v1/nodes/a%2Fb');
    expect(r).toEqual({
      ok: false,
      error: { code: 'ACTION_NOT_SUPPORTED', message: 'nope', request_id: 'r1', status: 400 },
    });
  });

  it('falls back to HTTP_<status> for non-JSON errors', async () => {
    const f = vi.fn().mockResolvedValue(new Response('<html>', { status: 502 }));
    const r = await createApiClient({ fetch: f }).getSystem();
    expect(r.ok === false && r.error.code).toBe('HTTP_502');
  });

  it('maps abort and network failures without throwing', async () => {
    const abort = vi.fn().mockRejectedValue(new DOMException('x', 'AbortError'));
    const ra = await createApiClient({ fetch: abort }).getSystem({ signal: new AbortController().signal });
    expect(ra.ok === false && ra.error.code).toBe('ABORTED');
    const net = vi.fn().mockRejectedValue(new TypeError('fail'));
    const rn = await createApiClient({ fetch: net }).getSystem();
    expect(rn.ok === false && rn.error.code).toBe('NETWORK_ERROR');
  });

  it('passes the AbortSignal through and sends no CSRF material on reads', async () => {
    const f = vi.fn().mockResolvedValue(json({ cpu_percent: 5 }));
    const ctl = new AbortController();
    const r = await createApiClient({ fetch: f }).getNodeMetrics('n1', { signal: ctl.signal });
    const init = f.mock.calls[0][1];
    expect(r.ok && r.data.cpu_percent).toBe(5);
    expect(f.mock.calls[0][0]).toBe('/api/v1/nodes/n1/metrics');
    expect(init.signal).toBe(ctl.signal);
    expect(init.method).toBe('GET');
    expect(init.body).toBeUndefined();
    expect(Object.keys(init.headers)).toEqual(['Accept']);
  });
});

describe('implemented read endpoints', () => {
  it('hits system, service, activity, compute and network endpoints', async () => {
    const f = vi.fn().mockImplementation(async () => json({ items: [] }));
    const c = createApiClient({ fetch: f });
    await c.getSystem();
    await c.getService('s 1');
    await c.listActivity();
    await c.getComputeSummary();
    await c.listWorkers();
    await c.listJobs();
    await c.getNetworkGraph();
    await c.listConnections();
    expect(f.mock.calls.map((x) => x[0])).toEqual([
      '/api/v1/system', '/api/v1/services/s%201', '/api/v1/activity',
      '/api/v1/compute/summary', '/api/v1/compute/workers', '/api/v1/jobs',
      '/api/v1/network/graph', '/api/v1/network/connections',
    ]);
  });
});

describe('automation endpoints and CSRF', () => {
  const ok = () => vi.fn().mockImplementation(async () => json({ items: [] }));

  it('sends the CSRF token on mutations only', async () => {
    const f = ok();
    const c = createApiClient({ fetch: f, csrfToken: () => 'tok-1' });
    await c.listWorkflows();
    await c.runWorkflow('wf 1');
    await c.updateWorkflow('wf_1', { name: 'x' });
    expect(f.mock.calls.map((x) => [x[1].method, x[0]])).toEqual([
      ['GET', '/api/v1/workflows'],
      ['POST', '/api/v1/workflows/wf%201/run'],
      ['PATCH', '/api/v1/workflows/wf_1'],
    ]);
    expect(Object.keys(f.mock.calls[0][1].headers)).toEqual(['Accept']);
    expect(f.mock.calls[1][1].headers['X-CSRF-Token']).toBe('tok-1');
    expect(f.mock.calls[1][1].body).toBeUndefined();
    expect(f.mock.calls[2][1].headers).toMatchObject({ 'X-CSRF-Token': 'tok-1', 'Content-Type': 'application/json' });
    expect(JSON.parse(f.mock.calls[2][1].body)).toEqual({ name: 'x' });
  });

  it('omits the header when there is no token and hits every automation path', async () => {
    const f = ok();
    const c = createApiClient({ fetch: f });
    await c.createWorkflow({ name: 'n', enabled: true, trigger: { type: 'manual' }, steps: [], on_success: {}, on_failure: { behavior: 'stop' } });
    await c.enableWorkflow('a');
    await c.disableWorkflow('a');
    await c.getWorkflow('a');
    await c.listWorkflowRuns('a', 5);
    await c.listRecentRuns(3);
    await c.getWorkflowRun('r1');
    await c.getAutomationSummary();
    await c.getWorkflowOptions();
    await c.listAudit({ limit: 10, target_id: 'a' });
    expect(f.mock.calls[0][1].headers['X-CSRF-Token']).toBeUndefined();
    expect(f.mock.calls.slice(1).map((x) => x[0])).toEqual([
      '/api/v1/workflows/a/enable', '/api/v1/workflows/a/disable', '/api/v1/workflows/a',
      '/api/v1/workflows/a/runs?limit=5', '/api/v1/workflow-runs?limit=3', '/api/v1/workflow-runs/r1',
      '/api/v1/automations/summary', '/api/v1/workflows/options', '/api/v1/audit?limit=10&target_id=a',
    ]);
  });

  it('exposes per-field validation details', async () => {
    const f = vi.fn().mockResolvedValue(
      json({ error: { code: 'INVALID_WORKFLOW', message: 'bad', request_id: 'r9', details: { name: 'Name is required.' } } }, 400),
    );
    const r = await createApiClient({ fetch: f }).createWorkflow({ name: '', enabled: true, trigger: { type: 'manual' }, steps: [], on_success: {}, on_failure: { behavior: 'stop' } });
    expect(r.ok === false && r.error.details).toEqual({ name: 'Name is required.' });
    expect(r.ok === false && r.error.status).toBe(400);
  });
});

describe('freshness', () => {
  const h = { state: 'healthy' as const, observed_at: '2026-01-01T00:00:00Z', stale_after_seconds: 60 };
  it('classifies live, stale and unavailable', () => {
    expect(freshnessOf(h, Date.parse('2026-01-01T00:00:30Z'))).toBe('live');
    expect(freshnessOf(h, Date.parse('2026-01-01T00:02:00Z'))).toBe('stale');
    expect(freshnessOf(undefined)).toBe('unavailable');
  });
});

describe('event stream', () => {
  it('parses events, reports status and closes', () => {
    const listeners: Record<string, (e: MessageEvent) => void> = {};
    const es = {
      readyState: 0,
      close: vi.fn(),
      addEventListener: (t: string, l: (e: MessageEvent) => void) => (listeners[t] = l),
    } as unknown as EventSource & { onopen: () => void; onerror: () => void };
    const events: unknown[] = [];
    const statuses: string[] = [];
    const close = connectEventStream({
      onEvent: (e) => events.push(e),
      onStatus: (s) => statuses.push(s),
      eventSourceFactory: () => es,
    });
    es.onopen!(new Event('open'));
    listeners['node.updated']({ data: '{"id":"n1"}' } as MessageEvent);
    listeners['activity.created']({ data: 'refetch' } as MessageEvent);
    listeners['connected']({ data: '{"mode":"demo"}' } as MessageEvent);
    es.onerror!(new Event('error'));
    close();
    expect(events).toEqual([
      { type: 'node.updated', data: { id: 'n1' } },
      { type: 'activity.created', data: 'refetch' },
      { type: 'connected', data: { mode: 'demo' } },
    ]);
    expect(statuses).toEqual(['connecting', 'open', 'reconnecting', 'closed']);
    expect(es.close).toHaveBeenCalled();
  });
});
