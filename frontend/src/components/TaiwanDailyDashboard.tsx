import { AlertTriangle, BarChart3, Bell, BookOpen, BriefcaseBusiness, LoaderCircle, RefreshCw, Star } from 'lucide-react';
import { useEffect, useRef, useState, type ReactNode } from 'react';
import { requestJSON, type BackendConfig } from '../lib/backend';
import { formatTaiwanPercent, formatTaiwanRatio, formatTaiwanTWD, taiwanErrorMessage, taiwanStatusLabel } from '../lib/taiwan-product';

type Status = 'available' | 'stale' | 'partial' | 'unavailable' | 'not_queried';
type Section = { status: Status; as_of: string | null; reason?: string };
type Movement = { canonical: string; name: string; price: number | null; change_percent: number | null; status: Status; as_of: string | null };
export type Dashboard = {
	generated_at: string;
	market: Section & { indexes: { id: string; name: string; price: number; change: number; change_percent: number; status: Status; as_of: string | null }[]; advancers: number | null; decliners: number | null; advance_ratio: number | null; turnover_twd: number | null; emotion?: string; industries: { id: string; name: string; relative_breadth: number | null; relative_capital: number | null }[] };
	portfolio: Section & { holdings_count: number; priced_count: number; unavailable_count: number; total_market_value: number | null; total_unrealized_pl: number | null; largest_weight_percent: number | null; top_3_percent: number | null; industries: { industry: string; weight_percent: number }[]; movers: Movement[] };
	watchlist: Section & { count: number; available_count: number; unavailable_count: number; movers: Movement[] };
	alerts: Section & { unread_count: number; items: { id: number; canonical: string; security_name: string; title: string; published_at?: string; source: string; source_url?: string; stale: boolean; partial: boolean }[] };
	research: Section & { items: { run_id: string; canonical: string; security_name: string; created_at: string; evidence_as_of?: string; completeness: string; stale: boolean; partial: boolean; has_previous: boolean; comparison?: { changed: number; newly_available: number; no_longer_available: number; stale: number; partial: number; unavailable: number; corporate_events_added: number } }[] };
	attention: { reason_code: string; priority: number; canonical?: string; title: string; target: string }[];
};

export class LatestDashboardRequestGate {
	private revision = 0;
	begin() { this.revision += 1; return this.revision; }
	invalidate() { this.revision += 1; }
	isCurrent(value: number) { return value === this.revision; }
}

function researchChangeCount(item: Dashboard['research']['items'][number]) {
	const summary = item.comparison;
	return summary ? summary.changed + summary.newly_available + summary.no_longer_available + summary.stale + summary.partial + summary.unavailable + summary.corporate_events_added : 0;
}

type DashboardTarget = 'taiwan-overview' | 'taiwan-breadth' | 'taiwan-industry' | 'taiwan-watchlist' | 'taiwan-portfolio' | 'taiwan-alerts' | 'taiwan-stock';

const reasonLabels: Record<string, string> = {
	new_corporate_event: '新公司事件', price_unavailable: '價格無法取得', stale_market_data: '市場資料較舊',
	market_data_unavailable: '市場資料無法取得', research_evidence_changed: '研究證據變化', partial_data: '部分資料', portfolio_concentration: '持股集中度',
};

function StatusBadge({ section }: { section: Section }) {
	return <span className={`taiwan-status ${section.status}`}>{taiwanStatusLabel(section.status)}</span>;
}

function SectionHeader({ icon, title, section, onOpen }: { icon: ReactNode; title: string; section: Section; onOpen: () => void }) {
	return <header><div>{icon}<span><strong>{title}</strong>{section.as_of && <small>資料時間 {section.as_of}</small>}</span><StatusBadge section={section} /></div><button type="button" onClick={onOpen}>查看詳情</button></header>;
}

function MovementList({ items, onOpenResearch }: { items: Movement[]; onOpenResearch: (canonical: string) => void }) {
	if (items.length === 0) return <p className="taiwan-dashboard-empty">目前沒有可排序的市場變動。</p>;
	return <ul className="taiwan-dashboard-list">{items.map((item) => <li key={item.canonical}><button type="button" onClick={() => onOpenResearch(item.canonical)}><span><strong>{item.name}</strong><small>{item.canonical}</small></span><b className={(item.change_percent || 0) >= 0 ? 'up' : 'down'}>{formatTaiwanPercent(item.change_percent, true)}</b></button></li>)}</ul>;
}

export function TaiwanDailyDashboard({ config, refreshKey, onNavigate, onOpenResearch, onOpenResearchHistory = onOpenResearch }: { config: BackendConfig | null; refreshKey: number; onNavigate: (target: DashboardTarget) => void; onOpenResearch: (canonical: string) => void; onOpenResearchHistory?: (canonical: string) => void }) {
	const [data, setData] = useState<Dashboard | null>(null);
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState('');
	const requestGate = useRef(new LatestDashboardRequestGate());

	const load = async (syncEvents = false) => {
		if (!config) return;
		const revision = requestGate.current.begin();
		setLoading(true); setError('');
		try {
			let syncWarning = '';
			if (syncEvents) {
				// Reuse the existing provider-neutral Watchlist event sync. A provider
				// outage must not hide the rest of the dashboard refresh.
				try { await requestJSON(config, '/api/v1/tw/corporate-events/sync', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}' }); } catch { syncWarning = '事件同步暫時無法完成；其他總覽資料已更新。'; }
			}
			const response = await requestJSON<{ data: Dashboard }>(config, '/api/v1/tw/dashboard');
			if (requestGate.current.isCurrent(revision)) { setData(response.data); setError(syncWarning); }
		} catch (reason) {
			if (requestGate.current.isCurrent(revision)) setError(taiwanErrorMessage(reason, '今日總覽暫時無法取得'));
		} finally {
			if (requestGate.current.isCurrent(revision)) setLoading(false);
		}
	};

	useEffect(() => { void load(); return () => { requestGate.current.invalidate(); }; /* eslint-disable-next-line react-hooks/exhaustive-deps */ }, [config, refreshKey]);

	const markAlertRead = async (id: number) => {
		if (!config || !data) return;
		try {
			await requestJSON(config, `/api/v1/tw/alerts/${id}/read`, { method: 'PUT' });
			await load();
		} catch (reason) {
			setError(taiwanErrorMessage(reason, '標記事件已讀失敗'));
		}
	};

	return <TaiwanDailyDashboardView data={data} loading={loading} error={error} onNavigate={onNavigate} onOpenResearch={onOpenResearch} onOpenResearchHistory={onOpenResearchHistory} onRefresh={() => void load(true)} onRetry={() => void load()} onMarkAlertRead={(id) => void markAlertRead(id)} />;
}

export function TaiwanDailyDashboardView({ data, loading, error, onNavigate, onOpenResearch, onOpenResearchHistory, onRefresh, onRetry, onMarkAlertRead }: { data: Dashboard | null; loading: boolean; error: string; onNavigate: (target: DashboardTarget) => void; onOpenResearch: (canonical: string) => void; onOpenResearchHistory: (canonical: string) => void; onRefresh: () => void; onRetry: () => void; onMarkAlertRead: (id: number) => void }) {
	return <div className="taiwan-product-workspace taiwan-dashboard" aria-busy={loading}>
		<div className="taiwan-dashboard-toolbar"><div><strong>台股今日總覽</strong><span>資訊整合與導覽，不構成投資建議</span></div><button type="button" onClick={onRefresh} disabled={loading}><RefreshCw className={loading ? 'spin' : ''} size={15} />重新整理</button></div>
		{loading && !data && <div className="taiwan-loading"><LoaderCircle className="spin" size={18} />正在整理市場、持股、自選股與研究活動</div>}
		{error && <div className="market-partial-warning" role="alert">{error}<button type="button" onClick={onRetry}>重試</button></div>}
		{data && <>
			<section className="taiwan-dashboard-card market"><SectionHeader icon={<BarChart3 size={16} />} title="今日市場" section={data.market} onOpen={() => onNavigate('taiwan-overview')} />
				<div className="taiwan-dashboard-metrics"><div><span>加權／主要指數</span><strong>{data.market.indexes[0]?.price?.toLocaleString('zh-TW') ?? '—'}</strong><small>{data.market.indexes[0] ? `${formatTaiwanPercent(data.market.indexes[0].change_percent, true)} · ${taiwanStatusLabel(data.market.indexes[0].status)} · ${data.market.indexes[0].as_of || '時間未提供'}` : '無法取得'}</small></div><div><span>上漲／下跌</span><strong>{data.market.advancers ?? '—'} / {data.market.decliners ?? '—'}</strong><small>上漲比 {formatTaiwanRatio(data.market.advance_ratio)}</small></div><div><span>市場狀態</span><strong>{taiwanStatusLabel(data.market.emotion)}</strong><small>成交額 {formatTaiwanTWD(data.market.turnover_twd)}</small></div></div>
				{data.market.industries.length > 0 && <p className="taiwan-dashboard-industries">產業相對廣度：{data.market.industries.map((item) => item.name).join('、')}</p>}
			</section>

			<section className="taiwan-dashboard-card"><SectionHeader icon={<BriefcaseBusiness size={16} />} title="我的持股" section={data.portfolio} onOpen={() => onNavigate('taiwan-portfolio')} />
				<div className="taiwan-dashboard-metrics"><div><span>總市值</span><strong>{formatTaiwanTWD(data.portfolio.total_market_value)}</strong><small>未實現損益 {formatTaiwanTWD(data.portfolio.total_unrealized_pl)}</small></div><div><span>可取得價格</span><strong>{data.portfolio.priced_count} / {data.portfolio.holdings_count}</strong><small>缺少 {data.portfolio.unavailable_count} 檔</small></div><div><span>集中度</span><strong>{formatTaiwanPercent(data.portfolio.largest_weight_percent)}</strong><small>Top 3 {formatTaiwanPercent(data.portfolio.top_3_percent)}</small></div></div>
				{data.portfolio.industries.length > 0 && <p className="taiwan-dashboard-industries">主要產業：{data.portfolio.industries.slice(0, 3).map((item) => `${item.industry} ${formatTaiwanPercent(item.weight_percent)}`).join('、')}</p>}
				<MovementList items={data.portfolio.movers} onOpenResearch={onOpenResearch} />
			</section>

			<section className="taiwan-dashboard-card"><SectionHeader icon={<Star size={16} />} title="自選股" section={data.watchlist} onOpen={() => onNavigate('taiwan-watchlist')} /><p className="taiwan-dashboard-meta">{data.watchlist.count} 檔 · {data.watchlist.unavailable_count > 0 ? `${data.watchlist.unavailable_count} 檔資料無法取得` : '價格資料可用'}</p><MovementList items={data.watchlist.movers} onOpenResearch={onOpenResearch} /></section>

			<section className="taiwan-dashboard-card"><SectionHeader icon={<Bell size={16} />} title="事件提醒" section={data.alerts} onOpen={() => onNavigate('taiwan-alerts')} /><p className="taiwan-dashboard-meta">未讀 {data.alerts.unread_count} 筆</p>{data.alerts.items.length === 0 ? <p className="taiwan-dashboard-empty">目前沒有未讀事件。</p> : <ul className="taiwan-dashboard-list">{data.alerts.items.map((item) => <li key={item.id}><button type="button" onClick={() => onOpenResearch(item.canonical)}><span><strong>{item.security_name} <small>{item.canonical}</small></strong><small>{item.title}</small><small>{item.published_at ? new Date(item.published_at).toLocaleString('zh-TW', { hour12: false }) : '發布時間未提供'} · {item.source || '來源未提供'}{item.stale ? ' · 資料較舊' : ''}{item.partial ? ' · 部分資料' : ''}</small></span></button>{item.source_url && <a className="compact" href={item.source_url} target="_blank" rel="noreferrer">來源</a>}<button type="button" className="compact" onClick={() => onMarkAlertRead(item.id)}>標記已讀</button></li>)}</ul>}</section>

			<section className="taiwan-dashboard-card"><SectionHeader icon={<BookOpen size={16} />} title="研究更新" section={data.research} onOpen={() => data.research.items[0] ? onOpenResearchHistory(data.research.items[0].canonical) : onNavigate('taiwan-stock')} />{data.research.items.length === 0 ? <p className="taiwan-dashboard-empty">尚無已完成的 AI 研究。</p> : <ul className="taiwan-dashboard-list">{data.research.items.map((item) => <li key={item.run_id}><button type="button" onClick={() => onOpenResearchHistory(item.canonical)}><span><strong>{item.security_name}</strong><small>{new Date(item.created_at).toLocaleString('zh-TW', { hour12: false })} · {item.completeness}{item.has_previous ? ' · 可與前次比較' : ''}{item.stale ? ' · 資料較舊' : ''}{item.partial ? ' · 部分資料' : ''}</small></span>{item.comparison && <b>{researchChangeCount(item)} 項證據狀態變化</b>}</button></li>)}</ul>}</section>

			<section className="taiwan-dashboard-card attention"><SectionHeader icon={<AlertTriangle size={16} />} title="需要注意" section={{ status: data.attention.length ? 'available' : 'not_queried', as_of: null }} onOpen={() => onNavigate('taiwan-alerts')} />{data.attention.length === 0 ? <p className="taiwan-dashboard-empty">目前沒有需要注意的資料狀態。</p> : <ul className="taiwan-dashboard-list">{data.attention.map((item, index) => <li key={`${item.reason_code}-${item.canonical || index}`}><button type="button" onClick={() => item.target === 'taiwan-stock' && item.canonical ? onOpenResearch(item.canonical) : onNavigate(item.target as DashboardTarget)}><span><strong>{reasonLabels[item.reason_code] || item.reason_code}</strong><small>{item.title}</small></span><b>P{item.priority}</b></button></li>)}</ul>}</section>
			<footer className="taiwan-dashboard-freshness">各區保留自己的資料時間；不以總覽產生時間取代來源 as-of。總覽整理於 {new Date(data.generated_at).toLocaleString('zh-TW', { hour12: false })}</footer>
		</>}
	</div>;
}
