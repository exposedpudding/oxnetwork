import type { Activity, List, Node, Service, System } from './types'

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
