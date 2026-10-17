import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import '../styles/index.css';
import { DevApp } from './index';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <DevApp />
  </StrictMode>,
);
