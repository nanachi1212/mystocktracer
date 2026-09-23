const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { pathToFileURL } = require('node:url');
const { app, BrowserWindow, ipcMain, session, shell, dialog } = require('electron');
const { autoUpdater } = require('electron-updater');
const identity = require('./identity.cjs');
const { selectUserData } = require('./user-data-migration.cjs');
const { resolveHermesRuntimeRoot, runtimePython } = require('./hermes-runtime-root.cjs');
const { createRotatingLogger } = require('./runtime-logger.cjs');
const { resolveBackendCommand, startBackend, findFreePort, waitForHealth, stopBackend } = require('./backend-process.cjs');
const { UpdateManager } = require('./update-manager.cjs');
const { resolveUpdateFeed, releasePageURL } = require('./update-feed.cjs');
const { createUpdateBackup, resolveBackupRoot } = require('./data-protection.cjs');
const { validateSubscriptionAIURL } = require('./subscription-ai-url.cjs');
const { externalWindowHandler } = require('./external-links.cjs');
const { completeShutdown } = require('./shutdown.cjs');
const { scheduleUpdateRestart, resumeUpdateAfterRestart, awaitNativeInstall } = require('./update-restart.cjs');

const compat = (name) => process.env[`MYSTOCKTRACER_${name}`] || process.env[`A_STOCK_${name}`] || '';

async function startDesktop() {
  app.setName(identity.name);
  if (process.platform === 'win32') app.setAppUserModelId(identity.appId);
  let child, logger, window, quitting = false, stopped, timer;
  const resources = app.isPackaged ? path.join(process.resourcesPath, 'resources') : path.join(__dirname, 'resources');
  const runtime = resolveHermesRuntimeRoot({ configuredRoot: compat('HERMES_RUNTIME_ROOT'), bundledRoot: resources, projectRoot: path.resolve(__dirname, '..') });
  const python = runtime ? runtimePython(runtime) : 'python';
  const copyHelper = app.isPackaged ? path.join(process.resourcesPath, 'state-copy.py') : path.join(__dirname, 'state-copy.py');
  const stop = () => stopped ||= (async () => {
    clearInterval(timer);
    try {
      if (app.isReady() && window) await session.defaultSession.flushStorageData();
    } finally {
      await stopBackend(child);
      child = undefined;
    }
  })();
  const finishQuit = () => completeShutdown({ stop, report: (error) => logger?.error(error), quit: () => { quitting = true; app.quit(); } });
  app.on('before-quit', (event) => {
    if (quitting) return;
    event.preventDefault();
    void finishQuit().catch((error) => console.error(error));
  });
  app.on('window-all-closed', () => app.quit());
  try {
    const data = selectUserData({ appDataPath: app.getPath('appData'), python, helper: copyHelper });
    app.setPath('userData', data.path);
    // Lock only after migration. A concurrent first launch fails destination activation
    // without overwrite; an existing profile uses Electron's single-instance lock.
    if (!app.requestSingleInstanceLock()) { quitting = true; app.quit(); return; }
    app.on('second-instance', () => { window?.restore(); window?.focus(); });
    const logDirectory = path.resolve(compat('LOG_DIR') || path.join(data.path, 'logs'));
    app.setAppLogsPath(logDirectory);
    logger = createRotatingLogger({ directory: logDirectory, fileName: 'desktop.log', component: 'desktop', mirror: console });
    const rendererLog = createRotatingLogger({ directory: logDirectory, fileName: 'renderer.log', component: 'renderer' });
    logger.info(`startup version=${app.getVersion()} data=${data.mode}`);
    process.on('uncaughtExceptionMonitor', (error) => logger.error(error));
    process.on('unhandledRejection', (error) => logger.error(error));
    await app.whenReady();

    const enabled = app.isPackaged && ['win32', 'darwin'].includes(process.platform);
    autoUpdater.logger = logger;
    autoUpdater.autoDownload = false;
    autoUpdater.autoInstallOnAppQuit = false;
    if (enabled) autoUpdater.setFeedURL(resolveUpdateFeed());
    const requestPath = path.join(resolveBackupRoot(data.path), 'pending-install.json');
    if (enabled && process.platform === 'win32' && fs.existsSync(requestPath)) {
      try {
        await resumeUpdateAfterRestart({ requestPath, currentVersion: app.getVersion(), updater: autoUpdater, createBackup: (versions) => createUpdateBackup({ userDataPath: data.path, python, helper: copyHelper, ...versions }), install: () => awaitNativeInstall({ updater: autoUpdater, app }) });
        return;
      } catch (error) {
        logger.error(error);
        dialog.showErrorBox('更新未安裝', '無法驗證發布版本或建立完整備份。原資料已保留，應用程式將正常啟動；請連線後重新檢查更新。');
      }
    }

    const backendDir = app.isPackaged ? path.join(resources, 'backend') : path.resolve(__dirname, '..', 'backend');
    const candidates = [compat('BACKEND_BIN'), path.join(app.isPackaged ? backendDir : path.join(__dirname, 'bin'), identity.backendName())];
    const backendBin = candidates.find((candidate) => candidate && fs.existsSync(candidate)) || '';
    const port = await findFreePort();
    const token = crypto.randomBytes(32).toString('hex');
    const backendUrl = `http://127.0.0.1:${port}`;
    const home = compat('HERMES_HOME') || path.join(data.path, 'hermes-home');
    const workspace = compat('HERMES_WORKDIR') || path.join(data.path, 'hermes-workspace');
    for (const directory of [home, workspace]) fs.mkdirSync(directory, { recursive: true, mode: 0o700 });
    const browserRoot = app.isPackaged ? path.join(resources, 'agent-browser') : path.join(__dirname, 'scripts', 'browser-bin');
    const browserBinary = app.isPackaged ? path.join(browserRoot, process.platform === 'win32' ? 'agent-browser-real.exe' : 'agent-browser-real') : path.resolve(__dirname, '..', 'node_modules', 'agent-browser', 'bin', process.platform === 'win32' ? 'agent-browser-win32-x64.exe' : `agent-browser-${process.platform}-${process.arch}`);
    child = startBackend({ ...resolveBackendCommand({ backendBin, backendDir, isPackaged: app.isPackaged }), addr: `127.0.0.1:${port}`, token, extraEnv: {
      MYSTOCKTRACER_LOG_DIR: logDirectory, MYSTOCKTRACER_APP_VERSION: app.getVersion(),
      MYSTOCKTRACER_SETTINGS_PATH: path.join(data.path, 'settings.json'),
      MYSTOCKTRACER_TAIWAN_WATCHLIST_DB: path.join(data.path, 'taiwan-watchlist.db'),
      MYSTOCKTRACER_TAIWAN_PORTFOLIO_DB: path.join(data.path, 'taiwan-portfolio.db'),
      MYSTOCKTRACER_CASHFLOW_CACHE: path.join(data.path, 'cashflow-cache'),
      MYSTOCKTRACER_HERMES_HOME: home, MYSTOCKTRACER_HERMES_WORKDIR: workspace,
      MYSTOCKTRACER_HERMES_RUNTIME_ROOT: runtime,
      MYSTOCKTRACER_AGENT_BROWSER_WRAPPER_DIR: browserRoot, MYSTOCKTRACER_AGENT_BROWSER_REAL: browserBinary,
      ...(compat('HERMES_PYTHON') ? { MYSTOCKTRACER_HERMES_PYTHON: compat('HERMES_PYTHON') } : {}),
    } });
    child.stdout.on('data', (chunk) => logger.event('info', 'backend', String(chunk)));
    child.stderr.on('data', (chunk) => logger.event('warn', 'backend', String(chunk)));
    child.on('error', (error) => logger.error(error));
    await waitForHealth(backendUrl, 20000, child);
    logger.info('backend ready');
    const updates = new UpdateManager({ updater: autoUpdater, enabled, platform: process.platform, currentVersion: app.getVersion(), stopRuntime: stop, createBackup: (versions) => createUpdateBackup({ userDataPath: data.path, python, helper: copyHelper, ...versions }), restartForInstall: process.platform === 'win32' ? (versions) => scheduleUpdateRestart({ requestPath, versions, app }) : undefined, logger });
    const frontendFile = app.isPackaged ? path.join(resources, 'frontend', 'dist', 'index.html') : path.resolve(__dirname, '..', 'frontend', 'dist', 'index.html');
    const frontendURL = !app.isPackaged && process.env.ELECTRON_RENDERER_URL ? process.env.ELECTRON_RENDERER_URL : pathToFileURL(frontendFile).href;
    const page = new URL(frontendURL);
    if (page.protocol !== 'file:' && !(page.protocol === 'http:' && ['localhost', '127.0.0.1'].includes(page.hostname))) throw new Error('Renderer must be bundled or local development content');
    window = new BrowserWindow({ width: 1280, height: 860, minWidth: 980, minHeight: 680, title: 'mystocktracer · 台股研究工作台', icon: path.join(__dirname, 'assets', 'mystocktracer.png'), webPreferences: { preload: path.join(__dirname, 'preload.cjs'), contextIsolation: true, nodeIntegration: false, webviewTag: false, sandbox: true } });
    window.webContents.setWindowOpenHandler(externalWindowHandler((url) => shell.openExternal(url), (error) => logger.warn(error)));
    window.webContents.on('will-attach-webview', (event) => event.preventDefault());
    window.webContents.on('will-navigate', (event, url) => { if (url !== frontendURL) event.preventDefault(); });
    const openDirectory = async (directory) => { fs.mkdirSync(directory, { recursive: true, mode: 0o700 }); if (await shell.openPath(directory)) throw new Error('無法開啟目錄'); };
    let logWindow = 0, logCount = 0;
    const actions = {
      'backend-config': () => ({ backendUrl, token }),
      'runtime-log-status': () => ({ available: true, directory: logDirectory, max_file_mb: 5, backup_files: 5 }),
      'runtime-open-logs': () => openDirectory(logDirectory),
      'runtime-log': (entry) => {
        if (Date.now() - logWindow >= 1000) { logWindow = Date.now(); logCount = 0; }
        if (++logCount > 50 || !entry || typeof entry.message !== 'string') return false;
        rendererLog.event(['debug', 'info', 'warn', 'error'].includes(entry.level) ? entry.level : 'info', String(entry.feature || 'renderer').slice(0, 80), entry.message.slice(0, 16384));
        return true;
      },
      'app-update-status': () => updates.getStatus(), 'app-update-check': () => updates.checkForUpdates(),
      'app-update-download': () => updates.downloadUpdate(), 'app-update-install': () => updates.installUpdate(),
      'app-update-open-release': () => shell.openExternal(releasePageURL(updates.getStatus().latestVersion || app.getVersion())),
      'app-update-open-backups': () => openDirectory(resolveBackupRoot(data.path)),
      'open-subscription-ai': (url) => { validateSubscriptionAIURL(url); return shell.openExternal(url); },
    };
    for (const [channel, action] of Object.entries(actions)) ipcMain.handle(channel, (event, ...args) => {
      if (event.sender !== window.webContents || event.senderFrame !== window.webContents.mainFrame || event.senderFrame.url.split('#')[0] !== frontendURL.split('#')[0]) throw new Error('Untrusted IPC sender');
      return action(...args);
    });
    updates.on('status', (value) => { if (!window.isDestroyed()) window.webContents.send('app-update-status-changed', value); });
    await window.loadURL(frontendURL);
    logger.info('frontend ready');
    if (enabled) {
      const check = () => updates.checkForUpdates().catch((error) => logger.warn(error));
      const initial = setTimeout(check, 30000); initial.unref();
      timer = setInterval(check, 12 * 60 * 60 * 1000); timer.unref();
    }
  } catch (error) {
    logger?.error(error);
    dialog.showErrorBox('mystocktracer 無法啟動', '資料未被刪除。請關閉舊版本、確認資料目錄與套件完整性後再試。');
    await finishQuit();
  }
}
module.exports = { startDesktop };
