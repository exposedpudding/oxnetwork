import { Card, StatCard } from './components/Card'
import { DataTable } from './components/DataTable'
import { MetricBar } from './components/MetricBar'
import { EmptyState } from './components/States'
import { StatusChip } from './components/Status'
import type { loadCompute, loadNode } from './api'
import type { ComputeProvider, Container, Job, NodeContainers, Worker } from './lib/api/types'

type ComputeData = Awaited<ReturnType<typeof loadCompute>>
type NodeData = Awaited<ReturnType<typeof loadNode>>

function timeLabel(value: string) {
  return new Date(value).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' })
}

export function ComputeView({ data, query, demoMode }: { data: ComputeData; query: string; demoMode: boolean }) {
  const lower = query.trim().toLowerCase()
  const workers = data.workers.filter(worker => worker.name.toLowerCase().includes(lower))
  const jobs = data.jobs.filter(job => job.name.toLowerCase().includes(lower))
  const providers = data.providers ?? []
  const hasProvider = providers.length > 0
  const providersOnline = providers.filter(provider => provider.state === 'online' && provider.freshness === 'fresh').length
  const providersIncomplete = providers.some(provider => provider.state === 'degraded')
  return <>
    <div className="page-heading"><div><p className="eyebrow">Command centre / Compute</p><h1>Compute</h1><p>Reported worker capacity and job activity.</p></div><span className="timestamp">{data.workers.length ? `Observed ${timeLabel(data.summary.observed_at)}` : 'No compute reports'}</span></div>
    {demoMode ? <div className="demo-banner" role="status"><strong>Demo mode</strong><span>Capacity is simulated. No job provider or scheduler is connected.</span></div> : hasProvider ? <div className="demo-banner" role="status"><strong>OpenCode connected</strong><span>{providersOnline} of {providers.length} OpenCode {providers.length === 1 ? 'server has' : 'servers have'} complete session readings. Host capacity comes from the host agents; vCPU use per session is not measured. Only session metadata is read; titles can contain user text, while messages and files are not read.</span></div> : <div className="demo-banner" role="status"><strong>Compute not connected</strong><span>Tailscale reports device identity only. CPU, RAM, worker capacity and jobs are not available from this source.</span></div>}
    {data.workers.length > 0 && data.summary.freshness !== 'fresh' && <div className="compute-notice" role="status">Worker capacity is {data.summary.freshness}; it may no longer be available.</div>}
    <section aria-label="Compute summary" className="ox-grid compute-summary">
      <StatCard label="Reported capacity" value={data.workers.length ? `${data.summary.total_vcpu} vCPU` : 'Not reported'} detail={data.workers.length ? data.summary.freshness === 'fresh' ? 'Current worker reports' : 'Includes stale worker reports' : 'No compute provider connected'} />
      <StatCard label="Workers" value={data.workers.length} detail={`${data.workers.filter(worker => worker.freshness === 'fresh' && worker.state === 'online').length} online and fresh`} />
      <StatCard label="Active jobs" value={hasProvider && data.summary.jobs_source ? providersIncomplete ? 'Incomplete' : data.summary.active_jobs : 'Unavailable'} detail={hasProvider ? providersIncomplete ? 'A server could not return a complete session list' : 'Running OpenCode sessions on reachable servers' : 'No job provider connected'} />
      <StatCard label="Queued jobs" value={hasProvider ? 'Not tracked' : 'Unavailable'} detail={hasProvider ? 'OpenCode has no job queue' : 'No job provider connected'} />
    </section>
    {hasProvider && <Card title="Compute providers" className="compute-section">
      <DataTable<ComputeProvider> caption="Compute providers" rows={providers} rowKey={provider => provider.id} columns={[
        { key: 'name', header: 'Server', render: provider => <>{provider.name}<small className="auto-sub">{provider.endpoint}</small></> },
        { key: 'state', header: 'State', render: provider => <><StatusChip status={provider.freshness === 'stale' ? 'stale' : provider.state}>{provider.freshness === 'stale' ? 'Stale' : provider.state === 'online' ? 'Online' : provider.state === 'degraded' ? 'Partial' : 'Offline'}</StatusChip>{provider.detail && <small className="auto-sub">{provider.detail}</small>}</> },
        { key: 'version', header: 'Version', render: provider => provider.version ?? '—' },
        { key: 'sessions', header: 'Sessions', render: provider => provider.state === 'degraded' ? 'Incomplete' : `${provider.active_sessions} running / ${provider.total_sessions} total`, align: 'right' },
        { key: 'observed', header: 'Last observed', render: provider => <time dateTime={provider.observed_at}>{timeLabel(provider.observed_at)}</time> }
      ]} />
    </Card>}
    <Card title="Workers" className="compute-section">
      <DataTable<Worker> caption="Compute workers" rows={workers} rowKey={worker => worker.id} emptyTitle="No workers found" emptyDescription={lower ? 'No workers match your search.' : 'No compute-capable nodes have reported capacity.'} columns={[
        { key: 'name', header: 'Worker', render: worker => <a className="inline-link" href={`/nodes/${encodeURIComponent(worker.node_id)}`}>{worker.name}</a> },
        { key: 'state', header: 'State', render: worker => <StatusChip status={worker.freshness === 'stale' ? 'stale' : worker.state} /> },
        { key: 'capacity', header: 'Reported capacity', render: worker => `${worker.vcpu_total} vCPU` },
        { key: 'observed', header: 'Last observed', render: worker => <time dateTime={worker.health.observed_at}>{timeLabel(worker.health.observed_at)}</time> }
      ]} />
    </Card>
    <Card title="Jobs" className="compute-section">
      {jobs.length ? <DataTable<Job> caption="OpenCode sessions" rows={jobs} rowKey={job => job.id} columns={[
        { key: 'name', header: 'Session', render: job => job.name || 'Untitled session' },
        { key: 'state', header: 'State', render: job => job.freshness === 'stale' ? <StatusChip status="stale" /> : job.state === 'running' ? <StatusChip status="running" /> : <StatusChip status="unknown">Idle</StatusChip> },
        { key: 'agent', header: 'Agent', render: job => job.agent ?? '—' },
        { key: 'model', header: 'Model', render: job => job.model ?? '—' },
        { key: 'tokens', header: 'Tokens', render: job => job.tokens == null ? '—' : job.tokens.toLocaleString(), align: 'right' },
        { key: 'updated', header: 'Last activity', render: job => job.updated_at ? <time dateTime={job.updated_at}>{timeLabel(job.updated_at)}</time> : '—' }
      ]} /> : hasProvider ? <EmptyState title="No sessions" description={lower ? 'No sessions match your search.' : 'The connected OpenCode servers report no sessions.'} /> : <EmptyState title="No jobs reported" description="No job provider is connected. Job counts and vCPU usage are unavailable." />}
    </Card>
    <Card title="Placement" className="compute-section"><p className="muted-copy">Placement rules can assess worker freshness, capabilities and capacity. Job dispatch and reservations are unavailable until a provider and authenticated actions are ready.</p></Card>
  </>
}

function containerStatus(container: Container, stale: boolean) {
  if (stale) return 'stale' as const
  if (container.health === 'unhealthy') return 'unhealthy' as const
  if (container.state === 'running') return container.health === 'healthy' ? 'healthy' as const : 'running' as const
  if (container.state === 'restarting') return 'degraded' as const
  if (container.state === 'paused') return 'unknown' as const
  return 'offline' as const
}

export function ContainersCard({ containers, simulated }: { containers: NodeContainers | null | undefined; simulated: boolean }) {
  if (simulated) return <Card title="Containers" className="compute-section"><p className="muted-copy">Demo nodes do not report containers.</p></Card>
  if (!containers) return <Card title="Containers" className="compute-section"><EmptyState title="Containers unavailable" description="The container list could not be read just now." /></Card>
  if (!containers.reported) return <Card title="Containers" className="compute-section"><EmptyState title="No container report" description="This host has not sent a container list. Install the optional Docker reporter to see containers here." /></Card>
  const stale = containers.freshness === 'stale'
  return <Card title="Containers" className="compute-section">
    {stale && <div className="compute-notice" role="status">The last container report is out of date{containers.reported_at ? ` (${timeLabel(containers.reported_at)})` : ''}; these containers may no longer be running.</div>}
    {containers.items.length ? <DataTable<Container> caption="Containers" rows={containers.items} rowKey={container => container.id} columns={[
      { key: 'name', header: 'Container', render: container => <>{container.name}{container.compose_project && <small className="auto-sub">{container.compose_project}{container.compose_service ? ` / ${container.compose_service}` : ''}</small>}</> },
      { key: 'image', header: 'Image', render: container => container.image },
      { key: 'state', header: 'State', render: container => <><StatusChip status={containerStatus(container, stale)}>{stale ? 'Stale' : container.health === 'unhealthy' ? 'Unhealthy' : container.state === 'running' && container.health === 'healthy' ? 'Healthy' : container.state[0].toUpperCase() + container.state.slice(1)}</StatusChip><small className="auto-sub">{container.status}</small></> },
      { key: 'ports', header: 'Published ports', render: container => container.ports.length ? container.ports.join(', ') : '—' },
      { key: 'created', header: 'Created', render: container => <time dateTime={container.created_at}>{timeLabel(container.created_at)}</time> }
    ]} /> : <EmptyState title="No containers" description="The reporter sees no containers on this host." />}
    <p className="muted-copy">Only name, image, state and published ports are read. Environment variables, mounts and commands are never collected, and no container action is available.</p>
  </Card>
}

export function NodeDetailView({ node }: { node: NodeData }) {
  const freshness = node.freshness ?? 'unknown'
  return <>
    <div className="page-heading"><div><p className="eyebrow"><a className="inline-link" href="/">Command centre / Overview</a> / Node</p><h1>{node.name}</h1><p>{node.role ?? 'Role not reported'} · {node.platform ?? 'Platform not reported'}</p></div><StatusChip status={freshness === 'stale' ? 'stale' : node.health.state}>{freshness === 'stale' ? 'Stale' : node.health.source === 'host-agent' ? 'Reporting' : node.health.state}</StatusChip></div>
    {node.simulated && <div className="demo-banner" role="status"><strong>Demo mode</strong><span>This node and its observations are simulated. No live agent is connected.</span></div>}
    <section aria-label="Node summary" className="ox-grid compute-summary">
      <StatCard label={node.health.source === 'host-agent' ? 'Agent report' : 'Health'} value={freshness === 'stale' ? 'Stale' : node.health.source === 'host-agent' ? 'Reporting' : node.health.state} detail={`Observed ${timeLabel(node.health.observed_at)}`} />
      <StatCard label="vCPU" value={node.metrics.vcpu ?? 'Not reported'} detail="Reported capacity" />
      <StatCard label="RAM" value={node.metrics.ram_gb == null ? 'Not reported' : `${node.metrics.ram_gb} GB`} detail="Reported capacity" />
      <StatCard label="Source" value={node.health.source ?? 'Not reported'} detail={node.simulated ? 'Simulated' : 'Observed'} />
    </section>
    <div className="detail-grid node-detail-grid">
      <Card title="Resource use"><div className="metric-stack"><MetricBar label="CPU" value={node.metrics.cpu_percent} /><MetricBar label="RAM" value={node.metrics.ram_percent} /><MetricBar label="Root disk" value={node.metrics.disk_percent} /></div><p className="muted-copy">Temperature and latency are not reported by this source. Host agent health means recent reporting, not that every service is working.</p></Card>
      <Card title="Capabilities">{node.capabilities.length ? <ul className="capability-list">{node.capabilities.map(capability => <li key={capability}>{capability}</li>)}</ul> : <EmptyState title="No capabilities reported" />}</Card>
    </div>
    {!node.simulated && <Card title="Tailscale identity" className="compute-section"><p className="muted-copy">Addresses: {node.addresses?.length ? node.addresses.join(', ') : 'Not reported'} · Last seen: {node.last_seen ? timeLabel(node.last_seen) : 'Not reported'}. Last seen is a Tailscale control-plane value, not a health check.</p></Card>}
    <ContainersCard containers={node.containers} simulated={Boolean(node.simulated)} />
    <Card title="More from this node" className="compute-section"><p className="muted-copy">Node-specific jobs and logs require read-only integrations. Recorded relationships are on the <a className="inline-link" href="/network">Network page</a>. Node actions are unavailable.</p></Card>
  </>
}
