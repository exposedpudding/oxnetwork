import { useEffect, useState } from 'react';
import { DevLayout } from './layout';
import { AutomationsPage, ComponentsPage, ComputePage, NetworkPage, NodeDetailPage, OverviewPage } from './pages';
import { ErrorState } from '../components';

const currentPath = () => window.location.hash.replace(/^#/, '') || '/dev/overview';

/** Hash-routed dev harness (no router dependency). Loaded only when import.meta.env.DEV. */
export function DevApp() {
  const [path, setPath] = useState(currentPath);
  useEffect(() => {
    const h = () => setPath(currentPath());
    window.addEventListener('hashchange', h);
    return () => window.removeEventListener('hashchange', h);
  }, []);

  let title = 'Not found';
  let body = <ErrorState title="Unknown dev route" message={path} />;
  if (path === '/dev/overview') (title = 'Overview', body = <OverviewPage />);
  else if (path === '/dev/compute') (title = 'Compute', body = <ComputePage />);
  else if (path === '/dev/network') (title = 'Network', body = <NetworkPage />);
  else if (path === '/dev/automations') (title = 'Automations', body = <AutomationsPage />);
  else if (path === '/dev/components') (title = 'Components', body = <ComponentsPage />);
  else if (path.startsWith('/dev/node/')) (title = 'Node detail', body = <NodeDetailPage nodeId={path.slice('/dev/node/'.length)} />);
  return <DevLayout path={path} title={title}>{body}</DevLayout>;
}
