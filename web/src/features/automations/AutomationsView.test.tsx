import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AutomationsView, describeError, duration, relativeFuture, toInput, type AutomationActions, type AutomationsData } from './AutomationsView'
import type { ApiError, AutomationOptions, WorkflowRecord, WorkflowRun } from '../../lib/api/types'
import type { Workflow } from '../workflows/types'

const options: AutomationOptions = {
  action_types: [{ value: 'check_node_health', label: 'Check node health' }],
  targets: [{ id: 'n1', label: 'Node One' }],
  channels: [{ value: 'activity', label: 'Activity log' }],
  planned_actions: [{ value: 'wake_node', label: 'Wake node', milestone: 'Milestone 9' }],
}

function record(over: Partial<WorkflowRecord> = {}): WorkflowRecord {
  return {
    id: 'wf_1', name: 'Health sweep', description: 'Checks the nodes', enabled: true, simulated: true,
    trigger: { type: 'cron', expression: '*/15 * * * *' },
    steps: [{ id: 's1', type: 'action', name: 'Check node one', action_type: 'check_node_health', target_id: 'n1' }],
    on_success: { notify_channel: 'activity' }, on_failure: { behavior: 'continue' },
    created_at: '2026-10-18T10:00:00Z', updated_at: '2026-10-18T10:00:00Z', next_run_at: '2026-10-18T12:15:00Z',
    last_run: { id: 'run_1', state: 'failed', trigger: 'cron', started_at: '2026-10-18T12:00:00Z', finished_at: '2026-10-18T12:00:02Z', duration_ms: 2000 },
    ...over,
  }
}

const run: WorkflowRun = {
  id: 'run_1', workflow_id: 'wf_1', workflow_name: 'Health sweep', trigger: 'cron', actor: 'scheduler', state: 'failed',
  started_at: '2026-10-18T12:00:00Z', finished_at: '2026-10-18T12:00:02Z', duration_ms: 2000, error_code: 'CHECK_FAILED',
  summary: 'Step 1 (Check node one) failed: Node One: degraded, stale.', simulated: true,
  steps: [{ position: 0, step_id: 's1', kind: 'action', name: 'Check node one', state: 'failed', started_at: null, finished_at: null, output: 'Node One: degraded, stale.', error_code: 'CHECK_FAILED' }],
}

function data(over: Partial<AutomationsData> = {}): AutomationsData {
  return {
    summary: { workflows: 2, enabled: 1, disabled: 1, scheduled_count: 1, next_scheduled_at: '2026-10-18T12:15:00Z', runs_last_7_days: 4, succeeded_last_7_days: 3, success_rate: 75, failed_last_24h: 1, active_runs: 0, generated_at: '2026-10-18T12:07:00Z', simulated: true },
    workflows: [record(), record({ id: 'wf_2', name: 'Stale alert', description: undefined, enabled: false, trigger: { type: 'manual' }, last_run: null, next_run_at: null })],
    options, recentRuns: [run], ...over,
  }
}

function makeActions(): AutomationActions {
  const ok = <T,>(value: T) => Promise.resolve({ ok: true as const, data: value, status: 200 })
  return {
    run: vi.fn(() => ok(run)),
    setEnabled: vi.fn((id: string, enabled: boolean) => ok(record({ id, enabled, name: id === 'wf_2' ? 'Stale alert' : 'Health sweep' }))),
    create: vi.fn(() => ok(record({ id: 'wf_new', name: 'New one' }))),
    update: vi.fn(() => ok(record())),
    listRuns: vi.fn(() => ok({ items: [run] })),
    getRun: vi.fn(() => ok(run)),
    listAudit: vi.fn(() => ok({ items: [{ id: 1, occurred_at: '2026-10-18T10:00:00Z', actor: 'system', action: 'workflow.created', target_type: 'workflow', target_id: 'wf_1', outcome: 'succeeded', detail: {} }] })),
  }
}

function setup(over: { data?: AutomationsData; mutations?: boolean; demo?: boolean; query?: string } = {}) {
  const actions = makeActions()
  const onChanged = vi.fn()
  render(<AutomationsView data={over.data ?? data()} system={{ demo_mode: over.demo ?? true, mutations_enabled: over.mutations ?? true }} query={over.query ?? ''} actions={actions} onChanged={onChanged} />)
  return { actions, onChanged }
}

describe('AutomationsView', () => {
  it('shows the summary, workflow table, recent runs and demo banner', () => {
    setup()
    expect(screen.getByText('Demo mode')).toBeInTheDocument()
    expect(within(screen.getByRole('group', { name: 'Active workflows' })).getByText('1')).toBeInTheDocument()
    expect(within(screen.getByRole('group', { name: 'Run success rate' })).getByText('75%')).toBeInTheDocument()
    expect(within(screen.getByRole('group', { name: 'Scheduled triggers' })).getByText(/Next run in 8 minutes/)).toBeInTheDocument()
    expect(within(screen.getByRole('group', { name: 'Failed runs' })).getByText('1')).toBeInTheDocument()
    const table = screen.getByRole('table', { name: 'Workflows' })
    expect(within(table).getByRole('button', { name: 'Health sweep' })).toBeInTheDocument()
    expect(within(table).getByText('Cron */15 * * * *')).toBeInTheDocument()
    expect(within(table).getByText('Never run')).toBeInTheDocument()
    expect(within(table).getAllByText('Simulated')).toHaveLength(2)
    expect(within(screen.getByRole('complementary', { name: 'Recent runs' })).getByText('Health sweep')).toBeInTheDocument()
  })

  it('shows an honest empty state when there are no runs or workflows', () => {
    setup({ data: data({ workflows: [], recentRuns: [], summary: { ...data().summary, workflows: 0, enabled: 0, disabled: 0, scheduled_count: 0, success_rate: null, runs_last_7_days: 0, succeeded_last_7_days: 0, failed_last_24h: 0, next_scheduled_at: null } }), demo: false })
    expect(screen.getByText('No runs yet', { selector: 'span' })).toBeInTheDocument()
    expect(screen.getByText('No workflows yet')).toBeInTheDocument()
    expect(screen.getByText('No workflow selected')).toBeInTheDocument()
    expect(screen.queryByText('Demo mode')).toBeNull()
  })

  it('filters workflows by the search query', () => {
    setup({ query: 'stale' })
    const table = screen.getByRole('table', { name: 'Workflows' })
    expect(within(table).queryByRole('button', { name: 'Health sweep' })).toBeNull()
    expect(within(table).getByRole('button', { name: 'Stale alert' })).toBeInTheDocument()
  })

  it('requires confirmation before running a workflow', async () => {
    const { actions, onChanged } = setup()
    await userEvent.click(screen.getByRole('button', { name: 'Run Health sweep' }))
    const dialog = screen.getByRole('alertdialog', { name: 'Run “Health sweep” now?' })
    expect(actions.run).not.toHaveBeenCalled()
    await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))
    expect(screen.queryByRole('alertdialog')).toBeNull()
    expect(actions.run).not.toHaveBeenCalled()

    await userEvent.click(screen.getByRole('button', { name: 'Run Health sweep' }))
    await userEvent.click(within(screen.getByRole('alertdialog')).getByRole('button', { name: 'Run now' }))
    await waitFor(() => expect(actions.run).toHaveBeenCalledWith('wf_1'))
    expect(await screen.findByText('Started a run of Health sweep.')).toBeInTheDocument()
    expect(onChanged).toHaveBeenCalled()
  })

  it('does not offer to run a disabled workflow and can enable it', async () => {
    const { actions } = setup()
    expect(screen.getByRole('button', { name: 'Run Stale alert' })).toBeDisabled()
    await userEvent.click(screen.getByRole('button', { name: 'Enable Stale alert' }))
    await waitFor(() => expect(actions.setEnabled).toHaveBeenCalledWith('wf_2', true))
    expect(await screen.findByText('Stale alert enabled.')).toBeInTheDocument()
  })

  it('shows API failures instead of hiding them', async () => {
    const { actions } = setup()
    ;(actions.run as ReturnType<typeof vi.fn>).mockResolvedValueOnce({ ok: false, error: { code: 'RUN_IN_PROGRESS', message: 'This workflow already has a run in progress', status: 409 } })
    await userEvent.click(screen.getByRole('button', { name: 'Run Health sweep' }))
    await userEvent.click(within(screen.getByRole('alertdialog')).getByRole('button', { name: 'Run now' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('already has a run in progress')
  })

  it('disables every change when mutations are not allowed', () => {
    setup({ mutations: false })
    expect(screen.getByText(/Changes are disabled/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Create workflow/ })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Run Health sweep' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Disable Health sweep' })).toBeDisabled()
    expect(screen.getByRole('button', { name: /Edit/ })).toBeDisabled()
  })

  it('creates a workflow from the editor and sends only writable fields', async () => {
    const { actions, onChanged } = setup()
    await userEvent.click(screen.getByRole('button', { name: /Create workflow/ }))
    expect(screen.getByText(/Not available yet: Wake node \(Milestone 9\)/)).toBeInTheDocument()
    await userEvent.type(screen.getByLabelText('Name'), 'Node check')
    await userEvent.click(screen.getByRole('button', { name: /Add action step/ }))
    await userEvent.type(screen.getByLabelText('Step 1 name'), 'Check it')
    await userEvent.selectOptions(screen.getByLabelText('Step 1 action'), 'check_node_health')
    await userEvent.selectOptions(screen.getByLabelText('Step 1 target'), 'n1')
    await userEvent.click(screen.getByRole('button', { name: 'Save workflow' }))
    await waitFor(() => expect(actions.create).toHaveBeenCalled())
    const sent = (actions.create as ReturnType<typeof vi.fn>).mock.calls[0][0]
    expect(Object.keys(sent).sort()).toEqual(['enabled', 'name', 'on_failure', 'on_success', 'steps', 'trigger'])
    expect(sent).toMatchObject({ name: 'Node check', enabled: true, trigger: { type: 'manual' }, on_failure: { behavior: 'stop' } })
    expect(sent.steps[0]).toEqual({ id: expect.any(String), type: 'action', name: 'Check it', action_type: 'check_node_health', target_id: 'n1' })
    expect(await screen.findByText('Saved “New one”.')).toBeInTheDocument()
    expect(onChanged).toHaveBeenCalled()
    expect(screen.queryByRole('form', { name: 'Workflow editor' })).toBeNull()
  })

  it('shows server-side validation messages in the editor', async () => {
    const { actions } = setup()
    ;(actions.create as ReturnType<typeof vi.fn>).mockResolvedValueOnce({ ok: false, error: { code: 'INVALID_WORKFLOW', message: 'The workflow definition is not valid', status: 400, details: { 'step.0.target': 'Choose a known target.' } } })
    await userEvent.click(screen.getByRole('button', { name: /Create workflow/ }))
    await userEvent.type(screen.getByLabelText('Name'), 'Bad one')
    await userEvent.click(screen.getByRole('button', { name: /Add action step/ }))
    await userEvent.type(screen.getByLabelText('Step 1 name'), 'x')
    await userEvent.selectOptions(screen.getByLabelText('Step 1 action'), 'check_node_health')
    await userEvent.selectOptions(screen.getByLabelText('Step 1 target'), 'n1')
    await userEvent.click(screen.getByRole('button', { name: 'Save workflow' }))
    const alerts = await screen.findAllByRole('alert')
    expect(alerts.some(alert => alert.textContent?.includes('Choose a known target.'))).toBe(true)
    expect(screen.getByRole('form', { name: 'Workflow editor' })).toBeInTheDocument()
  })

  it('edits the selected workflow through the same editor', async () => {
    const { actions } = setup()
    await userEvent.click(screen.getByRole('button', { name: /Edit/ }))
    const name = screen.getByLabelText('Name')
    expect(name).toHaveValue('Health sweep')
    await userEvent.clear(name)
    await userEvent.type(name, 'Renamed')
    await userEvent.click(screen.getByRole('button', { name: 'Save workflow' }))
    await waitFor(() => expect(actions.update).toHaveBeenCalledWith('wf_1', expect.objectContaining({ name: 'Renamed' })))
    const input = (actions.update as ReturnType<typeof vi.fn>).mock.calls[0][1]
    expect(input).not.toHaveProperty('id')
    expect(input).not.toHaveProperty('last_run')
  })

  it('switches tabs and shows run history with step results', async () => {
    const { actions } = setup()
    expect(screen.getByRole('list', { name: 'Health sweep workflow' })).toBeInTheDocument()
    await userEvent.click(screen.getByRole('tab', { name: 'Notifications' }))
    expect(screen.getByText('On success')).toBeInTheDocument()
    expect(screen.getByText(/external channels such as chat or email need secret storage/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('tab', { name: 'Settings' }))
    expect(await screen.findByText('workflow.created')).toBeInTheDocument()
    expect(actions.listAudit).toHaveBeenCalledWith('wf_1')

    await userEvent.click(screen.getByRole('tab', { name: 'Run history' }))
    expect(await screen.findByText('Step 1 (Check node one) failed: Node One: degraded, stale.')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Show steps' }))
    const steps = await screen.findByRole('list', { name: 'Steps for run run_1' })
    expect(within(steps).getByText('Check node one')).toBeInTheDocument()
    expect(within(steps).getByText(/Node One: degraded, stale\. \(CHECK_FAILED\)/)).toBeInTheDocument()
    expect(actions.getRun).toHaveBeenCalledWith('run_1')
  })
})

describe('helpers', () => {
  it('formats durations and relative times deterministically', () => {
    expect(duration(null)).toBe('—')
    expect(duration(420)).toBe('420 ms')
    expect(duration(2000)).toBe('2s')
    expect(duration(125000)).toBe('2m 5s')
    const from = '2026-10-18T12:00:00Z'
    expect(relativeFuture(null, from)).toBe('Not scheduled')
    expect(relativeFuture('2026-10-18T12:00:20Z', from)).toBe('in under a minute')
    expect(relativeFuture('2026-10-18T12:01:00Z', from)).toBe('in 1 minute')
    expect(relativeFuture('2026-10-18T12:12:00Z', from)).toBe('in 12 minutes')
    expect(relativeFuture('2026-10-18T15:00:00Z', from)).toBe('in 3 hours')
  })

  it('joins an API error message with its field details', () => {
    const error: ApiError = { code: 'INVALID_WORKFLOW', message: 'Not valid.', status: 400, details: { name: 'Name is required.' } }
    expect(describeError(error)).toBe('Not valid. Name is required.')
    expect(describeError({ code: 'X', message: 'Plain', status: 500 })).toBe('Plain')
  })

  it('toInput drops server-managed fields and empty optionals', () => {
    const wf = { ...record(), description: '' } as Workflow
    const input = toInput(wf)
    expect(input).toEqual({
      name: 'Health sweep', enabled: true, trigger: { type: 'cron', expression: '*/15 * * * *' },
      steps: [{ id: 's1', type: 'action', name: 'Check node one', action_type: 'check_node_health', target_id: 'n1' }],
      on_success: { notify_channel: 'activity' }, on_failure: { behavior: 'continue' },
    })
  })
})
