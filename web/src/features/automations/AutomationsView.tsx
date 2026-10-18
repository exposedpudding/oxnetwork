import { useCallback, useEffect, useMemo, useState } from 'react'
import { Pencil, Play, Plus, Power } from 'lucide-react'
import { Card, StatCard } from '../../components/Card'
import { ConfirmDangerDialog } from '../../components/ConfirmDangerDialog'
import { DataTable } from '../../components/DataTable'
import { EmptyState } from '../../components/States'
import { StatusChip, type Status } from '../../components/Status'
import type { Result } from '../../lib/api/client'
import type { ApiError, AuditEvent, AutomationOptions, AutomationSummary, ListResponse, RunState, RunStepState, WorkflowInput, WorkflowRecord, WorkflowRun } from '../../lib/api/types'
import { WorkflowDiagram, describeTrigger } from '../workflows/WorkflowDiagram'
import { WorkflowEditor } from '../workflows/WorkflowEditor'
import type { EditorOptions, Workflow } from '../workflows/types'

export interface AutomationsData {
  summary: AutomationSummary
  workflows: WorkflowRecord[]
  options: AutomationOptions
  recentRuns: WorkflowRun[]
}

/** Server calls the page makes. Injected so the view is testable without a network. */
export interface AutomationActions {
  run(id: string): Promise<Result<WorkflowRun>>
  setEnabled(id: string, enabled: boolean): Promise<Result<WorkflowRecord>>
  create(input: WorkflowInput): Promise<Result<WorkflowRecord>>
  update(id: string, input: WorkflowInput): Promise<Result<WorkflowRecord>>
  listRuns(id: string): Promise<Result<ListResponse<WorkflowRun>>>
  getRun(id: string): Promise<Result<WorkflowRun>>
  listAudit(workflowId: string): Promise<Result<ListResponse<AuditEvent>>>
}

export interface AutomationsViewProps {
  data: AutomationsData
  system: { demo_mode: boolean; mutations_enabled?: boolean }
  query: string
  actions: AutomationActions
  /** Called after any change so the page can reload its data. */
  onChanged: () => void
}

type Tab = 'workflow' | 'settings' | 'notifications' | 'history'
type Mode = { kind: 'none' } | { kind: 'create' } | { kind: 'edit'; id: string }

const TABS: { id: Tab; label: string }[] = [
  { id: 'workflow', label: 'Workflow' },
  { id: 'settings', label: 'Settings' },
  { id: 'notifications', label: 'Notifications' },
  { id: 'history', label: 'Run history' },
]

const RUN_STATUS: Record<RunState, Status> = { queued: 'queued', running: 'running', succeeded: 'success', failed: 'failed', cancelled: 'cancelled' }
const RUN_LABEL: Record<RunState, string> = { queued: 'Queued', running: 'Running', succeeded: 'Success', failed: 'Failed', cancelled: 'Cancelled' }
const STEP_STATUS: Record<RunStepState, Status> = { pending: 'queued', running: 'running', succeeded: 'success', failed: 'failed', skipped: 'cancelled' }
const STEP_LABEL: Record<RunStepState, string> = { pending: 'Pending', running: 'Running', succeeded: 'Success', failed: 'Failed', skipped: 'Skipped' }

function when(value: string | null | undefined) {
  return value ? new Date(value).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' }) : '—'
}

export function duration(ms: number | null | undefined) {
  if (ms == null) return '—'
  if (ms < 1000) return `${ms} ms`
  const seconds = Math.round(ms / 1000)
  return seconds < 60 ? `${seconds}s` : `${Math.floor(seconds / 60)}m ${seconds % 60}s`
}

/** "in 12 minutes" relative to `from`; deterministic because callers pass the server's clock. */
export function relativeFuture(iso: string | null | undefined, from: string) {
  if (!iso) return 'Not scheduled'
  const minutes = Math.max(0, Math.round((new Date(iso).getTime() - new Date(from).getTime()) / 60000))
  if (minutes < 1) return 'in under a minute'
  if (minutes < 60) return `in ${minutes} minute${minutes === 1 ? '' : 's'}`
  const hours = Math.round(minutes / 60)
  if (hours < 48) return `in ${hours} hour${hours === 1 ? '' : 's'}`
  return `on ${when(iso)}`
}

export function describeError(error: ApiError): string {
  const details = error.details ? Object.values(error.details) : []
  return [error.message, ...details].join(' ')
}

/** Only the writable fields are sent; the server rejects unknown properties. */
export function toInput(workflow: Workflow): WorkflowInput {
  return {
    name: workflow.name,
    ...(workflow.description ? { description: workflow.description } : {}),
    enabled: workflow.enabled,
    trigger: workflow.trigger,
    steps: workflow.steps.map(step => step.type === 'action'
      ? { id: step.id, type: 'action' as const, name: step.name, action_type: step.action_type, target_id: step.target_id }
      : { id: step.id, type: 'notify' as const, name: step.name, channel: step.channel, message: step.message }),
    on_success: workflow.on_success.notify_channel ? { notify_channel: workflow.on_success.notify_channel } : {},
    on_failure: workflow.on_failure.notify_channel
      ? { behavior: workflow.on_failure.behavior, notify_channel: workflow.on_failure.notify_channel }
      : { behavior: workflow.on_failure.behavior },
  }
}

function blankWorkflow(): Workflow {
  return { id: '', name: '', enabled: true, trigger: { type: 'manual' }, steps: [], on_success: {}, on_failure: { behavior: 'stop' } }
}

function RunChip({ run }: { run?: { state: RunState } | null }) {
  if (!run) return <StatusChip status="unknown">No runs</StatusChip>
  return <StatusChip status={RUN_STATUS[run.state]}>{RUN_LABEL[run.state]}</StatusChip>
}

export function AutomationsView({ data, system, query, actions, onChanged }: AutomationsViewProps) {
  const canChange = !!system.mutations_enabled
  const [selectedId, setSelectedId] = useState<string | undefined>(data.workflows[0]?.id)
  const [tab, setTab] = useState<Tab>('workflow')
  const [mode, setMode] = useState<Mode>({ kind: 'none' })
  const [confirmRun, setConfirmRun] = useState<WorkflowRecord | null>(null)
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState<string | null>(null)
  const [problem, setProblem] = useState<string | null>(null)
  const [formError, setFormError] = useState<string | null>(null)
  const [runs, setRuns] = useState<WorkflowRun[] | null>(null)
  const [runsError, setRunsError] = useState<string | null>(null)
  const [openRun, setOpenRun] = useState<WorkflowRun | null>(null)
  const [audit, setAudit] = useState<AuditEvent[] | null>(null)

  const lower = query.trim().toLowerCase()
  const visible = data.workflows.filter(wf => `${wf.name} ${wf.description ?? ''}`.toLowerCase().includes(lower))
  const selected = data.workflows.find(wf => wf.id === selectedId) ?? visible[0]
  const editorOptions: EditorOptions = useMemo(() => ({ actionTypes: data.options.action_types, targets: data.options.targets, channels: data.options.channels }), [data.options])
  const channelLabel = (value?: string) => data.options.channels.find(channel => channel.value === value)?.label ?? value ?? 'None'
  const runKey = selected ? `${selected.id}:${selected.last_run?.id ?? ''}:${selected.last_run?.state ?? ''}` : ''

  useEffect(() => {
    setOpenRun(null)
    setRuns(null)
    setAudit(null)
  }, [selected?.id])

  useEffect(() => {
    if (!selected || (tab !== 'history' && tab !== 'settings')) return
    let alive = true
    setRunsError(null)
    if (tab === 'history') {
      void actions.listRuns(selected.id).then(result => {
        if (!alive) return
        if (result.ok) setRuns(result.data.items)
        else setRunsError(describeError(result.error))
      })
    } else {
      void actions.listAudit(selected.id).then(result => { if (alive && result.ok) setAudit(result.data.items) })
    }
    return () => { alive = false }
    // runKey changes when a new run starts or finishes for the selected workflow.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tab, runKey, actions])

  const report = useCallback(async <T,>(promise: Promise<Result<T>>, success: (value: T) => string) => {
    setBusy(true)
    setProblem(null)
    setNotice(null)
    const result = await promise
    setBusy(false)
    if (!result.ok) {
      setProblem(describeError(result.error))
      return undefined
    }
    setNotice(success(result.data))
    onChanged()
    return result.data
  }, [onChanged])

  const openDetails = async (run: WorkflowRun) => {
    if (openRun?.id === run.id) { setOpenRun(null); return }
    const result = await actions.getRun(run.id)
    if (result.ok) setOpenRun(result.data)
    else setRunsError(describeError(result.error))
  }

  const save = async (workflow: Workflow) => {
    setBusy(true)
    setFormError(null)
    const input = toInput(workflow)
    const result = mode.kind === 'edit' ? await actions.update(mode.id, input) : await actions.create(input)
    setBusy(false)
    if (!result.ok) {
      setFormError(describeError(result.error))
      return
    }
    setMode({ kind: 'none' })
    setSelectedId(result.data.id)
    setNotice(`Saved “${result.data.name}”.`)
    onChanged()
  }

  const summary = data.summary
  const lastRunLabel = (wf: WorkflowRecord) => wf.last_run ? <><time dateTime={wf.last_run.started_at}>{when(wf.last_run.started_at)}</time><small className="auto-sub">{duration(wf.last_run.duration_ms)} · {wf.last_run.trigger}</small></> : <span className="ox-muted">Never run</span>
  const editing = mode.kind === 'edit' ? data.workflows.find(wf => wf.id === mode.id) : undefined

  return <>
    <div className="page-heading"><div><p className="eyebrow">Command centre / Automations</p><h1>Automations</h1><p>Workflows, triggers and operational routines.</p></div>
      <button type="button" className="ox-btn ox-btn--primary" disabled={!canChange || mode.kind === 'create'} title={canChange ? undefined : 'Changes are disabled on this server'} onClick={() => { setMode({ kind: 'create' }); setFormError(null) }}><Plus size={14} aria-hidden="true" /> Create workflow</button>
    </div>
    {system.demo_mode && <div className="demo-banner" role="status"><strong>Demo mode</strong><span>These workflows are simulated and only read the demo inventory. Nothing on your network is changed.</span></div>}
    {!canChange && <div className="compute-notice" role="status">Changes are disabled: sign-in is not configured and the server is not bound to loopback. Workflows can be viewed but not edited or run.</div>}
    {notice && <div className="auto-notice" role="status">{notice}</div>}
    {problem && <div role="alert" className="error">{problem}</div>}

    <section aria-label="Automation summary" className="ox-grid compute-summary">
      <StatCard label="Active workflows" value={summary.enabled} detail={`${summary.disabled} disabled`} />
      <StatCard label="Run success rate" value={summary.success_rate == null ? 'No runs yet' : `${Math.round(summary.success_rate)}%`} detail={`${summary.succeeded_last_7_days} of ${summary.runs_last_7_days} runs succeeded · 7 days`} />
      <StatCard label="Scheduled triggers" value={summary.scheduled_count} detail={summary.scheduled_count ? `Next run ${relativeFuture(summary.next_scheduled_at, summary.generated_at)}` : 'No schedules enabled'} />
      <StatCard label="Failed runs" value={summary.failed_last_24h} detail="In the last 24 hours" />
    </section>

    {mode.kind !== 'none' && <Card title={mode.kind === 'create' ? 'Create workflow' : `Edit “${editing?.name ?? ''}”`} className="compute-section">
      {formError && <div role="alert" className="error">{formError}</div>}
      {data.options.planned_actions.length > 0 && <p className="muted-copy auto-planned">Not available yet: {data.options.planned_actions.map(action => `${action.label} (${action.milestone})`).join(', ')}.</p>}
      <WorkflowEditor key={mode.kind === 'edit' ? mode.id : 'new'} initial={mode.kind === 'edit' && editing ? editing : blankWorkflow()} options={editorOptions} onSave={workflow => { void save(workflow) }} onCancel={() => setMode({ kind: 'none' })} />
      {busy && <p className="muted-copy" role="status">Saving…</p>}
    </Card>}

    <div className="auto-layout">
      <div className="auto-main">
        <Card title="Workflows" className="compute-section">
          <DataTable<WorkflowRecord> caption="Workflows" rows={visible} rowKey={wf => wf.id}
            emptyTitle={lower ? 'No workflows match your search' : 'No workflows yet'}
            emptyDescription={lower ? 'Try a different search.' : canChange ? 'Create a workflow to automate a routine check.' : 'No workflows have been created.'}
            columns={[
              { key: 'name', header: 'Name', render: wf => <><button type="button" className="auto-name" aria-pressed={selected?.id === wf.id} onClick={() => { setSelectedId(wf.id); setTab('workflow') }}>{wf.name}</button>{wf.simulated && <span className="ox-chip auto-sim">Simulated</span>}{wf.description && <small className="auto-sub">{wf.description}</small>}</> },
              { key: 'trigger', header: 'Trigger', render: wf => <>{describeTrigger(wf.trigger, { targets: data.options.targets })}{wf.next_run_at && <small className="auto-sub">Next {relativeFuture(wf.next_run_at, summary.generated_at)}</small>}</> },
              { key: 'last', header: 'Last run', render: lastRunLabel },
              { key: 'status', header: 'Status', render: wf => wf.enabled ? <RunChip run={wf.last_run} /> : <StatusChip status="disabled" /> },
              { key: 'steps', header: 'Steps', render: wf => wf.steps.length, align: 'right' },
              { key: 'actions', header: 'Actions', render: wf => <span className="auto-actions">
                <button type="button" className="ox-btn" aria-label={`Run ${wf.name}`} disabled={!canChange || !wf.enabled || busy} onClick={() => setConfirmRun(wf)}><Play size={14} aria-hidden="true" /></button>
                <button type="button" className="ox-btn" aria-label={`${wf.enabled ? 'Disable' : 'Enable'} ${wf.name}`} disabled={!canChange || busy} onClick={() => { void report(actions.setEnabled(wf.id, !wf.enabled), value => `${value.name} ${value.enabled ? 'enabled' : 'disabled'}.`) }}><Power size={14} aria-hidden="true" /></button>
              </span> },
            ]} />
        </Card>

        {selected ? <Card className="compute-section" title={<span className="auto-detail-title">{selected.name} {selected.enabled ? <StatusChip status="healthy">Enabled</StatusChip> : <StatusChip status="disabled" />}</span>}
          actions={<span className="auto-actions">
            <button type="button" className="ox-btn" disabled={!canChange || !selected.enabled || busy} onClick={() => setConfirmRun(selected)}><Play size={14} aria-hidden="true" /> Run now</button>
            <button type="button" className="ox-btn" disabled={!canChange || busy} onClick={() => { void report(actions.setEnabled(selected.id, !selected.enabled), value => `${value.name} ${value.enabled ? 'enabled' : 'disabled'}.`) }}><Power size={14} aria-hidden="true" /> {selected.enabled ? 'Disable' : 'Enable'}</button>
            <button type="button" className="ox-btn" disabled={!canChange || busy} onClick={() => { setMode({ kind: 'edit', id: selected.id }); setFormError(null) }}><Pencil size={14} aria-hidden="true" /> Edit</button>
          </span>}>
          <div role="tablist" aria-label="Workflow sections" className="auto-tabs">
            {TABS.map(item => <button key={item.id} type="button" role="tab" id={`auto-tab-${item.id}`} aria-selected={tab === item.id} aria-controls="auto-panel" className="auto-tab" onClick={() => setTab(item.id)}>{item.label}</button>)}
          </div>
          <div role="tabpanel" id="auto-panel" aria-labelledby={`auto-tab-${tab}`}>
            {tab === 'workflow' && <WorkflowDiagram workflow={selected} options={editorOptions} />}
            {tab === 'settings' && <>
              <dl className="auto-facts">
                <dt>Trigger</dt><dd>{describeTrigger(selected.trigger, { targets: data.options.targets })}</dd>
                <dt>On failure</dt><dd>{selected.on_failure.behavior === 'stop' ? 'Stop the run and skip remaining steps' : 'Continue with the remaining steps'}</dd>
                <dt>Step limits</dt><dd>30 seconds per step, 5 minutes per run, no automatic retries</dd>
                <dt>Concurrency</dt><dd>One run at a time; a trigger that arrives during a run is skipped and audited</dd>
                <dt>Created</dt><dd>{when(selected.created_at)}</dd>
                <dt>Updated</dt><dd>{when(selected.updated_at)}</dd>
                <dt>Source</dt><dd>{selected.simulated ? 'Simulated demo workflow' : 'Created in this workspace'}</dd>
              </dl>
              <h3 className="auto-h3">Audit trail</h3>
              {audit === null ? <p className="muted-copy" role="status">Loading audit trail…</p> : audit.length === 0 ? <p className="muted-copy">No audit events recorded.</p> : <ul className="auto-audit">{audit.slice(0, 8).map(event => <li key={event.id}><span className="ox-mono">{event.action}</span><span>{event.actor}</span><StatusChip status={event.outcome === 'succeeded' ? 'success' : event.outcome === 'failed' ? 'failed' : 'unknown'}>{event.outcome}</StatusChip><time dateTime={event.occurred_at}>{when(event.occurred_at)}</time></li>)}</ul>}
            </>}
            {tab === 'notifications' && <dl className="auto-facts">
              <dt>On success</dt><dd>{selected.on_success.notify_channel ? channelLabel(selected.on_success.notify_channel) : 'No notification'}</dd>
              <dt>On failure</dt><dd>{selected.on_failure.notify_channel ? channelLabel(selected.on_failure.notify_channel) : 'No notification'}</dd>
              <dt>Channels</dt><dd>{data.options.channels.map(channel => channel.label).join(', ')} only. Notifications are written to the audit trail; external channels such as chat or email need secret storage and are not available yet.</dd>
            </dl>}
            {tab === 'history' && (runsError ? <div role="alert" className="error">{runsError}</div> : runs === null ? <p className="muted-copy" role="status">Loading run history…</p> : runs.length === 0 ? <EmptyState title="No runs yet" description="Run this workflow to see step results here." /> : <ul className="auto-runs">{runs.map(run => <li key={run.id}>
              <div className="auto-run-head"><RunChip run={run} /><time dateTime={run.started_at}>{when(run.started_at)}</time><span className="ox-muted">{duration(run.duration_ms)} · {run.trigger} · {run.actor}</span>
                <button type="button" className="ox-btn auto-details" aria-expanded={openRun?.id === run.id} onClick={() => { void openDetails(run) }}>{openRun?.id === run.id ? 'Hide steps' : 'Show steps'}</button></div>
              <p className="muted-copy">{run.summary}</p>
              {openRun?.id === run.id && <ol className="auto-steps" aria-label={`Steps for run ${run.id}`}>{(openRun.steps ?? []).map(step => <li key={step.position}><StatusChip status={STEP_STATUS[step.state]}>{STEP_LABEL[step.state]}</StatusChip><strong>{step.name}</strong><span className="ox-muted">{step.output || '—'}{step.error_code ? ` (${step.error_code})` : ''}</span></li>)}</ol>}
            </li>)}</ul>)}
          </div>
        </Card> : <Card className="compute-section"><EmptyState title="No workflow selected" description={data.workflows.length ? 'Choose a workflow above to see its steps and history.' : 'Create a workflow to get started.'} /></Card>}
      </div>

      <aside aria-label="Recent runs"><Card title="Recent runs">
        {data.recentRuns.length === 0 ? <EmptyState title="No runs yet" description="Runs from every workflow appear here." /> : <ul className="auto-recent">{data.recentRuns.map(run => <li key={run.id}>
          <RunChip run={run} /><span className="auto-recent-name">{run.workflow_name}</span><time dateTime={run.started_at}>{when(run.started_at)}</time><span className="ox-muted">{duration(run.duration_ms)}</span>
        </li>)}</ul>}
      </Card></aside>
    </div>

    <ConfirmDangerDialog open={confirmRun !== null} tone="neutral" title={`Run “${confirmRun?.name ?? ''}” now?`} confirmLabel="Run now" busy={busy}
      description={<>This starts the workflow immediately with {confirmRun?.steps.length ?? 0} step{confirmRun?.steps.length === 1 ? '' : 's'}. Only read-only checks and notifications can run, and the run is recorded in the audit trail.</>}
      onCancel={() => setConfirmRun(null)}
      onConfirm={() => { const target = confirmRun; setConfirmRun(null); if (target) { setSelectedId(target.id); void report(actions.run(target.id), run => `Started a run of ${run.workflow_name}.`) } }} />
  </>
}
