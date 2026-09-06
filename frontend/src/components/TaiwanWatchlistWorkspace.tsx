import { LoaderCircle, Trash2 } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import type { BackendConfig, Quote } from '../lib/backend';
import { requestJSON } from '../lib/backend';
import { chunkTaiwanSymbols, fetchTaiwanWatchlist, formatTaiwanCashFlowTWD, formatTaiwanPercent, formatTaiwanPlainNumber, removeTaiwanWatchlistSecurity, runScopedRequest, taiwanErrorMessage, taiwanIntelligencePath, taiwanSecurityTypeLabel, type TaiwanWatchlistSecurity } from '../lib/taiwan-product';

type QuoteLookup = Record<string, Quote>;

// Fetches quotes for every saved security, chunked to the existing /tw/quotes batch limit.
// Each chunk is requested independently (Promise.allSettled): one chunk failing must not
// discard quotes a different, successful chunk already returned. A missing quote (chunk failed,
// or a symbol simply absent from a successful chunk's response) is represented by the symbol
// being absent from the returned map — never a fabricated zero/blank entry.
async function fetchWatchlistQuotes(config: BackendConfig, canonicals: string[]): Promise<QuoteLookup> {
	if (canonicals.length === 0) return {};
	const groups = chunkTaiwanSymbols(canonicals);
	const results = await Promise.allSettled(groups.map((group) => requestJSON<{ data: Quote[] }>(config, `/api/v1/tw/quotes?symbols=${encodeURIComponent(group.join(','))}`)));
	const quotes: QuoteLookup = {};
	for (const result of results) {
		if (result.status === 'fulfilled') {
			for (const quote of result.value.data) quotes[quote.symbol] = quote;
		}
	}
	return quotes;
}

// M8D — the on-demand, per-row intelligence summary. Deliberately a small narrow subset of the
// full `/intelligence` wire response (never the whole TaiwanFundamentals/TaiwanStatementData
// shape) — only the four approved M8D fields. Reusing the wider Intelligence/TaiwanValuationData/
// TaiwanStatementData types from TaiwanStockResearchWorkspace.tsx is deliberately avoided: those
// types promise a much larger, differently-scoped contract (full Research Snapshot), and widening
// this file's dependency on them would blur what M8D actually reads and displays.
type WatchlistSummaryValuation = { data_date?: string; pe: number | null; pb: number | null };
type WatchlistSummaryStatement = {
	balance_fiscal_year?: number; balance_fiscal_quarter?: number; debt_ratio_percent: number | null;
	cashflow_fiscal_year?: number; cashflow_fiscal_quarter?: number; cashflow_status?: string; operating_cash_flow: number | null;
};
type WatchlistIntelligenceResponse = { data: { fundamentals?: { data?: { valuation?: WatchlistSummaryValuation | null; financial_statement?: WatchlistSummaryStatement | null } } } };

type WatchlistSummary =
	| { status: 'loading' }
	| { status: 'available'; valuation: WatchlistSummaryValuation | null; statement: WatchlistSummaryStatement | null }
	| { status: 'error'; error: string };
type WatchlistSummaryLookup = Record<string, WatchlistSummary>;

// Calls ONLY the existing single-security intelligence endpoint — never /research, never
// GenerateTaiwanResearch. This is the sole network effect of expanding a row's summary.
async function fetchWatchlistSummary(config: BackendConfig, canonical: string): Promise<WatchlistSummary> {
	const payload = await requestJSON<WatchlistIntelligenceResponse>(config, taiwanIntelligencePath(canonical));
	const data = payload.data.fundamentals?.data;
	return { status: 'available', valuation: data?.valuation ?? null, statement: data?.financial_statement ?? null };
}

export function TaiwanWatchlistWorkspace({ config, refreshKey, onOpenResearch }: { config: BackendConfig | null; refreshKey: number; onOpenResearch: (canonical: string) => void }) {
	const [securities, setSecurities] = useState<TaiwanWatchlistSecurity[]>([]);
	const [quotes, setQuotes] = useState<QuoteLookup>({});
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState('');
	const [removingSymbol, setRemovingSymbol] = useState<string | null>(null);
	const loadRequestID = useRef(0);
	// M8D — per-row, on-demand intelligence summary state. `expanded` controls visibility only;
	// `summaries` is the session cache (never persisted to localStorage/DB/backend/URL) keyed by
	// canonical, so collapsing a row never discards already-loaded data and re-expanding it costs
	// zero new requests. `summaryRequestsRef` is a synchronous in-flight guard (checked/set before
	// any await) so a rapid repeated click on the same row can never launch a duplicate request —
	// mirrors the existing `removingSymbol` single-flight pattern, just keyed per-canonical instead
	// of a single scalar. `securitiesRef` mirrors `securities` so a stale intelligence response that
	// resolves after its security was removed can detect that and never resurrect a summary for an
	// item no longer on the list.
	const [expanded, setExpanded] = useState<Set<string>>(new Set());
	const [summaries, setSummaries] = useState<WatchlistSummaryLookup>({});
	const summaryRequestsRef = useRef<Set<string>>(new Set());
	const securitiesRef = useRef<Set<string>>(new Set());
	useEffect(() => { securitiesRef.current = new Set(securities.map((item) => item.canonical)); }, [securities]);

	// Fetches only while this view is mounted (App unmounts it when the user switches away), and
	// only on mount or an explicit global refresh — never on a timer, never merely because the
	// application opened elsewhere.
	useEffect(() => {
		if (!config) return;
		// M8D — a global refresh invalidates any previously loaded per-row intelligence summaries
		// (the underlying official data may have changed) but never automatically refetches them —
		// expanded rows are collapsed back to idle rather than kept open with stale data; the next
		// explicit expand click fetches fresh intelligence. This is the smaller, truthful choice over
		// silently keeping rows "expanded" with an ambiguous not-yet-reloaded state.
		setSummaries({});
		setExpanded(new Set());
		void runScopedRequest(loadRequestID, async () => {
			const list = await fetchTaiwanWatchlist(config);
			const quoteMap = await fetchWatchlistQuotes(config, list.map((item) => item.canonical));
			return { list, quoteMap };
		}, {
			onStart: () => { setLoading(true); setError(''); },
			onSuccess: ({ list, quoteMap }) => { setSecurities(list); setQuotes(quoteMap); },
			onError: (reason) => { setSecurities([]); setQuotes({}); setError(taiwanErrorMessage(reason, '自選股清單載入失敗')); },
			onSettle: () => setLoading(false),
		});
	}, [config, refreshKey]);

	// M8D — loads exactly one security's intelligence summary. Guarded so at most one request per
	// canonical is ever in flight; a stale response (the security was removed while loading) is
	// detected via securitiesRef and silently dropped rather than resurrecting removed state.
	const loadSummary = async (canonical: string) => {
		if (!config || summaryRequestsRef.current.has(canonical)) return;
		summaryRequestsRef.current.add(canonical);
		setSummaries((current) => ({ ...current, [canonical]: { status: 'loading' } }));
		try {
			const summary = await fetchWatchlistSummary(config, canonical);
			if (!securitiesRef.current.has(canonical)) return;
			setSummaries((current) => ({ ...current, [canonical]: summary }));
		} catch (reason) {
			if (!securitiesRef.current.has(canonical)) return;
			setSummaries((current) => ({ ...current, [canonical]: { status: 'error', error: taiwanErrorMessage(reason, '個股摘要載入失敗') } }));
		} finally {
			summaryRequestsRef.current.delete(canonical);
		}
	};

	// M8D — toggling only ever fetches on the transition into "expanded" AND only when no summary
	// is already cached for this canonical; collapsing never clears the cache, and re-expanding an
	// already-loaded (or already-loading) row issues zero new requests.
	const toggleSummary = (canonical: string) => {
		const wasExpanded = expanded.has(canonical);
		setExpanded((current) => {
			const next = new Set(current);
			if (wasExpanded) next.delete(canonical); else next.add(canonical);
			return next;
		});
		if (!wasExpanded && !summaries[canonical]) void loadSummary(canonical);
	};
	const retrySummary = (canonical: string) => { void loadSummary(canonical); };

	const remove = async (canonical: string) => {
		if (!config || removingSymbol) return;
		setRemovingSymbol(canonical);
		try {
			await removeTaiwanWatchlistSecurity(config, canonical);
			setSecurities((current) => current.filter((item) => item.canonical !== canonical));
			setQuotes((current) => { const next = { ...current }; delete next[canonical]; return next; });
			// M8D — a removed security's cached/in-flight summary state must not linger indefinitely.
			setSummaries((current) => { const next = { ...current }; delete next[canonical]; return next; });
			setExpanded((current) => { const next = new Set(current); next.delete(canonical); return next; });
		} catch (reason) {
			setError(taiwanErrorMessage(reason, '移除自選股失敗'));
		} finally {
			setRemovingSymbol(null);
		}
	};

	return <div className="taiwan-product-workspace taiwan-watchlist-workspace">
		{loading && <div className="taiwan-loading"><LoaderCircle className="spin" size={18} />正在讀取自選股清單</div>}
		{error && <div className="market-partial-warning">{error}</div>}
		{!loading && securities.length === 0 && !error && <div className="taiwan-empty-state"><strong>目前還沒有自選股</strong><p>可從台股總覽或個股研究加入。</p></div>}
		{securities.length > 0 && <div className="taiwan-watchlist-list">{securities.map((item) => (
			<WatchlistRow key={item.canonical} security={item} quote={quotes[item.canonical]} busy={removingSymbol === item.canonical}
				onOpen={() => onOpenResearch(item.canonical)} onRemove={() => void remove(item.canonical)}
				expanded={expanded.has(item.canonical)} summary={summaries[item.canonical]}
				onToggleSummary={() => toggleSummary(item.canonical)} onRetrySummary={() => retrySummary(item.canonical)} />
		))}</div>}
	</div>;
}

// M8D — year/quarter → "YYYY-QN", matching the same convention already used elsewhere for
// independent per-domain periods (never a fabricated period when the pair is absent/zero).
function fiscalPeriodLabel(year?: number, quarter?: number): string {
	return year ? `${year}-Q${quarter}` : '—';
}

// M8D — the on-demand summary panel: exactly four facts (PE, PB, debt_ratio, operating_cash_flow),
// each with its own domain's period shown independently (never one generic "資料期間" for all
// four). A missing/unavailable metric renders "—" via the shared formatters; the whole panel never
// fails just because one field is absent. A "partial" cash-flow status is shown as an explicit
// truthful qualifier next to the value — it is never silently upgraded to a plain value, and never
// downgraded to "unavailable" when a real value is present. Purely factual labels only — no
// financial-scoring language (便宜/昂貴/健康/危險/值得買/值得賣) anywhere here.
export function WatchlistSummaryPanel({ summary, onRetry }: { summary?: WatchlistSummary; onRetry: () => void }) {
	if (!summary || summary.status === 'loading') {
		return <div className="taiwan-watchlist-summary taiwan-loading"><LoaderCircle className="spin" size={14} />正在讀取個股摘要</div>;
	}
	if (summary.status === 'error') {
		return <div className="taiwan-watchlist-summary market-partial-warning">{summary.error}<button type="button" className="taiwan-watchlist-toggle" onClick={onRetry}>重試</button></div>;
	}
	const valuation = summary.valuation;
	const statement = summary.statement;
	return <div className="taiwan-watchlist-summary taiwan-detail-grid">
		<article><span>估值</span>
			<small>本益比 {formatTaiwanPlainNumber(valuation?.pe)}</small>
			<small>股價淨值比 {formatTaiwanPlainNumber(valuation?.pb)}</small>
			<small>資料日期 {valuation?.data_date || '—'}</small>
		</article>
		<article><span>資產負債</span>
			<small>負債比 {formatTaiwanPercent(statement?.debt_ratio_percent)}</small>
			<small>資產負債表期間 {fiscalPeriodLabel(statement?.balance_fiscal_year, statement?.balance_fiscal_quarter)}</small>
		</article>
		<article><span>現金流量</span>
			<small>營業活動現金流量 {formatTaiwanCashFlowTWD(statement?.operating_cash_flow)}{statement?.cashflow_status === 'partial' ? '（部分資料）' : ''}</small>
			<small>現金流量期間 {fiscalPeriodLabel(statement?.cashflow_fiscal_year, statement?.cashflow_fiscal_quarter)}</small>
		</article>
	</div>;
}

// Pure/presentational: one saved security's row. A missing `quote` (chunk failed, or the symbol
// was simply absent from a successful chunk) always renders the explicit unavailable state below
// — never a fabricated 0 price/percentage, and never omitting the security itself. The identity
// area is its own <button>, a sibling of the remove button (never nested inside one another) —
// valid markup, keyboard accessible, and clicking remove can never also trigger onOpen. Navigation
// is available regardless of whether `quote` loaded — a saved security must stay openable even
// when its live quote is currently unavailable.
// M8D — the new 展開摘要/收合摘要 toggle and the on-demand summary panel are additive: `expanded`/
// `summary`/`onToggleSummary`/`onRetrySummary` are optional with inert defaults so this component
// remains directly callable exactly as before (no summary UI renders unless a caller opts in by
// passing them) — existing call sites and tests are unaffected. The toggle is its own sibling
// <button>, never nested inside the identity button, so it can never also trigger Research
// navigation, and the identity button's own onOpen/Research-navigation behavior is unchanged.
export function WatchlistRow({ security, quote, busy, onOpen, onRemove, expanded = false, summary, onToggleSummary = () => {}, onRetrySummary = () => {} }: {
	security: TaiwanWatchlistSecurity; quote?: Quote; busy: boolean; onOpen: () => void; onRemove: () => void;
	expanded?: boolean; summary?: WatchlistSummary; onToggleSummary?: () => void; onRetrySummary?: () => void;
}) {
	return <article className="taiwan-watchlist-row">
		<button type="button" className="taiwan-watchlist-identity" onClick={onOpen}><strong>{security.name} {security.code}</strong><span>{security.exchange} · {taiwanSecurityTypeLabel(security.security_type)}</span></button>
		{quote
			? <div className="taiwan-watchlist-quote"><strong>{quote.price.toLocaleString('zh-TW')}</strong><em className={quote.change_percent > 0 ? 'up' : quote.change_percent < 0 ? 'down' : 'flat'}>{quote.change_percent > 0 ? '+' : ''}{quote.change_percent.toFixed(2)}%</em></div>
			: <div className="taiwan-watchlist-quote"><span>報價暫時無法取得</span></div>}
		<button type="button" className="taiwan-watchlist-toggle" aria-expanded={expanded} onClick={onToggleSummary}>{expanded ? '收合摘要' : '展開摘要'}</button>
		<button type="button" className="taiwan-watchlist-remove" onClick={onRemove} disabled={busy}>{busy ? <LoaderCircle className="spin" size={14} /> : <Trash2 size={14} />}移除自選</button>
		{expanded && <WatchlistSummaryPanel summary={summary} onRetry={onRetrySummary} />}
	</article>;
}
