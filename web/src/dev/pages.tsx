import { Fragment, useState } from 'react';
import { Card, ConfirmDangerDialog, DataTable, EmptyState, ErrorState, MetricBar, StaleBadge, StatCard, StatusChip, StatusDot, type Status } from '../components';
import { TopologyView } from '../features/topology/TopologyView';
import { WorkflowDiagram } from '../features/workflows/WorkflowDiagram';
import { WorkflowEditor } from '../features/workflows/WorkflowEditor';
import type { Workflow } from '../features/workflows/types';
import type { Node } from '../lib/api/types';
import { activity, computeSummary, editorOptions, FIXTURE_NOW, graph, graphDetails, jobs, metrics, nodes, services, workers, workflowRuns, workflows } from '../fixtures';

const statusOf = (n: Pick<Node, 'health' | 'freshness'>): Status => (n.freshness === 'stale' ? 'stale' : n.health.state);
const nodeName = (id: string) => nodes.find((n) => n.id === id)?.name ?? id;

/** Drain action with a name-typed confirmation. Local-only: the harness never calls an API. */
function DrainButton({ node }: { node: Node }) {
  const [open, setOpen] = useState(false);
  const [done, setDone] = useState(false);
  return (
    <>
      <button type="button" className="ox-btn ox-btn--danger" onClick={() => setOpen(true)}>Drain node</button>
      {done ? <span role="status">Drain requested for {node.name}</span> : null}
      <ConfirmDangerDialog
        open={open}
        title={`Drain ${node.name}?`}
        description="Running jobs finish, but no new work is scheduled on this node."
        confirmLabel="Drain node"
        requireText={node.name}
        onCancel={() => setOpen(false)}
        onConfirm={() => (setOpen(false), setDone(true))}
      />
    </>
  );
}

export function OverviewPage() {
  return (
    <>
      <p className="ox-muted" style={{ margin: 0 }}>Infrastructure, services and activity at a glance. (Fixture data)</p>
      <div className="ox-grid" aria-label="Nodes">
        {nodes.map((n) => {
          const m = n.metrics;
          const st = statusOf(n);
          return (
            <Card key={n.id} headingLevel={3} title={<span style={{ display: 'inline-flex', gap: 8, alignItems: 'center' }}><StatusDot status={st} />{n.name}</span>} actions={st === 'stale' ? <StaleBadge observedAt={n.health.observed_at} /> : undefined}>
              <p className="ox-muted" style={{ marginTop: 0 }}>{n.role ?? 'Role not reported'}</p>
              <MetricBar label="CPU" value={m?.cpu_percent} />
              <MetricBar label="RAM" value={m?.ram_percent} />
            </Card>
          );
        })}
      </div>
      <div className="ox-grid" style={{ gridTemplateColumns: 'repeat(3, minmax(0, 1fr))' }}>
        <Card title="Services health">
          <ul style={{ listStyle: 'none', margin: 0, padding: 0, display: 'grid', gap: 8 }}>
            {services.map((s) => (
              <li key={s.id} style={{ display: 'flex', justifyContent: 'space-between' }}>
                <span>{s.name}</span>
                <StatusChip status={statusOf(s)} />
              </li>
            ))}
          </ul>
        </Card>
        <Card title="Recent activity">
          <ul style={{ listStyle: 'none', margin: 0, padding: 0, display: 'grid', gap: 8 }}>
            {activity.map((a) => <li key={a.id}><StatusDot status={a.state as Status} /> {a.title} <span className="ox-muted">· {a.detail}</span></li>)}
          </ul>
        </Card>
        <Card title="Quick actions">
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            <button type="button" className="ox-btn ox-btn--primary">Run backup</button>
            <DrainButton node={nodes[1]} />
          </div>
        </Card>
      </div>
    </>
  );
}

export function ComputePage() {
  const s = computeSummary;
  return (
    <>
      <p className="ox-muted" style={{ margin: 0 }}>Aggregate values are informational; placement respects per-node limits.</p>
      <div className="ox-grid">
        <StatCard label="Total vCPU" value={s.total_vcpu} />
        <StatCard label="Used" value={s.used_vcpu} detail={`${s.reserved_vcpu} reserved`} />
        <StatCard label="Active jobs" value={s.active_jobs} />
        <StatCard label="Queued jobs" value={s.queued_jobs} />
      </div>
      <Card title="Workers">
        <DataTable
          caption="Workers"
          rows={workers}
          rowKey={(w) => w.id}
          columns={[
            { key: 'name', header: 'Worker', render: (w) => w.name },
            { key: 'node', header: 'Node', render: (w) => nodeName(w.node_id) },
            { key: 'state', header: 'State', render: (w) => <StatusChip status={w.state} /> },
            { key: 'cpu', header: 'vCPU used', align: 'right', render: (w) => `${w.vcpu_used} / ${w.vcpu_total}` },
          ]}
        />
      </Card>
      <Card title="Jobs">
        <DataTable
          caption="Jobs"
          rows={jobs}
          rowKey={(j) => j.id}
          columns={[
            { key: 'name', header: 'Job', render: (j) => j.name },
            { key: 'state', header: 'State', render: (j) => <StatusChip status={j.state === 'succeeded' ? 'success' : j.state === 'idle' ? 'unknown' : j.state} /> },
            { key: 'worker', header: 'Worker', render: (j) => workers.find((w) => w.id === j.worker_id)?.name ?? 'Unassigned' },
          ]}
        />
      </Card>
    </>
  );
}

export function NodeDetailPage({ nodeId }: { nodeId: string }) {
  const node = nodes.find((n) => n.id === nodeId);
  if (!node) return <ErrorState title="Node not found" message={`No node with id ${nodeId}.`} />;
  const m = metrics[node.id];
  const st = statusOf(node);
  return (
    <>
      <Card title={<span style={{ display: 'inline-flex', gap: 8, alignItems: 'center' }}><StatusDot status={st} />{node.name}</span>} actions={st === 'stale' ? <StaleBadge observedAt={node.health.observed_at} /> : <StatusChip status={st} />}>
        <p className="ox-muted" style={{ marginTop: 0 }}>{node.role} · {node.platform ?? 'Platform not reported'}</p>
        <div style={{ display: 'grid', gap: 8, maxWidth: 480 }}>
          <MetricBar label="CPU" value={m?.cpu_percent} />
          <MetricBar label="RAM" value={m?.ram_percent} />
          <MetricBar label="Disk" value={m?.storage_percent} />
          <div className="ox-metric" style={{ gridTemplateColumns: '88px 1fr' }}>
            <span className="ox-muted">Temperature</span>
            <span>{m?.temperature_celsius === undefined ? <span className="ox-muted">Not reported</span> : `${m.temperature_celsius} °C`}</span>
          </div>
        </div>
      </Card>
      <div style={{ display: 'flex', gap: 8 }}>
        {node.capabilities.includes('wake') ? <button type="button" className="ox-btn">Wake</button> : null}
        {node.capabilities.includes('compute') ? <DrainButton node={node} /> : null}
      </div>
    </>
  );
}

export function NetworkPage() {
  const [selected, setSelected] = useState('g-edge');
  const d = graphDetails[selected];
  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0, 1fr) 320px', gap: 16 }}>
      <TopologyView graph={graph} selectedId={selected} onSelect={setSelected} now={FIXTURE_NOW} />
      <Card title="Node details" headingLevel={2}>
        <h3 data-testid="detail-title" style={{ marginTop: 0 }}>{d.title}</h3>
        <dl style={{ display: 'grid', gridTemplateColumns: 'auto 1fr', gap: '6px 12px', margin: 0 }}>
          {d.rows.map(([k, v]) => <Fragment key={k}><dt className="ox-muted">{k}</dt><dd style={{ margin: 0 }}>{v}</dd></Fragment>)}
        </dl>
      </Card>
    </div>
  );
}

export function AutomationsPage() {
  const [selectedId, setSelectedId] = useState(workflows[0].id);
  const [confirming, setConfirming] = useState(false);
  const [message, setMessage] = useState('');
  const [editing, setEditing] = useState(false);
  const [saved, setSaved] = useState<Workflow[]>([]);
  const all = [...workflows, ...saved.map((w) => ({ ...w, last_run_status: 'disabled' as const }))];
  const wf = all.find((w) => w.id === selectedId)!;
  return (
    <>
      <Card title="Workflows" actions={<button type="button" className="ox-btn ox-btn--primary" onClick={() => setEditing(true)}>Create workflow</button>}>
        <DataTable
          caption="Workflows"
          rows={all}
          rowKey={(w) => w.id}
          selectedKey={selectedId}
          onRowSelect={(w) => (setSelectedId(w.id), setMessage(''))}
          columns={[
            { key: 'name', header: 'Name', render: (w) => w.name },
            { key: 'status', header: 'Status', render: (w) => <StatusChip status={w.enabled ? w.last_run_status : 'disabled'} /> },
          ]}
        />
      </Card>
      {editing ? (
        <Card title="New workflow">
          <WorkflowEditor
            initial={{ id: `wf-new-${saved.length + 1}`, name: '', enabled: false, trigger: { type: 'manual' }, steps: [], on_success: {}, on_failure: { behavior: 'stop' } }}
            options={editorOptions}
            onCancel={() => setEditing(false)}
            onSave={(w) => (setSaved((s) => [...s, w]), setSelectedId(w.id), setEditing(false))}
          />
        </Card>
      ) : null}
      <Card title={wf.name}>
        <WorkflowDiagram workflow={wf} options={editorOptions} />
        <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          <button type="button" className="ox-btn" disabled={!wf.enabled} onClick={() => setConfirming(true)}>Run now</button>
          {message ? <span role="status">{message}</span> : null}
        </div>
        <ConfirmDangerDialog
          open={confirming}
          tone="neutral"
          title={`Run "${wf.name}" now?`}
          description="This starts a manual run and creates an audit event."
          confirmLabel="Run workflow"
          onCancel={() => setConfirming(false)}
          onConfirm={() => (setConfirming(false), setMessage(`Run requested for ${wf.name}`))}
        />
      </Card>
      <Card title="Recent runs">
        <DataTable
          caption="Recent runs"
          rows={workflowRuns.filter((r) => r.workflow_id === wf.id)}
          rowKey={(r) => r.id}
          emptyTitle="No runs yet"
          columns={[
            { key: 's', header: 'Status', render: (r) => <StatusChip status={r.status} /> },
            { key: 'd', header: 'Duration', align: 'right', render: (r) => `${r.duration_seconds}s` },
          ]}
        />
      </Card>
    </>
  );
}

export function ComponentsPage() {
  const statuses: Status[] = ['healthy', 'degraded', 'unhealthy', 'unknown', 'disabled', 'stale', 'success', 'running', 'queued', 'failed', 'cancelled'];
  return (
    <>
      <Card title="Status chips">
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>{statuses.map((s) => <StatusChip key={s} status={s} />)}</div>
      </Card>
      <Card title="Metric bars">
        <div style={{ display: 'grid', gap: 8, maxWidth: 360 }}>
          <MetricBar label="Low" value={20} />
          <MetricBar label="High" value={80} />
          <MetricBar label="Critical" value={95} />
          <MetricBar label="Missing" value={undefined} />
        </div>
      </Card>
      <div className="ox-grid">
        <Card title="Empty"><EmptyState title="No items" description="Nothing here yet." /></Card>
        <Card title="Error"><ErrorState message="Could not load nodes." stale requestId="req-demo" onRetry={() => undefined} /></Card>
      </div>
    </>
  );
}
