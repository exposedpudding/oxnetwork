import { useState } from 'react'
import type { loadNetwork } from './api'
import { Card } from './components/Card'
import { DataTable } from './components/DataTable'
import { EmptyState } from './components/States'
import { StatusChip } from './components/Status'
import { TopologyView } from './features/topology/TopologyView'
import type { GraphEdge } from './lib/api/types'

type NetworkData = Awaited<ReturnType<typeof loadNetwork>>

const kinds: Record<string, string> = { edge: 'Edge', compute: 'Compute', device: 'Device', network: 'Network', service: 'Service' }

function timeLabel(value: string) {
  return new Date(value).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' })
}

export function NetworkView({ data, query, demoMode }: { data: NetworkData; query: string; demoMode?: boolean }) {
  const [selectedId, setSelectedId] = useState<string | undefined>()
  const lower = query.trim().toLowerCase()
  const nodes = data.graph.nodes.filter(node => `${node.label} ${node.kind}`.toLowerCase().includes(lower))
  const visibleIds = new Set(nodes.map(node => node.id))
  const graph = { nodes, edges: data.graph.edges.filter(edge => visibleIds.has(edge.source) && visibleIds.has(edge.target)) }
  const selected = nodes.find(node => node.id === selectedId) ?? nodes[0]
  const labels = new Map(data.graph.nodes.map(node => [node.id, node.label]))
  const connections = data.connections.filter(edge => `${labels.get(edge.source) ?? edge.source} ${labels.get(edge.target) ?? edge.target} ${edge.kind}`.toLowerCase().includes(lower))
  return <>
    <div className="page-heading"><div><p className="eyebrow">Command centre / Network</p><h1>Network</h1><p>Recorded nodes, services and their relationships.</p></div><span className="timestamp">{nodes.length} nodes · {graph.edges.length} visible connections</span></div>
    {demoMode === true && <div className="demo-banner" role="status"><strong>Demo mode</strong><span>Every displayed relationship is simulated. This map does not describe the live network.</span></div>}
    {demoMode === false && <div className="demo-banner" role="status"><strong>Tailscale inventory</strong><span>Devices are listed without inferred network links. Tailscale does not report their health here.</span></div>}
    <div className="network-layout">
      <section aria-label="Topology"><TopologyView graph={graph} selectedId={selected?.id} onSelect={setSelectedId} kindLabels={kinds} /></section>
      <Card title="Selected resource">
        {selected ? <div className="network-detail">
          <h3>{selected.label}</h3>
          <p>{kinds[selected.kind] ?? selected.kind}</p>
          <StatusChip status={selected.freshness === 'stale' ? 'stale' : selected.health?.state ?? 'unknown'} />
          <dl><dt>Source</dt><dd>{selected.health?.source ?? 'Not reported'}</dd><dt>Last observed</dt><dd>{selected.health?.observed_at ? timeLabel(selected.health.observed_at) : 'Not reported'}</dd><dt>Data</dt><dd>{selected.simulated ? 'Simulated' : 'Observed'}</dd></dl>
          {selected.id.startsWith('node:') && <a className="inline-link" href={`/nodes/${encodeURIComponent(selected.resource_id ?? selected.id.slice(5))}`}>Open node details →</a>}
        </div> : <EmptyState title="No resource selected" description="Try a different search to see more resources." />}
      </Card>
    </div>
    <Card title="Connections" className="compute-section">
      <DataTable<GraphEdge> caption="Network connections" rows={connections} rowKey={edge => edge.id} emptyTitle="No connections found" emptyDescription={lower ? 'No recorded connections match your search.' : 'No relationships have been observed.'} columns={[
        { key: 'source', header: 'From', render: edge => labels.get(edge.source) ?? edge.source },
        { key: 'target', header: 'To', render: edge => labels.get(edge.target) ?? edge.target },
        { key: 'kind', header: 'Relationship', render: edge => edge.kind },
        { key: 'freshness', header: 'Freshness', render: edge => <StatusChip status={edge.freshness === 'fresh' ? 'healthy' : edge.freshness ?? 'unknown'}>{edge.freshness ?? 'unknown'}</StatusChip> },
        { key: 'observed', header: 'Last observed', render: edge => edge.observed_at ? <time dateTime={edge.observed_at}>{timeLabel(edge.observed_at)}</time> : 'Not reported' }
      ]} />
    </Card>
    <p className="muted-copy">Only relationships supplied by a provider are drawn. An unconnected item may have links that have not been reported.</p>
  </>
}
