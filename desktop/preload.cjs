const { contextBridge, ipcRenderer } = require('electron');
// Every function closes over one fixed channel; no generic IPC escapes isolation.
const operations = {
  getBackendConfig: 'backend-config', getRuntimeLogStatus: 'runtime-log-status',
  openRuntimeLogs: 'runtime-open-logs', logRuntimeEvent: 'runtime-log',
  getUpdateStatus: 'app-update-status', checkForUpdates: 'app-update-check',
  downloadUpdate: 'app-update-download', installUpdate: 'app-update-install',
  openUpdateRelease: 'app-update-open-release', openUpdateBackups: 'app-update-open-backups',
  openSubscriptionAI: 'open-subscription-ai',
};
const api = Object.fromEntries(Object.entries(operations).map(([name, channel]) => [name, (...args) => ipcRenderer.invoke(channel, ...args)]));
api.onUpdateStatus = (receive) => {
  if (typeof receive !== 'function') throw new TypeError('Status listener must be a function');
  const notify = (_event, status) => receive(status);
  ipcRenderer.on('app-update-status-changed', notify);
  return () => ipcRenderer.removeListener('app-update-status-changed', notify);
};
contextBridge.exposeInMainWorld('mystocktracer', Object.freeze(api));