import { useState } from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { CommandPalette, ConfirmDangerDialog, DataTable, ErrorState, MetricBar, StatusChip, filterCommands, type PaletteCommand } from './index';

describe('MetricBar', () => {
  it('shows "Not reported" instead of inventing a value', () => {
    render(<MetricBar label="Temperature" value={undefined} />);
    expect(screen.getByText('Not reported')).toBeInTheDocument();
    expect(screen.queryByRole('progressbar')).toBeNull();
  });
  it('exposes value to assistive tech and clamps', () => {
    render(<MetricBar label="CPU" value={140} />);
    expect(screen.getByRole('progressbar', { name: 'CPU' })).toHaveAttribute('aria-valuenow', '100');
  });
});

describe('StatusChip', () => {
  it('always renders text, not just colour', () => {
    render(<StatusChip status="stale" />);
    expect(screen.getByText('Stale')).toBeInTheDocument();
  });
});

describe('DataTable', () => {
  const rows = [{ id: 'a', n: 'Alpha' }, { id: 'b', n: 'Beta' }];
  const columns = [{ key: 'n', header: 'Name', render: (r: (typeof rows)[number]) => r.n }];
  it('renders a semantic table and supports keyboard selection', async () => {
    const onRowSelect = vi.fn();
    render(<DataTable caption="Things" columns={columns} rows={rows} rowKey={(r) => r.id} onRowSelect={onRowSelect} selectedKey="a" />);
    expect(screen.getByRole('columnheader', { name: 'Name' })).toBeInTheDocument();
    const row = screen.getByRole('row', { name: /Beta/ });
    row.focus();
    await userEvent.keyboard('{Enter}');
    expect(onRowSelect).toHaveBeenCalledWith(rows[1]);
    expect(screen.getByRole('row', { name: /Alpha/ })).toHaveAttribute('aria-selected', 'true');
  });
  it('shows an empty state', () => {
    render(<DataTable caption="Things" columns={columns} rows={[]} rowKey={(r: { id: string }) => r.id} emptyTitle="None yet" />);
    expect(screen.getByText('None yet')).toBeInTheDocument();
  });
});

describe('ErrorState', () => {
  it('states what failed, staleness and offers retry', async () => {
    const retry = vi.fn();
    render(<ErrorState message="Could not load nodes." stale onRetry={retry} />);
    expect(screen.getByRole('alert')).toHaveTextContent('Showing last known data');
    await userEvent.click(screen.getByRole('button', { name: 'Retry' }));
    expect(retry).toHaveBeenCalled();
  });
});

function DialogHarness({ onConfirm, requireText }: { onConfirm: () => void; requireText?: string }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button onClick={() => setOpen(true)}>Open</button>
      <ConfirmDangerDialog open={open} title="Drain?" description="Sure?" confirmLabel="Do it" requireText={requireText} onConfirm={onConfirm} onCancel={() => setOpen(false)} />
    </>
  );
}

describe('ConfirmDangerDialog', () => {
  it('starts on Cancel, traps focus, closes on Escape and returns focus', async () => {
    render(<DialogHarness onConfirm={vi.fn()} />);
    const opener = screen.getByRole('button', { name: 'Open' });
    await userEvent.click(opener);
    expect(screen.getByRole('alertdialog', { name: 'Drain?' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Cancel' })).toHaveFocus();
    await userEvent.tab();
    expect(screen.getByRole('button', { name: 'Do it' })).toHaveFocus();
    await userEvent.tab();
    expect(screen.getByRole('button', { name: 'Cancel' })).toHaveFocus(); // wrapped, did not escape
    await userEvent.keyboard('{Escape}');
    expect(screen.queryByRole('alertdialog')).toBeNull();
    expect(opener).toHaveFocus();
  });

  it('does not confirm without the required text', async () => {
    const onConfirm = vi.fn();
    render(<DialogHarness onConfirm={onConfirm} requireText="forge" />);
    await userEvent.click(screen.getByRole('button', { name: 'Open' }));
    const confirm = screen.getByRole('button', { name: 'Do it' });
    expect(confirm).toBeDisabled();
    await userEvent.type(screen.getByLabelText(/Type forge to confirm/), 'forge');
    expect(confirm).toBeEnabled();
    await userEvent.click(confirm);
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });
});

describe('CommandPalette', () => {
  const commands: PaletteCommand[] = [
    { id: 'a', label: 'Go to Compute' },
    { id: 'b', label: 'Run backup', keywords: ['snapshot'] },
    { id: 'c', label: 'Drain node', danger: true },
  ];
  it('filters deterministically across label and keywords', () => {
    expect(filterCommands(commands, 'snap').map((c) => c.id)).toEqual(['b']);
    expect(filterCommands(commands, 'go comp').map((c) => c.id)).toEqual(['a']);
    expect(filterCommands(commands, '')).toHaveLength(3);
  });
  it('navigates with arrows, selects with Enter, closes with Escape', async () => {
    const onSelect = vi.fn();
    const onClose = vi.fn();
    render(<CommandPalette open commands={commands} onSelect={onSelect} onClose={onClose} />);
    expect(screen.getByRole('combobox')).toHaveFocus();
    await userEvent.keyboard('{ArrowDown}{Enter}');
    expect(onSelect).toHaveBeenCalledWith(commands[1]);
    await userEvent.keyboard('{Escape}');
    expect(onClose).toHaveBeenCalled();
  });
  it('shows an empty message when nothing matches', async () => {
    render(<CommandPalette open commands={commands} onSelect={vi.fn()} onClose={vi.fn()} />);
    await userEvent.type(screen.getByRole('combobox'), 'zzz');
    expect(screen.getByText('No matching commands')).toBeInTheDocument();
  });
});
