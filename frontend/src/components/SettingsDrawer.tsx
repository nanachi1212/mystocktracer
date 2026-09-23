import { Bell, CheckCircle2, FolderOpen, KeyRound, LoaderCircle, Save, ShieldCheck, X } from 'lucide-react';
import { FormEvent, useEffect, useState } from 'react';
import { AppSettings, BackendConfig, RuntimeLogStatus, requestJSON } from '../lib/backend';
import { AppUpdatePanel } from './AppUpdatePanel';
import { AgentCapabilitiesPanel } from './AgentCapabilitiesPanel';
import { ModelSettingsPanel } from './settings/ModelSettingsPanel';

type Props = { config: BackendConfig | null; open: boolean; onClose: () => void; onSaved?: () => void };

export function SettingsDrawer({ config, open, onClose, onSaved }: Props) {
	const [settings, setSettings] = useState<AppSettings | null>(null);
	const [corporateEventAlertsEnabled, setCorporateEventAlertsEnabled] = useState(true);
	const [state, setState] = useState<'idle' | 'loading' | 'saving' | 'saved' | 'error'>('idle');
	const [message, setMessage] = useState('');
	const [runtimeLogStatus, setRuntimeLogStatus] = useState<RuntimeLogStatus | null>(null);
	const [openingRuntimeLogs, setOpeningRuntimeLogs] = useState(false);

	useEffect(() => {
		if (!open) return;
		const onKeyDown = (event: KeyboardEvent) => { if (event.key === 'Escape') onClose(); };
		window.addEventListener('keydown', onKeyDown);
		return () => window.removeEventListener('keydown', onKeyDown);
	}, [onClose, open]);

	useEffect(() => {
		if (!open || !config) return;
		let cancelled = false;
		setState('loading'); setMessage('');
		requestJSON<{ data: AppSettings }>(config, '/api/v1/settings').then((payload) => {
			if (cancelled) return;
			setSettings(payload.data);
			setCorporateEventAlertsEnabled(payload.data.taiwan_alerts?.corporate_events_enabled !== false);
			setState('idle');
		}).catch((error) => {
			if (cancelled) return;
			setState('error'); setMessage(error instanceof Error ? error.message : '讀取設定失敗');
		});
		return () => { cancelled = true; };
	}, [config, open]);

	useEffect(() => {
		if (!open || !window.mystocktracer?.getRuntimeLogStatus) { setRuntimeLogStatus(null); return; }
		void window.mystocktracer.getRuntimeLogStatus().then(setRuntimeLogStatus).catch(() => setRuntimeLogStatus(null));
	}, [open]);

	const saveProductSettings = async (event?: FormEvent) => {
		event?.preventDefault();
		if (!config) return;
		setState('saving'); setMessage('');
		try {
			const payload = await requestJSON<{ data: AppSettings }>(config, '/api/v1/settings', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ taiwan_alerts: { corporate_events_enabled: corporateEventAlertsEnabled } }) });
			setSettings(payload.data); setCorporateEventAlertsEnabled(payload.data.taiwan_alerts?.corporate_events_enabled !== false);
			setState('saved'); setMessage('設定已儲存'); onSaved?.();
		} catch (error) { setState('error'); setMessage(error instanceof Error ? error.message : '儲存設定失敗'); }
	};

	const openRuntimeLogs = async () => {
		if (!window.mystocktracer?.openRuntimeLogs) return;
		setOpeningRuntimeLogs(true);
		try { await window.mystocktracer.openRuntimeLogs(); } catch (error) { setState('error'); setMessage(error instanceof Error ? error.message : '開啟日誌目錄失敗'); } finally { setOpeningRuntimeLogs(false); }
	};

	if (!open) return null;
	return <div className="settings-overlay" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}>
		<aside className="settings-drawer" role="dialog" aria-modal="true" aria-label="系統設定">
			<header className="settings-header"><div><span>LOCAL AI RUNTIME</span><h2>系統設定</h2><p>管理 AI 模型執行環境與 AI 研究所需的連線設定</p></div><button type="button" onClick={onClose} aria-label="關閉設定"><X size={20} /></button></header>
			{state === 'loading' && <div className="settings-loading"><LoaderCircle className="spin" size={22} /><span>讀取本機設定</span></div>}
			{state !== 'loading' && <form className="settings-form" onSubmit={saveProductSettings}>
				<section className="settings-security-note"><ShieldCheck size={19} /><div><strong>模型密鑰由 AI 執行環境管理</strong><span>API Key 只寫入執行環境的本機 secret store；頁面僅讀取是否已設定，不會取回密鑰原文。</span></div></section>
				<AgentCapabilitiesPanel config={config} open={open} />
				<ModelSettingsPanel config={config} open={open} onSaved={(next) => { setSettings(next); onSaved?.(); }} />
				<section className="settings-section"><div className="settings-section-title"><Bell size={18} /><div><h3>台股事件提醒</h3><p>控制新公司公告是否加入本機提醒中心；關閉後仍可查看公告，也不會刪除既有提醒。</p></div></div><label className="settings-toggle"><input type="checkbox" checked={corporateEventAlertsEnabled} onChange={(event) => setCorporateEventAlertsEnabled(event.target.checked)} /><span>建立新的公司事件提醒</span></label></section>
				<AppUpdatePanel />
				<section className="settings-section runtime-log-section"><div className="settings-section-title"><FolderOpen size={18} /><div><h3>執行日誌</h3><p>遇到問題時，可將此目錄中的日誌檔案提供給開發者排查。</p></div></div><div className="runtime-log-summary"><span><strong>{runtimeLogStatus?.available ? '日誌正在自動儲存' : '請在桌面應用程式中查看日誌'}</strong><small>{runtimeLogStatus ? `每個檔案最多 ${runtimeLogStatus.max_file_mb} MB，保留 ${runtimeLogStatus.backup_files} 份歷史記錄；密鑰和登入憑證會在寫入前隱藏。` : '瀏覽器開發模式不會儲存桌面執行日誌。'}</small>{runtimeLogStatus?.directory && <code title={runtimeLogStatus.directory}>{runtimeLogStatus.directory}</code>}</span><button type="button" onClick={() => void openRuntimeLogs()} disabled={!runtimeLogStatus?.available || openingRuntimeLogs}>{openingRuntimeLogs ? <LoaderCircle className="spin" size={15} /> : <FolderOpen size={15} />}開啟日誌目錄</button></div></section>
				<footer className="settings-footer"><div className={`settings-message ${state}`}>{state === 'saved' && <CheckCircle2 size={15} />}{state === 'error' && <KeyRound size={15} />}<span>{message || '模型設定由上方 AI Runtime 面板管理。'}</span></div><button type="button" onClick={onClose}>取消</button><button type="submit" className="settings-save" disabled={!config || state === 'saving'}>{state === 'saving' ? <LoaderCircle className="spin" size={16} /> : <Save size={16} />}儲存設定</button></footer>
			</form>}
		</aside>
	</div>;
}
