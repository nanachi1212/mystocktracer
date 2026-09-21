const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('aStock', {
  getBackendConfig: () => ipcRenderer.invoke('backend-config'),
	getRuntimeLogStatus: () => ipcRenderer.invoke('runtime-log-status'),
	openRuntimeLogs: () => ipcRenderer.invoke('runtime-open-logs'),
	logRuntimeEvent: (entry) => ipcRenderer.invoke('runtime-log', entry),
  getUpdateStatus: () => ipcRenderer.invoke('app-update-status'),
  checkForUpdates: () => ipcRenderer.invoke('app-update-check'),
  downloadUpdate: () => ipcRenderer.invoke('app-update-download'),
  installUpdate: () => ipcRenderer.invoke('app-update-install'),
  openUpdateRelease: () => ipcRenderer.invoke('app-update-open-release'),
  openUpdateBackups: () => ipcRenderer.invoke('app-update-open-backups'),
  onUpdateStatus: (listener) => {
    const handler = (_event, status) => listener(status);
    ipcRenderer.on('app-update-status-changed', handler);
    return () => ipcRenderer.removeListener('app-update-status-changed', handler);
  },
  openSubscriptionAI: (targetUrl) => ipcRenderer.invoke('open-subscription-ai', targetUrl),
});
