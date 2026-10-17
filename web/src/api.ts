import type { Activity, List, Node, Service, System } from './types'
import { createApiClient, type Result } from './lib/api/client'

const client = createApiClient()

function unwrap<T>(result: Result<T>): T {
  if (!result.ok) throw new Error(result.error.message)
  return result.data
}

async function get<T>(path: string): Promise<T> {
  const response = await fetch(path, { headers: { Accept: 'application/json' } })
  if (!response.ok) throw new Error(`${path} returned ${response.status}`)
  return response.json() as Promise<T>
}

export async function loadOverview() {
  const [system, nodes, services, activity] = await Promise.all([
    get<System>('/api/v1/system'),
    get<List<Node>>('/api/v1/nodes'),
    get<List<Service>>('/api/v1/services'),
    get<List<Activity>>('/api/v1/activity')
  ])
  return { system, nodes: nodes.items, services: services.items, activity: activity.items }
}

export async function loadCompute() {
  const [summary, workers, jobs] = await Promise.all([
    client.getComputeSummary(), client.listWorkers(), client.listJobs()
  ])
  return { summary: unwrap(summary), workers: unwrap(workers).items, jobs: unwrap(jobs).items }
}

export async function loadNode(id: string) {
  const [node, metrics] = await Promise.all([client.getNode(id), client.getNodeMetrics(id)])
  return { ...unwrap(node), metrics: unwrap(metrics) }
}
