import { DataTable } from './components/DataTable'
import { Card } from './components/Card'
import { StatusChip } from './components/Status'
import type { Integration } from './lib/api/types'

const STATE_LABEL: Record<Integration['state'], string> = {
  healthy: 'Healthy',
  degraded: 'Degraded',
  unhealthy: 'Unhealthy',
  unknown: 'Unknown',
  not_configured: 'Not configured'
}

function timeLabel(value: string) {
  return new Date(value).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' })
}

export function IntegrationsView({ items, query }: { items: Integration[]; query: string }) {
  const lower = query.trim().toLowerCase()
  const rows = items.filter(item => `${item.name} ${item.detail}`.toLowerCase().includes(lower))
  const attention = items.filter(item => item.state === 'degraded' || item.state === 'unhealthy').length
  return <>
    <div className="page-heading"><div><p className="eyebrow">Command centre / Integrations</p><h1>Integrations</h1><p>Whether each data source is connected, current and answering.</p></div><span className="timestamp">{attention ? `${attention} need attention` : 'No sources need attention'}</span></div>
    <Card title="Sources" className="compute-section">
      <DataTable<Integration> caption="Integration health" rows={rows} rowKey={item => item.id} emptyTitle="No integrations found" emptyDescription="No integration matches your search." columns={[
        { key: 'name', header: 'Source', render: item => item.name },
        { key: 'state', header: 'State', render: item => <StatusChip status={item.state === 'not_configured' ? 'disabled' : item.freshness === 'stale' && item.state === 'healthy' ? 'stale' : item.state}>{STATE_LABEL[item.state]}</StatusChip> },
        { key: 'detail', header: 'Detail', render: item => item.detail },
        { key: 'observed', header: 'Last observed', render: item => item.observed_at ? <time dateTime={item.observed_at}>{timeLabel(item.observed_at)}</time> : '—' }
      ]} />
    </Card>
  </>
}
