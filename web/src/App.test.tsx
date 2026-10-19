import { describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { OverviewView } from './App'
import type { Node, Service } from './types'

const health = { state: 'healthy' as const, observed_at: '2026-10-17T12:00:00Z', stale_after_seconds: 60, source: 'demo' }

describe('OverviewView', () => {
  it('marks simulated and stale data and does not invent missing metrics', () => {
    const node: Node = { id:'sample', name:'Sample node', role:'Compute', platform:'Linux', health, freshness:'stale', capabilities:[], metrics:{}, simulated:true }
    const service: Service = { id:'sample-service', name:'Sample service', health, freshness:'fresh', simulated:true }
    const html = renderToStaticMarkup(<OverviewView data={{ system:{name:'oxnet.work',site_name:'Test',server_time:'2026-10-17T12:00:00Z',demo_mode:true},nodes:[node],services:[service],activity:[] }} query="" />)
    expect(html).toContain('No live infrastructure is connected')
    expect(html).toContain('Sample node')
    expect(html).toContain('Stale')
    expect(html).toContain('Not reported')
    expect(html).toContain('Actions will appear when authenticated')
  })
  it('labels Tailscale data without claiming health or simulated services', () => {
    const node: Node = { id: 'tailscale-1', name: 'gateway', role: 'Tailnet device', platform: 'linux', health: { ...health, state: 'unknown', source: 'tailscale' }, freshness: 'fresh', capabilities: [], metrics: {}, addresses: ['100.64.0.10'], simulated: false }
    const html = renderToStaticMarkup(<OverviewView data={{ system: { name: 'oxnet.work', site_name: 'Test', server_time: '2026-10-17T12:00:00Z', demo_mode: false }, nodes: [node], services: [], activity: [] }} query="" />)
    expect(html).toContain('Live inventory')
    expect(html).toContain('Other devices and service health remain unverified')
    expect(html).not.toContain('All inventory and activity shown here is simulated')
    expect(html).toContain('Not reported')
  })
  it('shows probed services with their endpoint and result, and stops claiming all service health is unverified', () => {
    const system = { name: 'oxnet.work', site_name: 'Test', server_time: '2026-10-17T12:00:00Z', demo_mode: false }
    const probed: Service[] = [
      { id: 'probe-edge', name: 'oxnet.work edge', health: { ...health, source: 'http-probe' }, freshness: 'fresh', simulated: false, endpoint: 'https://oxnet.work/readyz', detail: 'HTTP 200 in 12 ms' },
      { id: 'probe-auth', name: 'Authentik', health: { ...health, state: 'unhealthy', source: 'http-probe' }, freshness: 'fresh', simulated: false, endpoint: 'https://auth.oxnet.work/-/health/live/', detail: 'no response within 3s' },
    ]
    const html = renderToStaticMarkup(<OverviewView data={{ system, nodes: [], services: probed, activity: [] }} query="" />)
    expect(html).toContain('HTTP 200 in 12 ms · https://oxnet.work/readyz')
    expect(html).toContain('no response within 3s · https://auth.oxnet.work/-/health/live/')
    expect(html).toContain('2 service probes check their configured endpoints; other devices and services remain unverified.')
    expect(html).not.toContain('service health remain unverified')
    expect(html).toContain('Unhealthy')
    expect(html).toContain('1 healthy')
    const single = renderToStaticMarkup(<OverviewView data={{ system, nodes: [], services: [probed[0]], activity: [] }} query="" />)
    expect(single).toContain('1 service probe checks its configured endpoint;')
    // A result that is no longer refreshed shows as stale, never as healthy.
    const stale = renderToStaticMarkup(<OverviewView data={{ system, nodes: [], services: [{ ...probed[0], freshness: 'stale' }], activity: [] }} query="" />)
    expect(stale).toContain('Stale')
    expect(stale).toContain('0 healthy')
  })
  it('shows host agent readings only for the reporting node', () => {
    const node: Node = { id: 'tailscale-1', name: 'home-gw', role: 'Tailnet device', platform: 'linux', health: { ...health, source: 'host-agent' }, freshness: 'fresh', capabilities: ['monitor'], metrics: { cpu_percent: 18, ram_percent: 42, disk_percent: 37, vcpu: 4, ram_gb: 16 }, simulated: false }
    const html = renderToStaticMarkup(<OverviewView data={{ system: { name: 'oxnet.work', site_name: 'Test', server_time: '2026-10-17T12:00:00Z', demo_mode: false }, nodes: [node], services: [], activity: [] }} query="" />)
    expect(html).toContain('1 host reports CPU, RAM and disk')
    expect(html).toContain('Disk')
    expect(html).toContain('37%')
    expect(html).toContain('Reporting')
    expect(html).toContain('host-agent')
  })
})
