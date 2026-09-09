import { Bot, Compass, LoaderCircle, Search, Star } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import type { FormEvent } from 'react';
import type { BackendConfig, SecurityIdentity } from '../lib/backend';
import { requestJSON } from '../lib/backend';
import { SubscriptionAIModal } from './SubscriptionAIModal';
import { addTaiwanWatchlistSecurity, formatTaiwanBookValuePerShare, formatTaiwanCashFlowTWD, formatTaiwanPercent, formatTaiwanPlainNumber, isTaiwanSecurityWatchlisted, removeTaiwanWatchlistSecurity, runScopedRequest, taiwanComponentList, taiwanErrorMessage, taiwanIntelligenceCorePath, taiwanIntelligencePath, taiwanReasonLabel, taiwanResearchPath, taiwanSecurityTypeLabel, taiwanStatusLabel, type TaiwanResearchEntryContext } from '../lib/taiwan-product';

export type Evidence = { status: string; freshness?: string; as_of?: string; reason?: string; data?: Record<string, unknown> };
export type Component = { state: string; status: string; freshness?: string; as_of?: string; reasons: string[] };
export type IntelligenceCore = {
	model_version: string;
	symbol: string;
	identity: { canonical_symbol: string; code: string; name: string; exchange: string; currency: string; security_type: string; industry_name?: string };
	quote: Evidence & { target_latest_completed_trading_date?: string; data?: { price: number; change_percent: number; meta?: { trade_date?: string; is_realtime?: boolean } } };
	price_history_summary: Evidence & { return_5d_percent: number | null; return_20d_percent: number | null; latest_bar_date?: string };
};
// M8A — the single-security valuation/statement shapes actually returned inside
// fundamentals.data.valuation / fundamentals.data.financial_statement, reusing the exact existing
// backend field names verbatim (never renamed, never rescaled here). Every metric stays nullable —
// missing/unavailable never becomes 0, and each domain (income/balance/cashflow) keeps its OWN
// fiscal_year/fiscal_quarter, never inferred from another domain's period.
export type TaiwanValuationData = { data_date?: string; pe: number | null; pb: number | null; dividend_yield_percent: number | null };
export type TaiwanStatementData = {
	fiscal_year: number; fiscal_quarter: number; accounting_category?: string;
	cumulative_eps: number | null; gross_margin_percent: number | null; operating_margin_percent: number | null; net_margin_percent: number | null;
	book_value_per_share: number | null;
	balance_fiscal_year?: number; balance_fiscal_quarter?: number;
	debt_ratio_percent: number | null; debt_to_equity_percent: number | null; current_ratio_percent: number | null;
	cashflow_fiscal_year?: number; cashflow_fiscal_quarter?: number;
	operating_cash_flow: number | null; cash_flow_to_net_income: number | null;
};
export type Intelligence = {
	model_version: string; symbol: string;
	identity: { canonical_symbol: string; code: string; name: string; exchange: string; currency: string; security_type: string; industry_name?: string };
	quote: Evidence & { target_latest_completed_trading_date?: string; data?: { price: number; change_percent: number; meta?: { trade_date?: string; is_realtime?: boolean } } };
	price_history_summary: Evidence & { return_5d_percent: number | null; return_20d_percent: number | null; latest_bar_date?: string };
	fundamentals: Evidence & { data?: { valuation?: TaiwanValuationData | null; financial_statement?: TaiwanStatementData | null } };
	institutional: Evidence; margin: Evidence;
	market_context: Evidence & { state?: string; confidence?: string; advance_ratio?: number | null; advancing_amount_ratio?: number | null };
	industry_context: Evidence & { taxonomy_status?: string; industry?: { industry_name: string; relative_breadth: number | null; relative_capital: number | null } };
	interpretation?: { model_version: string; components: Record<string, Component>; data_quality: { available_components: string[] | null; indeterminate_components: string[] | null; unavailable_components: string[] | null; stale_components: string[] | null; partial_components: string[] | null } };
};
export type ResearchSection = { text: string; evidence_keys: string[] };
// M8C — strengths/risks are evidence-grounded, currently-favorable/risk observations distinct from
// the six fixed domain sections above; reuses the exact same ResearchSection shape (text +
// evidence_keys), and both arrays may legitimately be empty (the AI never manufactures an entry
// merely to populate them).
export type Research = { status: string; model_version: string; generated_at?: string; headline?: string; summary?: string; sections: Record<string, ResearchSection>; strengths: ResearchSection[]; risks: ResearchSection[]; conflicts: string[]; data_limitations: string[]; research_notes: string[]; reason?: string };

const componentLabels: Record<string, string> = { price: '價格', market: '市場', price_market_relationship: '個股與市場關係', industry: '產業', institutional: '法人', margin: '融資融券', fundamentals: '基本面' };
const researchLabels: Record<string, string> = { price: '價格', market: '市場', industry: '產業', institutional: '法人', margin: '融資融券', fundamentals: '基本面' };

// M6C: an optional request from outside this workspace (a Watchlist row click) to load one exact
// canonical Taiwan security. `token` is a strictly increasing nonce, not just the canonical string
// — clicking the same saved security twice in a row must still be treated as a new request even
// though the canonical value did not change.
// M8B: `context` is optional — only a Screener-originated request carries the applied filters/sort
// as a TaiwanResearchEntryContext; a Watchlist-originated request (or any other caller) omits it,
// and the workspace then shows no "來自台股篩選器" block.
export type ExternalTaiwanSymbolRequest = { canonical: string; token: number; context?: TaiwanResearchEntryContext | null };

export function TaiwanStockResearchWorkspace({ config, refreshKey, externalSymbolRequest }: { config: BackendConfig | null; refreshKey: number; externalSymbolRequest?: ExternalTaiwanSymbolRequest | null }) {
	const [query, setQuery] = useState('2330');
	const [matches, setMatches] = useState<SecurityIdentity[]>([]);
	const [selected, setSelected] = useState<SecurityIdentity | null>(null);
	const [intelligence, setIntelligence] = useState<Intelligence | null>(null);
	const [coreData, setCoreData] = useState<IntelligenceCore | null>(null);
	const [research, setResearch] = useState<Research | null>(null);
	const [loading, setLoading] = useState(false);
	const [coreLoading, setCoreLoading] = useState(false);
	const [fullLoading, setFullLoading] = useState(false);
	const [researching, setResearching] = useState(false);
	const [error, setError] = useState('');
	const [fullError, setFullError] = useState('');
	const selectRequestID = useRef(0);
	const selectedRef = useRef<SecurityIdentity | null>(null);
	useEffect(() => { selectedRef.current = selected; }, [selected]);
	// Watchlist membership/mutation state, independent of the intelligence/research `error` above —
	// a failed membership check or toggle must never clear or interfere with the displayed research.
	const [inWatchlist, setInWatchlist] = useState<boolean | null>(null);
	const [watchlistBusy, setWatchlistBusy] = useState(false);
	const [watchlistError, setWatchlistError] = useState('');
	const watchlistCheckID = useRef(0);
	// M6C: tracks the last externalSymbolRequest.token already handled, and scopes the exact-symbol
	// resolution lookup so a stale in-flight resolution (a rapid second click) can never overwrite
	// a newer one — the same runScopedRequest pattern already used for select()/search() above.
	const lastExternalTokenRef = useRef<number | null>(null);
	const externalResolveRequestID = useRef(0);
	// M8B: the Screener-originated navigation context (if any) for the currently displayed security.
	// Set only when select() is explicitly given a context argument (undefined leaves it untouched,
	// so the refreshKey retry of the already-selected symbol below preserves it); explicit null clears
	// it for a manual search/selection, since that is not a Screener-originated navigation.
	const [screenerContext, setScreenerContext] = useState<TaiwanResearchEntryContext | null>(null);
	const [subscriptionAIModalOpen, setSubscriptionAIModalOpen] = useState(false);

	const search = async (value = query) => {
		if (!config || !value.trim()) return;
		setLoading(true); setError('');
		try {
			const payload = await requestJSON<{ data: { securities: SecurityIdentity[] } }>(config, `/api/v1/tw/securities?query=${encodeURIComponent(value.trim())}`);
			setMatches(payload.data.securities);
			if (payload.data.securities.length === 1) await select(payload.data.securities[0], null);
			else if (payload.data.securities.length === 0) setError(`找不到符合「${value.trim()}」的台灣證券`);
		} catch (reason) { setError(taiwanErrorMessage(reason, '台灣證券搜尋失敗')); }
		finally { setLoading(false); }
	};
	// M8B: `context` is optional and distinct from omission — undefined means "leave screenerContext
	// untouched" (used by the refreshKey retry of an already-selected symbol, which is not a new
	// navigation), while null explicitly clears it (a manual search/selection) and a real context
	// object sets it (a Screener-originated external request).
	const select = async (security: SecurityIdentity, context?: TaiwanResearchEntryContext | null) => {
		if (!config) return;
		if (context !== undefined) setScreenerContext(context);
		setSelected(security);
		// Independent of the main intelligence fetch below: check Watchlist membership for the
		// newly selected security. Scoped so a stale check for a since-abandoned selection can
		// never overwrite the current one. A failure here is silent (button stays hidden).
		setInWatchlist(null); setWatchlistError('');
		void runScopedRequest(watchlistCheckID, () => isTaiwanSecurityWatchlisted(config, security.canonical), {
			onSuccess: (saved) => setInWatchlist(saved),
		});

		// P5.5C.1 Two-Stage Progressive Loading:
		// Stage 1 (Fast Core): fetch Quote + PriceHistory + Identity (~1-2s).
		// Stage 2 (Full Intelligence): fetch complete intelligence (Market, Fundamentals, Institutional, Margin).
		// runScopedRequest guarantees last-wins ordering and cancellation of stale updates.
		await runScopedRequest(selectRequestID, async () => {
			const requestID = selectRequestID.current;
			let coreSettled = false;
			try {
				const corePayload = await requestJSON<{ data: IntelligenceCore }>(config, taiwanIntelligenceCorePath(security.canonical));
				if (requestID !== selectRequestID.current) return;
				setCoreData(corePayload.data);
				setCoreLoading(false);
				coreSettled = true;
			} catch (reason) {
				if (requestID !== selectRequestID.current) return;
				// If Core fails, we still allow Stage 2 to attempt or set the main error if both fail.
				setCoreLoading(false);
			}

			try {
				const fullPayload = await requestJSON<{ data: Intelligence }>(config, taiwanIntelligencePath(security.canonical));
				if (requestID !== selectRequestID.current) return;
				setIntelligence(fullPayload.data);
				setFullError('');
			} catch (reason) {
				if (requestID !== selectRequestID.current) return;
				if (coreSettled) {
					setFullError(taiwanErrorMessage(reason, '暫時無法取得完整分析資料（市場與籌碼面）'));
				} else {
					setIntelligence(null);
					setError(taiwanErrorMessage(reason, '台灣個股分析資料載入失敗'));
				}
			}
		}, {
			onStart: () => {
				setIntelligence(null);
				setCoreData(null);
				setResearch(null);
				setLoading(true);
				setCoreLoading(true);
				setFullLoading(true);
				setError('');
				setFullError('');
			},
			onSuccess: () => {},
			onSettle: () => {
				setLoading(false);
				setCoreLoading(false);
				setFullLoading(false);
			},
		});
	};

	// Adds/removes the currently selected security from the Watchlist. Deliberately does not
	// touch `selected`/intelligence/research — Watchlist membership is independent of the
	// currently displayed research, so a toggle (success or failure) never reloads or clears it.
	const toggleWatchlist = async () => {
		if (!config || !selected || watchlistBusy || inWatchlist === null) return;
		setWatchlistBusy(true); setWatchlistError('');
		try {
			if (inWatchlist) { await removeTaiwanWatchlistSecurity(config, selected.canonical); setInWatchlist(false); }
			else { await addTaiwanWatchlistSecurity(config, selected.canonical); setInWatchlist(true); }
		} catch (reason) {
			setWatchlistError(taiwanErrorMessage(reason, inWatchlist ? '移除自選股失敗' : '加入自選股失敗'));
		} finally {
			setWatchlistBusy(false);
		}
	};
	const generateResearch = async () => {
		if (!config || !selected) return;
		setResearching(true); setError('');
		try {
			const payload = await requestJSON<{ data: { intelligence: Intelligence; ai_research: Research } }>(config, taiwanResearchPath(selected.canonical), { method: 'POST' });
			setIntelligence(payload.data.intelligence); setResearch(payload.data.ai_research);
		} catch (reason) { setError(taiwanErrorMessage(reason, 'AI 研究目前無法使用')); }
		finally { setResearching(false); }
	};
	// Preserve the initial default (2330) on first load, but a later refresh (refreshKey change)
	// must retry whatever the user currently has selected rather than silently resetting to 2330.
	// M6C: a Watchlist-originated request (externalSymbolRequest) takes precedence — a new token
	// triggers an exact-canonical resolution instead of the default/retry path below. This is
	// deliberately ONE effect (not two) reacting to both refreshKey and externalSymbolRequest:
	// splitting "decide to resolve the external request" and "decide to fall back to default/retry"
	// into separate effects created a real ordering bug under React StrictMode's dev-only double
	// effect invocation (the fallback effect could observe "already marked handled" before the
	// resolution actually completed, and briefly fire search('2330') underneath it). A single
	// effect makes the precedence atomic: while a request's resolution is in flight (its token
	// already marked handled but `selectedRef.current` not yet populated), neither branch below
	// runs — we simply wait for that in-flight resolution instead of guessing a fallback.
	useEffect(() => {
		if (!config) return;
		if (externalSymbolRequest && externalSymbolRequest.token !== lastExternalTokenRef.current) {
			lastExternalTokenRef.current = externalSymbolRequest.token;
			const canonical = externalSymbolRequest.canonical;
			// Requires an exact `canonical` match from /tw/securities — never a fuzzy/first result.
			// Scoped so a stale in-flight resolution (a rapid second click) cannot overwrite a newer
			// one; select()'s own runScopedRequest guard then protects the intelligence fetch itself.
			void runScopedRequest(externalResolveRequestID, () => requestJSON<{ data: { securities: SecurityIdentity[] } }>(config, `/api/v1/tw/securities?query=${encodeURIComponent(canonical)}`), {
				onStart: () => { setLoading(true); setError(''); },
				onSuccess: (payload) => {
					const exact = payload.data.securities.find((item) => item.canonical === canonical);
					if (!exact) { setLoading(false); setError('找不到自選股對應的台灣證券資料。'); return; }
					void select(exact, externalSymbolRequest.context ?? null);
				},
				onError: (reason) => { setLoading(false); setError(taiwanErrorMessage(reason, '找不到自選股對應的台灣證券資料。')); },
			});
			return;
		}
		if (selectedRef.current) { void select(selectedRef.current); return; }
		if (!externalSymbolRequest) void search('2330');
	}, [config, refreshKey, externalSymbolRequest]);
	const submit = (event: FormEvent) => { event.preventDefault(); void search(); };
	const displayData = intelligence ?? coreData;

	return <div className="taiwan-product-workspace taiwan-stock-research">
		<form className="market-filter" onSubmit={submit}><label><Search size={15} /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="輸入 2330、台積電、2330.TWSE 或 6488.TPEX" aria-label="台灣證券名稱或代碼" /></label><button type="submit" disabled={loading}>{loading ? <LoaderCircle className="spin" size={14} /> : '搜尋'}</button></form>
		{error && <div className="market-partial-warning">{error}</div>}
		{matches.length > 1 && <div className="taiwan-search-results">{matches.map((item) => <button type="button" key={item.canonical} onClick={() => void select(item, null)}><strong>{item.code} {item.name}</strong><span>{item.exchange} · {taiwanSecurityTypeLabel(item.security_type)}</span></button>)}</div>}
		{selected && inWatchlist !== null && <div><button type="button" className={`taiwan-watchlist-toggle${inWatchlist ? ' saved' : ''}`} onClick={() => void toggleWatchlist()} disabled={watchlistBusy}>{watchlistBusy ? <LoaderCircle className="spin" size={14} /> : <Star size={14} />}{inWatchlist ? '移除自選' : '加入自選'}</button>{watchlistError && <span className="taiwan-watchlist-toggle-error">{watchlistError}</span>}</div>}
		{!displayData && loading && <div className="taiwan-loading"><LoaderCircle className="spin" size={18} />正在讀取官方個股資料</div>}
		{displayData && <>
			{screenerContext && <ScreenerEntryContext context={screenerContext} />}
			<section className="taiwan-stock-heading"><div><span>{displayData.identity.exchange} · {taiwanSecurityTypeLabel(displayData.identity.security_type)} · {displayData.identity.currency}</span><h2>{displayData.identity.name} {displayData.identity.code}</h2><small>{displayData.identity.canonical_symbol}{displayData.identity.industry_name ? ` · ${displayData.identity.industry_name}` : ''}</small></div>{displayData.quote.data && <div><strong>{displayData.quote.data.price.toLocaleString('zh-TW')}</strong><em className={displayData.quote.data.change_percent > 0 ? 'up' : displayData.quote.data.change_percent < 0 ? 'down' : 'flat'}>{formatTaiwanPercent(displayData.quote.data.change_percent, true)}</em></div>}</section>
			<section className="taiwan-status-card"><div><span className={`taiwan-status ${displayData.quote.status}`}>{taiwanStatusLabel(displayData.quote.status)}</span><span>{taiwanStatusLabel(displayData.quote.freshness)}</span></div><small>資料日期 {displayData.quote.as_of || displayData.quote.data?.meta?.trade_date || '未提供'} · 最新完成交易日 {displayData.quote.target_latest_completed_trading_date || '未提供'}</small></section>
			<section className="taiwan-detail-grid"><article><span>5 日收盤報酬</span><strong>{formatTaiwanPercent(displayData.price_history_summary.return_5d_percent, true)}</strong><small>{displayData.price_history_summary.latest_bar_date || '資料不足'}</small></article><article><span>20 日收盤報酬</span><strong>{formatTaiwanPercent(displayData.price_history_summary.return_20d_percent, true)}</strong></article><article><span>市場狀態</span><strong>{intelligence ? taiwanStatusLabel(intelligence.market_context.state) : (fullLoading ? '載入中…' : '—')}</strong><small>資料信心 {intelligence ? taiwanStatusLabel(intelligence.market_context.confidence) : (fullLoading ? '計算中…' : '—')}</small></article></section>

			{fullError && <div className="market-partial-warning" style={{ marginTop: '12px' }}>{fullError}</div>}
			{fullLoading && !intelligence && <div className="taiwan-loading" style={{ margin: '20px 0' }}><LoaderCircle className="spin" size={16} />正在載入市場廣度、法人與基本面分析…</div>}

			{intelligence && <>
				<EvidenceOverview intelligence={intelligence} />
				<TaiwanResearchSnapshot fundamentals={intelligence.fundamentals} />
				{intelligence.interpretation && <InterpretationView intelligence={intelligence} />}
				<section className="taiwan-ai-action">
					<div>
						<strong>AI 研究</strong>
						<p>可產生本機模型摘要，或複製客觀證據至 ChatGPT、Claude、Gemini 等已訂閱 AI 進行深入分析。</p>
					</div>
					<div className="taiwan-ai-actions-group">
						<button
							type="button"
							className="taiwan-subscription-ai-btn"
							onClick={() => setSubscriptionAIModalOpen(true)}
						>
							<Compass size={14} />
							使用已訂閱的 AI
						</button>
						<button type="button" onClick={() => void generateResearch()} disabled={researching}>
							{researching ? <><LoaderCircle className="spin" size={14} />產生中</> : <><Bot size={14} />產生 AI 研究摘要</>}
						</button>
					</div>
				</section>
				{subscriptionAIModalOpen && (
					<SubscriptionAIModal
						intelligence={intelligence}
						onClose={() => setSubscriptionAIModalOpen(false)}
					/>
				)}
				{research && <ResearchView research={research} />}
			</>}
		</>}
		{!displayData && !loading && <div className="taiwan-empty-state"><strong>選擇台灣證券開始分析</strong><p>可搜尋上市或上櫃股票；原始資料載入不會呼叫 AI。</p></div>}
	</div>;
}

// M8B — a small, secondary provenance block shown only when this Research page was opened via a
// Screener row click, carrying the exact APPLIED filters/sort at that moment. Deliberately separate
// from EvidenceOverview/TaiwanResearchSnapshot/InterpretationView below — this is navigation
// provenance, not evidence, and is never passed into BuildTaiwanResearchPayload/GenerateTaiwanResearch
// or seen by the AI. Wording uses provenance language ("你從以下篩選條件的結果中開啟此分析") rather
// than any present-tense match claim ("目前仍符合") — the underlying data may have changed since the
// Screener request, and this component never re-fetches Screener or re-validates the match.
export function ScreenerEntryContext({ context }: { context: TaiwanResearchEntryContext }) {
	return <section className="taiwan-screener-entry-context">
		<header><span>來自台股篩選器</span><h3>你從以下篩選條件的結果中開啟此分析</h3></header>
		<div className="taiwan-detail-grid">
			<article><span>篩選條件</span>{context.filterLabels.length > 0 ? <ul>{context.filterLabels.map((label) => <li key={label}>{label}</li>)}</ul> : <small>未設定篩選條件</small>}</article>
			<article><span>排序方式</span><small>{context.sortLabel}</small></article>
		</div>
	</section>;
}

function EvidenceOverview({ intelligence }: { intelligence: Intelligence }) {
	const industry = intelligence.industry_context.industry;
	const items: [string, Evidence, string][] = [
		['基本面', intelligence.fundamentals, intelligence.fundamentals.reason || '官方財務與營收資料'],
		['法人', intelligence.institutional, intelligence.institutional.reason || '外資、投信與自營商資料'],
		['融資融券', intelligence.margin, intelligence.margin.reason || '融資與融券餘額資料'],
		['產業', intelligence.industry_context, industry ? `${industry.industry_name} · 相對市場廣度 ${formatTaiwanPercent(industry.relative_breadth == null ? null : industry.relative_breadth * 100, true)} · 相對成交方向 ${formatTaiwanPercent(industry.relative_capital == null ? null : industry.relative_capital * 100, true)}` : intelligence.industry_context.reason || '官方產業分類資料'],
	];
	return <section className="taiwan-evidence-overview"><header><span>官方證據</span><h3>資料涵蓋與狀態</h3></header><div className="taiwan-detail-grid">{items.map(([label, item, detail]) => <article key={label}><span>{label}</span><strong>{taiwanStatusLabel(item.status)}</strong><small>{item.as_of ? `資料日期 ${item.as_of}` : taiwanStatusLabel(item.freshness)}</small><p>{taiwanReasonLabel(detail)}</p></article>)}</div></section>;
}

// M8A — snapshotPeriod formats one domain's own (fiscal_year, fiscal_quarter) pair as "YYYY-QN", never
// substituting another domain's period and never fabricating a period when the pair is absent/zero
// (the backend's own omitempty convention: 0 means "this domain was not joined for this security").
function snapshotPeriod(year?: number, quarter?: number): string {
	return year ? `${year}-Q${quarter}` : '—';
}

// M8A — Taiwan Research Snapshot: structured, period-aware, status-aware evidence for the four M8A
// domains (估值/獲利能力/資產負債/現金流量), reusing the exact backend values already carried inside
// fundamentals.data (valuation/financial_statement) — no derived/recalculated numbers, no AI. Each
// group renders independently: a missing statement/valuation (fundamentals unavailable/not_applicable/
// data_insufficient, or an ETF where fundamentals are not applicable at all) never hides the OTHER
// already-rendered Research sections above/below it — every field simply falls back to the existing
// null placeholder ("—") via the shared formatters, exactly like every other Screener/Research value.
export function TaiwanResearchSnapshot({ fundamentals }: { fundamentals: Intelligence['fundamentals'] }) {
	const valuation = fundamentals.data?.valuation;
	const statement = fundamentals.data?.financial_statement;
	return <section className="taiwan-research-snapshot"><header><span>結構化證據</span><h3>財務與現金流量現況</h3></header>
		<div className="taiwan-detail-grid">
			<article><span>估值</span>
				<small>PE {formatTaiwanPlainNumber(valuation?.pe)}</small>
				<small>PB {formatTaiwanPlainNumber(valuation?.pb)}</small>
				<small>殖利率 {formatTaiwanPercent(valuation?.dividend_yield_percent)}</small>
				<small>資料日期 {valuation?.data_date || '—'}</small>
			</article>
			<article><span>獲利能力</span>
				<small>累計 EPS {formatTaiwanPlainNumber(statement?.cumulative_eps)}</small>
				<small>毛利率 {formatTaiwanPercent(statement?.gross_margin_percent)}</small>
				<small>營業利益率 {formatTaiwanPercent(statement?.operating_margin_percent)}</small>
				<small>淨利率 {formatTaiwanPercent(statement?.net_margin_percent)}</small>
				<small>財報期間 {snapshotPeriod(statement?.fiscal_year, statement?.fiscal_quarter)}</small>
			</article>
			<article><span>資產負債</span>
				<small>每股參考淨值 {formatTaiwanBookValuePerShare(statement?.book_value_per_share)}</small>
				<small>負債比 {formatTaiwanPercent(statement?.debt_ratio_percent)}</small>
				<small>負債權益比 {formatTaiwanPercent(statement?.debt_to_equity_percent)}</small>
				<small>流動比 {formatTaiwanPercent(statement?.current_ratio_percent)}</small>
				<small>資產負債表期間 {snapshotPeriod(statement?.balance_fiscal_year, statement?.balance_fiscal_quarter)}</small>
			</article>
			<article><span>現金流量</span>
				<small>營業活動現金流量 {formatTaiwanCashFlowTWD(statement?.operating_cash_flow)}</small>
				<small>營業現金流／淨利 {formatTaiwanPercent(statement?.cash_flow_to_net_income)}</small>
				<small>現金流量期間 {snapshotPeriod(statement?.cashflow_fiscal_year, statement?.cashflow_fiscal_quarter)}</small>
			</article>
		</div>
	</section>;
}

export function InterpretationView({ intelligence }: { intelligence: Intelligence }) {
	const interpretation = intelligence.interpretation!;
	const availableComponents = taiwanComponentList(interpretation.data_quality.available_components);
	const indeterminateComponents = taiwanComponentList(interpretation.data_quality.indeterminate_components);
	const unavailableComponents = taiwanComponentList(interpretation.data_quality.unavailable_components);
	const staleComponents = taiwanComponentList(interpretation.data_quality.stale_components);
	const partialComponents = taiwanComponentList(interpretation.data_quality.partial_components);
	return <section className="taiwan-interpretation"><header><div><span>確定性解讀</span><h3>各項證據狀態</h3></div><small>{interpretation.model_version}</small></header><div>{Object.entries(interpretation.components).map(([key, item]) => <article key={key}><header><strong>{componentLabels[key] || key}</strong><span>{taiwanStatusLabel(item.state)} · {taiwanStatusLabel(item.status)}</span></header><small>{item.as_of ? `資料日期 ${item.as_of}` : taiwanStatusLabel(item.freshness)}</small><ul>{item.reasons.map((reason) => <li key={reason}>{taiwanReasonLabel(reason)}</li>)}</ul></article>)}</div><footer>可用 {availableComponents.length} 項 · 無法判定 {indeterminateComponents.length} 項 · 無法取得／不適用 {unavailableComponents.length} 項{staleComponents.length > 0 && ` · 資料較舊 ${staleComponents.length} 項`}{partialComponents.length > 0 && ` · 部分資料 ${partialComponents.length} 項`}</footer></section>;
}

export function ResearchView({ research }: { research: Research }) {
	if (research.status !== 'available') return <section className="taiwan-research-unavailable"><strong>AI 研究目前無法使用</strong><p>下方官方資料與確定性解讀仍可正常查看。</p></section>;
	return <section className="taiwan-research-result"><header><span>AI 研究</span><h3>{research.headline}</h3><p>{research.summary}</p><small>{research.generated_at ? new Date(research.generated_at).toLocaleString('zh-TW') : ''} · {research.model_version}</small></header><div>{Object.entries(research.sections).map(([key, section]) => <article key={key}><strong>{researchLabels[key] || key}</strong><p>{section.text}</p><small>證據：{section.evidence_keys.join('、')}</small></article>)}</div>{([['支持性證據', research.strengths], ['風險證據', research.risks]] as [string, ResearchSection[]][]).map(([label, items]) => items.length > 0 && <section key={label}><strong>{label}</strong>{items.map((item, index) => <article key={index}><p>{item.text}</p><small>證據：{item.evidence_keys.join('、')}</small></article>)}</section>)}{[['證據衝突', research.conflicts], ['資料限制', research.data_limitations], ['研究備註', research.research_notes]].map(([label, items]) => Array.isArray(items) && items.length > 0 && <section key={String(label)}><strong>{label}</strong><ul>{items.map((item) => <li key={item}>{item}</li>)}</ul></section>)}</section>;
}
