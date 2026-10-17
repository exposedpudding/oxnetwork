import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { WorkflowEditor } from './WorkflowEditor';
import { WorkflowDiagram } from './WorkflowDiagram';
import { validateCron, validateWorkflow } from './validate';
import type { EditorOptions, Workflow } from './types';

const options: EditorOptions = {
  actionTypes: [{ value: 'wake', label: 'Wake' }, { value: 'drain', label: 'Drain', dangerous: true }],
  targets: [{ id: 't1', label: 'Target One' }],
  channels: [{ value: 'ops', label: 'Ops channel' }],
};
const blank: Workflow = {
  id: 'w1', name: '', enabled: true, trigger: { type: 'manual' }, steps: [],
  on_success: {}, on_failure: { behavior: 'stop' },
};
let n = 0;
const createId = () => `id-${++n}`;

describe('validateCron', () => {
  it.each(['* * * * *', '*/15 0-6 1,15 * 1-5', '0 2 * * *'])('accepts %s', (c) => expect(validateCron(c)).toBeUndefined());
  it.each(['* * * *', '61 * * * *', '* 24 * * *', '*/0 * * * *', 'a b c d e', '5-1 * * * *'])('rejects %s', (c) => expect(validateCron(c)).toBeDefined());
});

describe('validateWorkflow', () => {
  it('requires name, steps and valid references', () => {
    const e = validateWorkflow({ ...blank, steps: [{ id: 'a', type: 'action', name: '', action_type: 'x', target_id: 'zz' }] }, options);
    expect(Object.keys(e).sort()).toEqual(['name', 'step.0.action', 'step.0.name', 'step.0.target']);
    expect(validateWorkflow(blank, options).steps).toBeDefined();
  });
});

describe('WorkflowEditor', () => {
  it('blocks saving an invalid workflow and shows errors', async () => {
    const onSave = vi.fn();
    render(<WorkflowEditor initial={blank} options={options} onSave={onSave} createId={createId} />);
    await userEvent.click(screen.getByRole('button', { name: 'Save workflow' }));
    expect(onSave).not.toHaveBeenCalled();
    expect(screen.getByText('Name is required.')).toBeInTheDocument();
    expect(screen.getByText('Add at least one step.')).toBeInTheDocument();
  });

  it('builds a cron workflow with action + notify steps, reorders, and saves', async () => {
    const onSave = vi.fn();
    render(<WorkflowEditor initial={blank} options={options} onSave={onSave} createId={createId} />);
    await userEvent.type(screen.getByLabelText('Name'), 'Nightly');
    await userEvent.selectOptions(screen.getByLabelText('Trigger type'), 'cron');
    const cron = screen.getByLabelText('Cron expression');
    await userEvent.clear(cron);
    await userEvent.type(cron, '0 2 * * *');

    await userEvent.click(screen.getByRole('button', { name: /Add notify step/ }));
    await userEvent.click(screen.getByRole('button', { name: /Add action step/ }));
    // action step is #2 -> move it up so it is #1
    await userEvent.click(screen.getByRole('button', { name: 'Move step 2 up' }));
    const s1 = screen.getByRole('listitem', { name: 'Step 1' });
    await userEvent.type(within(s1).getByLabelText('Step 1 name'), 'Wake it');
    await userEvent.selectOptions(within(s1).getByLabelText('Step 1 action'), 'wake');
    await userEvent.selectOptions(within(s1).getByLabelText('Step 1 target'), 't1');
    const s2 = screen.getByRole('listitem', { name: 'Step 2' });
    await userEvent.type(within(s2).getByLabelText('Step 2 name'), 'Tell ops');
    await userEvent.selectOptions(within(s2).getByLabelText('Step 2 channel'), 'ops');
    await userEvent.type(within(s2).getByLabelText('Step 2 message'), 'done');
    await userEvent.selectOptions(screen.getByLabelText('Notify on failure'), 'ops');
    await userEvent.selectOptions(screen.getByLabelText('On failure'), 'continue');

    await userEvent.click(screen.getByRole('button', { name: 'Save workflow' }));
    const saved = onSave.mock.calls[0][0] as Workflow;
    expect(saved.trigger).toEqual({ type: 'cron', expression: '0 2 * * *' });
    expect(saved.steps.map((s) => [s.type, s.name])).toEqual([['action', 'Wake it'], ['notify', 'Tell ops']]);
    expect(saved.on_failure).toEqual({ behavior: 'continue', notify_channel: 'ops' });
  });

  it('supports health-condition triggers and step removal', async () => {
    render(<WorkflowEditor initial={blank} options={options} onSave={vi.fn()} createId={createId} />);
    await userEvent.selectOptions(screen.getByLabelText('Trigger type'), 'health_condition');
    expect(screen.getByLabelText('Watch target')).toHaveValue('t1');
    await userEvent.click(screen.getByRole('button', { name: /Add action step/ }));
    await userEvent.click(screen.getByRole('button', { name: 'Remove step 1' }));
    expect(screen.queryByRole('listitem', { name: 'Step 1' })).toBeNull();
  });
});

describe('WorkflowDiagram', () => {
  it('renders trigger -> steps -> outcome in order', () => {
    const wf: Workflow = {
      ...blank, name: 'Demo', trigger: { type: 'cron', expression: '0 2 * * *' },
      steps: [{ id: 'a', type: 'action', name: 'Wake it', action_type: 'wake', target_id: 't1' }],
      on_failure: { behavior: 'stop', notify_channel: 'ops' },
    };
    render(<WorkflowDiagram workflow={wf} options={options} />);
    const items = within(screen.getByRole('list', { name: 'Demo workflow' })).getAllByRole('listitem');
    expect(items.map((i) => i.textContent)).toEqual([
      expect.stringContaining('Cron 0 2 * * *'),
      expect.stringContaining('Wake on Target One'),
      expect.stringContaining('On failure: stop and notify Ops channel'),
    ]);
  });
});
