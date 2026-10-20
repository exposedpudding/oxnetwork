import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { automationActions, loadAuthSession, loadAutomations, loadCompute, loadIntegrations, loadNetwork, loadNode, loadOverview, logout, type AuthSession } from './api'
import { AutomationsView, type AutomationsData } from './features/automations/AutomationsView'
import { IntegrationsView } from './IntegrationsView'
import { ComputeView, NodeDetailView } from './DetailViews'
import { NetworkView } from './NetworkView'
import { ErrorState } from './components/States'
import type { Activity, Node, Service, System } from './types'

type OverviewData = { system: System; nodes: Node[]; services: Service[]; activity: Activity[] }

const stateLabel: Record<string, string> = { healthy: 'Healthy', degraded: 'Degraded', unhealthy: 'Unhealthy', unknown: 'Unknown', disabled: 'Disabled' }

function Status({ state, freshness, source }: { state: string; freshness?: string; source?: string }) {
  const label = freshness === 'stale' ? 'Stale' : source === 'host-agent' && state === 'healthy' ? 'Reporting' : (stateLabel[state] ?? state)
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
  const knownVCPU = data.nodes.reduce((total, node) => total + (data.system.demo_mode || node.freshness === 'fresh' ? node.metrics.vcpu ?? 0 : 0), 0)
  const reporting = data.nodes.filter(node => node.health.source === 'host-agent' && node.freshness === 'fresh').length
  const fresh = data.nodes.filter(node => node.freshness === 'fresh').length
  const probes = data.services.filter(service => service.health.source === 'http-probe').length
  const attention = data.nodes.filter(node => node.freshness === 'stale' || node.health.state === 'degraded' || node.health.state === 'unhealthy').length
  return <>
    <div className="page-heading"><div><p className="eyebrow">Command centre / Overview</p><h1>Overview</h1><p>Infrastructure, services and activity at a glance.</p></div><span className="timestamp">Updated {timeLabel(data.system.server_time)}</span></div>
    {data.system.demo_mode ? <div className="demo-banner" role="status"><strong>Demo mode</strong><span>All inventory and activity shown here is simulated. No live infrastructure is connected.</span></div> : <div className="demo-banner" role="status"><strong>Live inventory</strong><span>Tailscale lists devices. {reporting} {reporting === 1 ? 'host reports' : 'hosts report'} CPU, RAM and disk through a local collector. {probes > 0 ? `${probes} service ${probes === 1 ? 'probe checks its' : 'probes check their'} configured ${probes === 1 ? 'endpoint' : 'endpoints'}; other devices and services remain unverified.` : 'Other devices and service health remain unverified.'}</span></div>}
    <section aria-label="Summary" className="summary-grid">
      <div className="summary-card"><span>Nodes</span><strong>{data.nodes.length}</strong><small>{fresh} fresh · {attention} need attention</small></div>
      <div className="summary-card"><span>Services</span><strong>{data.services.length}</strong><small>{data.services.filter(service => service.health.state === 'healthy' && service.freshness === 'fresh').length} healthy</small></div>
      <div className="summary-card"><span>Reported host capacity</span><strong>{knownVCPU ? <>{knownVCPU} <em>vCPU</em></> : 'Not reported'}</strong><small>{data.system.demo_mode ? 'Sample capacity, including stale reports' : `${reporting} host ${reporting === 1 ? 'collector' : 'collectors'} reporting`}</small></div>
      <div className="summary-card"><span>Recent activity</span><strong>{data.activity.length}</strong><small>{data.system.demo_mode ? 'Simulated events' : 'No event source connected'}</small></div>
    </section>
    <section className="section"><div className="section-heading"><div><p className="eyebrow">Inventory</p><h2>Nodes</h2></div><span>{nodes.length} shown</span></div><div className="node-grid">{nodes.map(node => <article className="node-card" key={node.id}><div className="node-title"><span className="node-icon" aria-hidden="true">▦</span><div><h3><a className="inline-link" href={`/nodes/${encodeURIComponent(node.id)}`}>{node.name}</a></h3><p>{node.role} · {node.platform}</p></div><Status state={node.health.state} freshness={node.freshness} source={node.health.source} /></div><div className="node-metrics"><Meter label="CPU" value={node.metrics.cpu_percent} /><Meter label="RAM" value={node.metrics.ram_percent} />{node.metrics.disk_percent != null && <Meter label="Disk" value={node.metrics.disk_percent} />}</div><div className="card-foot"><span>{node.metrics.vcpu == null ? 'Capacity not reported' : `${node.metrics.vcpu} vCPU · ${node.metrics.ram_gb ?? '—'} GB RAM`}</span><span>{node.simulated ? 'Simulated' : node.health.source}</span></div></article>)}{nodes.length === 0 && <p className="empty">{lower ? 'No nodes match your search.' : 'No nodes reported yet.'}</p>}</div></section>
    <div className="detail-grid"><section className="panel"><div className="section-heading"><div><p className="eyebrow">Availability</p><h2>Services health</h2></div></div><div className="rows">{services.map(service => <div className="row" key={service.id}><div><strong>{service.name}</strong><small>{service.endpoint ? `${service.detail ?? 'Probed'} · ${service.endpoint}` : service.health.source}</small></div><Status state={service.health.state} freshness={service.freshness} /></div>)}{services.length === 0 && <p className="empty">{lower ? 'No services match your search.' : 'No services reported by this source.'}</p>}</div></section><section className="panel"><div className="section-heading"><div><p className="eyebrow">Timeline</p><h2>Recent activity</h2></div></div><div className="rows">{data.activity.map(item => <div className="row activity-row" key={item.id}><span className={`activity-mark ${item.state}`} aria-hidden="true" /><div><strong>{item.title}</strong><small>{item.detail}</small></div><time dateTime={item.occurred_at}>{timeLabel(item.occurred_at)}</time></div>)}</div></section></div>
    <section className="actions-note"><div><strong>Quick actions</strong><p>Actions will appear when authenticated, capability-aware integrations are ready.</p></div><span>Unavailable</span></section>
  </>
}

type Page = { kind: 'overview' } | { kind: 'compute' } | { kind: 'network' } | { kind: 'automations' } | { kind: 'integrations' } | { kind: 'node'; id: string } | { kind: 'missing' }

export function pageFromPath(pathname: string): Page {
  if (pathname === '/') return { kind: 'overview' }
  if (pathname === '/compute') return { kind: 'compute' }
  if (pathname === '/network') return { kind: 'network' }
  if (pathname === '/automations') return { kind: 'automations' }
  if (pathname === '/integrations') return { kind: 'integrations' }
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
  const [network, setNetwork] = useState<Awaited<ReturnType<typeof loadNetwork>> | null>(null)
  const [node, setNode] = useState<Awaited<ReturnType<typeof loadNode>> | null>(null)
  const [automations, setAutomations] = useState<AutomationsData | null>(null)
  const [integrations, setIntegrations] = useState<Awaited<ReturnType<typeof loadIntegrations>> | null>(null)
  const [authSession, setAuthSession] = useState<AuthSession | null>(null)
  const [query, setQuery] = useState('')
  const searchRef = useRef<HTMLInputElement>(null)
  const reloadAutomations = useCallback(() => {
    void loadAutomations().then(result => { setAutomations(result); setPageError(null) }).catch(reason => setPageError(String(reason)))
  }, [])
  useEffect(() => {
    let alive = true
    const refresh = () => {
      if (page.kind === 'automations') reloadAutomations()
      void loadOverview().then(result => {
        if (alive) { setData(result); setError(null) }
        if (result.system.auth_required) void loadAuthSession().then(session => { if (alive) setAuthSession(session) }).catch(() => { if (alive) setAuthSession(null) })
      }).catch(reason => { if (alive) setError(String(reason)) })
      if (page.kind === 'compute') void loadCompute().then(result => { if (alive) { setCompute(result); setPageError(null) } }).catch(reason => { if (alive) setPageError(String(reason)) })
      if (page.kind === 'integrations') void loadIntegrations().then(result => { if (alive) { setIntegrations(result); setPageError(null) } }).catch(reason => { if (alive) setPageError(String(reason)) })
      if (page.kind === 'network') void loadNetwork().then(result => { if (alive) { setNetwork(result); setPageError(null) } }).catch(reason => { if (alive) setPageError(String(reason)) })
      if (page.kind === 'node') void loadNode(page.id).then(result => { if (alive) { setNode(result); setPageError(null) } }).catch(reason => { if (alive) setPageError(String(reason)) })
    }
    void refresh()
    const timer = window.setInterval(refresh, 30000)
    const events = new EventSource('/api/v1/events')
    events.addEventListener('node.updated', refresh)
    events.addEventListener('service.updated', refresh)
    events.addEventListener('workflow_run.updated', refresh)
    return () => { alive = false; window.clearInterval(timer); events.close() }
  }, [pathname, reloadAutomations])
  useEffect(() => {
    const keydown = (event: KeyboardEvent) => { if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') { event.preventDefault(); searchRef.current?.focus() } }
    window.addEventListener('keydown', keydown)
    return () => window.removeEventListener('keydown', keydown)
  }, [])
  const overall = useMemo(() => {
    if (!data) return 'Loading'
    if (!data.system.demo_mode) {
      if (data.nodes.some(node => node.health.source === 'host-agent' && node.freshness === 'stale')) return 'Host report stale'
      const count = data.nodes.filter(node => node.health.source === 'host-agent' && node.freshness === 'fresh').length
      return `${count} ${count === 1 ? 'host' : 'hosts'} reporting`
    }
    return data.nodes.some(node => node.freshness === 'stale' || node.health.state === 'degraded' || node.health.state === 'unhealthy') ? 'Needs attention' : 'All systems healthy'
  }, [data])
  const content = page.kind === 'overview'
    ? data ? <OverviewView data={data} query={query} /> : error ? <ErrorState message={error} /> : <div className="loading" role="status">Loading overview…</div>
    : page.kind === 'compute'
      ? compute ? <ComputeView data={compute} query={query} demoMode={data?.system.demo_mode ?? true} /> : pageError ? <ErrorState message={pageError} /> : <div className="loading" role="status">Loading compute…</div>
      : page.kind === 'network'
        ? network ? <NetworkView data={network} query={query} demoMode={data?.system.demo_mode} /> : pageError ? <ErrorState message={pageError} /> : <div className="loading" role="status">Loading network…</div>
      : page.kind === 'automations'
        ? automations && data ? <AutomationsView data={automations} system={data.system} query={query} actions={automationActions} onChanged={reloadAutomations} /> : pageError ? <ErrorState message={pageError} /> : <div className="loading" role="status">Loading automations…</div>
      : page.kind === 'integrations'
        ? integrations ? <IntegrationsView items={integrations} query={query} /> : pageError ? <ErrorState message={pageError} /> : <div className="loading" role="status">Loading integrations…</div>
      : page.kind === 'node'
        ? node ? <NodeDetailView node={node} /> : pageError ? <ErrorState message={pageError} /> : <div className="loading" role="status">Loading node…</div>
        : <ErrorState title="Page not found" message="This page does not exist." />
  const searchLabel = page.kind === 'compute' ? 'Search workers and jobs' : page.kind === 'network' ? 'Search network resources' : page.kind === 'automations' ? 'Search workflows' : page.kind === 'integrations' ? 'Search integrations' : 'Search nodes and services'
  const signOut = () => { if (authSession) void logout(authSession.csrf_token).then(() => { window.location.assign('/auth/signed-out') }).catch(reason => setError(String(reason))) }
  return <div className="app-shell"><aside className="sidebar"><div className="brand">oxnet<span>.work</span></div><nav aria-label="Primary"><a href="/" aria-current={page.kind === 'overview' ? 'page' : undefined} className={page.kind === 'overview' ? 'nav-active' : ''}><span aria-hidden="true">▦</span>Overview</a><a href="/compute" aria-current={page.kind === 'compute' ? 'page' : undefined} className={page.kind === 'compute' ? 'nav-active' : ''}><span aria-hidden="true">◫</span>Compute</a><a href="/network" aria-current={page.kind === 'network' ? 'page' : undefined} className={page.kind === 'network' ? 'nav-active' : ''}><span aria-hidden="true">⌁</span>Network</a><a href="/automations" aria-current={page.kind === 'automations' ? 'page' : undefined} className={page.kind === 'automations' ? 'nav-active' : ''}><span aria-hidden="true">▷</span>Automations</a><a href="/integrations" aria-current={page.kind === 'integrations' ? 'page' : undefined} className={page.kind === 'integrations' ? 'nav-active' : ''}><span aria-hidden="true">⚙</span>Integrations</a></nav><div className="sidebar-bottom"><span className="sidebar-dot" />{data ? data.system.demo_mode ? 'Demo workspace' : 'Live inventory' : 'Workspace'}</div></aside><div className="main-column"><header className="topbar"><div className="site-pill">⌂ <span>{data?.system.site_name ?? 'Homelab'}</span></div>{page.kind !== 'node' && <div className="search-wrap"><span aria-hidden="true">⌕</span><input ref={searchRef} type="search" value={query} onChange={event => setQuery(event.target.value)} placeholder={searchLabel} aria-label={searchLabel} /><kbd>⌘ K</kbd></div>}<span className="overall">{overall}</span>{authSession && <button type="button" className="signout" onClick={signOut} title={authSession.identity.email ?? authSession.identity.subject}>Sign out</button>}</header><main>{error && data && <div role="alert" className="error">Unable to refresh overview data: {error}. Showing the last available view.</div>}{pageError && (compute || network || node || automations || integrations) && <div role="alert" className="error">Unable to refresh this page: {pageError}. Showing the last available view.</div>}{content}</main></div></div>
}
