import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { IntegrationsView } from './IntegrationsView'
import type { Integration } from './lib/api/types'

const items: Integration[] = [
  { id: 'inventory', name: 'Tailscale device inventory', kind: 'inventory', state: 'healthy', detail: '4 devices; last refresh 25 seconds ago.', observed_at: '2026-10-19T01:00:00Z', freshness: 'fresh' },
  { id: 'service-probes', name: 'Service health probes', kind: 'probes', state: 'degraded', detail: '1 of 2 services answering; failing: auth.', observed_at: '2026-10-19T01:00:00Z', freshness: 'fresh' },
  { id: 'opencode', name: 'OpenCode compute servers', kind: 'compute', state: 'not_configured', detail: 'No OpenCode servers are configured.', observed_at: null, freshness: 'unknown' }
]

describe('IntegrationsView', () => {
  it('shows each source with its state and detail', () => {
    render(<IntegrationsView items={items} query="" />)
    expect(screen.getByRole('heading', { name: 'Integrations' })).toBeInTheDocument()
    expect(screen.getByText('1 need attention')).toBeInTheDocument()
    expect(screen.getByText('1 of 2 services answering; failing: auth.')).toBeInTheDocument()
    expect(screen.getByText('Not configured')).toBeInTheDocument()
    expect(screen.getByText('Degraded')).toBeInTheDocument()
  })

  it('filters by search text and says when nothing matches', () => {
    render(<IntegrationsView items={items} query="zzz" />)
    expect(screen.getByText('No integration matches your search.')).toBeInTheDocument()
  })
})
