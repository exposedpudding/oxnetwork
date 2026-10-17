/**
 * Minimal typed fetch wrapper for the oxnet.work REST API (/api/v1).
 *
 * Credentials policy
 * ------------------
 * The UI is served by the control plane, so API calls are same-origin and authenticated by the
 * Secure/HttpOnly/SameSite session cookie (docs/SECURITY.md section 5). We therefore use
 * `credentials: 'same-origin'` and NEVER `include`. No tokens are stored or read in JS.
 * CSRF: NO scheme is implemented yet (docs/DECISIONS.md ADR-008). The spec and server are
 * read-only, so this client is GET-only and sends no CSRF material. The auth milestone must add
 * its CSRF scheme and typed mutations together; do not add mutation calls before then.
 *
 * Errors never throw: every call resolves to a `Result`. Aborts resolve to code `ABORTED`.
 */
import type {
  Activity,
  ApiError,
  ComputeSummary,
  Job,
  ListResponse,
  NetworkGraph,
  GraphEdge,
  Node,
  Metrics,
  Service,
  SystemInfo,
  Worker,
} from './types';

export type Result<T> = { ok: true; data: T; status: number } | { ok: false; error: ApiError };

export interface ClientOptions {
  baseUrl?: string;
  fetch?: typeof fetch;
}

export interface RequestOptions {
  signal?: AbortSignal;
}

function fail(error: Omit<ApiError, 'status'> & { status?: number }): { ok: false; error: ApiError } {
  return { ok: false, error: { status: 0, ...error } };
}

export function createApiClient(options: ClientOptions = {}) {
  const baseUrl = (options.baseUrl ?? '/api/v1').replace(/\/+$/, '');
  const doFetch = options.fetch ?? ((...args: Parameters<typeof fetch>) => fetch(...args));

  async function request<T>(
    method: string,
    path: string,
    opts: RequestOptions & { body?: unknown; query?: Record<string, string | number | undefined> } = {},
  ): Promise<Result<T>> {
    let url = `${baseUrl}${path}`;
    if (opts.query) {
      const qs = new URLSearchParams();
      for (const [k, v] of Object.entries(opts.query)) if (v !== undefined) qs.set(k, String(v));
      const s = qs.toString();
      if (s) url += `?${s}`;
    }
    const headers: Record<string, string> = { Accept: 'application/json' };
    if (opts.body !== undefined) headers['Content-Type'] = 'application/json';

    let res: Response;
    try {
      res = await doFetch(url, {
        method,
        headers,
        credentials: 'same-origin',
        signal: opts.signal,
        body: opts.body === undefined ? undefined : JSON.stringify(opts.body),
      });
    } catch (e) {
      if (e instanceof DOMException && e.name === 'AbortError') {
        return fail({ code: 'ABORTED', message: 'Request was cancelled.' });
      }
      return fail({ code: 'NETWORK_ERROR', message: 'Could not reach the control plane.' });
    }

    const text = await res.text().catch(() => '');
    let json: unknown;
    if (text) {
      try {
        json = JSON.parse(text);
      } catch {
        if (res.ok) return fail({ code: 'INVALID_RESPONSE', message: 'Response was not valid JSON.', status: res.status });
      }
    }

    if (!res.ok) {
      const e = (json as { error?: Partial<ApiError> } | undefined)?.error;
      return fail({
        code: e?.code ?? `HTTP_${res.status}`,
        message: e?.message ?? res.statusText ?? 'Request failed.',
        request_id: e?.request_id,
        status: res.status,
      });
    }
    return { ok: true, data: json as T, status: res.status };
  }

  const id = encodeURIComponent;
  return {
    getSystem: (o?: RequestOptions) => request<SystemInfo>('GET', '/system', o),
    listNodes: (o?: RequestOptions) => request<ListResponse<Node>>('GET', '/nodes', o),
    getNode: (nodeId: string, o?: RequestOptions) => request<Node>('GET', `/nodes/${id(nodeId)}`, o),
    getNodeMetrics: (nodeId: string, o?: RequestOptions) =>
      request<Metrics>('GET', `/nodes/${id(nodeId)}/metrics`, o),
    listServices: (o?: RequestOptions) => request<ListResponse<Service>>('GET', '/services', o),
    getService: (serviceId: string, o?: RequestOptions) => request<Service>('GET', `/services/${id(serviceId)}`, o),
    listActivity: (o?: RequestOptions) => request<ListResponse<Activity>>('GET', '/activity', o),
    getComputeSummary: (o?: RequestOptions) => request<ComputeSummary>('GET', '/compute/summary', o),
    listWorkers: (o?: RequestOptions) => request<ListResponse<Worker>>('GET', '/compute/workers', o),
    listJobs: (o?: RequestOptions) => request<ListResponse<Job>>('GET', '/jobs', o),
    getNetworkGraph: (o?: RequestOptions) => request<NetworkGraph>('GET', '/network/graph', o),
    listConnections: (o?: RequestOptions) => request<ListResponse<GraphEdge>>('GET', '/network/connections', o),
  };
}

export type ApiClient = ReturnType<typeof createApiClient>;
