import { Bot, PanelLeftClose, PanelLeftOpen, Radio, RefreshCw, Settings, Wifi } from 'lucide-react';
import { type ReactNode } from 'react';
import { type BackendConfig } from '../lib/backend';
import { marketNavigation, primaryNavigation, workspaceIcons, workspaceTitles, type WorkspaceMode } from '../app-navigation';

type ApplicationFrameProps = {
	workspace: WorkspaceMode;
	sidebarExpanded: boolean;
	config: BackendConfig | null;
	configError: string;
	children: ReactNode;
	onWorkspaceChange: (mode: WorkspaceMode) => void;
	onSidebarToggle: () => void;
	onSettingsOpen: () => void;
	onRefresh: () => void;
};

export function ApplicationFrame(props: ApplicationFrameProps) {
	const isTaiwan = props.workspace !== 'ai';
	const heading = props.workspace !== 'ai'
		? workspaceTitles[props.workspace]
		: { title: 'AI 對話', description: '本機 AI 對話，與台股研究流程分離' };
	const loadState = props.configError ? 'error' : props.config ? 'ready' : 'loading';
	const status = props.configError || (props.config ? (isTaiwan ? '後端服務已設定' : 'AI 助手已連接') : '正在取得後端設定');
	const subStatus = isTaiwan ? 'TWSE · TPEx · 官方資料與可追溯狀態' : '本機 AI 對話';

	return (
		<main className={`workspace-frame ${props.sidebarExpanded ? 'sidebar-expanded' : 'sidebar-collapsed'}`}>
			<aside className="app-sidebar" aria-label="功能導航">
				<div className="sidebar-brand">
					<div className="sidebar-logo"><img src={`${import.meta.env.BASE_URL}mystocktracer-mark.svg`} alt="mystocktracer" /></div>
					{props.sidebarExpanded && <div><strong>mystocktracer</strong><span>台股分析工作台</span></div>}
				</div>
				<nav>
					<div className="sidebar-primary" role="group" aria-label="主要功能">
						{primaryNavigation.map(([mode, label]) => <NavigationButton key={mode} mode={mode} label={label} active={props.workspace === mode} onSelect={props.onWorkspaceChange} />)}
					</div>
					<div className="sidebar-market-details" role="group" aria-label="市場詳情">
						<small className="sidebar-group-heading">市場詳情</small>
						{marketNavigation.map(([mode, label]) => <NavigationButton key={mode} mode={mode} label={label} active={props.workspace === mode} onSelect={props.onWorkspaceChange} />)}
					</div>
				</nav>
				<div className="sidebar-guidance">{props.sidebarExpanded && <><strong>資料原則</strong><span>官方來源 · 完成交易日 · 缺漏狀態不隱藏</span></>}</div>
				<button type="button" className="sidebar-settings" onClick={props.onSettingsOpen} aria-label="開啟系統設定" title="系統設定"><Settings size={17} />{props.sidebarExpanded && <span>系統設定</span>}</button>
				<button type="button" className="sidebar-toggle" onClick={props.onSidebarToggle} aria-label={props.sidebarExpanded ? '收合側邊欄' : '展開側邊欄'}>
					{props.sidebarExpanded ? <PanelLeftClose size={17} /> : <PanelLeftOpen size={17} />}
					{props.sidebarExpanded && <span>收合側欄</span>}
				</button>
			</aside>
			<div className="app-shell">
				<header className="topbar">
					<div className="brand-block">
						<div className="brand-mark"><img src={`${import.meta.env.BASE_URL}mystocktracer-mark.svg`} alt="mystocktracer" /></div>
						<div><h1>{heading.title}</h1><p>{heading.description}</p></div>
					</div>
					<nav className="mode-nav" aria-label="工作台模式">
						<button type="button" className="active">{isTaiwan ? heading.title : <><Bot size={16} aria-hidden="true" />AI 對話</>}</button>
					</nav>
					<div className="top-actions">
						<div className={`data-status ${loadState}`}><span className="status-dot" /><div><strong>{status}</strong><small>{subStatus}</small></div></div>
						<button type="button" className="icon-button" onClick={props.onRefresh} aria-label="重新整理資料"><RefreshCw size={18} aria-hidden="true" /></button>
					</div>
				</header>
				{props.children}
				<footer className="data-footer">
					<div><Wifi size={15} aria-hidden="true" /><span>{props.config?.backendUrl || '正在連線本機資料服務'}</span></div>
					<div><Radio size={15} aria-hidden="true" /><span>{isTaiwan ? '台灣證交所與櫃買中心官方資料 · 顯示資料日期、延遲、部分與無法取得狀態' : '模型請求由本機後端轉發 · API Key 不會暴露給頁面 · 對話歷史保存在當前裝置'}</span></div>
				</footer>
			</div>
		</main>
	);
}

function NavigationButton({ mode, label, active, onSelect }: { mode: Exclude<WorkspaceMode, 'ai'>; label: string; active: boolean; onSelect: (mode: WorkspaceMode) => void }) {
	const Icon = workspaceIcons[mode];
	return <button type="button" className={active ? 'active' : ''} aria-current={active ? 'page' : undefined} onClick={() => onSelect(mode)} title={label} aria-label={label}><Icon size={18} aria-hidden="true" /><span>{label}</span></button>;
}
