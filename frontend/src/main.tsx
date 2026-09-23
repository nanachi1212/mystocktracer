import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { App } from './App';
import { installRuntimeLogging } from './lib/runtime-log';
import './styles.css';

const container = document.getElementById('root');
if (!container) throw new Error('mystocktracer root element is missing');
installRuntimeLogging();
createRoot(container).render(<StrictMode><App /></StrictMode>);