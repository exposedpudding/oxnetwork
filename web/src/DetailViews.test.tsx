import { describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { ComputeView } from './DetailViews'
import type { ComputeProvider, ComputeSummary, Job, Worker } from './lib/api/types'

const health = { state: 'healthy' as const, observed_at: '2026-10-19T01:00:00Z', stale_after_seconds: 90, source: 'host-agent' }
const worker: Worker = { id: 'tailscale-1', node_id: 'tailscale-1', name: 'OX-C-01', state: 'online', vcpu_total: 8, vcpu_used: 0, health, freshness: 'fresh', simulated: false }
const summary: ComputeSummary = { total_vcpu: 8, used_vcpu: 0, reserved_vcpu: 0, active_jobs: 0, queued_jobs: 0, observed_at: '2026-10-19T01:00:00Z', freshness: 'fresh', simulated: true }
const provider: ComputeProvider = { id: 'ox-c-01', name: 'OX-C-01 OpenCode', kind: 'opencode', endpoint: 'http://100.69.88.98:4096', node_id: 'tailscale-1', state: 'online', version: '1.19.3', active_sessions: 1, total_sessions: 2, observed_at: '2026-10-19T01:00:00Z', stale_after_seconds: 45, freshness: 'fresh' }
const running: Job = { id: 'ox-c-01:a', name: 'Build the report', state: 'running', provider_id: 'ox-c-01', source: 'opencode', agent: 'build', model: 'ollama/qwen2.5:3b', tokens: 12345, updated_at: '2026-10-19T01:00:00Z', freshness: 'fresh' }
const idle: Job = { ...running, id: 'ox-c-01:b', name: 'Older session', state: 'idle', tokens: 7 }

const render = (data: Parameters<typeof ComputeView>[0]['data'], query = '') => renderToStaticMarkup(<ComputeView data={data} query={query} demoMode={false} />)

describe('ComputeView with an OpenCode provider', () => {
  it('shows servers, real job counts and sessions, and never claims a queue', () => {
    const html = render({ summary: { ...summary, active_jobs: 1, jobs_source: 'opencode' }, workers: [worker], jobs: [running, idle], providers: [provider] })
    expect(html).toContain('OpenCode connected')
    expect(html).toContain('1 of 1 OpenCode server has complete session readings')
    expect(html).toContain('OX-C-01 OpenCode')
    expect(html).toContain('http://100.69.88.98:4096')
    expect(html).toContain('1 running / 2 total')
    expect(html).toContain('Build the report')
    expect(html).toContain('ollama/qwen2.5:3b')
    expect(html).toContain('12,345')
    expect(html).toContain('Idle')
    expect(html).toContain('Running OpenCode sessions on reachable servers')
    expect(html).toContain('Not tracked')
    expect(html).toContain('OpenCode has no job queue')
    expect(html).not.toContain('No job provider connected')
  })

  it('shows an offline server with the reason, and stale sessions as stale', () => {
    const offline: ComputeProvider = { ...provider, state: 'offline', detail: 'connection refused', active_sessions: 0 }
    const html = render({ summary: { ...summary, jobs_source: 'opencode' }, workers: [worker], jobs: [{ ...running, freshness: 'stale' }], providers: [offline] })
    expect(html).toContain('Offline')
    expect(html).toContain('connection refused')
    expect(html).toContain('0 of 1 OpenCode server has complete session readings')
    expect(html).toContain('Stale')
  })

  it('says so when a connected server has no sessions, and filters by search', () => {
    expect(render({ summary: { ...summary, jobs_source: 'opencode' }, workers: [worker], jobs: [], providers: [provider] })).toContain('The connected OpenCode servers report no sessions.')
    const filtered = render({ summary: { ...summary, jobs_source: 'opencode' }, workers: [worker], jobs: [running], providers: [provider] }, 'zzz')
    expect(filtered).toContain('No sessions match your search.')
    expect(filtered).not.toContain('Build the report')
  })

  it('does not show a partial session inventory as a complete active-job count', () => {
    const partial: ComputeProvider = { ...provider, state: 'degraded', detail: 'project list unavailable', active_sessions: 0 }
    const html = render({ summary: { ...summary, active_jobs: 0, jobs_source: 'opencode' }, workers: [worker], jobs: [{ ...running, freshness: 'stale' }], providers: [partial] })
    expect(html).toContain('Incomplete')
    expect(html).toContain('project list unavailable')
    expect(html).toContain('Stale')
  })
})

describe('ComputeView without a provider', () => {
  it('keeps the honest "unavailable" wording', () => {
    const html = render({ summary, workers: [worker], jobs: [], providers: [] })
    expect(html).toContain('Compute not connected')
    expect(html).toContain('No job provider connected')
    expect(html).toContain('No jobs reported')
    expect(html).not.toContain('OpenCode connected')
    expect(html).not.toContain('Compute providers')
  })
})
