import { StrictMode, lazy, Suspense } from 'react';
import { createRoot } from 'react-dom/client';
import './styles/index.css';

// Dev-only harness; tree-shaken out of production builds. The production app shell is owned by Codex.
const DevApp = import.meta.env.DEV ? lazy(() => import('./dev').then((m) => ({ default: m.DevApp }))) : null;

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    {DevApp ? (
      <Suspense fallback={null}><DevApp /></Suspense>
    ) : (
      <main style={{ padding: 24 }}>
        <h1>oxnet.work</h1>
        <p>Frontend components are available in <code>src/components</code>; screens are integrated by the primary agent.</p>
      </main>
    )}
  </StrictMode>,
);
