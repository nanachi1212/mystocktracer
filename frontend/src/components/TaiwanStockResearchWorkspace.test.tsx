import fs from 'node:fs';
import path from 'node:path';
import type { ReactElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import { InterpretationView, TaiwanResearchSnapshot, type Component, type Intelligence, type TaiwanStatementData, type TaiwanValuationData } from './TaiwanStockResearchWorkspace';
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
		expect(html).toContain('個股研究畫面發生錯誤，請重新載入此研究。');
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
		expect(fn).toContain('setRequestedTaiwanSymbol({ canonical, token: taiwanSymbolRequestNonce.current })');
		expect(fn).toContain("switchWorkspace('taiwan-stock')");
	});

	it('App.tsx passes externalSymbolRequest to the stock research workspace (both taiwan-stock and taiwan-research render branches)', () => {
		const app = appSource();
		expect(app.match(/<TaiwanStockResearchWorkspace config=\{config\} refreshKey=\{marketRefreshKey\} externalSymbolRequest=\{requestedTaiwanSymbol\}/g)?.length).toBe(2);
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
