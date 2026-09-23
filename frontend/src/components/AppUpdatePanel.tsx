import { useEffect, useRef, useState } from 'react';
import type { AppUpdateStatus, BackendBridge } from '../lib/backend';

const unavailable: AppUpdateStatus = { state: 'disabled', supported: false, currentVersion: '開發模式', progress: 0, message: '桌面安裝版提供版本更新。' };
export function updatePrimaryAction(status: AppUpdateStatus): 'check' | 'download' | 'install' | 'release' {
  const ready = status.state === 'available' || status.state === 'downloaded';
  if (ready && status.installMode === 'manual') return 'release';
  if (status.state === 'available') return 'download';
  return status.state === 'downloaded' ? 'install' : 'check';
}
export function updateAction(bridge: BackendBridge | undefined, action: ReturnType<typeof updatePrimaryAction>) {
  return { check: bridge?.checkForUpdates, download: bridge?.downloadUpdate, install: bridge?.installUpdate, release: bridge?.openUpdateRelease }[action];
}
export function AppUpdatePanel() {
  const bridge = window.mystocktracer;
  const [status, changeStatus] = useState(unavailable);
  const [error, changeError] = useState('');
  const [pending, changePending] = useState(false);
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    let eventReceived = false;
    const unsubscribe = bridge?.onUpdateStatus?.((next) => { eventReceived = true; if (mounted.current) changeStatus(next); });
    void bridge?.getUpdateStatus?.().then((next) => { if (mounted.current && !eventReceived) changeStatus(next); }).catch(() => { if (mounted.current) changeError('無法讀取更新狀態'); });
    return () => { mounted.current = false; unsubscribe?.(); };
  }, [bridge]);
  async function invoke(operation?: () => Promise<AppUpdateStatus | void>) {
    if (!operation || pending) return;
    changePending(true); changeError('');
    try { const result = await operation(); if (result && mounted.current) changeStatus(result); }
    catch (reason) { if (mounted.current) changeError(reason instanceof Error ? reason.message : '更新操作失敗'); }
    finally { if (mounted.current) changePending(false); }
  }
  const action = updatePrimaryAction(status);
  const labels = { check: '檢查更新', download: '下載更新', install: '備份並重新啟動安裝', release: '開啟發布頁下載' };
  const busy = pending || ['checking', 'downloading', 'installing'].includes(status.state);
  const percent = Math.min(100, Math.max(0, status.progress || 0));
  return <section className="settings-section" aria-label="mystocktracer 更新">
    <h3>mystocktracer 版本與自動更新</h3>
    <dl><dt>目前版本</dt><dd>{status.currentVersion}</dd><dt>最新版本</dt><dd>{status.latestVersion || '尚未檢查'}</dd></dl>
    <p role={error ? 'alert' : 'status'}>{error || status.message}</p>
    {status.state === 'downloading' && <label>下載進度 {Math.round(percent)}% <progress max={100} value={percent} /></label>}
    <div className="app-update-actions">
      <button type="button" disabled={!status.supported || busy} onClick={() => void invoke(updateAction(bridge, action))}>{busy ? '處理中…' : labels[action]}</button>
      {action !== 'release' && <button type="button" disabled={!status.supported || pending} onClick={() => void invoke(bridge?.openUpdateRelease)}>發布說明</button>}
      <button type="button" disabled={!status.supported || pending} onClick={() => void invoke(bridge?.openUpdateBackups)}>開啟備份目錄</button>
    </div>
    {status.backupPath && <p>最近備份：{status.backupPath}</p>}
    {status.releaseNotes && <details><summary>{status.releaseName || '更新內容'}</summary><p style={{ whiteSpace: 'pre-wrap' }}>{status.releaseNotes}</p></details>}
    <p className="settings-field-note">{status.installMode === 'manual'
      ? 'macOS 未簽署 Apple Developer ID 時，請結束 mystocktracer 後從發布頁下載新版。更換應用程式前請自行備份使用者資料。'
      : 'Windows 安裝前會停止服務並驗證資料備份；備份失敗會中止安裝。既有備份會保留，使用者資料與應用程式分開存放。'}</p>
  </section>;
}
