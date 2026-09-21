import { BarChart3, Bell, Bot, BrainCircuit, Gauge, LayoutDashboard, PanelLeftClose, PanelLeftOpen, Radio, RefreshCw, Search, Settings, Star, Activity, WalletCards, Wifi } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { BackendConfig, resolveBackendConfig } from './lib/backend';
import { SettingsDrawer } from './components/SettingsDrawer';
import { AIChatWorkspace } from './components/AIChatWorkspace';
import { TaiwanMarketWorkspace } from './components/TaiwanMarketWorkspace';
import { TaiwanScreenerWorkspace } from './components/TaiwanScreenerWorkspace';
import { TaiwanStockResearchWorkspace } from './components/TaiwanStockResearchWorkspace';
import { TaiwanStockResearchErrorBoundary } from './components/TaiwanStockResearchErrorBoundary';
import { TaiwanWatchlistWorkspace } from './components/TaiwanWatchlistWorkspace';
import { TaiwanPortfolioWorkspace } from './components/TaiwanPortfolioWorkspace';
import { TaiwanDailyDashboard } from './components/TaiwanDailyDashboard';
import { TaiwanEventAlertCenter } from './components/TaiwanEventAlertCenter';
import { resolveTaiwanWorkspace, taiwanPrimaryNavigation, taiwanMarketDetailNavigation, type TaiwanResearchEntryContext } from './lib/taiwan-product';

type WorkspaceMode = 'taiwan-dashboard' | 'taiwan-overview' | 'taiwan-breadth' | 'taiwan-emotion' | 'taiwan-industry' | 'taiwan-screener' | 'taiwan-stock' | 'taiwan-watchlist' | 'taiwan-portfolio' | 'taiwan-alerts' | 'ai';

const taiwanNavigationIcons = {
	'taiwan-dashboard': LayoutDashboard, 'taiwan-overview': BarChart3, 'taiwan-screener': Search, 'taiwan-watchlist': Star, 'taiwan-portfolio': WalletCards, 'taiwan-alerts': Bell,
	'taiwan-stock': BrainCircuit, 'taiwan-breadth': Activity, 'taiwan-emotion': Gauge, 'taiwan-industry': BarChart3,
};

const taiwanTitles: Record<Exclude<WorkspaceMode, 'ai'>, [string, string]> = {
	'taiwan-dashboard': ['今日總覽', '市場、持股、自選股、事件與研究活動的資訊中心'],
	'taiwan-overview': ['台股總覽', '上市、上櫃行情與官方市場資料'],
	'taiwan-breadth': ['市場廣度', '上漲、下跌家數與成交方向'],
	'taiwan-emotion': ['市場情緒', '以確定性規則呈現市場參與與訊號分歧'],
	'taiwan-industry': ['產業雷達', '官方產業分類的相對市場廣度與成交方向'],
	'taiwan-screener': ['台股選股器', '以官方市場快照篩選、排序台灣證券'],
	'taiwan-stock': ['個股分析', '官方證據與確定性解讀，不提供投資推薦'],
	'taiwan-watchlist': ['自選股', '已儲存的台灣證券與最新報價'],
	'taiwan-portfolio': ['持倉總覽', '台股市值、成本、損益與集中度'],
	'taiwan-alerts': ['事件提醒', '共用公司事件收件匣與已讀狀態'],
};

export function App() {
	const [workspaceMode, setWorkspaceMode] = useState<WorkspaceMode>(() => {
		// #ai stays reachable by bookmark only; it is deliberately absent from the Taiwan navigation
		// so the product never presents a general chat surface as a Taiwan research entry point.
		if (window.location.hash === '#ai') return 'ai';
		return resolveTaiwanWorkspace(window.location.hash);
	});
	const [sidebarExpanded, setSidebarExpanded] = useState(true);
	const [config, setConfig] = useState<BackendConfig | null>(null);
	const [marketRefreshKey, setMarketRefreshKey] = useState(0);
	const [aiRefreshKey, setAIRefreshKey] = useState(0);
	const [settingsOpen, setSettingsOpen] = useState(false);
	// M6C handoff: a Watchlist row requests the stock research workspace load this exact canonical
	// symbol. `token` is a strictly increasing nonce (not just the canonical string) so clicking the
	// same saved security twice in a row still produces a new value the workspace's effect reacts
	// to — React would otherwise bail out on an unchanged string. Null by default; never fetched or
	// touched by App itself, and never causes any cold-start request.
	// M8B: `context` is optional and only ever set by the Screener call site below — a Watchlist-
	// originated request never has Screener filters to attach, so it stays undefined there.
	const [requestedTaiwanSymbol, setRequestedTaiwanSymbol] = useState<{ canonical: string; token: number; context?: TaiwanResearchEntryContext | null; openHistory?: boolean } | null>(null);
	const taiwanSymbolRequestNonce = useRef(0);
	const [requestedPortfolioSymbol, setRequestedPortfolioSymbol] = useState<{ canonical: string; token: number } | null>(null);
	const portfolioSymbolRequestNonce = useRef(0);
	const [configError, setConfigError] = useState('');

	useEffect(() => {
		resolveBackendConfig()
			.then(setConfig)
			.catch((error) => setConfigError(error instanceof Error ? error.message : '後端設定失敗'));
	}, []);

	const refreshAll = () => {
		if (workspaceMode === 'ai') {
			setAIRefreshKey((current) => current + 1);
			return;
		}
		setMarketRefreshKey((current) => current + 1);
	};

	const switchWorkspace = (mode: WorkspaceMode) => {
		setWorkspaceMode(mode);
		window.history.replaceState(null, '', `#${mode}`);
	};

	// M6C: a Watchlist row was clicked. Carries the exact persisted canonical identity (never
	// code-only) into the existing stock research workspace and switches to it — no new fetch
	// logic here, no fuzzy re-resolution; the workspace itself resolves and loads the symbol.
	// M8B: `context` is optional — the Screener call site passes the applied filters/sort as a
	// TaiwanResearchEntryContext; the Watchlist call site (same function) omits it entirely, so a
	// Watchlist-originated navigation never shows Screener provenance in Research.
	const openTaiwanStockResearch = (canonical: string, context?: TaiwanResearchEntryContext | null) => {
		taiwanSymbolRequestNonce.current += 1;
		setRequestedTaiwanSymbol({ canonical, token: taiwanSymbolRequestNonce.current, context });
		switchWorkspace('taiwan-stock');
	};
	const openTaiwanResearchHistory = (canonical: string) => {
		taiwanSymbolRequestNonce.current += 1;
		setRequestedTaiwanSymbol({ canonical, token: taiwanSymbolRequestNonce.current, context: null, openHistory: true });
		switchWorkspace('taiwan-stock');
	};
	const openTaiwanPortfolio = (canonical: string) => {
		portfolioSymbolRequestNonce.current += 1;
		setRequestedPortfolioSymbol({ canonical, token: portfolioSymbolRequestNonce.current });
		switchWorkspace('taiwan-portfolio');
	};

	const isTaiwanWorkspace = workspaceMode !== 'ai';
	const currentLoadState = configError ? 'error' : config ? 'ready' : 'loading';
	const currentStatusText = configError || (isTaiwanWorkspace
		? config ? '後端服務已設定' : '正在取得後端設定'
		: config ? 'AI 助手已連接' : '正在取得後端設定');
	const currentSubStatus = isTaiwanWorkspace ? 'TWSE · TPEx · 官方資料與可追溯狀態' : '本機 Hermes AI 對話';
	const topbarTitle = isTaiwanWorkspace ? taiwanTitles[workspaceMode][0] : 'AI 對話';
	const topbarDescription = isTaiwanWorkspace ? taiwanTitles[workspaceMode][1] : '本機 Hermes 對話，與台股研究流程分離';

	return (
		<main className={`workspace-frame ${sidebarExpanded ? 'sidebar-expanded' : 'sidebar-collapsed'}`}>
			<aside className="app-sidebar" aria-label="功能導航">
				<div className="sidebar-brand"><div className="sidebar-logo"><img src={`${import.meta.env.BASE_URL}easy-stock-mark.svg`} alt="mystocktracer" /></div>{sidebarExpanded && <div><strong>mystocktracer</strong><span>台股分析工作台</span></div>}</div>
				<nav>
					<div className="sidebar-primary" role="group" aria-label="主要功能">{taiwanPrimaryNavigation.map(([mode, label]) => {
						const Icon = taiwanNavigationIcons[mode];
						return <button key={mode} type="button" className={workspaceMode === mode ? 'active' : ''} aria-current={workspaceMode === mode ? 'page' : undefined} onClick={() => switchWorkspace(mode)} title={label} aria-label={label}><Icon size={18} aria-hidden="true" /><span>{label}</span></button>;
					})}</div>
					<div className="sidebar-market-details" role="group" aria-label="市場詳情"><small className="sidebar-group-heading">市場詳情</small>{taiwanMarketDetailNavigation.map(([mode, label]) => {
						const Icon = taiwanNavigationIcons[mode];
						return <button key={mode} type="button" className={workspaceMode === mode ? 'active' : ''} aria-current={workspaceMode === mode ? 'page' : undefined} onClick={() => switchWorkspace(mode)} title={label} aria-label={label}><Icon size={18} aria-hidden="true" /><span>{label}</span></button>;
					})}</div>
				</nav>
				<div className="sidebar-guidance">{sidebarExpanded && <><strong>資料原則</strong><span>官方來源 · 完成交易日 · 缺漏狀態不隱藏</span></>}</div>
				<button type="button" className="sidebar-settings" onClick={() => setSettingsOpen(true)} aria-label="開啟系統設定" title="系統設定"><Settings size={17} />{sidebarExpanded && <span>系統設定</span>}</button>
				<button type="button" className="sidebar-toggle" onClick={() => setSidebarExpanded((value) => !value)} aria-label={sidebarExpanded ? '收合側邊欄' : '展開側邊欄'}>{sidebarExpanded ? <PanelLeftClose size={17} /> : <PanelLeftOpen size={17} />} {sidebarExpanded && <span>收合側欄</span>}</button>
			</aside>
			<div className="app-shell">
				<header className="topbar">
					<div className="brand-block">
						<div className="brand-mark"><img src={`${import.meta.env.BASE_URL}easy-stock-mark.svg`} alt="mystocktracer" /></div>
						<div>
							<h1>{topbarTitle}</h1>
							<p>{topbarDescription}</p>
						</div>
					</div>
					<nav className="mode-nav" aria-label="工作台模式">
						<button type="button" className="active">{isTaiwanWorkspace ? topbarTitle : <><Bot size={16} aria-hidden="true" />AI 對話</>}</button>
					</nav>
					<div className="top-actions">
						<div className={`data-status ${currentLoadState}`}>
							<span className="status-dot" />
							<div><strong>{currentStatusText}</strong><small>{currentSubStatus}</small></div>
						</div>
						<button type="button" className="icon-button" onClick={refreshAll} aria-label="重新整理資料">
							<RefreshCw size={18} aria-hidden="true" />
						</button>
					</div>
				</header>

				{workspaceMode === 'taiwan-dashboard' ? <TaiwanDailyDashboard config={config} refreshKey={marketRefreshKey} onNavigate={switchWorkspace} onOpenResearch={openTaiwanStockResearch} onOpenResearchHistory={openTaiwanResearchHistory} /> : workspaceMode === 'taiwan-overview' ? <TaiwanMarketWorkspace key={workspaceMode} config={config} refreshKey={marketRefreshKey} view="overview" onNavigate={switchWorkspace} /> : workspaceMode === 'taiwan-breadth' ? <TaiwanMarketWorkspace key={workspaceMode} config={config} refreshKey={marketRefreshKey} view="breadth" /> : workspaceMode === 'taiwan-emotion' ? <TaiwanMarketWorkspace key={workspaceMode} config={config} refreshKey={marketRefreshKey} view="emotion" /> : workspaceMode === 'taiwan-industry' ? <TaiwanMarketWorkspace key={workspaceMode} config={config} refreshKey={marketRefreshKey} view="industry" /> : workspaceMode === 'taiwan-screener' ? <TaiwanScreenerWorkspace config={config} refreshKey={marketRefreshKey} onOpenResearch={openTaiwanStockResearch} /> : workspaceMode === 'taiwan-stock' ? <TaiwanStockResearchErrorBoundary><TaiwanStockResearchWorkspace config={config} refreshKey={marketRefreshKey} externalSymbolRequest={requestedTaiwanSymbol} /></TaiwanStockResearchErrorBoundary> : workspaceMode === 'taiwan-watchlist' ? <TaiwanWatchlistWorkspace config={config} refreshKey={marketRefreshKey} onOpenResearch={openTaiwanStockResearch} onAddPortfolio={openTaiwanPortfolio} /> : workspaceMode === 'taiwan-portfolio' ? <TaiwanPortfolioWorkspace config={config} refreshKey={marketRefreshKey} onOpenResearch={openTaiwanStockResearch} externalSymbolRequest={requestedPortfolioSymbol} /> : workspaceMode === 'taiwan-alerts' ? <div className="taiwan-product-workspace"><TaiwanEventAlertCenter config={config} refreshKey={marketRefreshKey} defaultOpen /></div> : <AIChatWorkspace config={config} refreshKey={aiRefreshKey} initialPrompt="" onInitialPromptConsumed={() => undefined} onOpenSettings={() => setSettingsOpen(true)} />}

				<footer className="data-footer">
					<div><Wifi size={15} aria-hidden="true" /><span>{config?.backendUrl || '正在連線本機資料服務'}</span></div>
					<div><Radio size={15} aria-hidden="true" /><span>{isTaiwanWorkspace ? '台灣證交所與櫃買中心官方資料 · 顯示資料日期、延遲、部分與無法取得狀態' : '模型請求由本機後端轉發 · API Key 不會暴露給頁面 · 對話歷史保存在當前裝置'}</span></div>
				</footer>
			</div>
			<SettingsDrawer config={config} open={settingsOpen} onClose={() => setSettingsOpen(false)} onSaved={() => { setAIRefreshKey((current) => current + 1); setMarketRefreshKey((current) => current + 1); }} />
		</main>
	);
}
