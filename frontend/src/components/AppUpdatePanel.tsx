import { CheckCircle2, Download, ExternalLink, FolderOpen, HardDriveDownload, LoaderCircle, RefreshCw, RotateCcw, ShieldCheck } from 'lucide-react';
import { useEffect, useState } from 'react';
import type { AppUpdateStatus } from '../lib/backend';

const developmentStatus: AppUpdateStatus = {
	state: 'disabled',
	supported: false,
	currentVersion: '開發模式',
	message: '安裝版會自動檢查正式更新來源',
	progress: 0,
};

export function updatePrimaryAction(status: AppUpdateStatus): 'check' | 'download' | 'install' | 'release' {
	if (status.state === 'available') return status.installMode === 'manual' ? 'release' : 'download';
	if (status.state === 'downloaded') return status.installMode === 'manual' ? 'release' : 'install';
	return 'check';
}

export function AppUpdatePanel() {
	const bridge = window.aStock;
	const [status, setStatus] = useState<AppUpdateStatus>(developmentStatus);
	const [actionError, setActionError] = useState('');

	useEffect(() => {
		let active = true;
		void bridge?.getUpdateStatus?.().then((next) => { if (active) setStatus(next); }).catch((error) => {
			if (active) setActionError(error instanceof Error ? error.message : '讀取版本狀態失敗');
		});
		const unsubscribe = bridge?.onUpdateStatus?.((next) => { if (active) setStatus(next); });
		return () => {
			active = false;
			unsubscribe?.();
		};
	}, [bridge]);

	const run = async (action?: () => Promise<AppUpdateStatus | void>) => {
		if (!action) return;
		setActionError('');
		try {
			const next = await action();
			if (next) setStatus(next);
		} catch (error) {
			setActionError(error instanceof Error ? error.message : '更新操作失敗');
		}
	};

	const busy = status.state === 'checking' || status.state === 'downloading' || status.state === 'installing';
	const action = updatePrimaryAction(status);
	const primaryAction = action === 'release'
		? { label: '前往下載新版', icon: <ExternalLink size={15} />, action: bridge?.openUpdateRelease }
		: action === 'download'
		? { label: '下載更新', icon: <Download size={15} />, action: bridge?.downloadUpdate }
		: action === 'install'
			? { label: '重新啟動並安裝', icon: <RotateCcw size={15} />, action: bridge?.installUpdate }
			: { label: status.state === 'checking' ? '正在檢查' : '檢查更新', icon: status.state === 'checking' ? <LoaderCircle className="spin" size={15} /> : <RefreshCw size={15} />, action: bridge?.checkForUpdates };

	return (
		<section className="settings-section app-update-section">
			<div className="settings-section-title"><HardDriveDownload size={18} /><div><h3>版本與自動更新</h3><p>Windows 支援應用程式內更新；macOS 未設定 Apple Developer ID 時，需透過發布頁手動下載安裝。</p></div></div>
			<div className={`app-update-status ${status.state}`}>
				<div className="app-update-version">
					<span><strong>v{status.currentVersion}</strong><small>目前版本</small></span>
					{status.latestVersion && status.latestVersion !== status.currentVersion && <><em>→</em><span><strong>v{status.latestVersion}</strong><small>最新版本</small></span></>}
				</div>
				<div className="app-update-message">{status.state === 'downloaded' ? <CheckCircle2 size={15} /> : busy ? <LoaderCircle className="spin" size={15} /> : <ShieldCheck size={15} />}<span>{actionError || status.message}</span></div>
				{status.state === 'downloading' && <div className="app-update-progress" aria-label={`下載進度 ${Math.round(status.progress)}%`}><span style={{ width: `${status.progress}%` }} /><em>{Math.round(status.progress)}%</em></div>}
				{status.releaseNotes && <details className="app-update-notes"><summary>{status.releaseName || '查看更新說明'}</summary><p>{status.releaseNotes}</p></details>}
				<div className="app-update-actions">
					<button type="button" className="primary" onClick={() => void run(primaryAction.action)} disabled={!status.supported || busy}>{primaryAction.icon}{primaryAction.label}</button>
					{status.latestVersion && action !== 'release' && <button type="button" onClick={() => void run(bridge?.openUpdateRelease)}><ExternalLink size={14} />發布頁</button>}
					<button type="button" onClick={() => void run(bridge?.openUpdateBackups)} disabled={!status.supported}><FolderOpen size={14} />備份目錄</button>
				</div>
				{status.installMode === 'manual' && status.latestVersion && status.latestVersion !== status.currentVersion && <p className="settings-field-note app-update-manual-note">目前 macOS 安裝檔未使用 Apple Developer ID 簽署，系統暫不允許應用程式內替換。請結束 easy-stock，前往發布頁下載新版 DMG 後覆蓋安裝，不會刪除本機模型設定、文章、登入狀態或資料庫。</p>}
			</div>
			<p className="settings-field-note">{status.installMode === 'manual' ? '應用程式與使用者資料分開存放，覆蓋安裝只會替換 easy-stock 應用程式本身，不會清除本機模型密鑰、匯入文章、AI 摘要、Hermes 記憶、瀏覽器/微信登入狀態或資料庫。' : '應用程式內安裝前會停止背景同步，並在應用程式資料目錄外建立完整備份，保留模型設定與密鑰、匯入文章、AI 摘要、Hermes 記憶、瀏覽器/微信登入狀態及本機資料庫；僅排除可重建的快取，最近保留 3 份。'}</p>
		</section>
	);
}
