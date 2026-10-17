/**
 * SSE helper for GET /api/v1/events. Payloads may be `{refetch: true}` hints rather than resources. Deliberately separate from the REST client.
 * EventSource reconnects on its own; we only surface the connection status so the UI can
 * show a stale indicator while disconnected.
 */
export const EVENT_TYPES = [
  'connected', // sent once on open by the server; payload names the mode (e.g. demo)
  'node.updated',
  'node.health_changed',
  'service.updated',
  'service.health_changed',
  'job.updated',
  'workflow_run.updated',
  'action.updated',
  'activity.created',
  'integration.health_changed',
] as const;

export type EventType = (typeof EVENT_TYPES)[number];
export type StreamStatus = 'connecting' | 'open' | 'reconnecting' | 'closed';

export interface ServerEvent {
  type: EventType;
  data: unknown;
}

export interface EventStreamOptions {
  url?: string;
  onEvent: (event: ServerEvent) => void;
  onStatus?: (status: StreamStatus) => void;
  /** Injectable for tests. */
  eventSourceFactory?: (url: string) => EventSource;
}

/** Opens the stream; returns a function that closes it. */
export function connectEventStream(opts: EventStreamOptions): () => void {
  const url = opts.url ?? '/api/v1/events';
  const create = opts.eventSourceFactory ?? ((u: string) => new EventSource(u, { withCredentials: false }));
  const es = create(url);
  let closed = false;
  opts.onStatus?.('connecting');

  es.onopen = () => opts.onStatus?.('open');
  es.onerror = () => {
    if (closed) return;
    opts.onStatus?.(es.readyState === 2 /* CLOSED */ ? 'closed' : 'reconnecting');
  };
  for (const type of EVENT_TYPES) {
    es.addEventListener(type, (ev) => {
      const raw = (ev as MessageEvent<string>).data;
      let data: unknown = raw;
      try {
        data = JSON.parse(raw);
      } catch {
        /* payload may be a plain refetch hint; pass through as string */
      }
      opts.onEvent({ type, data });
    });
  }
  return () => {
    closed = true;
    es.close();
    opts.onStatus?.('closed');
  };
}
