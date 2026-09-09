import fs from 'node:fs';
import path from 'node:path';
import type { ReactElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import { InterpretationView, ResearchView, ScreenerEntryContext, TaiwanResearchSnapshot, type Component, type Intelligence, type IntelligenceCore, type Research, type TaiwanStatementData, type TaiwanValuationData } from './TaiwanStockResearchWorkspace';
import { runScopedRequest, type TaiwanResearchEntryContext } from '../lib/taiwan-product';
import { TaiwanStockResearchErrorBoundary } from './TaiwanStockResearchErrorBoundary';

const root = path.resolve(__dirname, '../../..');
const researchSource = () => fs.readFileSync(path.join(root, 'frontend/src/components/TaiwanStockResearchWorkspace.tsx'), 'utf8');
const combinedEffectBody = () => {
	const source = researchSource();
	return source.slice(source.indexOf('// Preserve the initial default (2330) on first load'), source.indexOf('}, [config, refreshKey, externalSymbolRequest]);'));
};
const appSource = () => fs.readFileSync(path.join(root, 'frontend/src/App.tsx'), 'utf8');
const watchlistSource = () => fs.readFileSync(path.join(root, 'frontend/src/components/TaiwanWatchlistWorkspace.tsx'), 'utf8');

const component = (overrides: Partial<Component> = {}): Component => ({
	state: 'positive', status: 'available', freshness: 'fresh', as_of: '2026-09-03', reasons: ['reason'],
	...overrides,
});

type DataQuality = NonNullable<Intelligence['interpretation']>['data_quality'];

const baseIntelligence = (components: Record<string, Component>, dataQuality: DataQuality): Intelligence => ({
	model_version: 'taiwan_stock_intelligence_v1', symbol: '2330.TWSE',
	identity: { canonical_symbol: '2330.TWSE', code: '2330', name: '台積電', exchange: 'TWSE', currency: 'TWD', security_type: 'stock' },
	quote: { status: 'available' },
	price_history_summary: { status: 'available', return_5d_percent: 1, return_20d_percent: 2 },
	fundamentals: { status: 'available' }, institutional: { status: 'available' }, margin: { status: 'available' },
	market_context: { status: 'available' }, industry_context: { status: 'available' },
	interpretation: { model_version: 'taiwan_stock_interpretation_v1', components, data_quality: dataQuality },
});

const fullComponents: Record<string, Component> = {
	price: component(), market: component(), price_market_relationship: component(),
	industry: component(), institutional: component(), margin: component(), fundamentals: component(),
};

describe('InterpretationView data_quality null handling', () => {
	it('renders the exact-null-crash fixture (2330-like: every component available, empty categories serialized as null) without throwing', () => {
		const intelligence = baseIntelligence(fullComponents, {
			available_components: ['price', 'market', 'price_market_relationship', 'industry', 'institutional', 'margin', 'fundamentals'],
			indeterminate_components: null, unavailable_components: null, stale_components: null, partial_components: null,
		});
		const html = renderToStaticMarkup(<InterpretationView intelligence={intelligence} />);
		expect(html).toContain('可用 7 項');
		expect(html).toContain('無法判定 0 項');
		expect(html).toContain('無法取得／不適用 0 項');
	});

	it('renders a 6488-like (TPEx) fixture with a mix of indeterminate and unavailable components', () => {
		const components: Record<string, Component> = {
			...fullComponents,
			institutional: component({ state: 'indeterminate', status: 'data_insufficient', reasons: ['latest official institutional flow is unavailable'] }),
			margin: component({ state: 'indeterminate', status: 'unavailable', reasons: ['margin section is unavailable'] }),
		};
		const intelligence = baseIntelligence(components, {
			available_components: ['price', 'market', 'price_market_relationship', 'industry', 'fundamentals'],
			indeterminate_components: ['institutional'], unavailable_components: ['margin'],
			stale_components: null, partial_components: null,
		});
		intelligence.identity = { ...intelligence.identity, canonical_symbol: '6488.TPEX', code: '6488', exchange: 'TPEX' };
		const html = renderToStaticMarkup(<InterpretationView intelligence={intelligence} />);
		expect(html).toContain('可用 5 項');
		expect(html).toContain('無法判定 1 項');
		expect(html).toContain('無法取得／不適用 1 項');
	});

	it('renders a 0050 ETF fixture where industry/fundamentals are not_applicable, not fabricated company data', () => {
		const components: Record<string, Component> = {
			...fullComponents,
			industry: component({ state: 'not_applicable', status: 'not_applicable', reasons: ['industry interpretation is not applicable to non-stock security types'] }),
			fundamentals: component({ state: 'not_applicable', status: 'not_applicable', reasons: ['ordinary-stock fundamentals are not applicable'] }),
		};
		const intelligence = baseIntelligence(components, {
			available_components: ['price', 'market', 'price_market_relationship', 'institutional', 'margin'],
			indeterminate_components: null, unavailable_components: ['industry', 'fundamentals'],
			stale_components: null, partial_components: null,
		});
		intelligence.identity = { canonical_symbol: '0050.TWSE', code: '0050', name: '元大台灣50', exchange: 'TWSE', currency: 'TWD', security_type: 'etf' };
		const html = renderToStaticMarkup(<InterpretationView intelligence={intelligence} />);
		expect(html).toContain('無法取得／不適用 2 項');
	});

	it('still counts stale and partial categories correctly when populated', () => {
		const intelligence = baseIntelligence(fullComponents, {
			available_components: ['price', 'market', 'price_market_relationship', 'industry', 'institutional', 'margin', 'fundamentals'],
			indeterminate_components: [], unavailable_components: [],
			stale_components: ['price'], partial_components: ['institutional'],
		});
		const html = renderToStaticMarkup(<InterpretationView intelligence={intelligence} />);
		expect(html).toContain('資料較舊 1 項');
		expect(html).toContain('部分資料 1 項');
	});
});

describe('TaiwanStockResearchErrorBoundary', () => {
	// renderToStaticMarkup (React's legacy SSR renderer, the only renderer this repo's test setup
	// exercises without adding a DOM/testing-library dependency) does not run error-boundary
	// recovery the way client rendering does — a thrown child still aborts the whole render.
	// So the catch/fallback contract is verified directly against the boundary's own lifecycle
	// methods instead of through a full render pass.
	it('getDerivedStateFromError flips the boundary into its error state', () => {
		expect(TaiwanStockResearchErrorBoundary.getDerivedStateFromError()).toEqual({ hasError: true });
	});

	it('renders the Traditional Chinese fallback with a reload action once in the error state', () => {
		const instance = new TaiwanStockResearchErrorBoundary({ children: <div>正常內容</div> });
		instance.state = { hasError: true };
		const html = renderToStaticMarkup(instance.render() as ReactElement);
		expect(html).toContain('個股分析畫面發生錯誤，請重新載入此分析。');
		expect(html).toContain('重新載入');
	});

	it('renders children normally when there is no error', () => {
		const html = renderToStaticMarkup(
			<TaiwanStockResearchErrorBoundary>
				<div>正常內容</div>
			</TaiwanStockResearchErrorBoundary>,
		);
		expect(html).toContain('正常內容');
	});
});

describe('M6C — Watchlist row passes canonical identity, not code, into App-level handoff state', () => {
	it('TaiwanWatchlistWorkspace opens research using item.canonical, never item.code', () => {
		const source = watchlistSource();
		expect(source).toContain('onOpen={() => onOpenResearch(item.canonical)}');
		expect(source).not.toContain('onOpenResearch(item.code)');
	});

	it('App.tsx switches to taiwan-stock (not taiwan-research) when a Watchlist row is opened, carrying a strictly increasing token', () => {
		const app = appSource();
		const fn = app.slice(app.indexOf('const openTaiwanStockResearch ='), app.indexOf('const askMasteryAI ='));
		expect(fn).toContain('taiwanSymbolRequestNonce.current += 1');
		expect(fn).toContain('setRequestedTaiwanSymbol({ canonical, token: taiwanSymbolRequestNonce.current, context });');
		expect(fn).toContain("switchWorkspace('taiwan-stock')");
	});

	it('App.tsx passes externalSymbolRequest to the stock research workspace (single canonical stock-analysis branch)', () => {
		const app = appSource();
		expect(app.match(/<TaiwanStockResearchWorkspace config=\{config\} refreshKey=\{marketRefreshKey\} externalSymbolRequest=\{requestedTaiwanSymbol\}/g)?.length).toBe(1);
	});
});

describe('M6C — external symbol resolution requires an exact canonical match, never a fuzzy result or a 2330 fallback', () => {
	it('resolves via /tw/securities?query= using the requested canonical symbol', () => {
		expect(combinedEffectBody()).toContain('/api/v1/tw/securities?query=${encodeURIComponent(canonical)}');
	});

	it('requires an exact canonical match (.find with strict equality), not the first/fuzzy result', () => {
		const body = combinedEffectBody();
		expect(body).toContain('payload.data.securities.find((item) => item.canonical === canonical)');
		expect(body).not.toMatch(/securities\[0\]/);
	});

	it('shows the safe not-found message and does not select anything when no exact match exists', () => {
		const body = combinedEffectBody();
		expect(body).toContain("setError('找不到自選股對應的台灣證券資料。')");
	});

	it('never falls back to 2330 on resolution failure', () => {
		const body = combinedEffectBody();
		// The only search('2330') call in this effect is the guarded default-mount branch, gated on
		// the ABSENCE of an external request — the resolution branch itself can never reach it.
		expect(body).toContain("if (!externalSymbolRequest) void search('2330');");
		expect(body).not.toMatch(/else void search\('2330'\)/);
	});

	it('does not automatically trigger AI research (generateResearch/taiwanResearchPath) after navigation', () => {
		expect(combinedEffectBody()).not.toContain('generateResearch');
		expect(combinedEffectBody()).not.toContain('taiwanResearchPath');
	});
});

describe('M6C — race protection: repeated/rapid Watchlist clicks resolve correctly', () => {
	it('the external-resolution branch is scoped with its own runScopedRequest, independent of search()/select()\'s own guards', () => {
		const source = researchSource();
		expect(source).toContain('const externalResolveRequestID = useRef(0);');
		expect(source).toContain('void runScopedRequest(externalResolveRequestID,');
	});

	it('a token equal to the last-handled one is ignored (protects against duplicate effect re-fires), but any new token is processed', () => {
		const source = researchSource();
		expect(source).toContain('if (externalSymbolRequest && externalSymbolRequest.token !== lastExternalTokenRef.current) {');
		expect(source).toContain('lastExternalTokenRef.current = externalSymbolRequest.token;');
	});

	it('the mount/refresh effect reacts to config, refreshKey, AND externalSymbolRequest together (a single effect, not two with an ordering dependency between them)', () => {
		const source = researchSource();
		expect(source).toContain('}, [config, refreshKey, externalSymbolRequest]);');
		// Only one useEffect references externalSymbolRequest — confirming the resolution logic and
		// the default/retry fallback live in the same effect, not two effects that could race.
		expect(source.match(/useEffect\(\(\) => \{[^]*?externalSymbolRequest/g)?.length).toBeGreaterThanOrEqual(1);
	});

	it('while a resolution is in flight (token already marked handled but no selection yet), neither the resolve branch nor the default-2330 branch re-fires', () => {
		const body = combinedEffectBody();
		expect(body).toContain('if (selectedRef.current) { void select(selectedRef.current); return; }');
		expect(body).toContain("if (!externalSymbolRequest) void search('2330');");
	});
});

describe('M6C — refresh and manual-entry behavior are preserved', () => {
	it('global refresh retries selectedRef.current once a resolved external selection exists (P1E preserved)', () => {
		const body = combinedEffectBody();
		expect(body).toContain('if (selectedRef.current) { void select(selectedRef.current); return; }');
	});

	it('normal manual entry (externalSymbolRequest never set) preserves the exact original default-2330 mount behavior', () => {
		const body = combinedEffectBody();
		expect(body).toContain("if (!externalSymbolRequest) void search('2330');");
	});
});

describe('M6C — no polling/timer introduced, no backend change required', () => {
	it('introduces no setInterval/setTimeout in TaiwanStockResearchWorkspace', () => {
		expect(researchSource()).not.toMatch(/setInterval|setTimeout/);
	});
});

// ==================================================
// M8A -- Taiwan Research Snapshot (估值/獲利能力/資產負債/現金流量), reusing existing backend evidence.
// ==================================================

const valuation = (overrides: Partial<TaiwanValuationData> = {}): TaiwanValuationData => ({
	data_date: '2026-09-04', pe: 18.5, pb: 6.2, dividend_yield_percent: 2.1,
	...overrides,
});
const statement = (overrides: Partial<TaiwanStatementData> = {}): TaiwanStatementData => ({
	fiscal_year: 2026, fiscal_quarter: 2, accounting_category: 'ci',
	cumulative_eps: 9.55, gross_margin_percent: 67.03, operating_margin_percent: 59.29, net_margin_percent: 53.22,
	book_value_per_share: 28.5,
	balance_fiscal_year: 2026, balance_fiscal_quarter: 1,
	debt_ratio_percent: 30.94, debt_to_equity_percent: 44.81, current_ratio_percent: 245.76,
	cashflow_fiscal_year: 2026, cashflow_fiscal_quarter: 2,
	operating_cash_flow: 1122637757, cash_flow_to_net_income: 148.06,
	...overrides,
});
const fundamentalsWith = (v?: TaiwanValuationData, s?: TaiwanStatementData): Intelligence['fundamentals'] => ({
	status: 'available', data: { valuation: v, financial_statement: s },
});

describe('M8A — four snapshot groups render with real values', () => {
	it('renders 估值/獲利能力/資產負債/現金流量 headings and their values, no ×100 regression', () => {
		const html = renderToStaticMarkup(<TaiwanResearchSnapshot fundamentals={fundamentalsWith(valuation(), statement())} />);
		expect(html).toContain('估值');
		expect(html).toContain('獲利能力');
		expect(html).toContain('資產負債');
		expect(html).toContain('現金流量');
		expect(html).toContain('67.03%');
		expect(html).not.toContain('6703%');
		expect(html).toContain('2.1%');
		expect(html).toContain('30.94%');
		expect(html).toContain('148.06%');
		expect(html).toContain('億元');
	});

	it('independent periods: 財報期間/資產負債表期間/現金流量期間 are shown separately, never collapsed into one', () => {
		const html = renderToStaticMarkup(<TaiwanResearchSnapshot fundamentals={fundamentalsWith(valuation(), statement({ fiscal_year: 2026, fiscal_quarter: 2, balance_fiscal_year: 2026, balance_fiscal_quarter: 1, cashflow_fiscal_year: 2025, cashflow_fiscal_quarter: 4 }))} />);
		expect(html).toContain('財報期間 2026-Q2');
		expect(html).toContain('資產負債表期間 2026-Q1');
		expect(html).toContain('現金流量期間 2025-Q4');
	});
});

describe('M8A — null / zero / negative semantics', () => {
	it('missing valuation/statement entirely renders — for every field, never 0 or a fabricated period', () => {
		const html = renderToStaticMarkup(<TaiwanResearchSnapshot fundamentals={{ status: 'unavailable' }} />);
		expect(html).not.toContain('0%');
		expect(html).not.toContain('0 億元');
		expect((html.match(/—/g) || []).length).toBeGreaterThanOrEqual(9);
	});

	it('zero renders as a real zero, never —', () => {
		const html = renderToStaticMarkup(<TaiwanResearchSnapshot fundamentals={fundamentalsWith(
			valuation({ dividend_yield_percent: 0 }),
			statement({ debt_ratio_percent: 0, operating_cash_flow: 0, cash_flow_to_net_income: 0 }),
		)} />);
		expect(html).toContain('殖利率 0%');
		expect(html).toContain('負債比 0%');
		expect(html).toContain('0 億元');
		expect(html).toContain('營業現金流／淨利 0%');
	});

	it('negative values are preserved (cash_flow_to_net_income and operating_cash_flow can be legitimately negative)', () => {
		const html = renderToStaticMarkup(<TaiwanResearchSnapshot fundamentals={fundamentalsWith(
			valuation(),
			statement({ operating_cash_flow: -150005, cash_flow_to_net_income: -6.34 }),
		)} />);
		expect(html).toContain('-6.34%');
		expect(html).toMatch(/-[\d.,]+\s*億元/);
	});

	it('cashflow fields null while the rest of the statement (valuation/EPS/margins/balance ratios) still renders -- an unavailable cashflow domain never hides other Research sections', () => {
		const html = renderToStaticMarkup(<TaiwanResearchSnapshot fundamentals={fundamentalsWith(
			valuation(),
			statement({ cashflow_fiscal_year: undefined, cashflow_fiscal_quarter: undefined, operating_cash_flow: null, cash_flow_to_net_income: null }),
		)} />);
		expect(html).toContain('現金流量期間 —');
		expect(html).toContain('營業活動現金流量 —');
		// Balance/income groups remain fully populated despite the cashflow domain being absent.
		expect(html).toContain('30.94%');
		expect(html).toContain('67.03%');
	});

	it('ETF / not_applicable fundamentals renders every field as — without throwing', () => {
		const html = renderToStaticMarkup(<TaiwanResearchSnapshot fundamentals={{ status: 'not_applicable', reason: 'ordinary-stock fundamentals are not applicable' }} />);
		expect(html).toContain('估值');
		expect((html.match(/—/g) || []).length).toBeGreaterThanOrEqual(9);
	});
});

// ==================================================
// M8B -- Screener → Research navigation context ("來自台股篩選器"), ephemeral and frontend-only.
// ==================================================

const screenerContext = (overrides: Partial<TaiwanResearchEntryContext> = {}): TaiwanResearchEntryContext => ({
	source: 'screener', filterLabels: ['本益比（PE） ≤ 25', '負債比率 ≤ 50%'], sortLabel: '本益比（PE）（低到高）',
	...overrides,
});

describe('M8B — ScreenerEntryContext rendering', () => {
	it('renders the exact heading and both subsections, separating 篩選條件 from 排序方式', () => {
		const html = renderToStaticMarkup(<ScreenerEntryContext context={screenerContext()} />);
		expect(html).toContain('來自台股篩選器');
		expect(html).toContain('你從以下篩選條件的結果中開啟此分析');
		expect(html).toContain('篩選條件');
		expect(html).toContain('排序方式');
		expect(html).toContain('本益比（PE） ≤ 25');
		expect(html).toContain('負債比率 ≤ 50%');
		expect(html).toContain('本益比（PE）（低到高）');
		// Never uses a present-tense match claim -- this is provenance, not a re-validated match.
		expect(html).not.toContain('目前仍符合');
		expect(html).not.toContain('此股票符合以下條件');
	});

	it('no active filters still shows the block (sort is always active) with an explicit "no criteria" note', () => {
		const html = renderToStaticMarkup(<ScreenerEntryContext context={screenerContext({ filterLabels: [] })} />);
		expect(html).toContain('未設定篩選條件');
		expect(html).toContain('排序方式');
	});

	it('zero and negative criterion values render exactly as given, never dropped or rewritten', () => {
		const html = renderToStaticMarkup(<ScreenerEntryContext context={screenerContext({ filterLabels: ['營業活動現金流量 ≥ 0仟元', '漲跌幅 ≥ -5%'] })} />);
		expect(html).toContain('營業活動現金流量 ≥ 0仟元');
		expect(html).toContain('漲跌幅 ≥ -5%');
	});
});

describe('M8B — navigation wiring (Screener context threaded through the existing M6C handoff)', () => {
	it('ExternalTaiwanSymbolRequest carries an optional context field, never required', () => {
		const source = researchSource();
		expect(source).toContain('export type ExternalTaiwanSymbolRequest = { canonical: string; token: number; context?: TaiwanResearchEntryContext | null };');
	});

	it('select() accepts an optional context override distinguishing "leave untouched" (undefined) from "clear" (null)', () => {
		const source = researchSource();
		expect(source).toContain('const select = async (security: SecurityIdentity, context?: TaiwanResearchEntryContext | null) => {');
		expect(source).toContain('if (context !== undefined) setScreenerContext(context);');
	});

	it('a Screener-originated external request threads its own context (or null) into select()', () => {
		const source = researchSource();
		expect(source).toContain('void select(exact, externalSymbolRequest.context ?? null);');
	});

	it('manual search/selection (not Screener-originated) always clears the context explicitly', () => {
		const source = researchSource();
		expect(source).toContain("await select(payload.data.securities[0], null);");
		expect(source).toContain('onClick={() => void select(item, null)}');
	});

	it('the refreshKey retry of an already-selected symbol calls select() with no context argument, preserving whatever context is already set', () => {
		const body = combinedEffectBody();
		expect(body).toContain('if (selectedRef.current) { void select(selectedRef.current); return; }');
	});

	it('the context block is rendered only when screenerContext is set, and appears before EvidenceOverview -- never inside it', () => {
		const source = researchSource();
		expect(source).toContain('{screenerContext && <ScreenerEntryContext context={screenerContext} />}');
		const contextIndex = source.indexOf('{screenerContext && <ScreenerEntryContext context={screenerContext} />}');
		const evidenceIndex = source.indexOf('<EvidenceOverview intelligence={intelligence} />');
		expect(contextIndex).toBeGreaterThan(-1);
		expect(contextIndex).toBeLessThan(evidenceIndex);
	});

	it('ScreenerEntryContext is never referenced from EvidenceOverview/TaiwanResearchSnapshot/InterpretationView', () => {
		const source = researchSource();
		const evidenceOverviewBody = source.slice(source.indexOf('function EvidenceOverview'), source.indexOf('function snapshotPeriod'));
		const snapshotBody = source.slice(source.indexOf('export function TaiwanResearchSnapshot'), source.indexOf('export function InterpretationView'));
		expect(evidenceOverviewBody).not.toContain('ScreenerEntryContext');
		expect(snapshotBody).not.toContain('ScreenerEntryContext');
	});

	it('App.tsx builds no separate screener-context plumbing beyond the existing requestedTaiwanSymbol/openTaiwanStockResearch handoff', () => {
		const app = appSource();
		expect(app).toContain('const openTaiwanStockResearch = (canonical: string, context?: TaiwanResearchEntryContext | null) => {');
		expect(app).toContain('setRequestedTaiwanSymbol({ canonical, token: taiwanSymbolRequestNonce.current, context });');
	});

	it('the Watchlist call site is unaffected -- it still calls onOpenResearch(item.canonical) with no context, per the existing M6C wiring', () => {
		expect(watchlistSource()).toContain('onOpen={() => onOpenResearch(item.canonical)}');
	});
});

// ==================================================
// M8C -- Evidence-grounded Taiwan Research Synthesis: minimal strengths/risks rendering.
// ==================================================

const researchResult = (overrides: Partial<Research> = {}): Research => ({
	status: 'available', model_version: 'taiwan_ai_research_v1', headline: '證據綜合摘要', summary: '摘要內容。',
	sections: { price: { text: '價格證據。', evidence_keys: ['interpretation.components.price'] } },
	strengths: [], risks: [], conflicts: [], data_limitations: [], research_notes: [],
	...overrides,
});

describe('M8C — ResearchView strengths/risks rendering', () => {
	it('renders 支持性證據/風險證據 with text and evidence_keys when non-empty', () => {
		const html = renderToStaticMarkup(<ResearchView research={researchResult({
			strengths: [{ text: '營業活動現金流量為正。', evidence_keys: ['fundamentals.data.cashflow.operating_cash_flow'] }],
			risks: [{ text: '營業現金流／淨利為負。', evidence_keys: ['fundamentals.data.cashflow.cash_flow_to_net_income'] }],
		})} />);
		expect(html).toContain('支持性證據');
		expect(html).toContain('營業活動現金流量為正。');
		expect(html).toContain('fundamentals.data.cashflow.operating_cash_flow');
		expect(html).toContain('風險證據');
		expect(html).toContain('營業現金流／淨利為負。');
		expect(html).toContain('fundamentals.data.cashflow.cash_flow_to_net_income');
	});

	it('renders neither block when both arrays are empty', () => {
		const html = renderToStaticMarkup(<ResearchView research={researchResult()} />);
		expect(html).not.toContain('支持性證據');
		expect(html).not.toContain('風險證據');
	});

	it('renders only the non-empty one when only strengths (or only risks) is populated', () => {
		const strengthsOnly = renderToStaticMarkup(<ResearchView research={researchResult({ strengths: [{ text: '正向觀察。', evidence_keys: ['fundamentals.data.valuation.pe'] }] })} />);
		expect(strengthsOnly).toContain('支持性證據');
		expect(strengthsOnly).not.toContain('風險證據');
	});
});

describe('P5.3A — Subscription AI integration in TaiwanStockResearchWorkspace', () => {
	it('contains the [使用已訂閱的 AI] button and wires SubscriptionAIModal correctly', () => {
		const source = researchSource();
		expect(source).toContain('使用已訂閱的 AI');
		expect(source).toContain('setSubscriptionAIModalOpen(true)');
		expect(source).toContain('<SubscriptionAIModal');
	});
});

describe('P5.5C.1 — Two-Stage Progressive Loading in TaiwanStockResearchWorkspace', () => {
	it('defines IntelligenceCore compatible with minimal quote, price_history, and identity', () => {
		const core: IntelligenceCore = {
			model_version: 'taiwan_stock_intelligence_core_v1',
			symbol: '2330.TWSE',
			identity: { canonical_symbol: '2330.TWSE', code: '2330', name: '台積電', exchange: 'TWSE', currency: 'TWD', security_type: 'stock' },
			quote: { status: 'available', data: { price: 950, change_percent: 1.5 } },
			price_history_summary: { status: 'available', return_5d_percent: 2.1, return_20d_percent: 5.3 },
		};
		expect(core.symbol).toBe('2330.TWSE');
		expect(core.quote.data?.price).toBe(950);
	});

	it('select() triggers two-stage fetch with core first paint before full intelligence', () => {
		const source = researchSource();
		expect(source).toContain('taiwanIntelligenceCorePath(security.canonical)');
		expect(source).toContain('taiwanIntelligencePath(security.canonical)');
		const coreIndex = source.indexOf('taiwanIntelligenceCorePath(security.canonical)');
		const fullIndex = source.indexOf('taiwanIntelligencePath(security.canonical)');
		expect(coreIndex).toBeGreaterThan(-1);
		expect(fullIndex).toBeGreaterThan(coreIndex);
	});

	it('maintains runScopedRequest selectRequestID guard across the two stages to prevent stale races', () => {
		const source = researchSource();
		expect(source).toContain('await runScopedRequest(selectRequestID, async () => {');
		expect(source).toContain('setCoreData(null)');
		expect(source).toContain('setIntelligence(null)');
	});

	it('renders displayData (coreData or intelligence) prioritizing header and quote while isolating full error', () => {
		const source = researchSource();
		expect(source).toContain('const displayData = intelligence ?? coreData;');
		expect(source).toContain('{fullError && <div className="market-partial-warning"');
		expect(source).toContain('{fullLoading && !intelligence && <div className="taiwan-loading"');
	});

	it('TaiwanStockResearchWorkspace source includes per-stage token guards and onSuccess handler', () => {
		const source = researchSource();
		expect(source).toContain('const requestID = selectRequestID.current;');
		const guardMatches = source.match(/if \(requestID !== selectRequestID\.current\) return;/g);
		expect(guardMatches?.length).toBe(4);
		expect(source).toContain('onSuccess: () => {},');
	});
});

function deferred<T>() {
	let resolve!: (value: T) => void;
	let reject!: (reason?: unknown) => void;
	const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej; });
	promise.catch(() => {});
	return { promise, resolve, reject };
}

describe('P5.5C.1 — Two-Stage Async Race & Lifecycle Behavior', () => {
	const createSampleCore = (symbol: string, code: string, name: string): IntelligenceCore => ({
		model_version: 'taiwan_stock_intelligence_core_v1',
		symbol,
		identity: { canonical_symbol: symbol, code, name, exchange: 'TWSE', currency: 'TWD', security_type: 'stock' },
		quote: { status: 'available', data: { price: 100, change_percent: 1.0 } },
		price_history_summary: { status: 'available', return_5d_percent: 2.0, return_20d_percent: 4.0 },
	});

	const createSampleFull = (symbol: string, code: string, name: string): Intelligence => ({
		model_version: 'taiwan_stock_intelligence_v1',
		symbol,
		identity: { canonical_symbol: symbol, code, name, exchange: 'TWSE', currency: 'TWD', security_type: 'stock' },
		quote: { status: 'available', data: { price: 100, change_percent: 1.0 } },
		price_history_summary: { status: 'available', return_5d_percent: 2.0, return_20d_percent: 4.0 },
		fundamentals: { status: 'available' },
		institutional: { status: 'available' },
		margin: { status: 'available' },
		market_context: { status: 'available', state: 'active', confidence: 'high' },
		industry_context: { status: 'available' },
	});

	interface WorkspaceState {
		selected: string | null;
		coreData: IntelligenceCore | null;
		intelligence: Intelligence | null;
		loading: boolean;
		coreLoading: boolean;
		fullLoading: boolean;
		error: string;
		fullError: string;
	}

	const createSelectRunner = () => {
		const selectRequestID = { current: 0 };
		const state: WorkspaceState = {
			selected: null,
			coreData: null,
			intelligence: null,
			loading: false,
			coreLoading: false,
			fullLoading: false,
			error: '',
			fullError: '',
		};

		const select = async (
			canonical: string,
			fetchCore: () => Promise<IntelligenceCore>,
			fetchFull: () => Promise<Intelligence>,
		) => {
			state.selected = canonical;
			await runScopedRequest(selectRequestID, async () => {
				const requestID = selectRequestID.current;
				let coreSettled = false;
				try {
					const corePayload = await fetchCore();
					if (requestID !== selectRequestID.current) return;
					state.coreData = corePayload;
					state.coreLoading = false;
					coreSettled = true;
				} catch {
					if (requestID !== selectRequestID.current) return;
					state.coreLoading = false;
				}

				try {
					const fullPayload = await fetchFull();
					if (requestID !== selectRequestID.current) return;
					state.intelligence = fullPayload;
					state.fullError = '';
				} catch (reason) {
					if (requestID !== selectRequestID.current) return;
					if (coreSettled) {
						state.fullError = '暫時無法取得完整分析資料（市場與籌碼面）';
					} else {
						state.intelligence = null;
						state.error = '台灣個股分析資料載入失敗';
					}
				}
			}, {
				onStart: () => {
					state.intelligence = null;
					state.coreData = null;
					state.loading = true;
					state.coreLoading = true;
					state.fullLoading = true;
					state.error = '';
					state.fullError = '';
				},
				onSuccess: () => {},
				onSettle: () => {
					state.loading = false;
					state.coreLoading = false;
					state.fullLoading = false;
				},
			});
		};

		return {
			state,
			select,
			selectRequestID,
		};
	};

	it('Behavior 1: Stock A Core late resolve does not overwrite Stock B data', async () => {
		const runner = createSelectRunner();
		const coreA = deferred<IntelligenceCore>();
		const fullA = deferred<Intelligence>();
		const coreB = deferred<IntelligenceCore>();
		const fullB = deferred<Intelligence>();

		const runA = runner.select('2330.TWSE', () => coreA.promise, () => fullA.promise);
		expect(runner.state.loading).toBe(true);
		expect(runner.state.selected).toBe('2330.TWSE');

		// Switch rapidly to Stock B
		const runB = runner.select('2454.TWSE', () => coreB.promise, () => fullB.promise);
		expect(runner.state.selected).toBe('2454.TWSE');

		// B's Core resolves first
		coreB.resolve(createSampleCore('2454.TWSE', '2454', '聯發科'));
		await Promise.resolve();
		expect(runner.state.coreData?.identity.code).toBe('2454');

		// A's Core resolves late
		coreA.resolve(createSampleCore('2330.TWSE', '2330', '台積電'));
		await Promise.resolve();
		// Must remain 2454, not overwritten by 2330
		expect(runner.state.coreData?.identity.code).toBe('2454');

		// Finish B's Full
		fullB.resolve(createSampleFull('2454.TWSE', '2454', '聯發科'));
		await runB;
		expect(runner.state.intelligence?.identity.code).toBe('2454');

		// Finish A's Full late
		fullA.resolve(createSampleFull('2330.TWSE', '2330', '台積電'));
		await runA;
		expect(runner.state.intelligence?.identity.code).toBe('2454');
	});

	it('Behavior 2: Stock A Full late resolve does not overwrite Stock B data', async () => {
		const runner = createSelectRunner();
		const coreA = deferred<IntelligenceCore>();
		const fullA = deferred<Intelligence>();
		const coreB = deferred<IntelligenceCore>();
		const fullB = deferred<Intelligence>();

		const runA = runner.select('2330.TWSE', () => coreA.promise, () => fullA.promise);
		coreA.resolve(createSampleCore('2330.TWSE', '2330', '台積電'));
		await Promise.resolve();

		// Now A has Core, but Full is still pending. Switch to B.
		const runB = runner.select('2454.TWSE', () => coreB.promise, () => fullB.promise);
		coreB.resolve(createSampleCore('2454.TWSE', '2454', '聯發科'));
		await Promise.resolve();

		// A's Full late resolves
		fullA.resolve(createSampleFull('2330.TWSE', '2330', '台積電'));
		await Promise.resolve();
		// B's display must not be polluted by A's full intelligence
		expect(runner.state.intelligence).toBeNull();
		expect(runner.state.coreData?.identity.code).toBe('2454');

		fullB.resolve(createSampleFull('2454.TWSE', '2454', '聯發科'));
		await runB;
		await runA;
		expect(runner.state.intelligence?.identity.code).toBe('2454');
	});

	it('Behavior 3A: Stale request Core rejection does not contaminate current UI with errors', async () => {
		const runner = createSelectRunner();
		const coreA = deferred<IntelligenceCore>();
		const fullA = deferred<Intelligence>();
		const coreB = deferred<IntelligenceCore>();
		const fullB = deferred<Intelligence>();

		const runA = runner.select('2330.TWSE', () => coreA.promise, () => fullA.promise);
		const runB = runner.select('2454.TWSE', () => coreB.promise, () => fullB.promise);

		coreB.resolve(createSampleCore('2454.TWSE', '2454', '聯發科'));
		fullB.resolve(createSampleFull('2454.TWSE', '2454', '聯發科'));
		await runB;

		// A throws late rejection on core
		coreA.reject(new Error('2330 core timeout'));
		await runA;

		// Must have no error on B
		expect(runner.state.error).toBe('');
		expect(runner.state.fullError).toBe('');
		expect(runner.state.intelligence?.identity.code).toBe('2454');
	});

	it('Behavior 3B: Stale request Full rejection does not contaminate current UI with errors', async () => {
		const runner = createSelectRunner();
		const coreA = deferred<IntelligenceCore>();
		const fullA = deferred<Intelligence>();
		const coreB = deferred<IntelligenceCore>();
		const fullB = deferred<Intelligence>();

		const runA = runner.select('2330.TWSE', () => coreA.promise, () => fullA.promise);
		// A's core succeeds before switch
		coreA.resolve(createSampleCore('2330.TWSE', '2330', '台積電'));
		await Promise.resolve();

		const runB = runner.select('2454.TWSE', () => coreB.promise, () => fullB.promise);
		coreB.resolve(createSampleCore('2454.TWSE', '2454', '聯發科'));
		fullB.resolve(createSampleFull('2454.TWSE', '2454', '聯發科'));
		await runB;

		// A's full throws late rejection
		fullA.reject(new Error('2330 full fetch aborted'));
		await runA;

		// Must have no error on B
		expect(runner.state.error).toBe('');
		expect(runner.state.fullError).toBe('');
		expect(runner.state.intelligence?.identity.code).toBe('2454');
	});

	it('Behavior 4: Stage 1 Core First Paint while Stage 2 Full is pending', async () => {
		const runner = createSelectRunner();
		const core = deferred<IntelligenceCore>();
		const full = deferred<Intelligence>();

		const run = runner.select('2330.TWSE', () => core.promise, () => full.promise);
		expect(runner.state.coreLoading).toBe(true);
		expect(runner.state.fullLoading).toBe(true);
		expect(runner.state.coreData).toBeNull();

		// Core resolves
		core.resolve(createSampleCore('2330.TWSE', '2330', '台積電'));
		await Promise.resolve();

		// Core is ready (First Paint), while Full is still loading in background
		expect(runner.state.coreLoading).toBe(false);
		expect(runner.state.coreData?.identity.code).toBe('2330');
		expect(runner.state.fullLoading).toBe(true);
		expect(runner.state.intelligence).toBeNull();

		// Full settles
		full.resolve(createSampleFull('2330.TWSE', '2330', '台積電'));
		await run;
		expect(runner.state.fullLoading).toBe(false);
		expect(runner.state.intelligence?.identity.code).toBe('2330');
	});

	it('Behavior 5: Core success + Full fail retains Core data and isolates fullError', async () => {
		const runner = createSelectRunner();
		const core = deferred<IntelligenceCore>();
		const full = deferred<Intelligence>();

		const run = runner.select('2330.TWSE', () => core.promise, () => full.promise);
		core.resolve(createSampleCore('2330.TWSE', '2330', '台積電'));
		await Promise.resolve();

		full.reject(new Error('Full intelligence provider unavailable'));
		await run;

		// Core data is preserved for First Paint displayData
		expect(runner.state.coreData?.identity.code).toBe('2330');
		expect(runner.state.intelligence).toBeNull();
		// Isolated fullError shown, main error NOT triggered
		expect(runner.state.fullError).toBe('暫時無法取得完整分析資料（市場與籌碼面）');
		expect(runner.state.error).toBe('');
		expect(runner.state.loading).toBe(false);
	});

	it('Behavior 6: Core fail + Full success gracefully recovers via full intelligence', async () => {
		const runner = createSelectRunner();
		const core = deferred<IntelligenceCore>();
		const full = deferred<Intelligence>();

		const run = runner.select('2330.TWSE', () => core.promise, () => full.promise);
		// Core fails
		core.reject(new Error('Core quote unavailable'));
		await Promise.resolve();
		expect(runner.state.coreLoading).toBe(false);
		expect(runner.state.coreData).toBeNull();

		// Full succeeds
		full.resolve(createSampleFull('2330.TWSE', '2330', '台積電'));
		await run;

		// intelligence is available, displayData = intelligence ?? coreData resolves to intelligence
		expect(runner.state.intelligence?.identity.code).toBe('2330');
		expect(runner.state.error).toBe('');
		expect(runner.state.fullError).toBe('');
		expect(runner.state.loading).toBe(false);
	});

	it('Behavior 7: Both Core and Full fail triggers main error', async () => {
		const runner = createSelectRunner();
		const core = deferred<IntelligenceCore>();
		const full = deferred<Intelligence>();

		const run = runner.select('2330.TWSE', () => core.promise, () => full.promise);
		core.reject(new Error('Core timeout'));
		await Promise.resolve();

		full.reject(new Error('Full timeout'));
		await run;

		expect(runner.state.coreData).toBeNull();
		expect(runner.state.intelligence).toBeNull();
		expect(runner.state.error).toBe('台灣個股分析資料載入失敗');
		expect(runner.state.loading).toBe(false);
	});
});
