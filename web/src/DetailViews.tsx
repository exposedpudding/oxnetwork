import { Card, StatCard } from './components/Card'
import { DataTable } from './components/DataTable'
import { MetricBar } from './components/MetricBar'
import { EmptyState } from './components/States'
import { StatusChip } from './components/Status'
import type { loadCompute, loadNode } from './api'
import type { Worker } from './lib/api/types'

type ComputeData = Awaited<ReturnType<typeof loadCompute>>
type NodeData = Awaited<ReturnType<typeof loadNode>>

function timeLabel(value: string) {
  return new Date(value).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' })
}

export function ComputeView({ data, query, demoMode }: { data: ComputeData; query: string; demoMode: boolean }) {
  const lower = query.trim().toLowerCase()
  const workers = data.workers.filter(worker => worker.name.toLowerCase().includes(lower))
  const jobs = data.jobs.filter(job => job.name.toLowerCase().includes(lower))
  return <>
    <div className="page-heading"><div><p className="eyebrow">Command centre / Compute</p><h1>Compute</h1><p>Reported worker capacity and job activity.</p></div><span className="timestamp">{data.workers.length ? `Observed ${timeLabel(data.summary.observed_at)}` : 'No compute reports'}</span></div>
    {demoMode ? <div className="demo-banner" role="status"><strong>Demo mode</strong><span>Capacity is simulated. No job provider or scheduler is connected.</span></div> : <div className="demo-banner" role="status"><strong>Compute not connected</strong><span>Tailscale reports device identity only. CPU, RAM, worker capacity and jobs are not available from this source.</span></div>}
    {data.workers.length > 0 && data.summary.freshness !== 'fresh' && <div className="compute-notice" role="status">Worker capacity is {data.summary.freshness}; it may no longer be available.</div>}
    <section aria-label="Compute summary" className="ox-grid compute-summary">
      <StatCard label="Reported capacity" value={data.workers.length ? `${data.summary.total_vcpu} vCPU` : 'Not reported'} detail={data.workers.length ? data.summary.freshness === 'fresh' ? 'Current worker reports' : 'Includes stale worker reports' : 'No compute provider connected'} />
      <StatCard label="Workers" value={data.workers.length} detail={`${data.workers.filter(worker => worker.freshness === 'fresh' && worker.state === 'online').length} online and fresh`} />
      <StatCard label="Active jobs" value="Unavailable" detail="No job provider connected" />
      <StatCard label="Queued jobs" value="Unavailable" detail="No job provider connected" />
    </section>
    <Card title="Workers" className="compute-section">
      <DataTable<Worker> caption="Compute workers" rows={workers} rowKey={worker => worker.id} emptyTitle="No workers found" emptyDescription={lower ? 'No workers match your search.' : 'No compute-capable nodes have reported capacity.'} columns={[
        { key: 'name', header: 'Worker', render: worker => <a className="inline-link" href={`/nodes/${encodeURIComponent(worker.node_id)}`}>{worker.name}</a> },
        { key: 'state', header: 'State', render: worker => <StatusChip status={worker.freshness === 'stale' ? 'stale' : worker.state} /> },
        { key: 'capacity', header: 'Reported capacity', render: worker => `${worker.vcpu_total} vCPU` },
        { key: 'observed', header: 'Last observed', render: worker => <time dateTime={worker.health.observed_at}>{timeLabel(worker.health.observed_at)}</time> }
      ]} />
    </Card>
    <Card title="Jobs" className="compute-section">
      {jobs.length ? <ul className="simple-list">{jobs.map(job => <li key={job.id}>{job.name} · {job.state}</li>)}</ul> : <EmptyState title="No jobs reported" description="No job provider is connected. Job counts and vCPU usage are unavailable." />}
    </Card>
    <Card title="Placement" className="compute-section"><p className="muted-copy">Placement rules can assess worker freshness, capabilities and capacity. Job dispatch and reservations are unavailable until a provider and authenticated actions are ready.</p></Card>
  </>
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
    <Card title="More from this node" className="compute-section"><p className="muted-copy">Node-specific containers, jobs and logs require read-only integrations. Recorded relationships are on the <a className="inline-link" href="/network">Network page</a>. Node actions are unavailable.</p></Card>
  </>
}
