import { useEffect, useRef, useState } from 'react';
import { type WorkspaceMode } from './app-navigation';
import { AIChatWorkspace } from './components/AIChatWorkspace';
import { ApplicationFrame } from './components/ApplicationFrame';
import { SettingsDrawer } from './components/SettingsDrawer';
import { TaiwanDailyDashboard } from './components/TaiwanDailyDashboard';
import { TaiwanEventAlertCenter } from './components/TaiwanEventAlertCenter';
import { TaiwanMarketWorkspace } from './components/TaiwanMarketWorkspace';
import { TaiwanPortfolioWorkspace } from './components/TaiwanPortfolioWorkspace';
import { TaiwanScreenerWorkspace } from './components/TaiwanScreenerWorkspace';
import { TaiwanStockResearchErrorBoundary } from './components/TaiwanStockResearchErrorBoundary';
import { TaiwanStockResearchWorkspace } from './components/TaiwanStockResearchWorkspace';
import { TaiwanWatchlistWorkspace } from './components/TaiwanWatchlistWorkspace';
import { type BackendConfig, resolveBackendConfig } from './lib/backend';
import { resolveTaiwanWorkspace, type TaiwanResearchEntryContext } from './lib/taiwan-product';

type ResearchRequest = { canonical: string; token: number; context?: TaiwanResearchEntryContext | null; openHistory?: boolean };
type PortfolioRequest = { canonical: string; token: number };

export function App() {
	const [workspaceMode, setWorkspaceMode] = useState<WorkspaceMode>(initialWorkspace);
	const [sidebarExpanded, setSidebarExpanded] = useState(true);
	const [config, setConfig] = useState<BackendConfig | null>(null);
	const [configError, setConfigError] = useState('');
	const [marketRefreshKey, setMarketRefreshKey] = useState(0);
	const [aiRefreshKey, setAIRefreshKey] = useState(0);
	const [settingsOpen, setSettingsOpen] = useState(false);
	const [requestedTaiwanSymbol, setRequestedTaiwanSymbol] = useState<ResearchRequest | null>(null);
	const [requestedPortfolioSymbol, setRequestedPortfolioSymbol] = useState<PortfolioRequest | null>(null);
	const taiwanSymbolRequestNonce = useRef(0);
	const portfolioSymbolRequestNonce = useRef(0);

	useEffect(() => {
		resolveBackendConfig().then(setConfig).catch((reason) => {
			setConfigError(reason instanceof Error ? reason.message : '後端設定失敗');
		});
	}, []);

	const switchWorkspace = (mode: WorkspaceMode) => {
		setWorkspaceMode(mode);
		window.history?.replaceState?.(null, '', '#' + mode);
	};
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
	const refresh = () => workspaceMode === 'ai'
		? setAIRefreshKey((value) => value + 1)
		: setMarketRefreshKey((value) => value + 1);

	const workspace = workspaceMode === 'taiwan-dashboard' ? <TaiwanDailyDashboard config={config} refreshKey={marketRefreshKey} onNavigate={switchWorkspace} onOpenResearch={openTaiwanStockResearch} onOpenResearchHistory={openTaiwanResearchHistory} /> : workspaceMode === 'taiwan-overview' ? <TaiwanMarketWorkspace key={workspaceMode} config={config} refreshKey={marketRefreshKey} view="overview" onNavigate={switchWorkspace} /> : workspaceMode === 'taiwan-breadth' ? <TaiwanMarketWorkspace key={workspaceMode} config={config} refreshKey={marketRefreshKey} view="breadth" /> : workspaceMode === 'taiwan-emotion' ? <TaiwanMarketWorkspace key={workspaceMode} config={config} refreshKey={marketRefreshKey} view="emotion" /> : workspaceMode === 'taiwan-industry' ? <TaiwanMarketWorkspace key={workspaceMode} config={config} refreshKey={marketRefreshKey} view="industry" /> : workspaceMode === 'taiwan-screener' ? <TaiwanScreenerWorkspace config={config} refreshKey={marketRefreshKey} onOpenResearch={openTaiwanStockResearch} /> : workspaceMode === 'taiwan-stock' ? <TaiwanStockResearchErrorBoundary><TaiwanStockResearchWorkspace config={config} refreshKey={marketRefreshKey} externalSymbolRequest={requestedTaiwanSymbol} /></TaiwanStockResearchErrorBoundary> : workspaceMode === 'taiwan-watchlist' ? <TaiwanWatchlistWorkspace config={config} refreshKey={marketRefreshKey} onOpenResearch={openTaiwanStockResearch} onAddPortfolio={openTaiwanPortfolio} /> : workspaceMode === 'taiwan-portfolio' ? <TaiwanPortfolioWorkspace config={config} refreshKey={marketRefreshKey} onOpenResearch={openTaiwanStockResearch} externalSymbolRequest={requestedPortfolioSymbol} /> : workspaceMode === 'taiwan-alerts' ? <div className="taiwan-product-workspace"><TaiwanEventAlertCenter config={config} refreshKey={marketRefreshKey} defaultOpen /></div> : <AIChatWorkspace config={config} refreshKey={aiRefreshKey} initialPrompt="" onInitialPromptConsumed={() => undefined} onOpenSettings={() => setSettingsOpen(true)} />;

	return <>
		<ApplicationFrame workspace={workspaceMode} sidebarExpanded={sidebarExpanded} config={config} configError={configError} onWorkspaceChange={switchWorkspace} onSidebarToggle={() => setSidebarExpanded((value) => !value)} onSettingsOpen={() => setSettingsOpen(true)} onRefresh={refresh}>
			{workspace}
		</ApplicationFrame>
		<SettingsDrawer config={config} open={settingsOpen} onClose={() => setSettingsOpen(false)} onSaved={() => { setAIRefreshKey((value) => value + 1); setMarketRefreshKey((value) => value + 1); }} />
	</>;
}

function initialWorkspace(): WorkspaceMode {
	if (window.location.hash === '#ai') return 'ai';
	return resolveTaiwanWorkspace(window.location.hash);
}
