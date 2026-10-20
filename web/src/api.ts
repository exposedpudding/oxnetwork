import type { Activity, List, Node, Service, System } from './types'
import { createApiClient, type Result } from './lib/api/client'
import type { AutomationActions, AutomationsData } from './features/automations/AutomationsView'

// The session CSRF token is attached to every mutation. It is set once the session loads.
let csrfToken: string | undefined
const client = createApiClient({ csrfToken: () => csrfToken })

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
    get<List<Activity>>('/api/v1/recent-events')
  ])
  return { system, nodes: nodes.items, services: services.items, activity: activity.items }
}

export async function loadCompute() {
  const [summary, workers, jobs, providers] = await Promise.all([
    client.getComputeSummary(), client.listWorkers(), client.listJobs(), client.listComputeProviders()
  ])
  return { summary: unwrap(summary), workers: unwrap(workers).items, jobs: unwrap(jobs).items, providers: unwrap(providers).items }
}

export async function loadIntegrations() {
  return unwrap(await client.listIntegrations()).items
}

export async function loadNode(id: string) {
  const [node, metrics, containers] = await Promise.all([client.getNode(id), client.getNodeMetrics(id), client.getNodeContainers(id)])
  // Containers are optional: a failed read shows as unavailable instead of hiding the node.
  return { ...unwrap(node), metrics: unwrap(metrics), containers: containers.ok ? containers.data : null }
}

export async function loadNetwork() {
  const [graph, connections] = await Promise.all([client.getNetworkGraph(), client.listConnections()])
  return { graph: unwrap(graph), connections: unwrap(connections).items }
}

export type AuthSession = { identity: { subject: string; email?: string }; csrf_token: string }

export function loadAuthSession() {
  return get<AuthSession>('/auth/session').then(session => { csrfToken = session.csrf_token; return session })
}

export async function loadAutomations(): Promise<AutomationsData> {
  const [summary, workflows, options, runs] = await Promise.all([
    client.getAutomationSummary(), client.listWorkflows(), client.getWorkflowOptions(), client.listRecentRuns(8)
  ])
  return { summary: unwrap(summary), workflows: unwrap(workflows).items, options: unwrap(options), recentRuns: unwrap(runs).items }
}

/** Server calls used by the Automations page. */
export const automationActions: AutomationActions = {
  run: id => client.runWorkflow(id),
  setEnabled: (id, enabled) => enabled ? client.enableWorkflow(id) : client.disableWorkflow(id),
  create: input => client.createWorkflow(input),
  update: (id, input) => client.updateWorkflow(id, input),
  listRuns: id => client.listWorkflowRuns(id, 25),
  getRun: id => client.getWorkflowRun(id),
  listAudit: id => client.listAudit({ target_type: 'workflow', target_id: id, limit: 10 })
}

export async function logout(csrfToken: string) {
  const response = await fetch('/auth/logout', { method: 'POST', credentials: 'same-origin', headers: { 'X-CSRF-Token': csrfToken } })
  if (!response.ok) throw new Error(`Sign out returned ${response.status}`)
}
