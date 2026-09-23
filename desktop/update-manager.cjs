const { EventEmitter } = require('node:events');
const { redactRuntimeLog } = require('./runtime-logger.cjs');
function readableError(error) {
  return redactRuntimeLog(error instanceof Error ? error.message : String(error || '未知錯誤')).replace(/(?:[A-Za-z]:\\|\/)[^\s)]+/g, '[本機路徑]');
}
function friendlyUpdateError(error) {
  return /code signature|did not pass validation|代码未能满足指定的代码要求/i.test(String(error))
    ? 'Apple Developer ID 簽章無法驗證，請從發布頁手動安裝'
    : readableError(error);
}
function normalizeReleaseNotes(value) {
  return (typeof value === 'string' ? value : Array.isArray(value) ? value.map((item) => item?.note || '').join('\n\n') : '').slice(0, 64000);
}
class UpdateManager extends EventEmitter {
  constructor({ updater, enabled, currentVersion, platform, canInstallAutomatically, stopRuntime, createBackup, restartForInstall, logger = console }) {
    super();
    Object.assign(this, { updater, currentVersion, stopRuntime, createBackup, restartForInstall, logger });
    this.enabled = Boolean(enabled && updater);
    this.automatic = canInstallAutomatically ?? platform === 'win32';
    this.status = { state: this.enabled ? 'idle' : 'disabled', supported: this.enabled, installMode: this.automatic ? 'automatic' : 'manual', currentVersion, progress: 0, message: this.enabled ? '可檢查 mystocktracer 更新' : '開發環境不啟用更新' };
    this.pending = new Map();
    this.latestInfo = null;
    if (!this.enabled) return;
    updater.autoDownload = false;
    updater.autoInstallOnAppQuit = false;
    updater.allowPrerelease = false;
    const release = (info) => {
      this.latestInfo = info;
      return { latestVersion: info?.version, releaseName: info?.releaseName || '', releaseNotes: normalizeReleaseNotes(info?.releaseNotes) };
    };
    const handlers = {
      'checking-for-update': () => ({ state: 'checking', progress: 0, message: '正在檢查版本' }),
      'update-available': (info) => ({ ...release(info), state: 'available', message: this.automatic ? '新版已可下載' : '請前往發布頁手動安裝新版' }),
      'update-not-available': (info) => ({ latestVersion: info?.version || currentVersion, state: 'not-available', progress: 0, message: '目前已是最新版本' }),
      'download-progress': (info) => ({ state: 'downloading', progress: Math.min(100, Math.max(0, Number(info.percent) || 0)), transferred: Number(info.transferred) || 0, total: Number(info.total) || 0, bytesPerSecond: Number(info.bytesPerSecond) || 0, message: '正在下載更新' }),
      'update-downloaded': (info) => ({ ...release(info || this.latestInfo), state: 'downloaded', progress: 100, message: '下載完成，安裝前會先建立資料備份' }),
      error: (error) => ({ state: 'error', message: friendlyUpdateError(error) }),
    };
    for (const [event, handler] of Object.entries(handlers)) updater.on(event, (value) => {
      if (this.status.state === 'installing' && event !== 'error') return;
      this.setStatus(handler(value));
    });
  }
  getStatus() { return { ...this.status }; }
  setStatus(patch) { Object.assign(this.status, patch); this.emit('status', this.getStatus()); return this.getStatus(); }
  ensureEnabled() { if (!this.enabled) throw new Error('目前環境不支援自動更新'); }
  operation(name, work) {
    if (this.pending.has(name)) return this.pending.get(name);
    const job = Promise.resolve().then(() => { this.ensureEnabled(); return work(); }).catch((error) => {
      const message = friendlyUpdateError(error);
      this.setStatus({ state: 'error', message });
      throw new Error(message);
    }).finally(() => this.pending.delete(name));
    this.pending.set(name, job);
    return job;
  }
  checkForUpdates() { return this.operation('check', async () => {
    if (['downloading', 'downloaded', 'installing'].includes(this.status.state)) return this.getStatus();
    await this.updater.checkForUpdates(); return this.getStatus();
  }); }
  downloadUpdate() { return this.operation('download', async () => {
    if (!this.automatic) throw new Error('未簽署 Apple Developer ID 的 macOS 版本請使用發布頁');
    if (!this.latestInfo?.version || !['available', 'error'].includes(this.status.state)) throw new Error('目前沒有可下載的新版');
    this.setStatus({ state: 'downloading', progress: 0, message: '開始下載更新' });
    await this.updater.downloadUpdate(); return this.getStatus();
  }); }
  installUpdate() { return this.operation('install', async () => {
    if (!this.automatic) throw new Error('未簽署 Apple Developer ID 的 macOS 版本請使用發布頁');
    if (this.status.state !== 'downloaded' || !this.latestInfo?.version) throw new Error('更新尚未下載完成');
    this.setStatus({ state: 'installing', message: '停止寫入並驗證資料備份' });
    if (this.restartForInstall) {
      await this.restartForInstall({ fromVersion: this.currentVersion, toVersion: this.latestInfo.version });
      return this.getStatus();
    }
    await this.stopRuntime();
    const backup = await this.createBackup({ fromVersion: this.currentVersion, toVersion: this.latestInfo.version });
    this.setStatus({ backupPath: backup.path, backupCreatedAt: backup.manifest?.createdAt, message: '備份完成，正在啟動安裝程式' });
    this.updater.quitAndInstall(false, true);
    return this.getStatus();
  }); }
}
module.exports = { UpdateManager, readableError, friendlyUpdateError, normalizeReleaseNotes };
