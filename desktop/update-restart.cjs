const fs = require('node:fs');
const path = require('node:path');

// Only version intent is persisted. Installer paths and commands never come from
// this file: electron-updater revalidates the requested release after restart.
function scheduleUpdateRestart({ requestPath, versions, app }) {
  fs.mkdirSync(path.dirname(requestPath), { recursive: true, mode: 0o700 });
  fs.writeFileSync(requestPath, JSON.stringify(versions), { flag: 'wx', mode: 0o600 });
  app.relaunch();
  app.quit();
}

async function resumeUpdateAfterRestart({ requestPath, currentVersion, updater, createBackup, install = () => updater.quitAndInstall(false, true) }) {
  const contents = fs.readFileSync(requestPath, 'utf8');
  // Consume once; a failed backup or offline release check must not cause a loop.
  fs.unlinkSync(requestPath);
  const intent = JSON.parse(contents);
  if (intent.fromVersion !== currentVersion || !/^\d+\.\d+\.\d+(?:[-+][\w.-]+)?$/.test(intent.toVersion || '')) throw new Error('更新要求已失效，請重新檢查版本');
  updater.autoDownload = false;
  updater.autoInstallOnAppQuit = false;
  // Snapshot before electron-updater uses electron.net (and its session).
  const backup = await createBackup(intent);
  const release = await updater.checkForUpdates();
  if (release?.updateInfo?.version !== intent.toVersion) throw new Error('發布版本已變更，請重新檢查更新');
  await updater.downloadUpdate();
  await install();
  return backup;
}

function awaitNativeInstall({ updater, app, timeout = 15000 }) {
  return new Promise((resolve, reject) => {
    const cleanup = () => { clearTimeout(timer); app.removeListener('before-quit', success); updater.removeListener('error', failure); };
    const success = () => { cleanup(); resolve(); };
    const failure = error => { cleanup(); reject(error); };
    const timer = setTimeout(() => failure(new Error('安裝程式未能啟動')), timeout);
    app.once('before-quit', success);
    updater.once('error', failure);
    try { updater.quitAndInstall(false, true); } catch (error) { failure(error); }
  });
}

module.exports = { scheduleUpdateRestart, resumeUpdateAfterRestart, awaitNativeInstall };
