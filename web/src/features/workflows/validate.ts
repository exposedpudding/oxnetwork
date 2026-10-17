import type { Workflow, WorkflowErrors, EditorOptions } from './types';

const FIELD_RANGES: [number, number][] = [
  [0, 59],
  [0, 23],
  [1, 31],
  [1, 12],
  [0, 7],
];

/** Validates a standard 5-field cron expression (numbers, *, ranges, lists, steps). */
export function validateCron(expr: string): string | undefined {
  const parts = expr.trim().split(/\s+/);
  if (parts.length !== 5) return 'Use 5 fields: minute hour day-of-month month day-of-week.';
  for (let i = 0; i < 5; i++) {
    const [lo, hi] = FIELD_RANGES[i];
    for (const item of parts[i].split(',')) {
      const m = /^(\*|\d+(?:-\d+)?)(?:\/(\d+))?$/.exec(item);
      if (!m) return `Invalid cron field "${parts[i]}".`;
      if (m[2] !== undefined && Number(m[2]) < 1) return 'Cron step must be at least 1.';
      if (m[1] !== '*') {
        const [a, b = a] = m[1].split('-').map(Number);
        if (a < lo || b > hi || a > b) return `Value out of range in "${parts[i]}".`;
      }
    }
  }
  return undefined;
}

export function validateWorkflow(w: Workflow, opts: EditorOptions): WorkflowErrors {
  const errors: WorkflowErrors = {};
  if (!w.name.trim()) errors.name = 'Name is required.';
  if (w.trigger.type === 'cron') {
    const e = validateCron(w.trigger.expression);
    if (e) errors.trigger = e;
  }
  if (w.trigger.type === 'health_condition') {
    if (!w.trigger.target_id) errors.trigger = 'Choose a target to watch.';
    else if (w.trigger.for_seconds < 0 || !Number.isFinite(w.trigger.for_seconds)) errors.trigger = 'Duration must be zero or more seconds.';
  }
  if (w.steps.length === 0) errors.steps = 'Add at least one step.';
  const actions = new Set(opts.actionTypes.map((a) => a.value));
  const targets = new Set(opts.targets.map((t) => t.id));
  const channels = new Set(opts.channels.map((c) => c.value));
  w.steps.forEach((s, i) => {
    if (!s.name.trim()) errors[`step.${i}.name`] = 'Step name is required.';
    if (s.type === 'action') {
      if (!actions.has(s.action_type)) errors[`step.${i}.action`] = 'Choose an action.';
      if (!targets.has(s.target_id)) errors[`step.${i}.target`] = 'Choose a target.';
    } else {
      if (!channels.has(s.channel)) errors[`step.${i}.channel`] = 'Choose a channel.';
      if (!s.message.trim()) errors[`step.${i}.message`] = 'Message is required.';
    }
  });
  for (const [key, ch] of [['on_success', w.on_success.notify_channel], ['on_failure', w.on_failure.notify_channel]] as const) {
    if (ch && !channels.has(ch)) errors[key] = 'Unknown notification channel.';
  }
  return errors;
}
