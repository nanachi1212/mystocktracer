import fs from 'node:fs';
import path from 'node:path';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import type { Quote, SourceMeta } from '../lib/backend';
import { chunkTaiwanSymbols, resolveTaiwanWorkspace, taiwanPrimaryNavigation, type TaiwanWatchlistSecurity } from '../lib/taiwan-product';
import { TaiwanWatchlistWorkspace, WatchlistRow, WatchlistSummaryPanel } from './TaiwanWatchlistWorkspace';

const root = path.resolve(__dirname, '../../..');
const workspaceSource = () => fs.readFileSync(path.join(root, 'frontend/src/components/TaiwanWatchlistWorkspace.tsx'), 'utf8');
const appSource = () => fs.readFileSync(path.join(root, 'frontend/src/App.tsx'), 'utf8');
const overviewSource = () => fs.readFileSync(path.join(root, 'frontend/src/components/market/TaiwanMarketView.tsx'), 'utf8');
const researchSource = () => fs.readFileSync(path.join(root, 'frontend/src/components/TaiwanStockResearchWorkspace.tsx'), 'utf8');
const productSource = () => fs.readFileSync(path.join(root, 'frontend/src/lib/taiwan-product.ts'), 'utf8');

const meta: SourceMeta = { source: 'twse:quote', fetched_at: '', latency_ms: 0, stale: false, status: 'official' };
const security = (overrides: Partial<TaiwanWatchlistSecurity> = {}): TaiwanWatchlistSecurity => ({
	canonical: '2330.TWSE', code: '2330', name: '台積電', exchange: 'TWSE', security_type: 'stock', created_at: '2026-09-05T00:00:00Z',
	...overrides,
});
const quote = (overrides: Partial<Quote> = {}): Quote => ({
	symbol: '2330.TWSE', name: '台積電', price: 2410, open: 2400, previous_close: 2390, high: 2420, low: 2395, change: 20, change_percent: 0.84, meta,
	...overrides,
});

describe('M6B — Watchlist navigation exists', () => {
	it('App.tsx declares a taiwan-watchlist workspace mode, hash mapping, and a 自選股 nav button', () => {
		const app = appSource();
		expect(app).toContain("'taiwan-watchlist'");
		expect(resolveTaiwanWorkspace('#taiwan-watchlist')).toBe('taiwan-watchlist');
		expect(app).toContain('return resolveTaiwanWorkspace(window.location.hash)');
		expect(app).toContain('onClick={() => switchWorkspace(mode)}');
		expect(taiwanPrimaryNavigation).toContainEqual(['taiwan-watchlist', '自選股']);
	});

	it('renders TaiwanWatchlistWorkspace for the taiwan-watchlist mode (not a stub/placeholder)', () => {
		const app = appSource();
		expect(app).toContain("workspaceMode === 'taiwan-watchlist' ? <TaiwanWatchlistWorkspace");
	});
});

describe('M6B — cold start / no unintended fetches', () => {
	it('Taiwan overview cold-start indexes effect does not fetch the Watchlist', () => {
		const source = overviewSource();
		const mountEffect = source.slice(source.indexOf('useEffect(() => {'), source.indexOf('[config, refreshKey]'));
		expect(mountEffect).toContain('/api/v1/tw/indexes');
		expect(mountEffect).not.toMatch(/watchlist/i);
	});

	it('TaiwanWatchlistWorkspace only fetches inside its own mount/refresh effect, guarded by config', () => {
		const source = workspaceSource();
		const effectBody = source.slice(source.indexOf('useEffect(() => {'), source.indexOf('}, [config, refreshKey]);'));
		expect(effectBody).toContain('if (!config) return;');
		expect(effectBody).toContain('fetchTaiwanWatchlist');
	});

	it('introduces no polling/timer', () => {
		expect(workspaceSource()).not.toMatch(/setInterval|setTimeout/);
	});

	it('does not reintroduce automatic 2330 selection/search', () => {
		const source = workspaceSource();
		expect(source).not.toContain("'2330'");
		expect(source).not.toMatch(/\bsearch\(/);
	});

	it('does not perform any M6C-style navigation (no switchWorkspace call from the Watchlist view)', () => {
		expect(workspaceSource()).not.toContain('switchWorkspace');
	});
});

describe('M6B — empty state', () => {
	it('renders the explicit Traditional Chinese empty state when config is unset (no securities loaded)', () => {
		const html = renderToStaticMarkup(<TaiwanWatchlistWorkspace config={null} refreshKey={0} onOpenResearch={() => {}} />);
		expect(html).toContain('目前還沒有自選股');
		expect(html).toContain('可從台股總覽或個股分析加入');
	});
});

describe('M6B — WatchlistRow renders saved identities and handles missing quotes safely', () => {
	it('renders a saved TWSE security with its quote', () => {
		const html = renderToStaticMarkup(<WatchlistRow security={security()} quote={quote()} busy={false} onOpen={() => {}} onRemove={() => {}} />);
		expect(html).toContain('台積電');
		expect(html).toContain('2330');
		expect(html).toContain('TWSE');
		expect(html).toContain('2,410');
	});

	it('renders a saved TPEX security correctly (exchange distinction preserved)', () => {
		const tpexSecurity = security({ canonical: '6488.TPEX', code: '6488', name: '環球晶', exchange: 'TPEX' });
		const html = renderToStaticMarkup(<WatchlistRow security={tpexSecurity} quote={quote({ symbol: '6488.TPEX', price: 981 })} busy={false} onOpen={() => {}} onRemove={() => {}} />);
		expect(html).toContain('環球晶');
		expect(html).toContain('TPEX');
		expect(html).toContain('981');
	});

	it('a missing quote does not hide the saved security — the identity still renders', () => {
		const html = renderToStaticMarkup(<WatchlistRow security={security()} quote={undefined} busy={false} onOpen={() => {}} onRemove={() => {}} />);
		expect(html).toContain('台積電');
		expect(html).toContain('2330');
	});

	it('a missing quote shows the explicit unavailable label, never a fabricated 0 price', () => {
		const html = renderToStaticMarkup(<WatchlistRow security={security()} quote={undefined} busy={false} onOpen={() => {}} onRemove={() => {}} />);
		expect(html).toContain('報價暫時無法取得');
		expect(html).not.toMatch(/taiwan-watchlist-quote"><strong>0/);
	});

	it('renders a disabled 移除自選 button while a removal is in flight (duplicate-click protection)', () => {
		const html = renderToStaticMarkup(<WatchlistRow security={security()} quote={quote()} busy={true} onOpen={() => {}} onRemove={() => {}} />);
		expect(html).toMatch(/<button[^>]*disabled[^>]*>/);
	});
});

describe('M6C — WatchlistRow: security area is interactive, remove is a separate action', () => {
	// WatchlistRow is a pure/presentational component (no hooks), so it can be called directly as
	// a plain function outside of React's render lifecycle. This lets us inspect the real element
	// tree it returns and assert the actual onClick wiring — not just source text.
	it('the identity area is its own <button> wired to onOpen, distinct from the remove button (no nested-button markup, real click wiring verified)', () => {
		const onOpen = () => {};
		const onRemove = () => {};
		const element = WatchlistRow({ security: security(), quote: quote(), busy: false, onOpen, onRemove });
		const children = (element.props.children as unknown[]).filter(Boolean) as { type: string; props: { onClick?: () => void; disabled?: boolean } }[];
		const identityButton = children[0];
		const removeButton = children[children.length - 1];
		expect(identityButton.type).toBe('button');
		expect(identityButton.props.onClick).toBe(onOpen);
		expect(removeButton.type).toBe('button');
		expect(removeButton.props.onClick).toBe(onRemove);
		// The two handlers are genuinely distinct functions — clicking remove can never also open.
		expect(identityButton.props.onClick).not.toBe(removeButton.props.onClick);
	});

	it('quote being unavailable does not disable or remove the navigable identity button', () => {
		const onOpen = () => {};
		const element = WatchlistRow({ security: security(), quote: undefined, busy: false, onOpen, onRemove: () => {} });
		const children = (element.props.children as unknown[]).filter(Boolean) as { type: string; props: { onClick?: () => void; disabled?: boolean } }[];
		const identityButton = children[0];
		expect(identityButton.type).toBe('button');
		expect(identityButton.props.onClick).toBe(onOpen);
		expect(identityButton.props.disabled).toBeFalsy();
	});
});

describe('M6B — quote composition uses the existing batch endpoint and chunks deterministically', () => {
	it('chunkTaiwanSymbols groups at most 10 symbols per chunk, in order', () => {
		const symbols = Array.from({ length: 25 }, (_, i) => `${1000 + i}.TWSE`);
		const groups = chunkTaiwanSymbols(symbols);
		expect(groups.length).toBe(3);
		expect(groups[0].length).toBe(10);
		expect(groups[1].length).toBe(10);
		expect(groups[2].length).toBe(5);
		expect(groups[0][0]).toBe('1000.TWSE');
		expect(groups[2][4]).toBe('1024.TWSE');
	});

	it('a Watchlist of 10 or fewer securities produces exactly one chunk', () => {
		const symbols = ['2330.TWSE', '6488.TPEX'];
		expect(chunkTaiwanSymbols(symbols)).toEqual([symbols]);
	});

	it('TaiwanWatchlistWorkspace fetches quotes via the existing /tw/quotes batch endpoint, chunked and settled independently', () => {
		const source = workspaceSource();
		expect(source).toContain('/api/v1/tw/quotes?symbols=');
		expect(source).toContain('chunkTaiwanSymbols(canonicals)');
		expect(source).toContain('Promise.allSettled(groups.map(');
	});
});

describe('M6B — add/remove wire correct HTTP method and canonical symbol', () => {
	it('addTaiwanWatchlistSecurity POSTs { symbol } to the watchlist endpoint', () => {
		const source = productSource();
		const fn = source.slice(source.indexOf('export async function addTaiwanWatchlistSecurity'), source.indexOf('export async function removeTaiwanWatchlistSecurity'));
		expect(fn).toContain("method: 'POST'");
		expect(fn).toContain('JSON.stringify({ symbol: canonical })');
	});

	it('removeTaiwanWatchlistSecurity sends DELETE to the canonical symbol path', () => {
		const source = productSource();
		const fn = source.slice(source.indexOf('export async function removeTaiwanWatchlistSecurity'));
		expect(fn).toContain("method: 'DELETE'");
		expect(source).toContain('taiwanWatchlistRemovePath = (canonical: string) => `/api/v1/tw/watchlist/${encodeURIComponent(canonical)}`');
	});

	it('TaiwanWatchlistWorkspace remove() guards against duplicate/concurrent removal clicks', () => {
		const source = workspaceSource();
		expect(source).toContain('if (!config || removingSymbol) return;');
	});
});

describe('M6B — add/remove toggle in the two existing Taiwan stock views', () => {
	it('TaiwanMarketView shows the toggle only when a valid security is selected and membership is known', () => {
		const source = overviewSource();
		expect(source).toContain("{selected && inWatchlist !== null &&");
		expect(source).toContain('taiwan-watchlist-toggle');
	});

	it('TaiwanStockResearchWorkspace shows the toggle only when a valid security is selected and membership is known', () => {
		const source = researchSource();
		expect(source).toContain("{selected && inWatchlist !== null &&");
		expect(source).toContain('taiwan-watchlist-toggle');
	});

	it('TaiwanMarketView toggleWatchlist guards against duplicate clicks while busy or membership unknown', () => {
		const source = overviewSource();
		expect(source).toContain('if (!config || !selected || watchlistBusy || inWatchlist === null) return;');
	});

	it('TaiwanStockResearchWorkspace toggleWatchlist guards against duplicate clicks while busy or membership unknown', () => {
		const source = researchSource();
		expect(source).toContain('if (!config || !selected || watchlistBusy || inWatchlist === null) return;');
	});

	it('a failed toggle in TaiwanMarketView only sets watchlistError, never touches the displayed stock data', () => {
		const source = overviewSource();
		const fn = source.slice(source.indexOf('const toggleWatchlist = async'), source.indexOf('useEffect(() => {\n\t\tif (!config) return;\n\t\trequestJSON<{ data: MarketIndexSeries[] }>'));
		expect(fn).toContain('setWatchlistError(');
		expect(fn).not.toMatch(/setQuote\(|setLines\(|setInstitutional\(|setMargin\(|setFundamentals\(|setSelected\(/);
	});

	it('a failed toggle in TaiwanStockResearchWorkspace only sets watchlistError, never touches the displayed research', () => {
		const source = researchSource();
		const fn = source.slice(source.indexOf('const toggleWatchlist = async'), source.indexOf('const generateResearch = async'));
		expect(fn).toContain('setWatchlistError(');
		expect(fn).not.toMatch(/setIntelligence\(|setResearch\(|setSelected\(/);
	});

	it('membership checks in both views use their own scoped ref, independent of the main data-fetch race guard', () => {
		expect(overviewSource()).toContain('const watchlistCheckID = useRef(0);');
		expect(researchSource()).toContain('const watchlistCheckID = useRef(0);');
	});
});

// ==================================================
// M8D -- Watchlist Intelligence Summary: lazy, on-demand, per-row expandable summary.
// ==================================================

const summaryValuation = (overrides: Partial<{ data_date?: string; pe: number | null; pb: number | null }> = {}) => ({
	data_date: '2026-09-04', pe: 18.5, pb: 6.2,
	...overrides,
});
const summaryStatement = (overrides: Partial<{ balance_fiscal_year?: number; balance_fiscal_quarter?: number; debt_ratio_percent: number | null; cashflow_fiscal_year?: number; cashflow_fiscal_quarter?: number; cashflow_status?: string; operating_cash_flow: number | null }> = {}) => ({
	balance_fiscal_year: 2026, balance_fiscal_quarter: 1, debt_ratio_percent: 30.94,
	cashflow_fiscal_year: 2026, cashflow_fiscal_quarter: 2, cashflow_status: 'available', operating_cash_flow: 1122637757,
	...overrides,
});

describe('M8D — WatchlistSummaryPanel rendering (real values, no accidental rescale)', () => {
	it('renders the four approved facts with correct Traditional Chinese labels and independent periods', () => {
		const html = renderToStaticMarkup(<WatchlistSummaryPanel summary={{ status: 'available', valuation: summaryValuation(), statement: summaryStatement() }} onRetry={() => {}} />);
		expect(html).toContain('本益比 18.5');
		expect(html).toContain('股價淨值比 6.2');
		expect(html).toContain('負債比 30.94%');
		expect(html).not.toContain('3094%');
		expect(html).toContain('億元');
		expect(html).toContain('資料日期 2026-09-04');
		expect(html).toContain('資產負債表期間 2026-Q1');
		expect(html).toContain('現金流量期間 2026-Q2');
	});

	it('loading (or absent) summary renders the loading indicator, not an empty/broken panel', () => {
		const loading = renderToStaticMarkup(<WatchlistSummaryPanel summary={{ status: 'loading' }} onRetry={() => {}} />);
		expect(loading).toContain('正在讀取個股摘要');
		const absent = renderToStaticMarkup(<WatchlistSummaryPanel summary={undefined} onRetry={() => {}} />);
		expect(absent).toContain('正在讀取個股摘要');
	});

	it('error state renders the error message and a 重試 button wired to onRetry', () => {
		const html = renderToStaticMarkup(<WatchlistSummaryPanel summary={{ status: 'error', error: '個股摘要載入失敗' }} onRetry={() => {}} />);
		expect(html).toContain('個股摘要載入失敗');
		expect(html).toContain('重試');
		expect(html).toMatch(/<button[^>]*>重試<\/button>/);
	});

	it('never contains financial-scoring language', () => {
		const html = renderToStaticMarkup(<WatchlistSummaryPanel summary={{ status: 'available', valuation: summaryValuation(), statement: summaryStatement() }} onRetry={() => {}} />);
		for (const forbidden of ['便宜', '昂貴', '健康', '危險', '值得買', '值得賣']) {
			expect(html).not.toContain(forbidden);
		}
	});
});

describe('M8D — null / zero / negative preservation', () => {
	it('missing valuation/statement renders — for every field, never a fabricated value', () => {
		const html = renderToStaticMarkup(<WatchlistSummaryPanel summary={{ status: 'available', valuation: null, statement: null }} onRetry={() => {}} />);
		expect(html).toContain('本益比 —');
		expect(html).toContain('股價淨值比 —');
		expect(html).toContain('負債比 —');
		expect(html).toContain('營業活動現金流量 —');
		expect(html).toContain('資產負債表期間 —');
		expect(html).toContain('現金流量期間 —');
	});

	it('PE = 0 renders a real zero, never — (not falsy-checked)', () => {
		const html = renderToStaticMarkup(<WatchlistSummaryPanel summary={{ status: 'available', valuation: summaryValuation({ pe: 0 }), statement: summaryStatement() }} onRetry={() => {}} />);
		expect(html).toContain('本益比 0');
		expect(html).not.toContain('本益比 —');
	});

	it('debt_ratio = 0 renders a real zero percent', () => {
		const html = renderToStaticMarkup(<WatchlistSummaryPanel summary={{ status: 'available', valuation: summaryValuation(), statement: summaryStatement({ debt_ratio_percent: 0 }) }} onRetry={() => {}} />);
		expect(html).toContain('負債比 0%');
	});

	it('operating_cash_flow = 0 renders the existing zero cash-flow representation, never —', () => {
		const html = renderToStaticMarkup(<WatchlistSummaryPanel summary={{ status: 'available', valuation: summaryValuation(), statement: summaryStatement({ operating_cash_flow: 0 }) }} onRetry={() => {}} />);
		expect(html).toContain('0 億元');
	});

	it('operating_cash_flow < 0 retains the negative sign', () => {
		const html = renderToStaticMarkup(<WatchlistSummaryPanel summary={{ status: 'available', valuation: summaryValuation(), statement: summaryStatement({ operating_cash_flow: -150005 }) }} onRetry={() => {}} />);
		expect(html).toMatch(/-[\d.,]+\s*億元/);
	});
});

describe('M8D — domain period/status truthfulness', () => {
	it('cashflow partial + a present value: the value remains displayed alongside a truthful partial indication', () => {
		const html = renderToStaticMarkup(<WatchlistSummaryPanel summary={{ status: 'available', valuation: summaryValuation(), statement: summaryStatement({ cashflow_status: 'partial' }) }} onRetry={() => {}} />);
		expect(html).toContain('億元');
		expect(html).toContain('部分資料');
	});

	it('cashflow unavailable (no cashflow fields at all) renders — without a stray partial marker', () => {
		const html = renderToStaticMarkup(<WatchlistSummaryPanel summary={{ status: 'available', valuation: summaryValuation(), statement: summaryStatement({ cashflow_fiscal_year: undefined, cashflow_fiscal_quarter: undefined, cashflow_status: undefined, operating_cash_flow: null }) }} onRetry={() => {}} />);
		expect(html).toContain('營業活動現金流量 —');
		expect(html).not.toContain('部分資料');
		expect(html).toContain('現金流量期間 —');
	});

	it('balance and cashflow periods are shown independently, never collapsed into one generic label', () => {
		const html = renderToStaticMarkup(<WatchlistSummaryPanel summary={{ status: 'available', valuation: summaryValuation(), statement: summaryStatement({ balance_fiscal_year: 2025, balance_fiscal_quarter: 4, cashflow_fiscal_year: 2026, cashflow_fiscal_quarter: 2 }) }} onRetry={() => {}} />);
		expect(html).toContain('資產負債表期間 2025-Q4');
		expect(html).toContain('現金流量期間 2026-Q2');
		expect(html).not.toContain('資料期間');
	});
});

describe('M8D — WatchlistRow toggle wiring (plain-function call, no React lifecycle)', () => {
	it('renders 展開摘要 and aria-expanded=false by default (backward compatible with existing call sites)', () => {
		const html = renderToStaticMarkup(<WatchlistRow security={security()} quote={quote()} busy={false} onOpen={() => {}} onRemove={() => {}} />);
		expect(html).toContain('展開摘要');
		expect(html).toMatch(/aria-expanded="false"/);
	});

	it('renders 收合摘要 and aria-expanded=true, and the summary panel, when expanded', () => {
		const html = renderToStaticMarkup(<WatchlistRow security={security()} quote={quote()} busy={false} onOpen={() => {}} onRemove={() => {}} expanded summary={{ status: 'available', valuation: summaryValuation(), statement: summaryStatement() }} />);
		expect(html).toContain('收合摘要');
		expect(html).toMatch(/aria-expanded="true"/);
		expect(html).toContain('本益比');
	});

	it('no summary panel renders when collapsed, even if a summary happens to be cached', () => {
		const html = renderToStaticMarkup(<WatchlistRow security={security()} quote={quote()} busy={false} onOpen={() => {}} onRemove={() => {}} expanded={false} summary={{ status: 'available', valuation: summaryValuation(), statement: summaryStatement() }} />);
		expect(html).not.toContain('本益比');
	});

	it('the toggle button is its own sibling button, wired to onToggleSummary, distinct from onOpen and onRemove', () => {
		const onOpen = () => {};
		const onRemove = () => {};
		const onToggleSummary = () => {};
		const element = WatchlistRow({ security: security(), quote: quote(), busy: false, onOpen, onRemove, onToggleSummary });
		const children = (element.props.children as unknown[]).filter(Boolean) as { type: string; props: { onClick?: () => void } }[];
		const toggleButton = children[2];
		expect(toggleButton.type).toBe('button');
		expect(toggleButton.props.onClick).toBe(onToggleSummary);
		expect(toggleButton.props.onClick).not.toBe(onOpen);
		expect(toggleButton.props.onClick).not.toBe(onRemove);
	});

	it('existing identity-first / remove-last child ordering is preserved (M6C regression)', () => {
		const onOpen = () => {};
		const onRemove = () => {};
		const element = WatchlistRow({ security: security(), quote: quote(), busy: false, onOpen, onRemove });
		const children = (element.props.children as unknown[]).filter(Boolean) as { type: string; props: { onClick?: () => void } }[];
		expect(children[0].type).toBe('button');
		expect(children[0].props.onClick).toBe(onOpen);
		expect(children[children.length - 1].type).toBe('button');
		expect(children[children.length - 1].props.onClick).toBe(onRemove);
	});
});

describe('M8D — lazy loading / request-cost contract (source-verified, matching this codebase\'s existing convention for hook-driven logic)', () => {
	it('the initial mount/refresh effect never calls the intelligence endpoint (0 calls on Watchlist open)', () => {
		const source = workspaceSource();
		const effectBody = source.slice(source.indexOf('useEffect(() => {'), source.indexOf('}, [config, refreshKey]);'));
		expect(effectBody).toContain('fetchTaiwanWatchlist');
		expect(effectBody).toContain('fetchWatchlistQuotes');
		expect(effectBody).not.toContain('fetchWatchlistSummary');
		expect(effectBody).not.toContain('taiwanIntelligencePath');
	});

	it('expanding calls loadSummary only on the transition into expanded AND only when no summary is already cached', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const toggleSummary = '), source.indexOf('const retrySummary = '));
		expect(fn).toContain('if (!wasExpanded && !summaries[canonical]) void loadSummary(canonical);');
	});

	it('collapsing never clears the cached summary (only expanded visibility changes)', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const toggleSummary = '), source.indexOf('const retrySummary = '));
		expect(fn).not.toMatch(/setSummaries/);
	});

	it('loadSummary guards against a duplicate in-flight request for the same canonical', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const loadSummary = '), source.indexOf('const toggleSummary = '));
		expect(fn).toContain('if (!config || summaryRequestsRef.current.has(canonical)) return;');
		expect(fn).toContain('summaryRequestsRef.current.add(canonical);');
	});

	it('retrySummary issues exactly one new loadSummary call, reusing the same guarded function', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const retrySummary = '), source.indexOf('const remove = '));
		expect(fn).toContain('const retrySummary = (canonical: string) => { void loadSummary(canonical); };');
	});

	it('global refresh invalidates the summary cache but never automatically refetches any row', () => {
		const source = workspaceSource();
		const effectBody = source.slice(source.indexOf('useEffect(() => {'), source.indexOf('}, [config, refreshKey]);'));
		expect(effectBody).toContain('setSummaries({});');
		expect(effectBody).toContain('setExpanded(new Set());');
		expect(effectBody).not.toContain('loadSummary');
		expect(effectBody).not.toContain('fetchWatchlistSummary');
	});
});

describe('M8D — removal clears summary state and guards against a stale async response', () => {
	it('remove() clears both the cached summary and expanded state for the removed canonical', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const remove = async'), source.indexOf('return <div className="taiwan-product-workspace'));
		expect(fn).toContain('setSummaries((current) => { const next = { ...current }; delete next[canonical]; return next; });');
		expect(fn).toContain('setExpanded((current) => { const next = new Set(current); next.delete(canonical); return next; });');
	});

	// M8D.1 — the useEffect that keeps securitiesRef in sync with `securities` only runs on the
	// render AFTER setSecurities is dispatched, leaving a real window in which an in-flight
	// loadSummary() response could still observe the stale ref and resurrect a removed security's
	// summary. remove() must therefore invalidate securitiesRef SYNCHRONOUSLY, in the same
	// synchronous continuation as the successful backend removal — not merely rely on the effect.
	// These assertions lock the actual statement ORDER inside remove(), not just that the lines
	// exist somewhere in the file.
	it('a successful remove synchronously deletes the canonical from securitiesRef immediately after the backend call succeeds, before any other state update', () => {
		const source = workspaceSource();
		const removeStart = source.indexOf('const remove = async');
		const tryStart = source.indexOf('try {', removeStart);
		const catchStart = source.indexOf('} catch (reason) {', removeStart);
		const successBlock = source.slice(tryStart, catchStart);
		const awaitIndex = successBlock.indexOf('await removeTaiwanWatchlistSecurity(config, canonical);');
		const refDeleteIndex = successBlock.indexOf('securitiesRef.current.delete(canonical);');
		const setSecuritiesIndex = successBlock.indexOf('setSecurities((current) => current.filter((item) => item.canonical !== canonical));');
		expect(awaitIndex).toBeGreaterThan(-1);
		expect(refDeleteIndex).toBeGreaterThan(-1);
		expect(setSecuritiesIndex).toBeGreaterThan(-1);
		// The ref invalidation must happen after the network call resolves successfully, and before
		// (or at latest, not after) any of remove()'s own React state updates -- closing the window
		// synchronously rather than waiting for the securities -> securitiesRef effect to catch up.
		expect(refDeleteIndex).toBeGreaterThan(awaitIndex);
		expect(refDeleteIndex).toBeLessThan(setSecuritiesIndex);
	});

	it('a failed remove never invalidates securitiesRef -- the ref-delete line lives only in the success path, never in the catch block', () => {
		const source = workspaceSource();
		const removeStart = source.indexOf('const remove = async');
		const catchStart = source.indexOf('} catch (reason) {', removeStart);
		const finallyStart = source.indexOf('} finally {', removeStart);
		const catchBlock = source.slice(catchStart, finallyStart);
		expect(catchBlock).not.toContain('securitiesRef.current.delete');
		// Confirms the delete call is guarded by the same try block as the network call itself: an
		// exception thrown by removeTaiwanWatchlistSecurity jumps straight to catch and never reaches
		// the ref-delete line, so a still-listed (removal-failed) security remains "present" for any
		// in-flight intelligence request to validly populate.
	});

	it('the in-flight request guard (summaryRequestsRef) is never touched by remove() -- an in-flight request is left to finish and discard itself naturally via the stale-response check', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const remove = async'), source.indexOf('return <div className="taiwan-product-workspace'));
		expect(fn).not.toContain('summaryRequestsRef');
	});

	it('loadSummary drops a stale response for a security no longer on the watchlist (checked before every state update)', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const loadSummary = '), source.indexOf('const toggleSummary = '));
		expect((fn.match(/if \(!securitiesRef\.current\.has\(canonical\)\) return;/g) || []).length).toBeGreaterThanOrEqual(2);
	});

	it('securitiesRef is kept in sync with the current securities list', () => {
		expect(workspaceSource()).toContain('useEffect(() => { securitiesRef.current = new Set(securities.map((item) => item.canonical)); }, [securities]);');
	});
});

describe('M8D — AI boundary: expanding a summary never calls Research/AI', () => {
	it('the Watchlist workspace never imports the research path helper or AI research type', () => {
		const source = workspaceSource();
		const imports = source.slice(0, source.indexOf('type QuoteLookup'));
		expect(imports).not.toContain('taiwanResearchPath');
		expect(imports).not.toContain('TaiwanAIResearch');
	});

	it('fetchWatchlistSummary calls only the existing intelligence endpoint', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('async function fetchWatchlistSummary'), source.indexOf('export function TaiwanWatchlistWorkspace'));
		expect(fn).toContain('taiwanIntelligencePath(canonical)');
	});

	it('Research navigation (onOpenResearch) remains wired to the identity button only, distinct from the summary toggle', () => {
		const source = workspaceSource();
		expect(source).toContain('onOpen={() => onOpenResearch(item.canonical)}');
		expect(source).toContain('onToggleSummary={() => toggleSummary(item.canonical)}');
	});
});

describe('M8D — no new dependency, no polling, no CSS beyond the documented minimum', () => {
	it('still introduces no polling/timer', () => {
		expect(workspaceSource()).not.toMatch(/setInterval|setTimeout/);
	});

	it('reuses existing formatters rather than duplicating their logic', () => {
		const source = workspaceSource();
		expect(source).toContain('formatTaiwanPlainNumber');
		expect(source).toContain('formatTaiwanPercent');
		expect(source).toContain('formatTaiwanCashFlowTWD');
	});
});

describe('P5.6 — ETF applicability differentiation in WatchlistSummaryPanel', () => {
	it('A. regular stock (security_type = stock) renders valuation, balance sheet, and cash flow', () => {
		const html = renderToStaticMarkup(<WatchlistSummaryPanel summary={{ status: 'available', valuation: summaryValuation(), statement: summaryStatement() }} onRetry={() => {}} securityType="stock" />);
		expect(html).toContain('估值');
		expect(html).toContain('本益比 18.5');
		expect(html).toContain('資產負債');
		expect(html).toContain('負債比 30.94%');
		expect(html).toContain('現金流量');
		expect(html).toContain('營業活動現金流量');
		expect(html).not.toContain('ETF 不適用一般公司的資產負債表與營業現金流指標。');
	});

	it('B. ETF (security_type = etf) replaces wall of corporate "—" with clear applicability note', () => {
		const html = renderToStaticMarkup(<WatchlistSummaryPanel summary={{ status: 'available', valuation: null, statement: null }} onRetry={() => {}} securityType="etf" />);
		expect(html).not.toContain('負債比 —');
		expect(html).not.toContain('資產負債表期間 —');
		expect(html).not.toContain('營業活動現金流量 —');
		expect(html).not.toContain('現金流量期間 —');
		expect(html).toContain('ETF 不適用一般公司的資產負債表與營業現金流指標。');
	});

	it('C. ETF retains valuation block when legitimate valuation data exists', () => {
		const html = renderToStaticMarkup(<WatchlistSummaryPanel summary={{ status: 'available', valuation: summaryValuation({ pe: 21.3, pb: 2.8 }), statement: null }} onRetry={() => {}} securityType="etf" />);
		expect(html).toContain('估值');
		expect(html).toContain('本益比 21.3');
		expect(html).toContain('股價淨值比 2.8');
		expect(html).toContain('ETF 不適用一般公司的資產負債表與營業現金流指標。');
		expect(html).not.toContain('負債比 —');
		expect(html).not.toContain('營業活動現金流量 —');
	});

	it('D. ETF never fakes debt_ratio as 0% or operating_cash_flow as 0', () => {
		const html = renderToStaticMarkup(<WatchlistSummaryPanel summary={{ status: 'available', valuation: null, statement: null }} onRetry={() => {}} securityType="etf" />);
		expect(html).not.toContain('負債比 0%');
		expect(html).not.toContain('0 億元');
	});

	it('E. ETF applicability is not treated as an error state', () => {
		const html = renderToStaticMarkup(<WatchlistSummaryPanel summary={{ status: 'available', valuation: null, statement: null }} onRetry={() => {}} securityType="etf" />);
		expect(html).not.toContain('market-partial-warning');
		expect(html).not.toContain('個股摘要載入失敗');
		expect(html).not.toContain('重試');
	});

	it('F. regular stock with truly missing/unavailable data still renders — without misapplying ETF note', () => {
		const html = renderToStaticMarkup(<WatchlistSummaryPanel summary={{ status: 'available', valuation: null, statement: null }} onRetry={() => {}} securityType="stock" />);
		expect(html).toContain('負債比 —');
		expect(html).toContain('營業活動現金流量 —');
		expect(html).not.toContain('ETF 不適用一般公司的資產負債表與營業現金流指標。');
	});

	it('G. ETF differentiation relies strictly on security_type, never on symbol code or name heuristic', () => {
		const etfSecurity = security({ code: '0050', name: '元大台灣50', security_type: 'etf' });
		const htmlETF = renderToStaticMarkup(<WatchlistRow security={etfSecurity} quote={quote()} busy={false} onOpen={() => {}} onRemove={() => {}} expanded={true} summary={{ status: 'available', valuation: null, statement: null }} />);
		expect(htmlETF).toContain('ETF 不適用一般公司的資產負債表與營業現金流指標。');

		const pseudoStock = security({ code: '0050X', name: '假裝ETF實為股票', security_type: 'stock' });
		const htmlStock = renderToStaticMarkup(<WatchlistRow security={pseudoStock} quote={quote()} busy={false} onOpen={() => {}} onRemove={() => {}} expanded={true} summary={{ status: 'available', valuation: null, statement: null }} />);
		expect(htmlStock).toContain('負債比 —');
		expect(htmlStock).not.toContain('ETF 不適用一般公司的資產負債表與營業現金流指標。');
	});
});
