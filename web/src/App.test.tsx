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
