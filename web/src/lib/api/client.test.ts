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
    expect(f.mock.calls[0][1].headers['X-Requested-With']).toBeUndefined();
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

  it('passes the AbortSignal through and adds CSRF headers on mutations', async () => {
    const f = vi.fn().mockResolvedValue(json({ id: 'a' }, 202));
    const ctl = new AbortController();
    await createApiClient({ fetch: f, csrfToken: () => 'tok' }).createNodeAction(
      'n1',
      { type: 'drain', idempotency_key: 'k' },
      { signal: ctl.signal },
    );
    const init = f.mock.calls[0][1];
    expect(init.signal).toBe(ctl.signal);
    expect(init.method).toBe('POST');
    expect(init.headers['X-CSRF-Token']).toBe('tok');
    expect(init.headers['X-Requested-With']).toBe('oxnet-web');
    expect(JSON.parse(init.body)).toEqual({ type: 'drain', idempotency_key: 'k' });
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
    es.onerror!(new Event('error'));
    close();
    expect(events).toEqual([
      { type: 'node.updated', data: { id: 'n1' } },
      { type: 'activity.created', data: 'refetch' },
    ]);
    expect(statuses).toEqual(['connecting', 'open', 'reconnecting', 'closed']);
    expect(es.close).toHaveBeenCalled();
  });
});
