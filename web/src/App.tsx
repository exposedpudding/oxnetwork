import { useEffect, useMemo, useRef, useState } from 'react'
import { loadCompute, loadNode, loadOverview } from './api'
import { ComputeView, NodeDetailView } from './DetailViews'
import { ErrorState } from './components/States'
import type { Activity, Node, Service, System } from './types'

type OverviewData = { system: System; nodes: Node[]; services: Service[]; activity: Activity[] }

const stateLabel: Record<string, string> = { healthy: 'Healthy', degraded: 'Degraded', unhealthy: 'Unhealthy', unknown: 'Unknown', disabled: 'Disabled' }

function Status({ state, freshness }: { state: string; freshness?: string }) {
  const label = freshness === 'stale' ? 'Stale' : (stateLabel[state] ?? state)
  return <span className={`status status-${freshness === 'stale' ? 'stale' : state}`}><span className="status-dot" aria-hidden="true" />{label}</span>
}

function Meter({ label, value }: { label: string; value?: number }) {
  return <div className="meter"><span>{label}</span><strong>{value == null ? 'Not reported' : `${Math.round(value)}%`}</strong>{value != null && <span className="meter-track"><span style={{ width: `${Math.max(0, Math.min(100, value))}%` }} /></span>}</div>
}

function timeLabel(value: string) { return new Date(value).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) }

export function OverviewView({ data, query }: { data: OverviewData; query: string }) {
  const lower = query.trim().toLowerCase()
  const nodes = data.nodes.filter(node => `${node.name} ${node.role}`.toLowerCase().includes(lower))
  const services = data.services.filter(service => service.name.toLowerCase().includes(lower))
  const knownVCPU = data.nodes.reduce((total, node) => total + (node.metrics.vcpu ?? 0), 0)
  const fresh = data.nodes.filter(node => node.freshness === 'fresh').length
  const attention = data.nodes.filter(node => node.freshness === 'stale' || node.health.state === 'degraded' || node.health.state === 'unhealthy').length
  return <>
    <div className="page-heading"><div><p className="eyebrow">Command centre / Overview</p><h1>Overview</h1><p>Infrastructure, services and activity at a glance.</p></div><span className="timestamp">Updated {timeLabel(data.system.server_time)}</span></div>
    <div className="demo-banner" role="status"><strong>Demo mode</strong><span>All inventory and activity shown here is simulated. No live infrastructure is connected.</span></div>
    <section aria-label="Summary" className="summary-grid">
      <div className="summary-card"><span>Nodes</span><strong>{data.nodes.length}</strong><small>{fresh} fresh · {attention} need attention</small></div>
      <div className="summary-card"><span>Services</span><strong>{data.services.length}</strong><small>{data.services.filter(service => service.health.state === 'healthy' && service.freshness === 'fresh').length} healthy</small></div>
      <div className="summary-card"><span>Reported compute</span><strong>{knownVCPU} <em>vCPU</em></strong><small>Sample capacity, including stale reports</small></div>
      <div className="summary-card"><span>Recent activity</span><strong>{data.activity.length}</strong><small>Simulated events</small></div>
    </section>
    <section className="section"><div className="section-heading"><div><p className="eyebrow">Inventory</p><h2>Nodes</h2></div><span>{nodes.length} shown</span></div><div className="node-grid">{nodes.map(node => <article className="node-card" key={node.id}><div className="node-title"><span className="node-icon" aria-hidden="true">▦</span><div><h3><a className="inline-link" href={`/nodes/${encodeURIComponent(node.id)}`}>{node.name}</a></h3><p>{node.role} · {node.platform}</p></div><Status state={node.health.state} freshness={node.freshness} /></div><div className="node-metrics"><Meter label="CPU" value={node.metrics.cpu_percent} /><Meter label="RAM" value={node.metrics.ram_percent} /></div><div className="card-foot"><span>{node.metrics.vcpu == null ? 'Capacity not reported' : `${node.metrics.vcpu} vCPU · ${node.metrics.ram_gb ?? '—'} GB RAM`}</span><span>Simulated</span></div></article>)}{nodes.length === 0 && <p className="empty">No nodes match your search.</p>}</div></section>
    <div className="detail-grid"><section className="panel"><div className="section-heading"><div><p className="eyebrow">Availability</p><h2>Services health</h2></div></div><div className="rows">{services.map(service => <div className="row" key={service.id}><div><strong>{service.name}</strong><small>Demo source</small></div><Status state={service.health.state} freshness={service.freshness} /></div>)}{services.length === 0 && <p className="empty">No services match your search.</p>}</div></section><section className="panel"><div className="section-heading"><div><p className="eyebrow">Timeline</p><h2>Recent activity</h2></div></div><div className="rows">{data.activity.map(item => <div className="row activity-row" key={item.id}><span className={`activity-mark ${item.state}`} aria-hidden="true" /><div><strong>{item.title}</strong><small>{item.detail}</small></div><time dateTime={item.occurred_at}>{timeLabel(item.occurred_at)}</time></div>)}</div></section></div>
    <section className="actions-note"><div><strong>Quick actions</strong><p>Actions will appear when authenticated, capability-aware integrations are ready.</p></div><span>Unavailable in demo</span></section>
  </>
}

type Page = { kind: 'overview' } | { kind: 'compute' } | { kind: 'node'; id: string } | { kind: 'missing' }

function pageFromPath(pathname: string): Page {
  if (pathname === '/') return { kind: 'overview' }
  if (pathname === '/compute') return { kind: 'compute' }
  const match = /^\/nodes\/([^/]+)$/.exec(pathname)
  if (match) {
    try { return { kind: 'node', id: decodeURIComponent(match[1]) } } catch { return { kind: 'missing' } }
  }
  return { kind: 'missing' }
}

export function App() {
  const pathname = window.location.pathname
  const page = pageFromPath(pathname)
  const [data, setData] = useState<OverviewData | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [pageError, setPageError] = useState<string | null>(null)
  const [compute, setCompute] = useState<Awaited<ReturnType<typeof loadCompute>> | null>(null)
  const [node, setNode] = useState<Awaited<ReturnType<typeof loadNode>> | null>(null)
  const [query, setQuery] = useState('')
  const searchRef = useRef<HTMLInputElement>(null)
  useEffect(() => {
    let alive = true
    const refresh = () => {
      void loadOverview().then(result => { if (alive) { setData(result); setError(null) } }).catch(reason => { if (alive) setError(String(reason)) })
      if (page.kind === 'compute') void loadCompute().then(result => { if (alive) { setCompute(result); setPageError(null) } }).catch(reason => { if (alive) setPageError(String(reason)) })
      if (page.kind === 'node') void loadNode(page.id).then(result => { if (alive) { setNode(result); setPageError(null) } }).catch(reason => { if (alive) setPageError(String(reason)) })
    }
    void refresh()
    const timer = window.setInterval(refresh, 30000)
    const events = new EventSource('/api/v1/events')
    events.addEventListener('node.updated', refresh)
    events.addEventListener('service.updated', refresh)
    return () => { alive = false; window.clearInterval(timer); events.close() }
  }, [pathname])
  useEffect(() => {
    const keydown = (event: KeyboardEvent) => { if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') { event.preventDefault(); searchRef.current?.focus() } }
    window.addEventListener('keydown', keydown)
    return () => window.removeEventListener('keydown', keydown)
  }, [])
  const overall = useMemo(() => !data ? 'Loading' : data.nodes.some(node => node.freshness === 'stale' || node.health.state === 'degraded' || node.health.state === 'unhealthy') ? 'Needs attention' : 'All systems healthy', [data])
  const content = page.kind === 'overview'
    ? data ? <OverviewView data={data} query={query} /> : error ? <ErrorState message={error} /> : <div className="loading" role="status">Loading overview…</div>
    : page.kind === 'compute'
      ? compute ? <ComputeView data={compute} query={query} /> : pageError ? <ErrorState message={pageError} /> : <div className="loading" role="status">Loading compute…</div>
      : page.kind === 'node'
        ? node ? <NodeDetailView node={node} /> : pageError ? <ErrorState message={pageError} /> : <div className="loading" role="status">Loading node…</div>
        : <ErrorState title="Page not found" message="This page does not exist." />
  return <div className="app-shell"><aside className="sidebar"><div className="brand">oxnet<span>.work</span></div><nav aria-label="Primary"><a href="/" aria-current={page.kind === 'overview' ? 'page' : undefined} className={page.kind === 'overview' ? 'nav-active' : ''}><span aria-hidden="true">▦</span>Overview</a><a href="/compute" aria-current={page.kind === 'compute' ? 'page' : undefined} className={page.kind === 'compute' ? 'nav-active' : ''}><span aria-hidden="true">◫</span>Compute</a></nav><div className="sidebar-bottom"><span className="sidebar-dot" />Demo workspace</div></aside><div className="main-column"><header className="topbar"><div className="site-pill">⌂ <span>{data?.system.site_name ?? 'Homelab'}</span></div>{page.kind !== 'node' && <div className="search-wrap"><span aria-hidden="true">⌕</span><input ref={searchRef} type="search" value={query} onChange={event => setQuery(event.target.value)} placeholder={page.kind === 'compute' ? 'Search workers and jobs' : 'Search nodes and services'} aria-label={page.kind === 'compute' ? 'Search workers and jobs' : 'Search nodes and services'} /><kbd>⌘ K</kbd></div>}<span className="overall">{overall}</span></header><main>{error && data && <div role="alert" className="error">Unable to refresh overview data: {error}. Showing the last available view.</div>}{pageError && (compute || node) && <div role="alert" className="error">Unable to refresh this page: {pageError}. Showing the last available view.</div>}{content}</main></div></div>
}
