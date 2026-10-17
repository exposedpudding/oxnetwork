import { useId, useState } from 'react';
import { ArrowDown, ArrowUp, Plus, Trash2 } from 'lucide-react';
import type { EditorOptions, Step, Trigger, Workflow, WorkflowErrors } from './types';
import { validateWorkflow } from './validate';
import { WorkflowDiagram } from './WorkflowDiagram';

export interface WorkflowEditorProps {
  initial: Workflow;
  options: EditorOptions;
  onSave: (workflow: Workflow) => void;
  onCancel?: () => void;
  /** Id factory for new steps; defaults to crypto.randomUUID. */
  createId?: () => string;
}

function Field({ label, error, children, htmlFor }: { label: string; error?: string; htmlFor: string; children: React.ReactNode }) {
  return (
    <div className="ox-field">
      <label htmlFor={htmlFor}>{label}</label>
      {children}
      {error ? <span className="ox-field__error" role="alert">{error}</span> : null}
    </div>
  );
}

export function WorkflowEditor({ initial, options, onSave, onCancel, createId = () => crypto.randomUUID() }: WorkflowEditorProps) {
  const [wf, setWf] = useState<Workflow>(initial);
  const [errors, setErrors] = useState<WorkflowErrors>({});
  const uid = useId();
  const id = (s: string) => `${uid}-${s}`;
  const patch = (p: Partial<Workflow>) => setWf((w) => ({ ...w, ...p }));
  const setStep = (i: number, s: Step) => patch({ steps: wf.steps.map((x, j) => (j === i ? s : x)) });
  const move = (i: number, d: -1 | 1) => {
    const steps = [...wf.steps];
    [steps[i], steps[i + d]] = [steps[i + d], steps[i]];
    patch({ steps });
  };

  const setTriggerType = (type: Trigger['type']) => {
    patch({
      trigger:
        type === 'manual'
          ? { type }
          : type === 'cron'
            ? { type, expression: '0 * * * *' }
            : { type, target_id: options.targets[0]?.id ?? '', condition: 'unhealthy', for_seconds: 0 },
    });
  };

  const addStep = (type: Step['type']) =>
    patch({
      steps: [
        ...wf.steps,
        type === 'action'
          ? { id: createId(), type, name: '', action_type: '', target_id: '' }
          : { id: createId(), type, name: '', channel: '', message: '' },
      ],
    });

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    const errs = validateWorkflow(wf, options);
    setErrors(errs);
    if (Object.keys(errs).length === 0) onSave(wf);
  };

  const channelSelect = (value: string | undefined, onChange: (v: string) => void, htmlId: string, allowNone: boolean) => (
    <select id={htmlId} className="ox-input" value={value ?? ''} onChange={(e) => onChange(e.target.value)}>
      <option value="">{allowNone ? 'None' : 'Select…'}</option>
      {options.channels.map((c) => <option key={c.value} value={c.value}>{c.label}</option>)}
    </select>
  );

  return (
    <form onSubmit={submit} noValidate aria-label="Workflow editor" style={{ display: 'grid', gap: 'var(--ox-space-4)' }}>
      <div className="ox-row">
        <Field label="Name" htmlFor={id('name')} error={errors.name}>
          <input id={id('name')} className="ox-input" value={wf.name} onChange={(e) => patch({ name: e.target.value })} />
        </Field>
        <Field label="Description" htmlFor={id('desc')}>
          <input id={id('desc')} className="ox-input" value={wf.description ?? ''} onChange={(e) => patch({ description: e.target.value })} />
        </Field>
      </div>

      <fieldset className="ox-card">
        <legend>Trigger</legend>
        <div className="ox-row">
          <Field label="Trigger type" htmlFor={id('ttype')}>
            <select id={id('ttype')} className="ox-input" value={wf.trigger.type} onChange={(e) => setTriggerType(e.target.value as Trigger['type'])}>
              <option value="manual">Manual</option>
              <option value="cron">Cron schedule</option>
              <option value="health_condition">Health condition</option>
            </select>
          </Field>
          {wf.trigger.type === 'cron' && (
            <Field label="Cron expression" htmlFor={id('cron')} error={errors.trigger}>
              <input id={id('cron')} className="ox-input ox-mono" value={wf.trigger.expression} onChange={(e) => patch({ trigger: { type: 'cron', expression: e.target.value } })} />
            </Field>
          )}
          {wf.trigger.type === 'health_condition' && (() => {
            const t = wf.trigger;
            return (
              <>
                <Field label="Watch target" htmlFor={id('ttarget')} error={errors.trigger}>
                  <select id={id('ttarget')} className="ox-input" value={t.target_id} onChange={(e) => patch({ trigger: { ...t, target_id: e.target.value } })}>
                    <option value="">Select…</option>
                    {options.targets.map((x) => <option key={x.id} value={x.id}>{x.label}</option>)}
                  </select>
                </Field>
                <Field label="When it is" htmlFor={id('tcond')}>
                  <select id={id('tcond')} className="ox-input" value={t.condition} onChange={(e) => patch({ trigger: { ...t, condition: e.target.value as typeof t.condition } })}>
                    <option value="unhealthy">Unhealthy</option>
                    <option value="degraded">Degraded</option>
                    <option value="stale">Stale</option>
                  </select>
                </Field>
                <Field label="For (seconds)" htmlFor={id('tfor')}>
                  <input id={id('tfor')} type="number" min={0} className="ox-input" value={t.for_seconds} onChange={(e) => patch({ trigger: { ...t, for_seconds: Number(e.target.value) } })} />
                </Field>
              </>
            );
          })()}
        </div>
      </fieldset>

      <fieldset className="ox-card">
        <legend>Steps</legend>
        {errors.steps ? <p className="ox-field__error" role="alert">{errors.steps}</p> : null}
        <ol className="ox-steps">
          {wf.steps.map((s, i) => (
            <li key={s.id} className="ox-card" aria-label={`Step ${i + 1}`}>
              <div className="ox-card__head">
                <strong>Step {i + 1} · {s.type === 'action' ? 'Action' : 'Notify'}</strong>
                <span style={{ display: 'inline-flex', gap: 4 }}>
                  <button type="button" className="ox-btn" aria-label={`Move step ${i + 1} up`} disabled={i === 0} onClick={() => move(i, -1)} style={{ padding: 6 }}><ArrowUp size={14} aria-hidden="true" /></button>
                  <button type="button" className="ox-btn" aria-label={`Move step ${i + 1} down`} disabled={i === wf.steps.length - 1} onClick={() => move(i, 1)} style={{ padding: 6 }}><ArrowDown size={14} aria-hidden="true" /></button>
                  <button type="button" className="ox-btn ox-btn--danger" aria-label={`Remove step ${i + 1}`} onClick={() => patch({ steps: wf.steps.filter((_, j) => j !== i) })} style={{ padding: 6 }}><Trash2 size={14} aria-hidden="true" /></button>
                </span>
              </div>
              <div className="ox-row">
                <Field label={`Step ${i + 1} name`} htmlFor={id(`s${i}n`)} error={errors[`step.${i}.name`]}>
                  <input id={id(`s${i}n`)} className="ox-input" value={s.name} onChange={(e) => setStep(i, { ...s, name: e.target.value })} />
                </Field>
                {s.type === 'action' ? (
                  <>
                    <Field label={`Step ${i + 1} action`} htmlFor={id(`s${i}a`)} error={errors[`step.${i}.action`]}>
                      <select id={id(`s${i}a`)} className="ox-input" value={s.action_type} onChange={(e) => setStep(i, { ...s, action_type: e.target.value })}>
                        <option value="">Select…</option>
                        {options.actionTypes.map((a) => <option key={a.value} value={a.value}>{a.label}{a.dangerous ? ' (destructive)' : ''}</option>)}
                      </select>
                    </Field>
                    <Field label={`Step ${i + 1} target`} htmlFor={id(`s${i}t`)} error={errors[`step.${i}.target`]}>
                      <select id={id(`s${i}t`)} className="ox-input" value={s.target_id} onChange={(e) => setStep(i, { ...s, target_id: e.target.value })}>
                        <option value="">Select…</option>
                        {options.targets.map((t) => <option key={t.id} value={t.id}>{t.label}</option>)}
                      </select>
                    </Field>
                  </>
                ) : (
                  <>
                    <Field label={`Step ${i + 1} channel`} htmlFor={id(`s${i}c`)} error={errors[`step.${i}.channel`]}>
                      {channelSelect(s.channel, (v) => setStep(i, { ...s, channel: v }), id(`s${i}c`), false)}
                    </Field>
                    <Field label={`Step ${i + 1} message`} htmlFor={id(`s${i}m`)} error={errors[`step.${i}.message`]}>
                      <input id={id(`s${i}m`)} className="ox-input" value={s.message} onChange={(e) => setStep(i, { ...s, message: e.target.value })} />
                    </Field>
                  </>
                )}
              </div>
            </li>
          ))}
        </ol>
        <div className="ox-row" style={{ marginTop: 'var(--ox-space-3)' }}>
          <button type="button" className="ox-btn" onClick={() => addStep('action')}><Plus size={14} aria-hidden="true" /> Add action step</button>
          <button type="button" className="ox-btn" onClick={() => addStep('notify')}><Plus size={14} aria-hidden="true" /> Add notify step</button>
        </div>
      </fieldset>

      <fieldset className="ox-card">
        <legend>Outcome</legend>
        <div className="ox-row">
          <Field label="Notify on success" htmlFor={id('succ')} error={errors.on_success}>
            {channelSelect(wf.on_success.notify_channel, (v) => patch({ on_success: { notify_channel: v || undefined } }), id('succ'), true)}
          </Field>
          <Field label="On failure" htmlFor={id('fail')}>
            <select id={id('fail')} className="ox-input" value={wf.on_failure.behavior} onChange={(e) => patch({ on_failure: { ...wf.on_failure, behavior: e.target.value as 'stop' | 'continue' } })}>
              <option value="stop">Stop the run</option>
              <option value="continue">Continue with remaining steps</option>
            </select>
          </Field>
          <Field label="Notify on failure" htmlFor={id('failn')} error={errors.on_failure}>
            {channelSelect(wf.on_failure.notify_channel, (v) => patch({ on_failure: { ...wf.on_failure, notify_channel: v || undefined } }), id('failn'), true)}
          </Field>
        </div>
      </fieldset>

      <section aria-label="Preview">
        <h3 className="ox-card__title">Preview</h3>
        <WorkflowDiagram workflow={wf} options={options} />
      </section>

      <div className="ox-row" style={{ justifyContent: 'flex-end' }}>
        {onCancel ? <button type="button" className="ox-btn" onClick={onCancel}>Cancel</button> : null}
        <button type="submit" className="ox-btn ox-btn--primary">Save workflow</button>
      </div>
    </form>
  );
}
