import { ChevronDown, ChevronUp, LoaderCircle, Star } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import type { BackendConfig } from '../lib/backend';
import { requestJSON } from '../lib/backend';
import {
	addTaiwanWatchlistSecurity, fetchTaiwanWatchlist, formatTaiwanPercent, formatTaiwanPlainNumber, formatTaiwanRatioPercent, formatTaiwanRevenueTWD, formatTaiwanSignedShares, formatTaiwanTWD,
	removeTaiwanWatchlistSecurity, runScopedRequest, taiwanErrorMessage, taiwanFinancialsStatusLabel, taiwanScopes, taiwanScreenerDefaultFilters, taiwanScreenerHasDividendCriteria, taiwanScreenerHasFinancialsCriteria, taiwanScreenerHasInstitutionalCriteria,
	taiwanScreenerHasMarginCriteria, taiwanScreenerHasRevenueCriteria, taiwanScreenerHasValuationCriteria, taiwanScreenerOrderOptions, taiwanScreenerPath, taiwanScreenerSortOptions, taiwanSecurityTypeLabel, taiwanStatusLabel,
	validateTaiwanScreenerFilters, type TaiwanScreenerFilters, type TaiwanScreenerResponse, type TaiwanScreenerSecurity,
} from '../lib/taiwan-product';

export type WatchlistMembershipState = 'idle' | 'loading' | 'ready' | 'error';

// M7B — Taiwan Screener. Loads/screens/sorts/paginates entirely through the existing M7A backend
// contract (GET /api/v1/tw/screener); this component never fetches the full market and filters it
// locally. All form controls are draft state — nothing here issues a request until 套用條件 is
// clicked, so typing alone can never fan out requests. `applied` is the only state the fetch effect
// depends on, so Apply, Clear, pagination, and the global refresh all funnel through one request
// path, and runScopedRequest (the same guard used across every other Taiwan view) guarantees a
// slower stale response can never overwrite a newer one.
//
// M7C — Watchlist integration. Membership is loaded ONCE per mount/refresh via the existing
// fetchTaiwanWatchlist(config) — deliberately its own effect depending only on [config, refreshKey],
// never on `applied`, so changing filters/sort/scope/page never re-fetches the Watchlist. Add/remove
// reuse the existing addTaiwanWatchlistSecurity/removeTaiwanWatchlistSecurity helpers with a
// pessimistic update (local Set only changes after the server call succeeds), matching the same
// pattern already used in TaiwanMarketView/TaiwanStockResearchWorkspace — no new Watchlist contract,
// no per-row membership or quote requests.
//
// M7D — institutional/margin filters. Still exactly ONE `GET /api/v1/tw/screener` request per load
// (the backend already decides internally whether to fetch institutional/margin — this component
// never calls a separate endpoint for them, never knows about provider internals). Result
// presentation (which advanced columns to show) is derived from `applied` (never `draft`), so typing
// into an advanced field never changes what is currently displayed before Apply.
export function TaiwanScreenerWorkspace({ config, refreshKey, onOpenResearch }: { config: BackendConfig | null; refreshKey: number; onOpenResearch: (canonical: string) => void }) {
	const [draft, setDraft] = useState<TaiwanScreenerFilters>(taiwanScreenerDefaultFilters);
	const [applied, setApplied] = useState<TaiwanScreenerFilters>(taiwanScreenerDefaultFilters);
	const [localError, setLocalError] = useState('');
	const [data, setData] = useState<TaiwanScreenerResponse | null>(null);
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState('');
	const requestID = useRef(0);

	const [watchlistState, setWatchlistState] = useState<WatchlistMembershipState>('idle');
	const [watchlistedCanonicals, setWatchlistedCanonicals] = useState<Set<string>>(new Set());
	const [busyCanonicals, setBusyCanonicals] = useState<Set<string>>(new Set());
	const [mutationErrors, setMutationErrors] = useState<Record<string, string>>({});
	const watchlistRequestID = useRef(0);

	// Fetches only while this workspace is mounted, and only in reaction to `applied` changing (Apply,
	// Clear, pagination) or an explicit global refresh — never on a timer, never merely because the
	// filter panel re-rendered.
	useEffect(() => {
		if (!config) return;
		void runScopedRequest(requestID, () => requestJSON<{ data: TaiwanScreenerResponse }>(config, taiwanScreenerPath(applied)), {
			onStart: () => { setData(null); setLoading(true); setError(''); },
			onSuccess: (payload) => setData(payload.data),
			onError: (reason) => { setData(null); setError(taiwanErrorMessage(reason, '台股選股資料載入失敗')); },
			onSettle: () => setLoading(false),
		});
	}, [config, refreshKey, applied]);

	// Watchlist membership: independent of `applied` on purpose — depends only on [config, refreshKey]
	// so it loads once per mount and once per explicit global refresh, never per filter/sort/page
	// change. A load failure never touches Screener data/error state above — it only moves this state
	// to 'error', which the row-level rendering below treats as "unknown", never as "not saved".
	useEffect(() => {
		if (!config) return;
		void runScopedRequest(watchlistRequestID, () => fetchTaiwanWatchlist(config), {
			onStart: () => setWatchlistState('loading'),
			onSuccess: (list) => { setWatchlistedCanonicals(new Set(list.map((item) => item.canonical))); setWatchlistState('ready'); },
			onError: () => setWatchlistState('error'),
		});
	}, [config, refreshKey]);

	const applyFilters = () => {
		const message = validateTaiwanScreenerFilters(draft);
		if (message) { setLocalError(message); return; }
		setLocalError('');
		setApplied({ ...draft, offset: 0 });
	};

	const clearFilters = () => {
		const defaults = taiwanScreenerDefaultFilters();
		setDraft(defaults);
		setLocalError('');
		setApplied(defaults);
	};

	const goPrevious = () => {
		if (applied.offset <= 0) return;
		setApplied((current) => ({ ...current, offset: Math.max(0, current.offset - current.limit) }));
	};

	const goNext = () => {
		if (!data || applied.offset + data.securities.length >= data.total) return;
		setApplied((current) => ({ ...current, offset: current.offset + current.limit }));
	};

	// Pessimistic add/remove: the local Set only changes after the server call actually succeeds, so
	// a failure always leaves the prior confirmed membership in place — never a false optimistic state.
	// The busy-set guard at entry (mirrored by disabling the button while busy) makes a second click
	// for the same canonical a no-op until the first mutation settles, so POST/DELETE can never race
	// for one security.
	const toggleWatchlist = async (canonical: string) => {
		if (!config || watchlistState !== 'ready' || busyCanonicals.has(canonical)) return;
		const saved = watchlistedCanonicals.has(canonical);
		setBusyCanonicals((current) => new Set(current).add(canonical));
		setMutationErrors((current) => {
			if (!(canonical in current)) return current;
			const next = { ...current };
			delete next[canonical];
			return next;
		});
		try {
			if (saved) {
				await removeTaiwanWatchlistSecurity(config, canonical);
				setWatchlistedCanonicals((current) => { const next = new Set(current); next.delete(canonical); return next; });
			} else {
				await addTaiwanWatchlistSecurity(config, canonical);
				setWatchlistedCanonicals((current) => new Set(current).add(canonical));
			}
		} catch (reason) {
			setMutationErrors((current) => ({ ...current, [canonical]: taiwanErrorMessage(reason, saved ? '移除自選失敗' : '加入自選失敗') }));
		} finally {
			setBusyCanonicals((current) => { const next = new Set(current); next.delete(canonical); return next; });
		}
	};

	// M7D/M7E-A — result presentation is derived from APPLIED filters only, never draft: typing into
	// an advanced field must never change what is currently displayed before the user clicks Apply.
	const showInstitutional = taiwanScreenerHasInstitutionalCriteria(applied);
	const showMargin = taiwanScreenerHasMarginCriteria(applied);
	const showRevenue = taiwanScreenerHasRevenueCriteria(applied);
	const showValuation = taiwanScreenerHasValuationCriteria(applied);
	const showDividends = taiwanScreenerHasDividendCriteria(applied);
	const showFinancials = taiwanScreenerHasFinancialsCriteria(applied);

	return <div className="taiwan-product-workspace taiwan-screener-workspace">
		<ScreenerFilterPanel draft={draft} onChange={setDraft} onApply={applyFilters} onClear={clearFilters} localError={localError} />
		{error && <div className="market-partial-warning">{error}</div>}
		{watchlistState === 'error' && <div className="market-partial-warning">自選股狀態暫時無法取得</div>}
		{data && showInstitutional && <ScreenerDomainFreshness label="法人資料" asOf={data.institutional_as_of} status={data.institutional_status} unavailableMessage="法人資料暫時無法取得" />}
		{data && showMargin && <ScreenerDomainFreshness label="融資融券資料" asOf={data.margin_as_of} status={data.margin_status} unavailableMessage="融資融券資料暫時無法取得" />}
		{data && showRevenue && <ScreenerDomainFreshness label="營收資料" asOf={data.revenue_as_of} status={data.revenue_status} unavailableMessage="營收資料暫時無法取得" />}
		{data && showValuation && <ScreenerDomainFreshness label="估值資料" asOf={data.valuation_as_of} status={data.valuation_status} unavailableMessage="估值資料暫時無法取得" />}
		{data && showDividends && <ScreenerDomainFreshness label="股利資料" asOf={data.dividends_as_of} status={data.dividends_status} unavailableMessage="股利資料暫時無法取得" />}
		{data && showFinancials && <ScreenerFinancialsFreshness period={data.financials_period} status={data.financials_status} />}
		{loading && <div className="taiwan-loading"><LoaderCircle className="spin" size={18} />正在讀取台股選股資料</div>}
		{data && <ScreenerSummary data={data} />}
		{data && data.total === 0 && <div className="taiwan-empty-state"><strong>沒有符合目前條件的台灣證券</strong><p>可放寬篩選條件或按下「清除條件」查看全部結果。</p></div>}
		{data && data.securities.length > 0 && <ScreenerTable
			securities={data.securities} onOpenResearch={onOpenResearch}
			watchlistState={watchlistState} watchlistedCanonicals={watchlistedCanonicals}
			busyCanonicals={busyCanonicals} mutationErrors={mutationErrors}
			onToggleWatchlist={(canonical) => void toggleWatchlist(canonical)}
			showInstitutional={showInstitutional} showMargin={showMargin}
			showRevenue={showRevenue} showValuation={showValuation} showDividends={showDividends}
			showFinancials={showFinancials}
		/>}
		{data && <ScreenerPagination data={data} loading={loading} onPrevious={goPrevious} onNext={goNext} />}
	</div>;
}

// Pure/presentational: previous/next disabled purely from `data` (offset/total/securities.length) —
// no hidden state of its own, so its enabled/disabled logic is directly testable without hooks.
export function ScreenerPagination({ data, loading, onPrevious, onNext }: { data: TaiwanScreenerResponse; loading: boolean; onPrevious: () => void; onNext: () => void }) {
	const hasPrevious = data.offset > 0;
	const hasNext = data.offset + data.securities.length < data.total;
	return <div className="taiwan-screener-pagination" aria-label="選股結果分頁">
		<span>{data.total > 0 ? `第 ${data.offset + 1}–${data.offset + data.securities.length} 筆，共 ${data.total.toLocaleString('zh-TW')} 筆` : '共 0 筆'}</span>
		<div>
			<button type="button" disabled={!hasPrevious || loading} onClick={onPrevious}>上一頁</button>
			<button type="button" disabled={!hasNext || loading} onClick={onNext}>下一頁</button>
		</div>
	</div>;
}

// M7D — pure/presentational domain freshness/unavailable indicator. `status` undefined means the
// backend never returned that domain at all (not requested) — renders nothing, never a guess.
// 'unavailable' renders a safe warning instead of freshness text. Never shows published_at/
// available_at — only the existing trade-date as_of/status/days-behind the backend already exposes.
export function ScreenerDomainFreshness({ label, asOf, status, unavailableMessage }: { label: string; asOf?: string | null; status?: string; unavailableMessage: string }) {
	if (!status) return null;
	if (status === 'unavailable') return <div className="market-partial-warning">{unavailableMessage}</div>;
	return <div className="taiwan-screener-domain-freshness"><span>{label}：{asOf || '未提供'}</span><span className={`taiwan-status ${status}`}>{taiwanStatusLabel(status)}</span></div>;
}

// M7E-B — dedicated financial-statement domain freshness/status renderer. Distinct from
// ScreenerDomainFreshness above because financials_status has its own truthful 3-state vocabulary
// (available/partial/unavailable, via taiwanFinancialsStatusLabel — never the daily-cadence "最新"
// used elsewhere) and "partial" needs an additional non-blocking warning while STILL showing the
// domain period line (unlike "unavailable", which — matching the existing unavailable convention —
// replaces the line entirely with a warning). Successful rows render independently in the table
// regardless of this component's output; it never disables or hides anything.
export function ScreenerFinancialsFreshness({ period, status }: { period?: string | null; status?: string }) {
	if (!status) return null;
	if (status === 'unavailable') return <div className="market-partial-warning">財務報表資料目前無法取得。</div>;
	return <>
		<div className="taiwan-screener-domain-freshness">
			<span>財務報表：{period || '未提供'}</span>
			<span className={`taiwan-status ${status}`}>{taiwanFinancialsStatusLabel(status)}</span>
		</div>
		{status === 'partial' && <div className="market-partial-warning">部分財務報表資料暫時無法取得，已顯示目前可用資料。</div>}
	</>;
}

export function ScreenerFilterPanel({ draft, onChange, onApply, onClear, localError }: {
	draft: TaiwanScreenerFilters;
	onChange: (next: TaiwanScreenerFilters) => void;
	onApply: () => void;
	onClear: () => void;
	localError: string;
}) {
	// M7D/M7E-A — collapse/expand is local UI state only: it never touches draft/applied, never sends
	// a request, and is intentionally NOT reset by Clear (clearing filter values is a data concern;
	// whether a section is currently open is a display concern).
	const [institutionalExpanded, setInstitutionalExpanded] = useState(false);
	const [marginExpanded, setMarginExpanded] = useState(false);
	const [fundamentalsExpanded, setFundamentalsExpanded] = useState(false);
	const set = <K extends keyof TaiwanScreenerFilters>(key: K, value: TaiwanScreenerFilters[K]) => onChange({ ...draft, [key]: value });
	return <section className="taiwan-screener-filters">
		<div className="taiwan-screener-filter-grid">
			<label><span>市場</span><select value={draft.scope} onChange={(event) => set('scope', event.target.value as TaiwanScreenerFilters['scope'])}>{taiwanScopes.map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}</select></label>
			<label><span>最低價格</span><input type="number" inputMode="decimal" value={draft.minPrice} onChange={(event) => set('minPrice', event.target.value)} placeholder="不限" /></label>
			<label><span>最高價格</span><input type="number" inputMode="decimal" value={draft.maxPrice} onChange={(event) => set('maxPrice', event.target.value)} placeholder="不限" /></label>
			<label><span>最低漲跌幅 %</span><input type="number" inputMode="decimal" value={draft.minChangePercent} onChange={(event) => set('minChangePercent', event.target.value)} placeholder="不限" /></label>
			<label><span>最高漲跌幅 %</span><input type="number" inputMode="decimal" value={draft.maxChangePercent} onChange={(event) => set('maxChangePercent', event.target.value)} placeholder="不限" /></label>
			<label><span>最低成交量（股）</span><input type="number" inputMode="numeric" value={draft.minVolume} onChange={(event) => set('minVolume', event.target.value)} placeholder="不限" /></label>
			<label><span>最高成交量（股）</span><input type="number" inputMode="numeric" value={draft.maxVolume} onChange={(event) => set('maxVolume', event.target.value)} placeholder="不限" /></label>
			<label><span>最低成交金額（元）</span><input type="number" inputMode="numeric" value={draft.minAmount} onChange={(event) => set('minAmount', event.target.value)} placeholder="不限" /></label>
			<label><span>最高成交金額（元）</span><input type="number" inputMode="numeric" value={draft.maxAmount} onChange={(event) => set('maxAmount', event.target.value)} placeholder="不限" /></label>
			<label><span>排序欄位</span><select value={draft.sort} onChange={(event) => set('sort', event.target.value as TaiwanScreenerFilters['sort'])}>{taiwanScreenerSortOptions.map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}</select></label>
			<label><span>排序方向</span><select value={draft.order} onChange={(event) => set('order', event.target.value as TaiwanScreenerFilters['order'])}>{taiwanScreenerOrderOptions.map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}</select></label>
		</div>

		<div className="taiwan-screener-advanced-group">
			<button type="button" className="taiwan-screener-advanced-toggle" onClick={() => setInstitutionalExpanded((value) => !value)} aria-expanded={institutionalExpanded}>
				{institutionalExpanded ? <ChevronUp size={14} /> : <ChevronDown size={14} />}法人籌碼
			</button>
			{institutionalExpanded && <div className="taiwan-screener-filter-grid taiwan-screener-advanced-grid">
				<label><span>最低外資買賣超（股）</span><input type="number" inputMode="numeric" value={draft.minForeignNet} onChange={(event) => set('minForeignNet', event.target.value)} placeholder="不限" /></label>
				<label><span>最高外資買賣超（股）</span><input type="number" inputMode="numeric" value={draft.maxForeignNet} onChange={(event) => set('maxForeignNet', event.target.value)} placeholder="不限" /></label>
				<label><span>最低投信買賣超（股）</span><input type="number" inputMode="numeric" value={draft.minTrustNet} onChange={(event) => set('minTrustNet', event.target.value)} placeholder="不限" /></label>
				<label><span>最高投信買賣超（股）</span><input type="number" inputMode="numeric" value={draft.maxTrustNet} onChange={(event) => set('maxTrustNet', event.target.value)} placeholder="不限" /></label>
				<label><span>最低自營商買賣超（股）</span><input type="number" inputMode="numeric" value={draft.minDealerNet} onChange={(event) => set('minDealerNet', event.target.value)} placeholder="不限" /></label>
				<label><span>最高自營商買賣超（股）</span><input type="number" inputMode="numeric" value={draft.maxDealerNet} onChange={(event) => set('maxDealerNet', event.target.value)} placeholder="不限" /></label>
				<label><span>最低三大法人合計（股）</span><input type="number" inputMode="numeric" value={draft.minInstitutionalNet} onChange={(event) => set('minInstitutionalNet', event.target.value)} placeholder="不限" /></label>
				<label><span>最高三大法人合計（股）</span><input type="number" inputMode="numeric" value={draft.maxInstitutionalNet} onChange={(event) => set('maxInstitutionalNet', event.target.value)} placeholder="不限" /></label>
			</div>}
		</div>

		<div className="taiwan-screener-advanced-group">
			<button type="button" className="taiwan-screener-advanced-toggle" onClick={() => setMarginExpanded((value) => !value)} aria-expanded={marginExpanded}>
				{marginExpanded ? <ChevronUp size={14} /> : <ChevronDown size={14} />}融資融券
			</button>
			{marginExpanded && <div className="taiwan-screener-filter-grid taiwan-screener-advanced-grid">
				<label><span>最低融資餘額（股）</span><input type="number" inputMode="numeric" value={draft.minMarginBalance} onChange={(event) => set('minMarginBalance', event.target.value)} placeholder="不限" /></label>
				<label><span>最高融資餘額（股）</span><input type="number" inputMode="numeric" value={draft.maxMarginBalance} onChange={(event) => set('maxMarginBalance', event.target.value)} placeholder="不限" /></label>
				<label><span>最低融資增減（股）</span><input type="number" inputMode="numeric" value={draft.minMarginChange} onChange={(event) => set('minMarginChange', event.target.value)} placeholder="不限" /></label>
				<label><span>最高融資增減（股）</span><input type="number" inputMode="numeric" value={draft.maxMarginChange} onChange={(event) => set('maxMarginChange', event.target.value)} placeholder="不限" /></label>
				<label><span>最低融券餘額（股）</span><input type="number" inputMode="numeric" value={draft.minShortBalance} onChange={(event) => set('minShortBalance', event.target.value)} placeholder="不限" /></label>
				<label><span>最高融券餘額（股）</span><input type="number" inputMode="numeric" value={draft.maxShortBalance} onChange={(event) => set('maxShortBalance', event.target.value)} placeholder="不限" /></label>
				<label><span>最低融券增減（股）</span><input type="number" inputMode="numeric" value={draft.minShortChange} onChange={(event) => set('minShortChange', event.target.value)} placeholder="不限" /></label>
				<label><span>最高融券增減（股）</span><input type="number" inputMode="numeric" value={draft.maxShortChange} onChange={(event) => set('maxShortChange', event.target.value)} placeholder="不限" /></label>
				<label><span>最低券資比（%）</span><input type="number" inputMode="decimal" value={draft.minShortMarginRatio} onChange={(event) => set('minShortMarginRatio', event.target.value)} placeholder="不限" /></label>
				<label><span>最高券資比（%）</span><input type="number" inputMode="decimal" value={draft.maxShortMarginRatio} onChange={(event) => set('maxShortMarginRatio', event.target.value)} placeholder="不限" /></label>
			</div>}
		</div>

		<div className="taiwan-screener-advanced-group">
			<button type="button" className="taiwan-screener-advanced-toggle" onClick={() => setFundamentalsExpanded((value) => !value)} aria-expanded={fundamentalsExpanded}>
				{fundamentalsExpanded ? <ChevronUp size={14} /> : <ChevronDown size={14} />}基本面
			</button>
			{fundamentalsExpanded && <>
				<h4 className="taiwan-screener-advanced-subheading">營收</h4>
				<div className="taiwan-screener-filter-grid taiwan-screener-advanced-grid">
					<label><span>最低月營收（元）</span><input type="number" inputMode="numeric" value={draft.minMonthlyRevenue} onChange={(event) => set('minMonthlyRevenue', event.target.value)} placeholder="不限" /></label>
					<label><span>最高月營收（元）</span><input type="number" inputMode="numeric" value={draft.maxMonthlyRevenue} onChange={(event) => set('maxMonthlyRevenue', event.target.value)} placeholder="不限" /></label>
					<label><span>最低月營收年增率（%）</span><input type="number" inputMode="decimal" value={draft.minRevenueYoY} onChange={(event) => set('minRevenueYoY', event.target.value)} placeholder="不限" /></label>
					<label><span>最高月營收年增率（%）</span><input type="number" inputMode="decimal" value={draft.maxRevenueYoY} onChange={(event) => set('maxRevenueYoY', event.target.value)} placeholder="不限" /></label>
				</div>
				<h4 className="taiwan-screener-advanced-subheading">估值</h4>
				<div className="taiwan-screener-filter-grid taiwan-screener-advanced-grid">
					<label><span>最低本益比（PE）</span><input type="number" inputMode="decimal" value={draft.minPE} onChange={(event) => set('minPE', event.target.value)} placeholder="不限" /></label>
					<label><span>最高本益比（PE）</span><input type="number" inputMode="decimal" value={draft.maxPE} onChange={(event) => set('maxPE', event.target.value)} placeholder="不限" /></label>
					<label><span>最低股價淨值比（PB）</span><input type="number" inputMode="decimal" value={draft.minPB} onChange={(event) => set('minPB', event.target.value)} placeholder="不限" /></label>
					<label><span>最高股價淨值比（PB）</span><input type="number" inputMode="decimal" value={draft.maxPB} onChange={(event) => set('maxPB', event.target.value)} placeholder="不限" /></label>
					<label><span>最低殖利率（%）</span><input type="number" inputMode="decimal" value={draft.minDividendYield} onChange={(event) => set('minDividendYield', event.target.value)} placeholder="不限" /></label>
					<label><span>最高殖利率（%）</span><input type="number" inputMode="decimal" value={draft.maxDividendYield} onChange={(event) => set('maxDividendYield', event.target.value)} placeholder="不限" /></label>
				</div>
				<h4 className="taiwan-screener-advanced-subheading">股利</h4>
				<div className="taiwan-screener-filter-grid taiwan-screener-advanced-grid">
					<label><span>最低現金股利</span><input type="number" inputMode="decimal" value={draft.minCashDividend} onChange={(event) => set('minCashDividend', event.target.value)} placeholder="不限" /></label>
					<label><span>最高現金股利</span><input type="number" inputMode="decimal" value={draft.maxCashDividend} onChange={(event) => set('maxCashDividend', event.target.value)} placeholder="不限" /></label>
					<label><span>最低股票股利</span><input type="number" inputMode="decimal" value={draft.minStockDividend} onChange={(event) => set('minStockDividend', event.target.value)} placeholder="不限" /></label>
					<label><span>最高股票股利</span><input type="number" inputMode="decimal" value={draft.maxStockDividend} onChange={(event) => set('maxStockDividend', event.target.value)} placeholder="不限" /></label>
					<label><span>最低合計股利</span><input type="number" inputMode="decimal" value={draft.minTotalDividend} onChange={(event) => set('minTotalDividend', event.target.value)} placeholder="不限" /></label>
					<label><span>最高合計股利</span><input type="number" inputMode="decimal" value={draft.maxTotalDividend} onChange={(event) => set('maxTotalDividend', event.target.value)} placeholder="不限" /></label>
				</div>
				<h4 className="taiwan-screener-advanced-subheading">財務報表</h4>
				<div className="taiwan-screener-filter-grid taiwan-screener-advanced-grid">
					<label><span>累計 EPS 最小</span><input type="number" inputMode="decimal" value={draft.minCumulativeEPS} onChange={(event) => set('minCumulativeEPS', event.target.value)} placeholder="不限" /></label>
					<label><span>累計 EPS 最大</span><input type="number" inputMode="decimal" value={draft.maxCumulativeEPS} onChange={(event) => set('maxCumulativeEPS', event.target.value)} placeholder="不限" /></label>
					<label><span>毛利率最小（%）</span><input type="number" inputMode="decimal" value={draft.minGrossMargin} onChange={(event) => set('minGrossMargin', event.target.value)} placeholder="不限" /></label>
					<label><span>毛利率最大（%）</span><input type="number" inputMode="decimal" value={draft.maxGrossMargin} onChange={(event) => set('maxGrossMargin', event.target.value)} placeholder="不限" /></label>
					<label><span>營業利益率最小（%）</span><input type="number" inputMode="decimal" value={draft.minOperatingMargin} onChange={(event) => set('minOperatingMargin', event.target.value)} placeholder="不限" /></label>
					<label><span>營業利益率最大（%）</span><input type="number" inputMode="decimal" value={draft.maxOperatingMargin} onChange={(event) => set('maxOperatingMargin', event.target.value)} placeholder="不限" /></label>
				</div>
				<p className="taiwan-screener-advanced-note">部分金融相關產業不提供毛利率／營業利益率。</p>
			</>}
		</div>

		{localError && <p className="taiwan-screener-filter-error">{localError}</p>}
		<div className="taiwan-screener-filter-actions">
			<button type="button" className="taiwan-screener-apply" onClick={onApply}>套用條件</button>
			<button type="button" className="taiwan-screener-clear" onClick={onClear}>清除條件</button>
		</div>
	</section>;
}

export function ScreenerSummary({ data }: { data: TaiwanScreenerResponse }) {
	return <section className="taiwan-status-card"><div><strong>{data.scope}</strong><span className={`taiwan-status ${data.freshness}`}>{taiwanStatusLabel(data.freshness)}</span><span>{data.total.toLocaleString('zh-TW')} 檔符合條件</span></div><small>資料日期 {data.as_of || '未提供'}</small></section>;
}

export function ScreenerTable({ securities, onOpenResearch, watchlistState, watchlistedCanonicals, busyCanonicals, mutationErrors, onToggleWatchlist, showInstitutional, showMargin, showRevenue, showValuation, showDividends, showFinancials }: {
	securities: TaiwanScreenerSecurity[];
	onOpenResearch: (canonical: string) => void;
	watchlistState: WatchlistMembershipState;
	watchlistedCanonicals: Set<string>;
	busyCanonicals: Set<string>;
	mutationErrors: Record<string, string>;
	onToggleWatchlist: (canonical: string) => void;
	showInstitutional: boolean;
	showMargin: boolean;
	showRevenue: boolean;
	showValuation: boolean;
	showDividends: boolean;
	showFinancials: boolean;
}) {
	const showAdvanced = showInstitutional || showMargin || showRevenue || showValuation || showDividends || showFinancials;
	return <div className="taiwan-screener-table-wrap"><table className="taiwan-screener-table">
		<thead><tr><th>證券</th><th>市場</th><th>價格</th><th>漲跌幅</th><th>成交量</th><th>成交金額</th><th>資料日期</th><th>自選</th>{showAdvanced && <th>進階資料</th>}</tr></thead>
		<tbody>{securities.map((item) => <ScreenerRow
			key={item.canonical} security={item} onOpen={() => onOpenResearch(item.canonical)}
			membershipState={watchlistState} saved={watchlistedCanonicals.has(item.canonical)}
			busy={busyCanonicals.has(item.canonical)} mutationError={mutationErrors[item.canonical]}
			onToggleWatchlist={() => onToggleWatchlist(item.canonical)}
			showInstitutional={showInstitutional} showMargin={showMargin}
			showRevenue={showRevenue} showValuation={showValuation} showDividends={showDividends}
			showFinancials={showFinancials}
		/>)}</tbody>
	</table></div>;
}

// Pure/presentational row. Clicking the identity area hands the exact backend `canonical` (never
// code alone, never re-inferred) to the caller's existing stock-research handoff — this component
// owns no navigation state of its own. The Watchlist action is a separate sibling <td>/button (never
// nested inside the identity button), so clicking it can never also trigger onOpen. The base 8
// columns are always rendered identically to M7A/M7B/M7C — the optional 9th (advanced) cell is only
// appended when at least one advanced domain is active, keeping the legacy table byte-identical
// when neither institutional nor margin criteria apply.
export function ScreenerRow({ security, onOpen, membershipState, saved, busy, mutationError, onToggleWatchlist, showInstitutional, showMargin, showRevenue, showValuation, showDividends, showFinancials }: {
	security: TaiwanScreenerSecurity;
	onOpen: () => void;
	membershipState: WatchlistMembershipState;
	saved: boolean;
	busy: boolean;
	mutationError?: string;
	onToggleWatchlist: () => void;
	showInstitutional: boolean;
	showMargin: boolean;
	showRevenue: boolean;
	showValuation: boolean;
	showDividends: boolean;
	showFinancials: boolean;
}) {
	const tone = security.change_percent == null ? '' : security.change_percent > 0 ? 'up' : security.change_percent < 0 ? 'down' : 'flat';
	return <tr>
		<td><button type="button" className="taiwan-screener-identity" onClick={onOpen}><strong>{security.name} {security.code}</strong></button></td>
		<td>{security.exchange} · {taiwanSecurityTypeLabel(security.security_type)}</td>
		<td>{security.price == null ? '—' : security.price.toLocaleString('zh-TW')}</td>
		<td className={tone}>{formatTaiwanPercent(security.change_percent, true)}</td>
		<td>{security.volume == null ? '—' : security.volume.toLocaleString('zh-TW')}</td>
		<td>{formatTaiwanTWD(security.amount)}</td>
		<td>{security.trade_date || '—'}</td>
		<td><ScreenerWatchlistAction membershipState={membershipState} saved={saved} busy={busy} mutationError={mutationError} onToggle={onToggleWatchlist} /></td>
		{(showInstitutional || showMargin || showRevenue || showValuation || showDividends || showFinancials) && <ScreenerAdvancedCell
			security={security} showInstitutional={showInstitutional} showMargin={showMargin}
			showRevenue={showRevenue} showValuation={showValuation} showDividends={showDividends}
			showFinancials={showFinancials}
		/>}
	</tr>;
}

// M7D — pure/presentational compact advanced-data cell. Renders only the domain block(s) that are
// currently active (per applied filters/sort), never all nine fields permanently. Missing values
// render "—"; a genuine zero renders "0"; signed net/change values carry an explicit + only when
// positive (never "+0").
export function ScreenerAdvancedCell({ security, showInstitutional, showMargin, showRevenue, showValuation, showDividends, showFinancials }: {
	security: TaiwanScreenerSecurity; showInstitutional: boolean; showMargin: boolean;
	showRevenue: boolean; showValuation: boolean; showDividends: boolean; showFinancials: boolean;
}) {
	return <td className="taiwan-screener-advanced-cell">
		{showInstitutional && <div className="taiwan-screener-advanced-block">
			<span>外資 {formatTaiwanSignedShares(security.foreign_net)}</span>
			<span>投信 {formatTaiwanSignedShares(security.trust_net)}</span>
			<span>自營商 {formatTaiwanSignedShares(security.dealer_net)}</span>
			<span>三大法人 {formatTaiwanSignedShares(security.institutional_net)}</span>
		</div>}
		{showMargin && <div className="taiwan-screener-advanced-block">
			<span>融資 {security.margin_balance == null ? '—' : security.margin_balance.toLocaleString('zh-TW')}</span>
			<span>融資增減 {formatTaiwanSignedShares(security.margin_change)}</span>
			<span>融券 {security.short_balance == null ? '—' : security.short_balance.toLocaleString('zh-TW')}</span>
			<span>融券增減 {formatTaiwanSignedShares(security.short_change)}</span>
			<span>券資比 {formatTaiwanRatioPercent(security.short_margin_ratio)}</span>
		</div>}
		{showRevenue && <div className="taiwan-screener-advanced-block">
			<span>月營收 {formatTaiwanRevenueTWD(security.monthly_revenue)}</span>
			<span>年增 {formatTaiwanPercent(security.revenue_yoy, true)}</span>
		</div>}
		{showValuation && <div className="taiwan-screener-advanced-block">
			<span>PE {formatTaiwanPlainNumber(security.pe)}</span>
			<span>PB {formatTaiwanPlainNumber(security.pb)}</span>
			<span>殖利率 {formatTaiwanPercent(security.dividend_yield)}</span>
		</div>}
		{showDividends && <div className="taiwan-screener-advanced-block">
			<span>現金 {formatTaiwanPlainNumber(security.cash_dividend)}</span>
			<span>股票 {formatTaiwanPlainNumber(security.stock_dividend)}</span>
			<span>合計 {formatTaiwanPlainNumber(security.total_dividend)}</span>
		</div>}
		{showFinancials && <div className="taiwan-screener-advanced-block">
			<span>期間 {security.financial_period || '—'}</span>
			<span>累計 EPS {formatTaiwanPlainNumber(security.cumulative_eps)}</span>
			<span>毛利率 {formatTaiwanPercent(security.gross_margin)}</span>
			<span>營業利益率 {formatTaiwanPercent(security.operating_margin)}</span>
		</div>}
	</td>;
}

// Pure/presentational. `membershipState !== 'ready'` (idle/loading/error) never renders an add/remove
// button — an unknown membership must never be misrepresented as "not saved". Only 'ready' shows the
// actionable toggle, mirroring the existing `.taiwan-watchlist-toggle` style/semantics used in
// TaiwanMarketView/TaiwanStockResearchWorkspace (same Star icon, same `.saved` styling).
export function ScreenerWatchlistAction({ membershipState, saved, busy, mutationError, onToggle }: {
	membershipState: WatchlistMembershipState;
	saved: boolean;
	busy: boolean;
	mutationError?: string;
	onToggle: () => void;
}) {
	if (membershipState === 'error') {
		return <span className="taiwan-screener-watchlist-unavailable">自選狀態無法取得</span>;
	}
	if (membershipState !== 'ready') {
		return <span className="taiwan-screener-watchlist-unavailable">自選狀態讀取中</span>;
	}
	return <div>
		<button type="button" className={`taiwan-watchlist-toggle${saved ? ' saved' : ''}`} onClick={onToggle} disabled={busy}>
			{busy ? <LoaderCircle className="spin" size={14} /> : <Star size={14} />}
			{saved ? '移除自選' : '加入自選'}
		</button>
		{mutationError && <span className="taiwan-watchlist-toggle-error">{mutationError}</span>}
	</div>;
}
