import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { TopologyView } from './TopologyView';
import type { NetworkGraph } from '../../lib/api/types';

const health = (state: 'healthy' | 'unhealthy') => ({ state, observed_at: '2026-01-01T00:00:00Z' });
const graph: NetworkGraph = {
  nodes: [
    { id: 'a', kind: 'gateway', label: 'Gateway', health: health('healthy') },
    { id: 'b', kind: 'compute', label: 'Worker', health: health('unhealthy') },
    { id: 'c', kind: 'device', label: 'Laptop' },
  ],
  edges: [{ id: 'e', source: 'a', target: 'b', kind: 'tunnel' }],
};

describe('TopologyView', () => {
  it('selects nodes by click and keyboard, marking the selection', async () => {
    const onSelect = vi.fn();
    const { rerender } = render(<TopologyView graph={graph} onSelect={onSelect} />);
    const node = screen.getByRole('button', { name: /^Worker, compute, unhealthy$/ });
    await userEvent.click(node);
    expect(onSelect).toHaveBeenCalledWith('b');
    node.focus();
    await userEvent.keyboard('{Enter}');
    expect(onSelect).toHaveBeenCalledTimes(2);
    rerender(<TopologyView graph={graph} selectedId="b" onSelect={onSelect} />);
    expect(screen.getByRole('button', { name: /^Worker, compute/ })).toHaveAttribute('aria-pressed', 'true');
  });

  it('reports missing health as unknown and renders a text fallback with connections', () => {
    render(<TopologyView graph={graph} />);
    expect(screen.getByRole('button', { name: /^Laptop, device, unknown$/ })).toBeInTheDocument();
    expect(screen.getByText(/to Worker \(tunnel\)/)).toBeInTheDocument();
    expect(screen.getByText('no connections')).toBeInTheDocument();
  });

  it('zooms and resets', async () => {
    render(<TopologyView graph={graph} />);
    expect(screen.getByText('Fit')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Zoom in' }));
    expect(screen.getByText('125%')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Reset' }));
    expect(screen.getByText('100%')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Zoom to fit' }));
    expect(screen.getByText('Fit')).toBeInTheDocument();
  });

  it('renders an empty state', () => {
    render(<TopologyView graph={{ nodes: [], edges: [] }} />);
    expect(screen.getByText('No nodes in the network graph.')).toBeInTheDocument();
  });
});
