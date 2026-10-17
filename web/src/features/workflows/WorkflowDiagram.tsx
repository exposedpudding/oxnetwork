import { ArrowRight } from 'lucide-react';
import type { EditorOptions, Trigger, Workflow } from './types';

export function describeTrigger(t: Trigger, opts?: Pick<EditorOptions, 'targets'>): string {
  switch (t.type) {
    case 'manual':
      return 'Manual trigger';
    case 'cron':
      return `Cron ${t.expression}`;
    case 'health_condition': {
      const target = opts?.targets.find((x) => x.id === t.target_id)?.label ?? t.target_id;
      return `${target} ${t.condition}${t.for_seconds ? ` for ${t.for_seconds}s` : ''}`;
    }
  }
}

/** Linear Trigger -> Step 1..n -> Outcome diagram. Read-only; used by the viewer and editor preview. */
export function WorkflowDiagram({ workflow, options }: { workflow: Workflow; options?: Partial<EditorOptions> }) {
  const label = (list: { id?: string; value?: string; label: string }[] | undefined, key: string) =>
    list?.find((x) => (x.id ?? x.value) === key)?.label ?? key;
  const items: { kind: string; title: string; detail?: string; outcome?: boolean }[] = [
    { kind: 'Trigger', title: describeTrigger(workflow.trigger, { targets: options?.targets ?? [] }) },
    ...workflow.steps.map((s, i) =>
      s.type === 'action'
        ? { kind: `Step ${i + 1}`, title: s.name, detail: `${label(options?.actionTypes?.map((a) => ({ value: a.value, label: a.label })), s.action_type)} on ${label(options?.targets, s.target_id)}` }
        : { kind: `Step ${i + 1}`, title: s.name, detail: `Notify via ${label(options?.channels?.map((c) => ({ value: c.value, label: c.label })), s.channel)}` },
    ),
    {
      kind: 'Outcome',
      title: 'Success or failure',
      detail: `On failure: ${workflow.on_failure.behavior === 'stop' ? 'stop' : 'continue'}${workflow.on_failure.notify_channel ? ` and notify ${label(options?.channels?.map((c) => ({ value: c.value, label: c.label })), workflow.on_failure.notify_channel)}` : ''}`,
      outcome: true,
    },
  ];
  return (
    <ol className="ox-flow" aria-label={`${workflow.name} workflow`}>
      {items.map((it, i) => (
        <li key={i} className="ox-flow__item">
          <div className={`ox-flow__card${it.outcome ? ' ox-flow__card--outcome' : ''}`}>
            <span className="ox-flow__kind">{it.kind}</span>
            <strong>{it.title}</strong>
            {it.detail ? <span className="ox-muted" style={{ fontSize: 12 }}>{it.detail}</span> : null}
          </div>
          {i < items.length - 1 ? <ArrowRight className="ox-flow__arrow" size={18} aria-label="then" role="img" /> : null}
        </li>
      ))}
    </ol>
  );
}
