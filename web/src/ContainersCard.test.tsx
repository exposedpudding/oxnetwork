import { describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { ContainersCard } from './DetailViews'
import type { Container, NodeContainers } from './lib/api/types'

const web: Container = { id: '0123456789ab', name: 'oxnet-work', image: 'oxnet/work:local', state: 'running', status: 'Up 3 hours (healthy)', health: 'healthy', created_at: '2026-10-19T08:00:00Z', ports: ['127.0.0.1:8080->8080/tcp'], compose_project: 'oxnet', compose_service: 'web' }
const broken: Container = { ...web, id: 'ba9876543210', name: 'auth-worker', health: 'unhealthy', status: 'Up 1 minute (unhealthy)', ports: [], compose_project: undefined, compose_service: undefined }
const stopped: Container = { ...web, id: 'aaaaaaaaaaaa', name: 'old-job', state: 'exited', health: '', status: 'Exited (0) 2 days ago', ports: [] }
const reported = (items: Container[], freshness: NodeContainers['freshness'] = 'fresh'): NodeContainers => ({ reported: true, reported_at: '2026-10-19T12:00:00Z', freshness, items })
const render = (containers: NodeContainers | null | undefined, simulated = false) => renderToStaticMarkup(<ContainersCard containers={containers} simulated={simulated} />)

describe('ContainersCard', () => {
  it('lists containers with state, health, ports and Compose names', () => {
    const html = render(reported([web, broken, stopped]))
    expect(html).toContain('oxnet-work')
    expect(html).toContain('oxnet / web')
    expect(html).toContain('127.0.0.1:8080-&gt;8080/tcp')
    expect(html).toContain('Healthy')
    expect(html).toContain('Unhealthy')
    expect(html).toContain('Exited')
    expect(html).toContain('Exited (0) 2 days ago')
    expect(html).toContain('never collected')
  })

  it('marks a stale report instead of showing running containers as current', () => {
    const html = render(reported([web], 'stale'))
    expect(html).toContain('out of date')
    expect(html).toContain('Stale')
    expect(html).not.toContain('>Healthy<')
  })

  it('distinguishes never reported, empty, unreadable and simulated nodes', () => {
    expect(render({ reported: false, reported_at: null, freshness: 'unknown', items: [] })).toContain('No container report')
    expect(render(reported([]))).toContain('The reporter sees no containers on this host.')
    expect(render(null)).toContain('Containers unavailable')
    expect(render(undefined)).toContain('Containers unavailable')
    expect(render(reported([web]), true)).toContain('Demo nodes do not report containers.')
  })
})
