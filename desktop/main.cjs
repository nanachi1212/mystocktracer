const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const { app, BrowserWindow, ipcMain, session, shell } = require('electron');
const { autoUpdater } = require('electron-updater');
const {
  findFreePort,
  resolveBackendCommand,
  startBackend,
  waitForHealth,
} = require('./backend-process.cjs');
const { resolveUserDataPath } = require('./user-data.cjs');
const { resolveHermesRuntimeRoot } = require('./hermes-runtime-root.cjs');
const { createUpdateBackup, resolveBackupRoot } = require('./data-protection.cjs');
const { UpdateManager } = require('./update-manager.cjs');
const { resolveUpdateFeed, releasePageURL } = require('./update-feed.cjs');
const { createRotatingLogger } = require('./runtime-logger.cjs');
const { validateSubscriptionAIURL } = require('./subscription-ai-url.cjs');

app.setName('easy-stock');
const defaultUserDataPath = app.getPath('userData');
const selectedUserDataPath = resolveUserDataPath({
  appDataPath: app.getPath('appData'),
  currentUserDataPath: defaultUserDataPath,
  configuredPath: process.env.A_STOCK_USER_DATA_DIR,
});
if (selectedUserDataPath !== defaultUserDataPath) {
  app.setPath('userData', selectedUserDataPath);
}

const runtimeLogDirectory = path.resolve(process.env.MYSTOCKTRACER_LOG_DIR || process.env.A_STOCK_LOG_DIR || path.join(app.getPath('userData'), 'logs'));
fs.mkdirSync(runtimeLogDirectory, { recursive: true, mode: 0o700 });
app.setAppLogsPath(runtimeLogDirectory);
const desktopLogger = createRotatingLogger({
  directory: runtimeLogDirectory,
  fileName: 'desktop.log',
  component: 'desktop',
  mirror: console,
});
const rendererLogger = createRotatingLogger({
  directory: runtimeLogDirectory,
  fileName: 'renderer.log',
  component: 'renderer',
  mirror: console,
});

desktopLogger.event('info', 'runtime', `started version=${app.getVersion()} packaged=${app.isPackaged} platform=${process.platform} arch=${process.arch}`);
process.on('uncaughtExceptionMonitor', (error, origin) => desktopLogger.event('error', 'runtime', `uncaught exception origin=${origin}`, error));
process.on('unhandledRejection', (reason) => desktopLogger.event('error', 'runtime', 'unhandled rejection', reason));

let backendProcess;
let backendConfig;
let updateManager;
let updateCheckTimer;

function featureLogger(feature) {
  return {
    debug: (...values) => desktopLogger.event('debug', feature, ...values),
    info: (...values) => desktopLogger.event('info', feature, ...values),
    log: (...values) => desktopLogger.event('info', feature, ...values),
    warn: (...values) => desktopLogger.event('warn', feature, ...values),
    error: (...values) => desktopLogger.event('error', feature, ...values),
  };
}

function logChildOutput(feature, level, chunk) {
  const message = String(chunk || '').trim();
  if (message) desktopLogger.event(level, feature, message);
}

function resourcesRoot() {
  return app.isPackaged
    ? path.join(process.resourcesPath, 'resources')
    : path.join(__dirname, 'resources');
}

async function bootBackend() {
	desktopLogger.event('info', 'backend-process', 'starting');
  const port = await findFreePort();
  const token = crypto.randomBytes(24).toString('hex');
  const addr = `127.0.0.1:${port}`;
  const backendUrl = `http://${addr}`;
  const developmentBackendDir = path.resolve(__dirname, '..', 'backend');
  const devBinary = path.join(__dirname, 'bin', 'easy-stock-backend');
  const bundledRoot = resourcesRoot();
  const executableName = process.platform === 'win32' ? 'easy-stock-backend.exe' : 'easy-stock-backend';
  const packagedBinary = path.join(bundledRoot, 'backend', executableName);
  const backendDir = app.isPackaged ? path.dirname(packagedBinary) : developmentBackendDir;
  const configuredBinary = process.env.A_STOCK_BACKEND_BIN || '';
  const backendBin = [configuredBinary, app.isPackaged ? packagedBinary : devBinary].find((candidate) => candidate && fs.existsSync(candidate)) || '';
  const command = resolveBackendCommand({
    backendBin,
    backendDir,
    isPackaged: app.isPackaged,
  });

  backendProcess = startBackend({
    ...command,
    addr,
    token,
    extraEnv: buildRuntimeEnv(bundledRoot),
  });
	backendProcess.stdout?.on('data', (chunk) => logChildOutput('backend-process', 'info', chunk));
	backendProcess.stderr?.on('data', (chunk) => logChildOutput('backend-process', 'warn', chunk));
	backendProcess.once('error', (error) => desktopLogger.event('error', 'backend-process', 'process error', error));
	backendProcess.once('exit', (code, signal) => desktopLogger.event(app.isQuitting ? 'info' : 'error', 'backend-process', `process exited code=${code ?? 'none'} signal=${signal || 'none'} quitting=${Boolean(app.isQuitting)}`));

  await waitForHealth(backendUrl);
  backendConfig = { backendUrl, token };
	desktopLogger.event('info', 'backend-process', 'ready');
  return backendConfig;
}

async function createWindow() {
  await bootBackend();
	const windowIcon = path.join(__dirname, 'assets', 'easy-stock.png');

  const window = new BrowserWindow({
    width: 1280,
    height: 860,
    minWidth: 980,
    minHeight: 680,
		title: 'mystocktracer · 台股研究工作台 · 僅限個人非商業使用',
		...(fs.existsSync(windowIcon) ? { icon: windowIcon } : {}),
    backgroundColor: '#f4f5f7',
    webPreferences: {
      preload: path.join(__dirname, 'preload.cjs'),
      contextIsolation: true,
      nodeIntegration: false,
      webviewTag: true,
    },
  });
  window.webContents.on('will-attach-webview', (event) => {
    event.preventDefault();
  });
	window.webContents.on('did-fail-load', (_event, errorCode, errorDescription, validatedURL, isMainFrame) => {
		if (isMainFrame) desktopLogger.event('error', 'renderer', `load failed code=${errorCode} description=${errorDescription} path=${safeURLPath(validatedURL)}`);
	});
	window.webContents.on('render-process-gone', (_event, details) => {
		desktopLogger.event('error', 'renderer', `process gone reason=${details.reason} exit_code=${details.exitCode}`);
	});

  const rendererURL = process.env.ELECTRON_RENDERER_URL;
  if (rendererURL) {
    await window.loadURL(rendererURL);
    return;
  }
  const frontendPath = app.isPackaged
    ? path.join(process.resourcesPath, 'resources', 'frontend', 'dist', 'index.html')
    : path.resolve(__dirname, '..', 'frontend', 'dist', 'index.html');
  await window.loadFile(frontendPath);
}

function terminateChild(child, timeoutMs = 10000) {
  if (!child || child.killed || child.exitCode !== null) return Promise.resolve();
  return new Promise((resolve, reject) => {
    let settled = false;
    let timer;
    let forceTimer;
    const finish = () => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      clearTimeout(forceTimer);
      resolve();
    };
    child.once('exit', finish);
    child.kill();
    timer = setTimeout(() => {
      if (child.exitCode !== null) return finish();
      try { child.kill('SIGKILL'); } catch {}
      forceTimer = setTimeout(() => {
        if (child.exitCode !== null) return finish();
        if (settled) return;
        settled = true;
        reject(new Error('本机后台服务未能及时停止'));
      }, 2000);
    }, timeoutMs);
  });
}

async function stopRuntime() {
  await Promise.all([session.defaultSession?.flushStorageData()].filter(Boolean));
  await Promise.all([
    terminateChild(backendProcess),
  ].filter(Boolean));
  backendProcess = undefined;
  backendConfig = undefined;
}

function initializeUpdateManager() {
  const enabled = app.isPackaged && ['darwin', 'win32'].includes(process.platform);
  if (enabled) {
    autoUpdater.setFeedURL(resolveUpdateFeed());
  }
  updateManager = new UpdateManager({
    updater: autoUpdater,
    enabled,
    currentVersion: app.getVersion(),
    platform: process.platform,
    canInstallAutomatically: process.platform === 'win32',
    stopRuntime,
    createBackup: ({ fromVersion, toVersion }) => createUpdateBackup({
      userDataPath: app.getPath('userData'),
      backupRoot: resolveBackupRoot(app.getPath('userData')),
      fromVersion,
      toVersion,
    }),
		logger: featureLogger('updater'),
  });
  updateManager.on('status', (status) => {
    for (const window of BrowserWindow.getAllWindows()) {
      if (!window.isDestroyed()) window.webContents.send('app-update-status-changed', status);
    }
  });
  if (enabled) {
    setTimeout(() => void updateManager.checkForUpdates().catch(() => {}), 30000).unref?.();
    updateCheckTimer = setInterval(() => void updateManager.checkForUpdates().catch(() => {}), 12 * 60 * 60 * 1000);
    updateCheckTimer.unref?.();
  }
}

function buildRuntimeEnv(resourcesRoot) {
  const userData = app.getPath('userData');
  const hermesHome = process.env.MYSTOCKTRACER_HERMES_HOME || process.env.A_STOCK_HERMES_HOME || path.join(userData, 'hermes-home');
  const hermesWorkDir = process.env.MYSTOCKTRACER_HERMES_WORKDIR || process.env.A_STOCK_HERMES_WORKDIR || path.join(userData, 'hermes-workspace');
  const bundledRuntime = path.join(resourcesRoot, 'hermes-runtime');
  const packagedBrowserWrapperDir = path.join(resourcesRoot, 'agent-browser');
  const developmentBrowserWrapperDir = path.join(__dirname, 'scripts', 'browser-bin');
  const browserWrapperDir = fs.existsSync(path.join(packagedBrowserWrapperDir, 'agent-browser'))
    ? packagedBrowserWrapperDir
    : developmentBrowserWrapperDir;
  const packagedBrowserReal = path.join(packagedBrowserWrapperDir, process.platform === 'win32' ? 'agent-browser-real.exe' : 'agent-browser-real');
  const developmentBrowserReal = path.resolve(__dirname, '..', 'node_modules', 'agent-browser', 'bin', agentBrowserBinaryName());
  const browserReal = fs.existsSync(packagedBrowserReal) ? packagedBrowserReal : developmentBrowserReal;
  const hermesRuntimeRoot = resolveHermesRuntimeRoot({
    configuredRoot: process.env.MYSTOCKTRACER_HERMES_RUNTIME_ROOT || process.env.A_STOCK_HERMES_RUNTIME_ROOT,
    bundledRoot: resourcesRoot,
    projectRoot: path.resolve(__dirname, '..'),
  });
  fs.mkdirSync(hermesHome, { recursive: true });
  fs.mkdirSync(hermesWorkDir, { recursive: true });
  return {
		MYSTOCKTRACER_LOG_DIR: runtimeLogDirectory,
		MYSTOCKTRACER_APP_VERSION: app.getVersion(),
    MYSTOCKTRACER_SETTINGS_PATH: path.join(userData, 'settings.json'),
    MYSTOCKTRACER_TAIWAN_WATCHLIST_DB: path.join(userData, 'taiwan-watchlist.db'),
    MYSTOCKTRACER_TAIWAN_PORTFOLIO_DB: path.join(userData, 'taiwan-portfolio.db'),
    MYSTOCKTRACER_HERMES_HOME: hermesHome,
    MYSTOCKTRACER_HERMES_WORKDIR: hermesWorkDir,
    MYSTOCKTRACER_HERMES_RUNTIME_ROOT: hermesRuntimeRoot,
    A_STOCK_AGENT_BROWSER_WRAPPER_DIR: browserWrapperDir,
    A_STOCK_AGENT_BROWSER_REAL: browserReal,
    ...(process.env.MYSTOCKTRACER_HERMES_PYTHON || process.env.A_STOCK_HERMES_PYTHON
      ? { MYSTOCKTRACER_HERMES_PYTHON: process.env.MYSTOCKTRACER_HERMES_PYTHON || process.env.A_STOCK_HERMES_PYTHON }
      : {}),
  };
}

function agentBrowserBinaryName() {
  if (process.platform === 'win32') return 'agent-browser-win32-x64.exe';
  return `agent-browser-${process.platform}-${process.arch}`;
}

ipcMain.handle('backend-config', () => backendConfig);
ipcMain.handle('runtime-log-status', () => ({
	available: true,
	directory: runtimeLogDirectory,
	max_file_mb: 5,
	backup_files: 5,
}));
ipcMain.handle('runtime-open-logs', async () => {
	fs.mkdirSync(runtimeLogDirectory, { recursive: true, mode: 0o700 });
	const error = await shell.openPath(runtimeLogDirectory);
	if (error) throw new Error(error);
	desktopLogger.event('info', 'runtime', 'opened log directory');
});
ipcMain.handle('runtime-log', (_event, entry = {}) => {
	const value = entry && typeof entry === 'object' ? entry : {};
	const level = ['debug', 'info', 'warn', 'error'].includes(value.level) ? value.level : 'info';
	const feature = String(value.feature || 'renderer').replace(/[^a-zA-Z0-9._-]/g, '').slice(0, 80) || 'renderer';
	const message = String(value.message || '').slice(0, 16 * 1024);
	rendererLogger.event(level, feature, message);
	return true;
});
ipcMain.handle('app-update-status', () => updateManager?.getStatus() || {
  state: 'disabled', supported: false, currentVersion: app.getVersion(), progress: 0, message: '自动更新尚未初始化',
});
ipcMain.handle('app-update-check', () => updateManager.checkForUpdates());
ipcMain.handle('app-update-download', () => updateManager.downloadUpdate());
ipcMain.handle('app-update-install', () => updateManager.installUpdate());
ipcMain.handle('app-update-open-release', () => shell.openExternal(releasePageURL(updateManager?.getStatus().latestVersion || app.getVersion())));
ipcMain.handle('app-update-open-backups', async () => {
  const backupRoot = resolveBackupRoot(app.getPath('userData'));
  fs.mkdirSync(backupRoot, { recursive: true, mode: 0o700 });
  const error = await shell.openPath(backupRoot);
  if (error) throw new Error(error);
});
ipcMain.handle('open-subscription-ai', async (_event, targetUrl) => {
  validateSubscriptionAIURL(targetUrl);
  return shell.openExternal(targetUrl);
});

function safeURLPath(value) {
	try {
		return new URL(value).pathname;
	} catch {
		return '';
	}
}

app.whenReady().then(() => {
	desktopLogger.event('info', 'runtime', 'electron ready');
  const dockIcon = path.join(__dirname, 'assets', 'easy-stock.png');
  if (process.platform === 'darwin' && app.dock && fs.existsSync(dockIcon)) app.dock.setIcon(dockIcon);
  try {
    initializeUpdateManager();
  } catch (error) {
		desktopLogger.event('error', 'updater', 'initialization failed', error);
    updateManager = new UpdateManager({
      updater: autoUpdater,
      enabled: false,
      currentVersion: app.getVersion(),
      platform: process.platform,
      canInstallAutomatically: process.platform === 'win32',
			logger: featureLogger('updater'),
    });
  }
  createWindow().catch((error) => {
		desktopLogger.event('error', 'runtime', 'window startup failed', error);
    app.quit();
  });
});

app.on('before-quit', () => {
	desktopLogger.event('info', 'runtime', 'stopping');
  app.isQuitting = true;
  clearInterval(updateCheckTimer);
  if (backendProcess && !backendProcess.killed) backendProcess.kill();
});

app.on('window-all-closed', () => {
  app.quit();
});
