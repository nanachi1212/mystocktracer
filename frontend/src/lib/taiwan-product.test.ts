import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import {
	buildTaiwanScreenerContext, buildTaiwanScreenerMatchReason, formatTaiwanBookValuePerShare, formatTaiwanCashFlowTWD, formatTaiwanPercent, formatTaiwanPlainNumber, formatTaiwanRatio, formatTaiwanRevenueTWD, formatTaiwanTWD, resolveTaiwanWorkspace, runScopedRequest,
	taiwanComponentList, taiwanDefaultWorkspace, taiwanFinancialsStatusLabel, taiwanIntelligencePath, taiwanMarketPath, taiwanPrimaryNavigation, taiwanMarketDetailNavigation, taiwanResearchPath,
	taiwanScreenerDefaultFilters, taiwanScreenerHasBalanceCriteria, taiwanScreenerHasCashflowCriteria, taiwanScreenerHasDividendCriteria, taiwanScreenerHasFinancialsCriteria, taiwanScreenerHasRevenueCriteria, taiwanScreenerHasValuationCriteria,
	taiwanScreenerMatchReasonSuffix, taiwanScreenerPath, taiwanScreenerSortOptions, taiwanStatusLabel, validateTaiwanScreenerFilters, type TaiwanScreenerFilters, type TaiwanScreenerSecurity,
} from './taiwan-product';

// M8F -- minimal TaiwanScreenerSecurity fixture (only the fields a given test actually needs are
// overridden; every other numeric field defaults to null so a test never accidentally exercises an
// unrelated field).
function screenerSecurityFixture(overrides: Partial<TaiwanScreenerSecurity> = {}): TaiwanScreenerSecurity {
	return {
		canonical: '2330.TWSE', code: '2330', name: '台積電', exchange: 'TWSE', security_type: 'stock', trade_date: '2026-09-04',
		price: null, change: null, change_percent: null, volume: null, amount: null,
		foreign_net: null, trust_net: null, dealer_net: null, institutional_net: null,
		margin_balance: null, margin_change: null, short_balance: null, short_change: null, short_margin_ratio: null,
		monthly_revenue: null, revenue_yoy: null, revenue_mom: null, cumulative_revenue_yoy: null, pe: null, pb: null, dividend_yield: null,
		cash_dividend: null, stock_dividend: null, total_dividend: null,
		financial_period: null, cumulative_eps: null, gross_margin: null, operating_margin: null,
		net_margin: null, book_value_per_share: null,
		debt_ratio: null, debt_to_equity: null, current_ratio: null, balance_period: null,
		operating_cash_flow: null, cash_flow_to_net_income: null, cashflow_period: null,
		...overrides,
	};
}

/** A promise plus its resolve/reject, so a test can control settlement order explicitly. */
function deferred<T>() {
	let resolve!: (value: T) => void;
	let reject!: (reason: unknown) => void;
	const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej; });
	return { promise, resolve, reject };
}

describe('runScopedRequest (scope/selection race safety)', () => {
	it('clears stale state synchronously via onStart, before the request settles', async () => {
		const ref = { current: 0 };
		const events: string[] = [];
		const first = deferred<string>();
		const run = runScopedRequest(ref, () => first.promise, {
			onStart: () => events.push('start'),
			onSuccess: () => events.push('success'),
			onError: () => events.push('error'),
		});
		// onStart must already have run before the task settles.
		expect(events).toEqual(['start']);
		first.resolve('A');
		await run;
		expect(events).toEqual(['start', 'success']);
	});

	it('drops a stale response when a newer request has since started (scope A resolves after B)', async () => {
		const ref = { current: 0 };
		const applied: string[] = [];
		const a = deferred<string>();
		const runA = runScopedRequest(ref, () => a.promise, { onSuccess: (v) => applied.push(v) });
		const b = deferred<string>();
		const runB = runScopedRequest(ref, () => b.promise, { onSuccess: (v) => applied.push(v) });
		// B starts after A but resolves first; A resolves last and must be ignored.
		b.resolve('B');
		await runB;
		a.resolve('A');
		await runA;
		expect(applied).toEqual(['B']);
	});

	it('a stale rejection is also dropped once a newer request has started', async () => {
		const ref = { current: 0 };
		const applied: string[] = [];
		const errors: unknown[] = [];
		const a = deferred<string>();
		const runA = runScopedRequest(ref, () => a.promise, { onSuccess: (v) => applied.push(v), onError: (e) => errors.push(e) });
		const b = deferred<string>();
		const runB = runScopedRequest(ref, () => b.promise, { onSuccess: (v) => applied.push(v), onError: (e) => errors.push(e) });
		b.resolve('B');
		await runB;
		a.reject(new Error('stale failure'));
		await runA;
		expect(applied).toEqual(['B']);
		expect(errors).toEqual([]);
	});

	it('onSettle only fires for the request that is still current', async () => {
		const ref = { current: 0 };
		const settled: string[] = [];
		const a = deferred<string>();
		const runA = runScopedRequest(ref, () => a.promise, { onSuccess: () => {}, onSettle: () => settled.push('A') });
		const b = deferred<string>();
		const runB = runScopedRequest(ref, () => b.promise, { onSuccess: () => {}, onSettle: () => settled.push('B') });
		b.resolve('B');
		await runB;
		a.resolve('A');
		await runA;
		expect(settled).toEqual(['B']);
	});
});

const root = path.resolve(__dirname, '../../..');

describe('taiwanComponentList', () => {
	it('normalizes the Go nil-slice-as-null quirk to an empty list', () => {
		expect(taiwanComponentList(null)).toEqual([]);
		expect(taiwanComponentList(undefined)).toEqual([]);
	});

	it('passes through a populated list unchanged', () => {
		expect(taiwanComponentList(['industry', 'fundamentals'])).toEqual(['industry', 'fundamentals']);
	});
});

describe('Taiwan-first product shell', () => {
	it('uses zh-TW metadata without an A-share identity', () => {
		const html = fs.readFileSync(path.join(root, 'frontend/index.html'), 'utf8');
		expect(html).toContain('<html lang="zh-TW">');
		expect(html).not.toContain('A股');
		expect(html).toContain('台灣股票分析工作台');
		expect(html).toContain('<title>mystocktracer · 台股分析工作台 · 僅限個人非商業使用</title>');
	});

	it('uses the Taiwan overview for empty and unknown hashes', () => {
		expect(resolveTaiwanWorkspace('')).toBe(taiwanDefaultWorkspace);
		expect(resolveTaiwanWorkspace('#unknown')).toBe(taiwanDefaultWorkspace);
		expect(resolveTaiwanWorkspace('#taiwan-emotion')).toBe('taiwan-emotion');
	});

	it('keeps old research bookmarks on stock analysis and preserves all market-detail routes', () => {
		expect(resolveTaiwanWorkspace('#taiwan-research')).toBe('taiwan-stock');
		expect(taiwanMarketDetailNavigation.map(([, label]) => label)).toEqual(['市場廣度', '市場情緒', '產業雷達']);
		for (const [id] of [...taiwanPrimaryNavigation, ...taiwanMarketDetailNavigation]) {
			expect(resolveTaiwanWorkspace(`#${id}`)).toBe(id);
		}
	});

	it('keeps only Taiwan-safe primary navigation labels', () => {
		expect(taiwanPrimaryNavigation.map((item) => item[1])).toEqual(['台股總覽', '台股選股器', '自選股', '個股分析']);
		expect(taiwanPrimaryNavigation.join(' ')).not.toMatch(/遊資|連板|龍虎榜|打板|首板|炸板/);
		const app = fs.readFileSync(path.join(root, 'frontend/src/App.tsx'), 'utf8');
		const primaryNav = app.slice(app.indexOf('<aside className="app-sidebar"'), app.indexOf('<div className="sidebar-guidance">'));
		expect(primaryNav).not.toMatch(/大V|个股分析|持仓|短线|趋势题材|游资|龙虎榜|連板|遊資/);
	});

	it('mounts no legacy market workspace on the Taiwan default path', () => {
		const app = fs.readFileSync(path.join(root, 'frontend/src/App.tsx'), 'utf8');
		expect(app).toContain("return 'taiwan-overview'");
		expect(app).toContain("workspaceMode === 'taiwan-overview' ? <TaiwanMarketWorkspace");
		const taiwanMarket = fs.readFileSync(path.join(root, 'frontend/src/components/TaiwanMarketWorkspace.tsx'), 'utf8');
		expect(taiwanMarket).not.toMatch(/\/api\/v1\/themes|source=cls|\/api\/v1\/market\/|billboard/);
	});

	it('keeps Taiwan AI explicit and separate from legacy AI', () => {
		const source = fs.readFileSync(path.join(root, 'frontend/src/components/TaiwanStockResearchWorkspace.tsx'), 'utf8');
		expect(source).toContain('產生 AI 研究摘要');
		expect(source).toContain('onClick={() => void generateResearch()}');
		expect(source).not.toContain('/api/v1/stocks/ai-analysis');
		expect(source).not.toContain('/api/v1/ai/ws');
	});

	it('does not claim Taiwan provider data is connected when only the backend endpoint resolved', () => {
		const app = fs.readFileSync(path.join(root, 'frontend/src/App.tsx'), 'utf8');
		// `config` only means the backend HTTP endpoint was resolved — it says nothing about
		// whether TWSE/TPEx data actually loaded, and (P1F) resolving `config` never proves the
		// backend process is actually reachable either. The topbar wording must not overclaim
		// a live connection; per-scope status (available/partial/stale/unavailable) is shown by
		// each Taiwan view itself, sourced from backend status/freshness fields.
		expect(app).not.toContain('台股官方資料服務已連線');
		expect(app).not.toContain('後端服務已連線');
		expect(app).toContain('後端服務已設定');
	});

	it('clears the previous scope/selection before a new Taiwan request settles, with a race guard', () => {
		const market = fs.readFileSync(path.join(root, 'frontend/src/components/TaiwanMarketWorkspace.tsx'), 'utf8');
		expect(market).toContain('runScopedRequest(scopeRequestID');
		expect(market).toContain('setData(null); setLoading(true)');
		const stockResearch = fs.readFileSync(path.join(root, 'frontend/src/components/TaiwanStockResearchWorkspace.tsx'), 'utf8');
		expect(stockResearch).toContain('runScopedRequest(selectRequestID');
		const overview = fs.readFileSync(path.join(root, 'frontend/src/components/market/TaiwanMarketView.tsx'), 'utf8');
		expect(overview).toContain('runScopedRequest(selectRequestID');
	});

	it('keeps data_date and latest_completed_trading_day as distinct, backend-sourced labels', () => {
		const stockResearch = fs.readFileSync(path.join(root, 'frontend/src/components/TaiwanStockResearchWorkspace.tsx'), 'utf8');
		expect(stockResearch).toMatch(/資料日期[\s\S]*最新完成交易日/);
		const market = fs.readFileSync(path.join(root, 'frontend/src/components/TaiwanMarketWorkspace.tsx'), 'utf8');
		expect(market).toMatch(/資料日期[\s\S]*最新完成交易日/);
		// Neither computes a trading day from the client clock — only backend response fields.
		expect(stockResearch).not.toContain('new Date()');
		expect(market).not.toContain('new Date()');
	});

	it('builds only Taiwan M2-M5 contracts', () => {
		expect(taiwanMarketPath('market-breadth', 'combined')).toBe('/api/v1/tw/market-breadth?scope=combined');
		expect(taiwanMarketPath('market-emotion', 'twse')).toBe('/api/v1/tw/market-emotion?scope=twse');
		expect(taiwanMarketPath('industry-radar', 'tpex')).toBe('/api/v1/tw/industry-radar?scope=tpex');
		expect(taiwanIntelligencePath('2330.TWSE')).toBe('/api/v1/tw/stocks/2330.TWSE/intelligence');
		expect(taiwanResearchPath('6488.TPEX')).toBe('/api/v1/tw/stocks/6488.TPEX/research');
		const contracts = [taiwanIntelligencePath('2330.TWSE'), taiwanResearchPath('2330.TWSE')].join(' ');
		expect(contracts).not.toContain('/api/v1/stocks/ai-analysis');
		expect(contracts).not.toContain('/api/v1/ai/ws');
	});

	it('uses Taiwan locale, units and visible data-quality labels', () => {
		expect(formatTaiwanRatio(0.5123)).toBe('51.23%');
		expect(formatTaiwanTWD(123_000_000)).toBe('1.23 億元');
		expect(taiwanStatusLabel('partial')).toBe('部分資料');
		expect(taiwanStatusLabel('stale')).toBe('資料較舊');
		expect(taiwanStatusLabel('unavailable')).toBe('無法取得');
	});

	it('preserves Taiwan red-rise and green-fall colors', () => {
		const css = fs.readFileSync(path.join(root, 'frontend/src/styles.css'), 'utf8');
		expect(css).toMatch(/--up:\s*#e33b46/);
		expect(css).toMatch(/--down:\s*#14865f/);
	});

	it('keeps visible desktop metadata free of A-share identity', () => {
		const main = fs.readFileSync(path.join(root, 'desktop/main.cjs'), 'utf8');
		const manifest = fs.readFileSync(path.join(root, 'desktop/package.json'), 'utf8');
		const windows = fs.readFileSync(path.join(root, 'desktop/scripts/package-windows.mjs'), 'utf8');
		expect(main.match(/title:\s*'([^']+)'/)?.[1]).not.toMatch(/A股|A-share/i);
		expect(JSON.parse(manifest).description).not.toMatch(/A股|A-share/i);
		expect(windows.match(/FileDescription:\s*'([^']+)'/)?.[1]).not.toMatch(/A股|A-share/i);
	});
});

describe('P1E refresh behavior — retries the current selection instead of a fixed default', () => {
	const overviewSource = () => fs.readFileSync(path.join(root, 'frontend/src/components/market/TaiwanMarketView.tsx'), 'utf8');
	const researchSource = () => fs.readFileSync(path.join(root, 'frontend/src/components/TaiwanStockResearchWorkspace.tsx'), 'utf8');

	it('TaiwanMarketView: a selected security is retried on refresh via a ref, not by adding `selected` to the effect dependency list (no refresh-induced request loop)', () => {
		const source = overviewSource();
		expect(source).toContain('const selectedRef = useRef<SecurityIdentity | null>(null);');
		expect(source).toContain('useEffect(() => { selectedRef.current = selected; }, [selected]);');
		expect(source).toContain('if (selectedRef.current) void select(selectedRef.current);');
		expect(source).not.toMatch(/\[config, refreshKey, selected\]/);
	});

	it('TaiwanStockResearchWorkspace: refresh retries the current selection and no longer unconditionally resets to search(\'2330\')', () => {
		const source = researchSource();
		expect(source).toContain('const selectedRef = useRef<SecurityIdentity | null>(null);');
		expect(source).toContain('useEffect(() => { selectedRef.current = selected; }, [selected]);');
		// M6C merged this into one effect (also reacting to externalSymbolRequest — see the M6C
		// describe block in TaiwanStockResearchWorkspace.test.tsx), but the P1E guarantee itself —
		// refresh retries selectedRef.current instead of unconditionally resetting to 2330 — holds.
		expect(source).toContain('if (selectedRef.current) { void select(selectedRef.current); return; }');
		expect(source).toContain("if (!externalSymbolRequest) void search('2330');");
		expect(source).not.toMatch(/\[config, refreshKey, selected\]/);
	});

	it('TaiwanStockResearchWorkspace: the initial default (2330) is preserved for a fresh mount with nothing selected yet (and no external Watchlist request)', () => {
		const source = researchSource();
		expect(source).toMatch(/if \(selectedRef\.current\) \{ void select\(selectedRef\.current\); return; \}\s*if \(!externalSymbolRequest\) void search\('2330'\);/);
	});
});

describe('P1F search zero-result feedback — TaiwanStockResearchWorkspace', () => {
	const researchSource = () => fs.readFileSync(path.join(root, 'frontend/src/components/TaiwanStockResearchWorkspace.tsx'), 'utf8');
	const searchBody = () => {
		const source = researchSource();
		return source.slice(source.indexOf('const search = async'), source.indexOf('const select = async'));
	};

	it('shows an explicit Traditional Chinese not-found message (naming the query) when securities search returns zero results', () => {
		expect(searchBody()).toContain('else if (payload.data.securities.length === 0) setError(`找不到符合「${value.trim()}」的台灣證券`);');
	});

	it('zero-result feedback is set directly, not routed through taiwanErrorMessage (stays distinct from request-failure wording)', () => {
		const body = searchBody();
		expect(body).toContain('找不到符合');
		expect(body).not.toMatch(/length === 0\) setError\(taiwanErrorMessage/);
	});

	it('search() clears any previous error/not-found message at the start of a new search, so a later 1-result or multi-result search clears a stale not-found message', () => {
		expect(searchBody()).toContain("setLoading(true); setError('');");
	});

	it('a zero-result search does not clear the previously selected security or its already-loaded data', () => {
		const body = searchBody();
		expect(body).not.toMatch(/length === 0\)[^;]*setSelected/);
		expect(body).not.toMatch(/length === 0\)[^;]*setIntelligence/);
	});
});

describe('P1C Taiwan product localization (settings drawer)', () => {
	it('hides China-market-only review automation and data-provider credentials, with no Taiwan equivalent', () => {
		const source = fs.readFileSync(path.join(root, 'frontend/src/components/SettingsDrawer.tsx'), 'utf8');
		// The rendered <form> — from the security note through the footer — must not surface any
		// of the confirmed China-market-only settings. WechatServiceCard/ReviewProfileCard and their
		// state are intentionally left defined-but-unrendered (not deleted), so this checks what is
		// actually reachable from the JSX return, not the whole file.
		const renderedForm = source.slice(source.indexOf('<form className="settings-form"'), source.indexOf('</form>'));
		expect(renderedForm).not.toMatch(/大V复盘自动化|同花顺|雪球|淘股吧|微信公众号|Tushare Pro Token|行情与内容数据源|涨停/);
	});

	it('keeps the SecretKey credential contract intact even though some fields are hidden from the UI', () => {
		const source = fs.readFileSync(path.join(root, 'frontend/src/components/SettingsDrawer.tsx'), 'utf8');
		// Hiding is a UI-only change — the backend credential contract (SecretKey union, emptySecrets)
		// must not be rewritten or truncated as a side effect.
		expect(source).toContain("type SecretKey = 'llm_api_key' | 'tushare_token' | 'ths_cookie' | 'xueqiu_cookie' | 'eastmoney_cookie' | 'wechat_api_token';");
	});

	it('uses Traditional Chinese for the settings drawer chrome that remains active', () => {
		const source = fs.readFileSync(path.join(root, 'frontend/src/components/SettingsDrawer.tsx'), 'utf8');
		expect(source).toContain('系統設定');
		expect(source).not.toContain('系统设置');
		const renderedForm = source.slice(source.indexOf('<form className="settings-form"'), source.indexOf('</form>'));
		// 保存 is valid in both scripts (保 and 存 don't differ), so it isn't in this list.
		expect(renderedForm).not.toMatch(/设置|连接|读取|获取/);
	});
});

describe('P1C.1 Taiwan-first localization completion', () => {
	it('uses Traditional Chinese for the always-visible Hermes Skill/MCP panel', () => {
		const source = fs.readFileSync(path.join(root, 'frontend/src/components/HermesAgentSettingsPanel.tsx'), 'utf8');
		expect(source).toContain('Skill 與 MCP');
		expect(source).not.toMatch(/设置|连接|读取|获取|删除|启用|添加/);
	});

	it('uses Traditional Chinese for the always-visible App update panel', () => {
		const source = fs.readFileSync(path.join(root, 'frontend/src/components/AppUpdatePanel.tsx'), 'utf8');
		expect(source).toContain('版本與自動更新');
		expect(source).not.toMatch(/设置|连接|读取|获取|应用|检查|备份/);
	});
});

describe('P1D Taiwan overview startup fan-out', () => {
	const overviewSource = () => fs.readFileSync(path.join(root, 'frontend/src/components/market/TaiwanMarketView.tsx'), 'utf8');

	it('does not auto-search 2330 (or any symbol) on mount — only indexes load on startup', () => {
		const source = overviewSource();
		const mountEffect = source.slice(source.indexOf('useEffect(() => {'), source.indexOf('[config, refreshKey]'));
		expect(mountEffect).toContain('/api/v1/tw/indexes');
		expect(mountEffect).not.toMatch(/search\(/);
		// The query field starts empty; a hardcoded default symbol would silently reintroduce
		// the fan-out this fixes, so guard the exact initial state too.
		expect(source).toContain("useState('')");
		expect(source).not.toContain("useState('2330')");
	});

	it('shows an empty state before any security is selected, without hiding indexes or the search box', () => {
		const source = overviewSource();
		expect(source).toContain('taiwan-empty-state');
		expect(source).toContain('請選擇一檔證券查看台股資料');
		// Empty state, search form, and CoreIndexView must all be reachable from the same render —
		// none of them gated behind `selected` in a way that would hide the others.
		expect(source).toContain('{!selected && !loading &&');
		expect(source.indexOf('<form className="market-filter"')).toBeLessThan(source.indexOf('taiwan-empty-state'));
		expect(source).toContain('<CoreIndexView');
	});

	it('keeps manual search and selection wired to the existing securities resolver and market datasets', () => {
		const source = overviewSource();
		expect(source).toContain('/api/v1/tw/securities?query=');
		expect(source).toContain('/api/v1/tw/quotes?symbols=');
		expect(source).toContain('/api/v1/tw/kline?symbol=');
		expect(source).toContain('/api/v1/tw/institutional?symbol=');
		expect(source).toContain('/api/v1/tw/margin?symbol=');
		expect(source).toContain('/api/v1/tw/fundamentals?symbol=');
	});

	it('preserves the P1B race guard on the selection path unchanged', () => {
		const source = overviewSource();
		expect(source).toContain('runScopedRequest(selectRequestID');
		expect(source).toContain('setQuote(null); setLines([]); setInstitutional(null); setMargin(null); setFundamentals(null);');
	});
});

describe('M7B — Taiwan Screener navigation', () => {
	it('taiwan-screener appears in Taiwan primary navigation with the correct label', () => {
		expect(taiwanPrimaryNavigation.some(([id, label]) => id === 'taiwan-screener' && label === '台股選股器')).toBe(true);
	});

	it('#taiwan-screener resolves to the taiwan-screener workspace', () => {
		expect(resolveTaiwanWorkspace('#taiwan-screener')).toBe('taiwan-screener');
	});
});

describe('M7B — Taiwan Screener query builder', () => {
	it('the default filter set uses combined scope, amount sort, desc order, limit 50, offset 0', () => {
		const defaults = taiwanScreenerDefaultFilters();
		expect(defaults.scope).toBe('combined');
		expect(defaults.sort).toBe('amount');
		expect(defaults.order).toBe('desc');
		expect(defaults.limit).toBe(50);
		expect(defaults.offset).toBe(0);
	});

	it('builds the default query with scope/sort/order/limit/offset but no blank range filters', () => {
		const path = taiwanScreenerPath(taiwanScreenerDefaultFilters());
		expect(path).toBe('/api/v1/tw/screener?scope=combined&sort=amount&order=desc&limit=50&offset=0');
	});

	it('omits blank optional range filters', () => {
		const filters = { ...taiwanScreenerDefaultFilters(), minPrice: '', maxPrice: '  ' };
		const path = taiwanScreenerPath(filters);
		expect(path).not.toContain('min_price');
		expect(path).not.toContain('max_price');
	});

	it('preserves an explicit zero filter value', () => {
		const filters = { ...taiwanScreenerDefaultFilters(), minChangePercent: '0' };
		const path = taiwanScreenerPath(filters);
		expect(path).toContain('min_change_percent=0');
	});

	it('never sends a NaN or Infinity value from invalid input', () => {
		const filters = { ...taiwanScreenerDefaultFilters(), minVolume: 'abc', maxVolume: 'Infinity' };
		const path = taiwanScreenerPath(filters);
		expect(path).not.toContain('min_volume');
		expect(path).not.toContain('max_volume');
		expect(path).not.toMatch(/NaN|Infinity/);
	});

	it('includes every populated range filter using the exact M7A parameter names', () => {
		const filters: TaiwanScreenerFilters = {
			...taiwanScreenerDefaultFilters(),
			scope: 'twse', minPrice: '10', maxPrice: '20', minChangePercent: '-5', maxChangePercent: '5',
			minVolume: '1000', maxVolume: '2000', minAmount: '100000', maxAmount: '200000',
			sort: 'volume', order: 'asc', limit: 50, offset: 100,
		};
		const path = taiwanScreenerPath(filters);
		expect(path).toContain('scope=twse');
		expect(path).toContain('min_price=10');
		expect(path).toContain('max_price=20');
		expect(path).toContain('min_change_percent=-5');
		expect(path).toContain('max_change_percent=5');
		expect(path).toContain('min_volume=1000');
		expect(path).toContain('max_volume=2000');
		expect(path).toContain('min_amount=100000');
		expect(path).toContain('max_amount=200000');
		expect(path).toContain('sort=volume');
		expect(path).toContain('order=asc');
		expect(path).toContain('offset=100');
	});

	it('never clamps offset below zero silently swaps to zero instead of a negative value', () => {
		const filters = { ...taiwanScreenerDefaultFilters(), offset: -5 };
		expect(taiwanScreenerPath(filters)).toContain('offset=0');
	});
});

describe('M7B — Taiwan Screener local range validation', () => {
	it('rejects an inverted price range', () => {
		const filters = { ...taiwanScreenerDefaultFilters(), minPrice: '100', maxPrice: '50' };
		expect(validateTaiwanScreenerFilters(filters)).toBe('最低價格不可高於最高價格');
	});

	it('rejects an inverted change-percent range', () => {
		const filters = { ...taiwanScreenerDefaultFilters(), minChangePercent: '10', maxChangePercent: '5' };
		expect(validateTaiwanScreenerFilters(filters)).toBe('最低漲跌幅不可高於最高漲跌幅');
	});

	it('accepts an equal min/max value (boundary, not an inversion)', () => {
		const filters = { ...taiwanScreenerDefaultFilters(), minVolume: '100', maxVolume: '100' };
		expect(validateTaiwanScreenerFilters(filters)).toBe('');
	});

	it('accepts a blank pair without complaint', () => {
		expect(validateTaiwanScreenerFilters(taiwanScreenerDefaultFilters())).toBe('');
	});
});

describe('M7E-A — Taiwan Screener fundamentals query builder', () => {
	it('16. sort dropdown offers exactly 33 options (M7A 4 + M7D 9 + M7E-A 8 + M7E-B 3 + M7E-C 2 + M7F 2 + M7G 3 + M7H 2)', () => {
		expect(taiwanScreenerSortOptions.length).toBe(33);
	});

	it('preserves every M7A/M7D sort id and adds the 8 new M7E-A sort ids', () => {
		const ids = taiwanScreenerSortOptions.map((item) => item.id);
		for (const id of ['price', 'change_percent', 'volume', 'amount', 'foreign_net', 'trust_net', 'dealer_net', 'institutional_net', 'margin_balance', 'margin_change', 'short_balance', 'short_change', 'short_margin_ratio']) {
			expect(ids).toContain(id);
		}
		for (const id of ['monthly_revenue', 'revenue_yoy', 'pe', 'pb', 'dividend_yield', 'cash_dividend', 'stock_dividend', 'total_dividend']) {
			expect(ids).toContain(id);
		}
	});

	it('12. default filters include the 16 new fundamentals fields, all blank', () => {
		const defaults = taiwanScreenerDefaultFilters();
		for (const key of ['minMonthlyRevenue', 'maxMonthlyRevenue', 'minRevenueYoY', 'maxRevenueYoY', 'minPE', 'maxPE', 'minPB', 'maxPB', 'minDividendYield', 'maxDividendYield', 'minCashDividend', 'maxCashDividend', 'minStockDividend', 'maxStockDividend', 'minTotalDividend', 'maxTotalDividend'] as const) {
			expect(defaults[key]).toBe('');
		}
		expect(taiwanScreenerPath(defaults)).not.toMatch(/min_monthly_revenue|max_monthly_revenue|min_revenue_yoy|max_revenue_yoy|min_pe|max_pe|min_pb|max_pb|min_dividend_yield|max_dividend_yield|min_cash_dividend|max_cash_dividend|min_stock_dividend|max_stock_dividend|min_total_dividend|max_total_dividend/);
	});

	it('7. Apply serializes monthly_revenue verbatim in TWD, never divided by 1,000', () => {
		const filters = { ...taiwanScreenerDefaultFilters(), minMonthlyRevenue: '1000000000' };
		const path = taiwanScreenerPath(filters);
		expect(path).toContain('min_monthly_revenue=1000000000');
	});

	it('8. Apply serializes revenue_yoy verbatim (never divided or multiplied by 100)', () => {
		const filters = { ...taiwanScreenerDefaultFilters(), minRevenueYoY: '12.16' };
		const path = taiwanScreenerPath(filters);
		expect(path).toContain('min_revenue_yoy=12.16');
	});

	it('9. Apply serializes PE/PB/dividend_yield', () => {
		const filters = { ...taiwanScreenerDefaultFilters(), minPE: '0', minPB: '1.5', minDividendYield: '5.3' };
		const path = taiwanScreenerPath(filters);
		expect(path).toContain('min_pe=0');
		expect(path).toContain('min_pb=1.5');
		expect(path).toContain('min_dividend_yield=5.3');
	});

	it('10. Apply serializes cash/stock/total dividends', () => {
		const filters = { ...taiwanScreenerDefaultFilters(), minCashDividend: '5', minStockDividend: '0', minTotalDividend: '5' };
		const path = taiwanScreenerPath(filters);
		expect(path).toContain('min_cash_dividend=5');
		expect(path).toContain('min_stock_dividend=0');
		expect(path).toContain('min_total_dividend=5');
	});

	it('11. combined fundamentals filters serialize together (revenue + valuation + dividend in one query)', () => {
		const filters = { ...taiwanScreenerDefaultFilters(), minRevenueYoY: '0', minPE: '0', minCashDividend: '0' };
		const path = taiwanScreenerPath(filters);
		expect(path).toContain('min_revenue_yoy=0');
		expect(path).toContain('min_pe=0');
		expect(path).toContain('min_cash_dividend=0');
	});

	it('never sends NaN/Infinity/blank for any new fundamentals field', () => {
		const filters = { ...taiwanScreenerDefaultFilters(), minPE: 'abc', maxPE: 'Infinity', minMonthlyRevenue: '', maxRevenueYoY: '   ' };
		const path = taiwanScreenerPath(filters);
		expect(path).not.toMatch(/min_pe=|max_pe=|min_monthly_revenue=|max_revenue_yoy=/);
		expect(path).not.toMatch(/NaN|Infinity/);
	});

	it('rejects inverted fundamentals ranges', () => {
		expect(validateTaiwanScreenerFilters({ ...taiwanScreenerDefaultFilters(), minMonthlyRevenue: '100', maxMonthlyRevenue: '50' })).toBe('最低月營收不可高於最高月營收');
		expect(validateTaiwanScreenerFilters({ ...taiwanScreenerDefaultFilters(), minRevenueYoY: '10', maxRevenueYoY: '5' })).toBe('最低月營收年增率不可高於最高月營收年增率');
		expect(validateTaiwanScreenerFilters({ ...taiwanScreenerDefaultFilters(), minPE: '10', maxPE: '5' })).toBe('最低本益比不可高於最高本益比');
		expect(validateTaiwanScreenerFilters({ ...taiwanScreenerDefaultFilters(), minPB: '10', maxPB: '5' })).toBe('最低股價淨值比不可高於最高股價淨值比');
		expect(validateTaiwanScreenerFilters({ ...taiwanScreenerDefaultFilters(), minDividendYield: '10', maxDividendYield: '5' })).toBe('最低殖利率不可高於最高殖利率');
		expect(validateTaiwanScreenerFilters({ ...taiwanScreenerDefaultFilters(), minCashDividend: '10', maxCashDividend: '5' })).toBe('最低現金股利不可高於最高現金股利');
		expect(validateTaiwanScreenerFilters({ ...taiwanScreenerDefaultFilters(), minStockDividend: '10', maxStockDividend: '5' })).toBe('最低股票股利不可高於最高股票股利');
		expect(validateTaiwanScreenerFilters({ ...taiwanScreenerDefaultFilters(), minTotalDividend: '10', maxTotalDividend: '5' })).toBe('最低合計股利不可高於最高合計股利');
	});
});

describe('M7E-A — domain criteria helpers (applied-only)', () => {
	it('taiwanScreenerHasRevenueCriteria is true only when a revenue filter or sort is active', () => {
		expect(taiwanScreenerHasRevenueCriteria(taiwanScreenerDefaultFilters())).toBe(false);
		expect(taiwanScreenerHasRevenueCriteria({ ...taiwanScreenerDefaultFilters(), minRevenueYoY: '0' })).toBe(true);
		expect(taiwanScreenerHasRevenueCriteria({ ...taiwanScreenerDefaultFilters(), sort: 'monthly_revenue' })).toBe(true);
		expect(taiwanScreenerHasRevenueCriteria({ ...taiwanScreenerDefaultFilters(), sort: 'revenue_yoy' })).toBe(true);
	});

	it('taiwanScreenerHasValuationCriteria is true only when a valuation filter or sort is active', () => {
		expect(taiwanScreenerHasValuationCriteria(taiwanScreenerDefaultFilters())).toBe(false);
		expect(taiwanScreenerHasValuationCriteria({ ...taiwanScreenerDefaultFilters(), minPE: '0' })).toBe(true);
		expect(taiwanScreenerHasValuationCriteria({ ...taiwanScreenerDefaultFilters(), sort: 'pb' })).toBe(true);
		expect(taiwanScreenerHasValuationCriteria({ ...taiwanScreenerDefaultFilters(), sort: 'dividend_yield' })).toBe(true);
	});

	it('taiwanScreenerHasDividendCriteria is true only when a dividend filter or sort is active', () => {
		expect(taiwanScreenerHasDividendCriteria(taiwanScreenerDefaultFilters())).toBe(false);
		expect(taiwanScreenerHasDividendCriteria({ ...taiwanScreenerDefaultFilters(), minCashDividend: '0' })).toBe(true);
		expect(taiwanScreenerHasDividendCriteria({ ...taiwanScreenerDefaultFilters(), sort: 'total_dividend' })).toBe(true);
	});

	it('17. Clear (taiwanScreenerDefaultFilters()) resets all fundamentals fields and criteria go false', () => {
		const cleared = taiwanScreenerDefaultFilters();
		expect(taiwanScreenerHasRevenueCriteria(cleared)).toBe(false);
		expect(taiwanScreenerHasValuationCriteria(cleared)).toBe(false);
		expect(taiwanScreenerHasDividendCriteria(cleared)).toBe(false);
	});
});

describe('M7E-A — number formatting', () => {
	it('21-23. monthly_revenue: null -> —, 0 -> 0, positive -> locale-grouped integer, never divided by 1,000', () => {
		expect(formatTaiwanRevenueTWD(null)).toBe('—');
		expect(formatTaiwanRevenueTWD(0)).toBe('0');
		expect(formatTaiwanRevenueTWD(52340000000)).toBe((52340000000).toLocaleString('zh-TW'));
		expect(formatTaiwanRevenueTWD(52340000000)).not.toContain('52340000');
	});

	it('24. revenue_yoy: null -> —, 0 -> 0%, positive -> +signed, negative -> signed', () => {
		expect(formatTaiwanPercent(null, true)).toBe('—');
		expect(formatTaiwanPercent(0, true)).toBe('0%');
		expect(formatTaiwanPercent(12.16, true)).toBe('+12.16%');
		expect(formatTaiwanPercent(-8.4, true)).toBe('-8.4%');
	});

	it('25-26. PE/PB: null -> —, real zero remains 0, plain decimal formatting', () => {
		expect(formatTaiwanPlainNumber(null)).toBe('—');
		expect(formatTaiwanPlainNumber(0)).toBe('0');
		expect(formatTaiwanPlainNumber(27.94)).toBe((27.94).toLocaleString('zh-TW', { maximumFractionDigits: 2 }));
	});

	it('27. dividend_yield: null -> —, 0 -> 0%, positive -> unsigned percent (no leading +)', () => {
		expect(formatTaiwanPercent(null)).toBe('—');
		expect(formatTaiwanPercent(0)).toBe('0%');
		expect(formatTaiwanPercent(5.3)).toBe('5.3%');
	});

	it('28-30. dividends: null -> —, zero -> 0, normal decimal formatting', () => {
		expect(formatTaiwanPlainNumber(null)).toBe('—');
		expect(formatTaiwanPlainNumber(0)).toBe('0');
		expect(formatTaiwanPlainNumber(5)).toBe((5).toLocaleString('zh-TW', { maximumFractionDigits: 2 }));
	});
});

describe('M7E-B — financial statement query builder', () => {
	it('default filters include the 6 new financial-statement fields, all blank', () => {
		const defaults = taiwanScreenerDefaultFilters();
		for (const key of ['minCumulativeEPS', 'maxCumulativeEPS', 'minGrossMargin', 'maxGrossMargin', 'minOperatingMargin', 'maxOperatingMargin'] as const) {
			expect(defaults[key]).toBe('');
		}
	});

	it('A. blank financial fields are omitted from the query', () => {
		const path = taiwanScreenerPath(taiwanScreenerDefaultFilters());
		expect(path).not.toMatch(/min_cumulative_eps|max_cumulative_eps|min_gross_margin|max_gross_margin|min_operating_margin|max_operating_margin/);
	});

	it('B. each exact key is emitted correctly', () => {
		const withValue = (overrides: Partial<TaiwanScreenerFilters>) => taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), ...overrides });
		expect(withValue({ minCumulativeEPS: '10' })).toContain('min_cumulative_eps=10');
		expect(withValue({ maxCumulativeEPS: '20' })).toContain('max_cumulative_eps=20');
		expect(withValue({ minGrossMargin: '30' })).toContain('min_gross_margin=30');
		expect(withValue({ maxGrossMargin: '40' })).toContain('max_gross_margin=40');
		expect(withValue({ minOperatingMargin: '50' })).toContain('min_operating_margin=50');
		expect(withValue({ maxOperatingMargin: '60' })).toContain('max_operating_margin=60');
	});

	it('C. all six combined correctly in one query', () => {
		const path = taiwanScreenerPath({
			...taiwanScreenerDefaultFilters(),
			minCumulativeEPS: '1', maxCumulativeEPS: '2', minGrossMargin: '3', maxGrossMargin: '4', minOperatingMargin: '5', maxOperatingMargin: '6',
		});
		expect(path).toContain('min_cumulative_eps=1');
		expect(path).toContain('max_cumulative_eps=2');
		expect(path).toContain('min_gross_margin=3');
		expect(path).toContain('max_gross_margin=4');
		expect(path).toContain('min_operating_margin=5');
		expect(path).toContain('max_operating_margin=6');
	});

	it('D. negative values are preserved', () => {
		const path = taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), minCumulativeEPS: '-12.3', minGrossMargin: '-4', minOperatingMargin: '-5.5' });
		expect(path).toContain('min_cumulative_eps=-12.3');
		expect(path).toContain('min_gross_margin=-4');
		expect(path).toContain('min_operating_margin=-5.5');
	});

	it('E. malformed draft can never produce NaN/Infinity in the query output', () => {
		const path = taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), minCumulativeEPS: 'abc', maxGrossMargin: 'Infinity', minOperatingMargin: '-Infinity' });
		expect(path).not.toMatch(/min_cumulative_eps=|max_gross_margin=|min_operating_margin=/);
		expect(path).not.toMatch(/NaN|Infinity/);
	});

	it('F. existing M7D/M7E-A query params remain unchanged when financial fields are set', () => {
		const path = taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), minForeignNet: '100', minPE: '10', minCumulativeEPS: '1' });
		expect(path).toContain('min_foreign_net=100');
		expect(path).toContain('min_pe=10');
		expect(path).toContain('min_cumulative_eps=1');
	});

	it('rejects inverted financial-statement ranges', () => {
		expect(validateTaiwanScreenerFilters({ ...taiwanScreenerDefaultFilters(), minCumulativeEPS: '10', maxCumulativeEPS: '5' })).toBe('最低累計 EPS 不可高於最高累計 EPS');
		expect(validateTaiwanScreenerFilters({ ...taiwanScreenerDefaultFilters(), minGrossMargin: '10', maxGrossMargin: '5' })).toBe('最低毛利率不可高於最高毛利率');
		expect(validateTaiwanScreenerFilters({ ...taiwanScreenerDefaultFilters(), minOperatingMargin: '10', maxOperatingMargin: '5' })).toBe('最低營業利益率不可高於最高營業利益率');
	});
});

describe('M7E-B — sort options', () => {
	it('exposes cumulative_eps/gross_margin/operating_margin without removing or duplicating existing options', () => {
		const ids = taiwanScreenerSortOptions.map((item) => item.id);
		expect(ids).toContain('cumulative_eps');
		expect(ids).toContain('gross_margin');
		expect(ids).toContain('operating_margin');
		expect(new Set(ids).size).toBe(ids.length); // no duplicate values
		for (const id of ['price', 'change_percent', 'volume', 'amount', 'monthly_revenue', 'pe', 'cash_dividend']) {
			expect(ids).toContain(id);
		}
	});

	it('sort=cumulative_eps/gross_margin/operating_margin query construction', () => {
		expect(taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), sort: 'cumulative_eps' })).toContain('sort=cumulative_eps');
		expect(taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), sort: 'gross_margin' })).toContain('sort=gross_margin');
		expect(taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), sort: 'operating_margin' })).toContain('sort=operating_margin');
	});
});

describe('M7E-B — applied-domain detection (financial statement)', () => {
	it('taiwanScreenerHasFinancialsCriteria is false by default, true with an active filter, true with a financial sort key', () => {
		expect(taiwanScreenerHasFinancialsCriteria(taiwanScreenerDefaultFilters())).toBe(false);
		expect(taiwanScreenerHasFinancialsCriteria({ ...taiwanScreenerDefaultFilters(), minCumulativeEPS: '0' })).toBe(true);
		expect(taiwanScreenerHasFinancialsCriteria({ ...taiwanScreenerDefaultFilters(), maxGrossMargin: '0' })).toBe(true);
		expect(taiwanScreenerHasFinancialsCriteria({ ...taiwanScreenerDefaultFilters(), minOperatingMargin: '0' })).toBe(true);
		expect(taiwanScreenerHasFinancialsCriteria({ ...taiwanScreenerDefaultFilters(), sort: 'cumulative_eps' })).toBe(true);
		expect(taiwanScreenerHasFinancialsCriteria({ ...taiwanScreenerDefaultFilters(), sort: 'gross_margin' })).toBe(true);
		expect(taiwanScreenerHasFinancialsCriteria({ ...taiwanScreenerDefaultFilters(), sort: 'operating_margin' })).toBe(true);
	});

	it('Clear (taiwanScreenerDefaultFilters()) resets financial criteria to false', () => {
		expect(taiwanScreenerHasFinancialsCriteria(taiwanScreenerDefaultFilters())).toBe(false);
	});
});

describe('M7E-B — financials status label', () => {
	it('maps available/partial/unavailable to the required Chinese labels, never a daily-cadence 最新', () => {
		expect(taiwanFinancialsStatusLabel('available')).toBe('可使用');
		expect(taiwanFinancialsStatusLabel('partial')).toBe('部分可使用');
		expect(taiwanFinancialsStatusLabel('unavailable')).toBe('無法取得');
		expect(taiwanFinancialsStatusLabel('available')).not.toBe('最新');
	});

	it('returns empty for an absent status (domain not requested)', () => {
		expect(taiwanFinancialsStatusLabel(undefined)).toBe('');
	});
});

describe('M7E-C — BVPS + net margin query builder', () => {
	it('default filters include the 4 new fields, all blank', () => {
		const defaults = taiwanScreenerDefaultFilters();
		for (const key of ['minNetMargin', 'maxNetMargin', 'minBookValuePerShare', 'maxBookValuePerShare'] as const) {
			expect(defaults[key]).toBe('');
		}
	});

	it('A. blank fields are omitted from the query', () => {
		const path = taiwanScreenerPath(taiwanScreenerDefaultFilters());
		expect(path).not.toMatch(/min_net_margin|max_net_margin|min_book_value_per_share|max_book_value_per_share/);
	});

	it('B. each exact key is emitted correctly', () => {
		const withValue = (overrides: Partial<TaiwanScreenerFilters>) => taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), ...overrides });
		expect(withValue({ minNetMargin: '10' })).toContain('min_net_margin=10');
		expect(withValue({ maxNetMargin: '20' })).toContain('max_net_margin=20');
		expect(withValue({ minBookValuePerShare: '30' })).toContain('min_book_value_per_share=30');
		expect(withValue({ maxBookValuePerShare: '40' })).toContain('max_book_value_per_share=40');
	});

	it('C. all four combined correctly in one query', () => {
		const path = taiwanScreenerPath({
			...taiwanScreenerDefaultFilters(),
			minNetMargin: '1', maxNetMargin: '2', minBookValuePerShare: '3', maxBookValuePerShare: '4',
		});
		expect(path).toContain('min_net_margin=1');
		expect(path).toContain('max_net_margin=2');
		expect(path).toContain('min_book_value_per_share=3');
		expect(path).toContain('max_book_value_per_share=4');
	});

	it('D. negative values are preserved', () => {
		const path = taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), minNetMargin: '-12.3', minBookValuePerShare: '-4.5' });
		expect(path).toContain('min_net_margin=-12.3');
		expect(path).toContain('min_book_value_per_share=-4.5');
	});

	it('D2. an explicit zero is serialized, never dropped as if blank', () => {
		const path = taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), minNetMargin: '0', minBookValuePerShare: '0' });
		expect(path).toContain('min_net_margin=0');
		expect(path).toContain('min_book_value_per_share=0');
	});

	it('E. malformed draft can never produce NaN/Infinity in the query output', () => {
		const path = taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), minNetMargin: 'abc', maxBookValuePerShare: 'Infinity' });
		expect(path).not.toMatch(/min_net_margin=|max_book_value_per_share=/);
		expect(path).not.toMatch(/NaN|Infinity/);
	});

	it('F. existing M7D/M7E-A/M7E-B query params remain unchanged when the new fields are set', () => {
		const path = taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), minForeignNet: '100', minPE: '10', minCumulativeEPS: '1', minNetMargin: '2' });
		expect(path).toContain('min_foreign_net=100');
		expect(path).toContain('min_pe=10');
		expect(path).toContain('min_cumulative_eps=1');
		expect(path).toContain('min_net_margin=2');
	});

	it('rejects inverted ranges', () => {
		expect(validateTaiwanScreenerFilters({ ...taiwanScreenerDefaultFilters(), minNetMargin: '10', maxNetMargin: '5' })).toBe('最低淨利率不可高於最高淨利率');
		expect(validateTaiwanScreenerFilters({ ...taiwanScreenerDefaultFilters(), minBookValuePerShare: '10', maxBookValuePerShare: '5' })).toBe('最低每股參考淨值不可高於最高每股參考淨值');
	});
});

describe('M7E-C — sort options', () => {
	it('exposes net_margin/book_value_per_share without removing or duplicating existing options', () => {
		const ids = taiwanScreenerSortOptions.map((item) => item.id);
		expect(ids).toContain('net_margin');
		expect(ids).toContain('book_value_per_share');
		expect(new Set(ids).size).toBe(ids.length); // no duplicate values
		for (const id of ['price', 'cumulative_eps', 'gross_margin', 'operating_margin', 'pe', 'cash_dividend']) {
			expect(ids).toContain(id);
		}
	});

	it('sort=net_margin/book_value_per_share query construction', () => {
		expect(taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), sort: 'net_margin' })).toContain('sort=net_margin');
		expect(taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), sort: 'book_value_per_share' })).toContain('sort=book_value_per_share');
	});
});

describe('M7E-C — applied-domain detection (net margin / BVPS join the existing financials criteria)', () => {
	it('taiwanScreenerHasFinancialsCriteria is true for an active net_margin/BVPS filter or sort key', () => {
		expect(taiwanScreenerHasFinancialsCriteria({ ...taiwanScreenerDefaultFilters(), minNetMargin: '0' })).toBe(true);
		expect(taiwanScreenerHasFinancialsCriteria({ ...taiwanScreenerDefaultFilters(), maxBookValuePerShare: '0' })).toBe(true);
		expect(taiwanScreenerHasFinancialsCriteria({ ...taiwanScreenerDefaultFilters(), sort: 'net_margin' })).toBe(true);
		expect(taiwanScreenerHasFinancialsCriteria({ ...taiwanScreenerDefaultFilters(), sort: 'book_value_per_share' })).toBe(true);
	});

	it('Clear (taiwanScreenerDefaultFilters()) resets it to false', () => {
		expect(taiwanScreenerHasFinancialsCriteria(taiwanScreenerDefaultFilters())).toBe(false);
	});
});

describe('M7E-C — book value per share formatter', () => {
	it('appends 元／股 without rescaling', () => {
		expect(formatTaiwanBookValuePerShare(248.05)).toBe('248.05 元／股');
	});

	it('null renders as —, never 0', () => {
		expect(formatTaiwanBookValuePerShare(null)).toBe('—');
		expect(formatTaiwanBookValuePerShare(undefined)).toBe('—');
	});

	it('real zero is preserved, never treated as missing', () => {
		expect(formatTaiwanBookValuePerShare(0)).toBe('0 元／股');
	});

	it('negative value is preserved with its sign', () => {
		expect(formatTaiwanBookValuePerShare(-3.2)).toBe('-3.2 元／股');
	});
});

describe('M7E-C — net margin uses the existing percent formatter (no ×100 rescale)', () => {
	it('53.19 renders as 53.19%, not 5319%', () => {
		expect(formatTaiwanPercent(53.19)).toBe('53.19%');
	});

	it('null renders as — (financial-holding categories like 2882.TWSE)', () => {
		expect(formatTaiwanPercent(null)).toBe('—');
	});
});

describe('M7F — revenue growth query builder', () => {
	it('default filters include the 4 new fields, all blank', () => {
		const defaults = taiwanScreenerDefaultFilters();
		for (const key of ['minRevenueMoM', 'maxRevenueMoM', 'minCumulativeRevenueYoY', 'maxCumulativeRevenueYoY'] as const) {
			expect(defaults[key]).toBe('');
		}
	});

	it('A. blank fields are omitted from the query', () => {
		const path = taiwanScreenerPath(taiwanScreenerDefaultFilters());
		expect(path).not.toMatch(/min_revenue_mom|max_revenue_mom|min_cumulative_revenue_yoy|max_cumulative_revenue_yoy/);
	});

	it('B. each exact key is emitted correctly', () => {
		const withValue = (overrides: Partial<TaiwanScreenerFilters>) => taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), ...overrides });
		expect(withValue({ minRevenueMoM: '10' })).toContain('min_revenue_mom=10');
		expect(withValue({ maxRevenueMoM: '20' })).toContain('max_revenue_mom=20');
		expect(withValue({ minCumulativeRevenueYoY: '30' })).toContain('min_cumulative_revenue_yoy=30');
		expect(withValue({ maxCumulativeRevenueYoY: '40' })).toContain('max_cumulative_revenue_yoy=40');
	});

	it('C. all four combined correctly in one query', () => {
		const path = taiwanScreenerPath({
			...taiwanScreenerDefaultFilters(),
			minRevenueMoM: '1', maxRevenueMoM: '2', minCumulativeRevenueYoY: '3', maxCumulativeRevenueYoY: '4',
		});
		expect(path).toContain('min_revenue_mom=1');
		expect(path).toContain('max_revenue_mom=2');
		expect(path).toContain('min_cumulative_revenue_yoy=3');
		expect(path).toContain('max_cumulative_revenue_yoy=4');
	});

	it('D. negative values are preserved', () => {
		const path = taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), minRevenueMoM: '-11.5', minCumulativeRevenueYoY: '-4.44' });
		expect(path).toContain('min_revenue_mom=-11.5');
		expect(path).toContain('min_cumulative_revenue_yoy=-4.44');
	});

	it('D2. an explicit zero is serialized, never dropped as if blank', () => {
		const path = taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), minRevenueMoM: '0', minCumulativeRevenueYoY: '0' });
		expect(path).toContain('min_revenue_mom=0');
		expect(path).toContain('min_cumulative_revenue_yoy=0');
	});

	it('E. malformed draft can never produce NaN/Infinity in the query output', () => {
		const path = taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), minRevenueMoM: 'abc', maxCumulativeRevenueYoY: 'Infinity' });
		expect(path).not.toMatch(/min_revenue_mom=|max_cumulative_revenue_yoy=/);
		expect(path).not.toMatch(/NaN|Infinity/);
	});

	it('F. existing fundamentals params remain unchanged when the new fields are set', () => {
		const path = taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), minMonthlyRevenue: '100', minRevenueYoY: '5', minRevenueMoM: '1' });
		expect(path).toContain('min_monthly_revenue=100');
		expect(path).toContain('min_revenue_yoy=5');
		expect(path).toContain('min_revenue_mom=1');
	});

	it('rejects inverted ranges', () => {
		expect(validateTaiwanScreenerFilters({ ...taiwanScreenerDefaultFilters(), minRevenueMoM: '10', maxRevenueMoM: '5' })).toBe('最低月營收月增率不可高於最高月營收月增率');
		expect(validateTaiwanScreenerFilters({ ...taiwanScreenerDefaultFilters(), minCumulativeRevenueYoY: '10', maxCumulativeRevenueYoY: '5' })).toBe('最低累計營收年增率不可高於最高累計營收年增率');
	});
});

describe('M7F — sort options', () => {
	it('exposes revenue_mom/cumulative_revenue_yoy without removing or duplicating existing options', () => {
		const ids = taiwanScreenerSortOptions.map((item) => item.id);
		expect(ids).toContain('revenue_mom');
		expect(ids).toContain('cumulative_revenue_yoy');
		expect(new Set(ids).size).toBe(ids.length);
		for (const id of ['price', 'monthly_revenue', 'revenue_yoy', 'cumulative_eps', 'net_margin', 'book_value_per_share']) {
			expect(ids).toContain(id);
		}
	});

	it('sort=revenue_mom/cumulative_revenue_yoy query construction', () => {
		expect(taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), sort: 'revenue_mom' })).toContain('sort=revenue_mom');
		expect(taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), sort: 'cumulative_revenue_yoy' })).toContain('sort=cumulative_revenue_yoy');
	});
});

describe('M7F — applied-domain detection (revenue growth joins the existing revenue criteria)', () => {
	it('taiwanScreenerHasRevenueCriteria is true for an active revenue_mom/cumulative_revenue_yoy filter or sort key', () => {
		expect(taiwanScreenerHasRevenueCriteria({ ...taiwanScreenerDefaultFilters(), minRevenueMoM: '0' })).toBe(true);
		expect(taiwanScreenerHasRevenueCriteria({ ...taiwanScreenerDefaultFilters(), maxCumulativeRevenueYoY: '0' })).toBe(true);
		expect(taiwanScreenerHasRevenueCriteria({ ...taiwanScreenerDefaultFilters(), sort: 'revenue_mom' })).toBe(true);
		expect(taiwanScreenerHasRevenueCriteria({ ...taiwanScreenerDefaultFilters(), sort: 'cumulative_revenue_yoy' })).toBe(true);
	});

	it('Clear (taiwanScreenerDefaultFilters()) resets it to false', () => {
		expect(taiwanScreenerHasRevenueCriteria(taiwanScreenerDefaultFilters())).toBe(false);
	});
});

describe('M7F — revenue growth percentage rendering (existing formatter, no rescale)', () => {
	it('5.62 renders as 5.62%, not 562%', () => {
		expect(formatTaiwanPercent(5.62)).toBe('5.62%');
	});

	it('-11.5 renders as -11.5%, sign preserved', () => {
		expect(formatTaiwanPercent(-11.5)).toBe('-11.5%');
	});

	it('0 renders as 0%, not missing', () => {
		expect(formatTaiwanPercent(0)).toBe('0%');
	});

	it('null renders as —', () => {
		expect(formatTaiwanPercent(null)).toBe('—');
	});
});

describe('M7G — balance ratio query builder', () => {
	it('default filters include the 6 new fields, all blank', () => {
		const defaults = taiwanScreenerDefaultFilters();
		for (const key of ['minDebtRatio', 'maxDebtRatio', 'minDebtToEquity', 'maxDebtToEquity', 'minCurrentRatio', 'maxCurrentRatio'] as const) {
			expect(defaults[key]).toBe('');
		}
	});

	it('A. blank fields are omitted from the query', () => {
		const path = taiwanScreenerPath(taiwanScreenerDefaultFilters());
		expect(path).not.toMatch(/min_debt_ratio|max_debt_ratio|min_debt_to_equity|max_debt_to_equity|min_current_ratio|max_current_ratio/);
	});

	it('B. each exact key is emitted correctly', () => {
		const withValue = (overrides: Partial<TaiwanScreenerFilters>) => taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), ...overrides });
		expect(withValue({ minDebtRatio: '10' })).toContain('min_debt_ratio=10');
		expect(withValue({ maxDebtRatio: '20' })).toContain('max_debt_ratio=20');
		expect(withValue({ minDebtToEquity: '30' })).toContain('min_debt_to_equity=30');
		expect(withValue({ maxDebtToEquity: '40' })).toContain('max_debt_to_equity=40');
		expect(withValue({ minCurrentRatio: '50' })).toContain('min_current_ratio=50');
		expect(withValue({ maxCurrentRatio: '60' })).toContain('max_current_ratio=60');
	});

	it('C. all six combined correctly in one query', () => {
		const path = taiwanScreenerPath({
			...taiwanScreenerDefaultFilters(),
			minDebtRatio: '1', maxDebtRatio: '2', minDebtToEquity: '3', maxDebtToEquity: '4', minCurrentRatio: '5', maxCurrentRatio: '6',
		});
		expect(path).toContain('min_debt_ratio=1');
		expect(path).toContain('max_debt_ratio=2');
		expect(path).toContain('min_debt_to_equity=3');
		expect(path).toContain('max_debt_to_equity=4');
		expect(path).toContain('min_current_ratio=5');
		expect(path).toContain('max_current_ratio=6');
	});

	it('D. negative values are preserved', () => {
		const path = taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), minDebtRatio: '-12.3', minDebtToEquity: '-4.5', minCurrentRatio: '-1' });
		expect(path).toContain('min_debt_ratio=-12.3');
		expect(path).toContain('min_debt_to_equity=-4.5');
		expect(path).toContain('min_current_ratio=-1');
	});

	it('D2. an explicit zero is serialized, never dropped as if blank', () => {
		const path = taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), minDebtRatio: '0', minDebtToEquity: '0', minCurrentRatio: '0' });
		expect(path).toContain('min_debt_ratio=0');
		expect(path).toContain('min_debt_to_equity=0');
		expect(path).toContain('min_current_ratio=0');
	});

	it('E. malformed/Infinity/NaN draft can never produce those in the query output', () => {
		const path = taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), minDebtRatio: 'abc', maxDebtToEquity: 'Infinity', minCurrentRatio: 'NaN' });
		expect(path).not.toMatch(/min_debt_ratio=|max_debt_to_equity=|min_current_ratio=/);
		expect(path).not.toMatch(/NaN|Infinity/);
	});

	it('F. existing M7E-C/M7F query params remain unchanged when the new fields are set', () => {
		const path = taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), minNetMargin: '2', minBookValuePerShare: '3', minDebtRatio: '4' });
		expect(path).toContain('min_net_margin=2');
		expect(path).toContain('min_book_value_per_share=3');
		expect(path).toContain('min_debt_ratio=4');
	});

	it('rejects inverted ranges', () => {
		expect(validateTaiwanScreenerFilters({ ...taiwanScreenerDefaultFilters(), minDebtRatio: '10', maxDebtRatio: '5' })).toBe('負債比率最小不可高於最大');
		expect(validateTaiwanScreenerFilters({ ...taiwanScreenerDefaultFilters(), minDebtToEquity: '10', maxDebtToEquity: '5' })).toBe('負債權益比最小不可高於最大');
		expect(validateTaiwanScreenerFilters({ ...taiwanScreenerDefaultFilters(), minCurrentRatio: '10', maxCurrentRatio: '5' })).toBe('流動比率最小不可高於最大');
	});
});

describe('M7G — sort options', () => {
	it('exposes debt_ratio/debt_to_equity/current_ratio without removing or duplicating existing options', () => {
		const ids = taiwanScreenerSortOptions.map((item) => item.id);
		expect(ids).toContain('debt_ratio');
		expect(ids).toContain('debt_to_equity');
		expect(ids).toContain('current_ratio');
		expect(new Set(ids).size).toBe(ids.length); // no duplicate values
		for (const id of ['price', 'cumulative_eps', 'net_margin', 'book_value_per_share', 'revenue_mom']) {
			expect(ids).toContain(id);
		}
	});

	it('sort=debt_ratio/debt_to_equity/current_ratio query construction', () => {
		expect(taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), sort: 'debt_ratio' })).toContain('sort=debt_ratio');
		expect(taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), sort: 'debt_to_equity' })).toContain('sort=debt_to_equity');
		expect(taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), sort: 'current_ratio' })).toContain('sort=current_ratio');
	});
});

// M7G's balance-ratio domain is a genuinely INDEPENDENT domain on the backend (its own
// balance_period/balance_status, decoupled from financials_period/financials_status), so it gets its
// own applied-domain detection helper rather than joining taiwanScreenerHasFinancialsCriteria — unlike
// M7E-C's net_margin/BVPS, which reuse the existing financials criteria because the backend combines
// their status into one financials_status.
describe('M7G — applied-domain detection (independent balance criteria helper)', () => {
	it('taiwanScreenerHasBalanceCriteria is true for an active debt_ratio/debt_to_equity/current_ratio filter or sort key', () => {
		expect(taiwanScreenerHasBalanceCriteria({ ...taiwanScreenerDefaultFilters(), minDebtRatio: '0' })).toBe(true);
		expect(taiwanScreenerHasBalanceCriteria({ ...taiwanScreenerDefaultFilters(), maxDebtToEquity: '0' })).toBe(true);
		expect(taiwanScreenerHasBalanceCriteria({ ...taiwanScreenerDefaultFilters(), minCurrentRatio: '0' })).toBe(true);
		expect(taiwanScreenerHasBalanceCriteria({ ...taiwanScreenerDefaultFilters(), sort: 'debt_ratio' })).toBe(true);
		expect(taiwanScreenerHasBalanceCriteria({ ...taiwanScreenerDefaultFilters(), sort: 'debt_to_equity' })).toBe(true);
		expect(taiwanScreenerHasBalanceCriteria({ ...taiwanScreenerDefaultFilters(), sort: 'current_ratio' })).toBe(true);
	});

	it('Clear (taiwanScreenerDefaultFilters()) resets it to false', () => {
		expect(taiwanScreenerHasBalanceCriteria(taiwanScreenerDefaultFilters())).toBe(false);
	});

	it('is NOT triggered by an active net_margin/BVPS filter (a separate domain on the backend)', () => {
		expect(taiwanScreenerHasBalanceCriteria({ ...taiwanScreenerDefaultFilters(), minNetMargin: '0' })).toBe(false);
		expect(taiwanScreenerHasBalanceCriteria({ ...taiwanScreenerDefaultFilters(), minBookValuePerShare: '0' })).toBe(false);
		expect(taiwanScreenerHasBalanceCriteria({ ...taiwanScreenerDefaultFilters(), sort: 'book_value_per_share' })).toBe(false);
	});

	it('taiwanScreenerHasFinancialsCriteria remains unaffected by an active M7G filter (still income/BVPS only)', () => {
		expect(taiwanScreenerHasFinancialsCriteria({ ...taiwanScreenerDefaultFilters(), minDebtRatio: '0' })).toBe(false);
		expect(taiwanScreenerHasFinancialsCriteria({ ...taiwanScreenerDefaultFilters(), sort: 'current_ratio' })).toBe(false);
	});
});

describe('M7G — percentage rendering (existing formatter, no rescale)', () => {
	it('30.94 renders as 30.94%, not 3094%', () => {
		expect(formatTaiwanPercent(30.94)).toBe('30.94%');
	});

	it('126.65 renders as 126.65%', () => {
		expect(formatTaiwanPercent(126.65)).toBe('126.65%');
	});

	it('0 renders as 0%, not missing', () => {
		expect(formatTaiwanPercent(0)).toBe('0%');
	});

	it('-5.5 renders as -5.5%, sign preserved', () => {
		expect(formatTaiwanPercent(-5.5)).toBe('-5.5%');
	});

	it('null renders as — (e.g. a non-ci category like 2882.TWSE)', () => {
		expect(formatTaiwanPercent(null)).toBe('—');
	});
});

describe('M7H — cash-flow query builder', () => {
	it('default filters include the 4 new fields, all blank', () => {
		const defaults = taiwanScreenerDefaultFilters();
		for (const key of ['minOperatingCashFlow', 'maxOperatingCashFlow', 'minCashFlowToNetIncome', 'maxCashFlowToNetIncome'] as const) {
			expect(defaults[key]).toBe('');
		}
	});

	it('A. blank fields are omitted from the query', () => {
		const path = taiwanScreenerPath(taiwanScreenerDefaultFilters());
		expect(path).not.toMatch(/min_operating_cash_flow|max_operating_cash_flow|min_cash_flow_to_net_income|max_cash_flow_to_net_income/);
	});

	it('B. each exact key is emitted correctly', () => {
		const withValue = (overrides: Partial<TaiwanScreenerFilters>) => taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), ...overrides });
		expect(withValue({ minOperatingCashFlow: '10' })).toContain('min_operating_cash_flow=10');
		expect(withValue({ maxOperatingCashFlow: '20' })).toContain('max_operating_cash_flow=20');
		expect(withValue({ minCashFlowToNetIncome: '30' })).toContain('min_cash_flow_to_net_income=30');
		expect(withValue({ maxCashFlowToNetIncome: '40' })).toContain('max_cash_flow_to_net_income=40');
	});

	it('C. all four combined correctly in one query', () => {
		const path = taiwanScreenerPath({
			...taiwanScreenerDefaultFilters(),
			minOperatingCashFlow: '1', maxOperatingCashFlow: '2', minCashFlowToNetIncome: '3', maxCashFlowToNetIncome: '4',
		});
		expect(path).toContain('min_operating_cash_flow=1');
		expect(path).toContain('max_operating_cash_flow=2');
		expect(path).toContain('min_cash_flow_to_net_income=3');
		expect(path).toContain('max_cash_flow_to_net_income=4');
	});

	it('D. negative values are preserved (both metrics can legitimately be negative)', () => {
		const path = taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), minOperatingCashFlow: '-500000', minCashFlowToNetIncome: '-6.34' });
		expect(path).toContain('min_operating_cash_flow=-500000');
		expect(path).toContain('min_cash_flow_to_net_income=-6.34');
	});

	it('D2. an explicit zero is serialized, never dropped as if blank', () => {
		const path = taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), minOperatingCashFlow: '0', minCashFlowToNetIncome: '0' });
		expect(path).toContain('min_operating_cash_flow=0');
		expect(path).toContain('min_cash_flow_to_net_income=0');
	});

	it('E. malformed/Infinity/NaN draft can never produce those in the query output', () => {
		const path = taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), minOperatingCashFlow: 'abc', maxCashFlowToNetIncome: 'Infinity' });
		expect(path).not.toMatch(/min_operating_cash_flow=|max_cash_flow_to_net_income=/);
		expect(path).not.toMatch(/NaN|Infinity/);
	});

	it('F. existing M7G query params remain unchanged when the new fields are set', () => {
		const path = taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), minDebtRatio: '4', minOperatingCashFlow: '5' });
		expect(path).toContain('min_debt_ratio=4');
		expect(path).toContain('min_operating_cash_flow=5');
	});

	it('rejects inverted ranges', () => {
		expect(validateTaiwanScreenerFilters({ ...taiwanScreenerDefaultFilters(), minOperatingCashFlow: '10', maxOperatingCashFlow: '5' })).toBe('營業活動現金流量最小不可高於最大');
		expect(validateTaiwanScreenerFilters({ ...taiwanScreenerDefaultFilters(), minCashFlowToNetIncome: '10', maxCashFlowToNetIncome: '5' })).toBe('營業現金流／淨利最小不可高於最大');
	});
});

describe('M7H — sort options', () => {
	it('exposes operating_cash_flow/cash_flow_to_net_income without removing or duplicating existing options', () => {
		const ids = taiwanScreenerSortOptions.map((item) => item.id);
		expect(ids).toContain('operating_cash_flow');
		expect(ids).toContain('cash_flow_to_net_income');
		expect(new Set(ids).size).toBe(ids.length); // no duplicate values
		for (const id of ['price', 'debt_ratio', 'net_margin', 'book_value_per_share']) {
			expect(ids).toContain(id);
		}
	});

	it('sort=operating_cash_flow/cash_flow_to_net_income query construction', () => {
		expect(taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), sort: 'operating_cash_flow' })).toContain('sort=operating_cash_flow');
		expect(taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), sort: 'cash_flow_to_net_income' })).toContain('sort=cash_flow_to_net_income');
	});
});

// M7H's cash-flow domain is a genuinely INDEPENDENT domain on the backend (its own cashflow_period/
// cashflow_status, sourced from the MOPS XBRL bulk archive rather than TWSE OpenAPI/TPEx bulk JSON), so
// it gets its own applied-domain detection helper rather than joining taiwanScreenerHasFinancialsCriteria
// or taiwanScreenerHasBalanceCriteria.
describe('M7H — applied-domain detection (independent cash-flow criteria helper)', () => {
	it('taiwanScreenerHasCashflowCriteria is true for an active operating_cash_flow/cash_flow_to_net_income filter or sort key', () => {
		expect(taiwanScreenerHasCashflowCriteria({ ...taiwanScreenerDefaultFilters(), minOperatingCashFlow: '0' })).toBe(true);
		expect(taiwanScreenerHasCashflowCriteria({ ...taiwanScreenerDefaultFilters(), maxCashFlowToNetIncome: '0' })).toBe(true);
		expect(taiwanScreenerHasCashflowCriteria({ ...taiwanScreenerDefaultFilters(), sort: 'operating_cash_flow' })).toBe(true);
		expect(taiwanScreenerHasCashflowCriteria({ ...taiwanScreenerDefaultFilters(), sort: 'cash_flow_to_net_income' })).toBe(true);
	});

	it('Clear (taiwanScreenerDefaultFilters()) resets it to false', () => {
		expect(taiwanScreenerHasCashflowCriteria(taiwanScreenerDefaultFilters())).toBe(false);
	});

	it('is NOT triggered by an active debt_ratio/net_margin filter (separate domains on the backend)', () => {
		expect(taiwanScreenerHasCashflowCriteria({ ...taiwanScreenerDefaultFilters(), minDebtRatio: '0' })).toBe(false);
		expect(taiwanScreenerHasCashflowCriteria({ ...taiwanScreenerDefaultFilters(), minNetMargin: '0' })).toBe(false);
		expect(taiwanScreenerHasCashflowCriteria({ ...taiwanScreenerDefaultFilters(), sort: 'current_ratio' })).toBe(false);
	});

	it('taiwanScreenerHasFinancialsCriteria/taiwanScreenerHasBalanceCriteria remain unaffected by an active M7H filter', () => {
		expect(taiwanScreenerHasFinancialsCriteria({ ...taiwanScreenerDefaultFilters(), minOperatingCashFlow: '0' })).toBe(false);
		expect(taiwanScreenerHasBalanceCriteria({ ...taiwanScreenerDefaultFilters(), sort: 'cash_flow_to_net_income' })).toBe(false);
	});
});

describe('M7H — ratio rendering (existing formatter, no ×100 regression)', () => {
	it('132.62 renders as 132.62%, not 13262%', () => {
		expect(formatTaiwanPercent(132.62)).toBe('132.62%');
	});

	it('-6.34 renders as -6.34%, sign preserved', () => {
		expect(formatTaiwanPercent(-6.34)).toBe('-6.34%');
	});

	it('0 renders as 0%, not missing', () => {
		expect(formatTaiwanPercent(0)).toBe('0%');
	});

	it('null renders as —', () => {
		expect(formatTaiwanPercent(null)).toBe('—');
	});
});

describe('M7H — operating_cash_flow display (thousand-TWD raw contract preserved)', () => {
	it('positive value renders in 億元, derived from the raw thousand-TWD number', () => {
		expect(formatTaiwanCashFlowTWD(1122637757)).toBe(`${(1122637757 / 100_000).toLocaleString('zh-TW', { maximumFractionDigits: 2 })} 億元`);
	});

	it('negative value stays visibly negative', () => {
		const rendered = formatTaiwanCashFlowTWD(-150005);
		expect(rendered).toContain('-');
		expect(rendered).toBe(`${(-150005 / 100_000).toLocaleString('zh-TW', { maximumFractionDigits: 2 })} 億元`);
	});

	it('zero renders as a real zero, not —', () => {
		expect(formatTaiwanCashFlowTWD(0)).toBe('0 億元');
	});

	it('null renders as —', () => {
		expect(formatTaiwanCashFlowTWD(null)).toBe('—');
	});

	it('does not rescale the underlying raw value for query/filter purposes (raw thousand-TWD number is untouched)', () => {
		const path = taiwanScreenerPath({ ...taiwanScreenerDefaultFilters(), minOperatingCashFlow: '1122637757' });
		expect(path).toContain('min_operating_cash_flow=1122637757');
	});
});

describe('M8B — buildTaiwanScreenerContext (Screener → Research navigation provenance)', () => {
	it('no active criteria: filterLabels is empty, sortLabel reflects the default sort/order', () => {
		const context = buildTaiwanScreenerContext(taiwanScreenerDefaultFilters());
		expect(context.source).toBe('screener');
		expect(context.filterLabels).toEqual([]);
		expect(context.sortLabel).toBe('成交金額（高到低）');
	});

	it('min only renders "≥"', () => {
		const context = buildTaiwanScreenerContext({ ...taiwanScreenerDefaultFilters(), minPE: '10' });
		expect(context.filterLabels).toContain('本益比（PE） ≥ 10');
	});

	it('max only renders "≤"', () => {
		const context = buildTaiwanScreenerContext({ ...taiwanScreenerDefaultFilters(), maxPE: '25' });
		expect(context.filterLabels).toContain('本益比（PE） ≤ 25');
	});

	it('min+max renders a single range constraint', () => {
		const context = buildTaiwanScreenerContext({ ...taiwanScreenerDefaultFilters(), minDebtRatio: '10', maxDebtRatio: '50' });
		expect(context.filterLabels).toContain('負債比率 10% ～ 50%');
	});

	it('explicit zero is included, never treated as inactive', () => {
		const context = buildTaiwanScreenerContext({ ...taiwanScreenerDefaultFilters(), minOperatingCashFlow: '0' });
		expect(context.filterLabels).toContain('營業活動現金流量 ≥ 0仟元');
	});

	it('a negative threshold is included and preserves its sign', () => {
		const context = buildTaiwanScreenerContext({ ...taiwanScreenerDefaultFilters(), minChangePercent: '-5' });
		expect(context.filterLabels).toContain('漲跌幅 ≥ -5%');
	});

	it('percentage fields keep the % suffix, never rescaled', () => {
		const context = buildTaiwanScreenerContext({ ...taiwanScreenerDefaultFilters(), minCashFlowToNetIncome: '100' });
		expect(context.filterLabels).toContain('營業現金流／淨利 ≥ 100%');
	});

	it('cash-flow amount keeps its raw thousand-TWD (仟元) semantics, never converted to 億元', () => {
		const context = buildTaiwanScreenerContext({ ...taiwanScreenerDefaultFilters(), minForeignNet: '1000' });
		expect(context.filterLabels).toContain('外資買賣超 ≥ 1,000股');
	});

	it('sort ascending is labeled distinctly from descending', () => {
		const context = buildTaiwanScreenerContext({ ...taiwanScreenerDefaultFilters(), sort: 'pe', order: 'asc' });
		expect(context.sortLabel).toBe('本益比（PE）（低到高）');
	});

	it('sort descending', () => {
		const context = buildTaiwanScreenerContext({ ...taiwanScreenerDefaultFilters(), sort: 'pe', order: 'desc' });
		expect(context.sortLabel).toBe('本益比（PE）（高到低）');
	});

	it('pagination (limit/offset) is never surfaced as a criterion', () => {
		const context = buildTaiwanScreenerContext({ ...taiwanScreenerDefaultFilters(), limit: 100, offset: 50 });
		expect(context.filterLabels).toEqual([]);
	});

	it('scope alone (no numeric filter) is not surfaced as a criterion', () => {
		const context = buildTaiwanScreenerContext({ ...taiwanScreenerDefaultFilters(), scope: 'twse' });
		expect(context.filterLabels).toEqual([]);
	});
});

describe('M8F — buildTaiwanScreenerMatchReason / taiwanScreenerMatchReasonSuffix ("why did this stock match?")', () => {
	it('no active filter for the field -> null (no reason)', () => {
		const applied = taiwanScreenerDefaultFilters();
		const row = screenerSecurityFixture({ revenue_yoy: 28.4 });
		expect(buildTaiwanScreenerMatchReason('revenue_yoy', applied, row)).toBeNull();
		expect(taiwanScreenerMatchReasonSuffix('revenue_yoy', applied, row)).toBe('');
	});

	it('min-only formatting: "≥ threshold"', () => {
		const applied = { ...taiwanScreenerDefaultFilters(), minRevenueYoY: '15' };
		const row = screenerSecurityFixture({ revenue_yoy: 28.4 });
		const reason = buildTaiwanScreenerMatchReason('revenue_yoy', applied, row);
		expect(reason).toEqual({ label: '月營收年增率', actual: '28.4%', condition: '≥ 15%' });
		expect(taiwanScreenerMatchReasonSuffix('revenue_yoy', applied, row)).toBe('（≥ 15%）');
	});

	it('max-only formatting: "≤ threshold"', () => {
		const applied = { ...taiwanScreenerDefaultFilters(), maxDebtRatio: '50' };
		const row = screenerSecurityFixture({ debt_ratio: 42.1 });
		const reason = buildTaiwanScreenerMatchReason('debt_ratio', applied, row);
		expect(reason).toEqual({ label: '負債比率', actual: '42.1%', condition: '≤ 50%' });
	});

	it('min + max formatting: "X–Y" range', () => {
		const applied = { ...taiwanScreenerDefaultFilters(), minPE: '0.01', maxPE: '20' };
		const row = screenerSecurityFixture({ pe: 12.4 });
		const reason = buildTaiwanScreenerMatchReason('pe', applied, row);
		expect(reason).toEqual({ label: '本益比（PE）', actual: '12.4', condition: '0.01–20' });
	});

	it('an explicit zero threshold is still active and rendered (not treated as unset)', () => {
		const applied = { ...taiwanScreenerDefaultFilters(), minOperatingCashFlow: '0' };
		const row = screenerSecurityFixture({ operating_cash_flow: 1122637757 });
		const reason = buildTaiwanScreenerMatchReason('operating_cash_flow', applied, row);
		expect(reason?.condition).toBe(`≥ ${(0).toLocaleString('zh-TW')} 億元`);
	});

	it('a negative threshold is still active and rendered', () => {
		const applied = { ...taiwanScreenerDefaultFilters(), minChangePercent: '-5' };
		const row = screenerSecurityFixture({ change_percent: -2 });
		const reason = buildTaiwanScreenerMatchReason('change_percent', applied, row);
		expect(reason?.condition).toContain('-5');
	});

	it('null row actual value -> reason omitted even though the filter is active', () => {
		const applied = { ...taiwanScreenerDefaultFilters(), minRevenueYoY: '15' };
		const row = screenerSecurityFixture({ revenue_yoy: null });
		expect(buildTaiwanScreenerMatchReason('revenue_yoy', applied, row)).toBeNull();
	});

	it('non-finite row actual value (NaN/Infinity) -> reason omitted, never rendered', () => {
		const applied = { ...taiwanScreenerDefaultFilters(), minRevenueYoY: '15' };
		expect(buildTaiwanScreenerMatchReason('revenue_yoy', applied, screenerSecurityFixture({ revenue_yoy: NaN }))).toBeNull();
		expect(buildTaiwanScreenerMatchReason('revenue_yoy', applied, screenerSecurityFixture({ revenue_yoy: Infinity }))).toBeNull();
	});

	it('cash-flow threshold and actual value share the SAME 億元 unit -- never raw 仟元 beside a converted actual value', () => {
		const applied = { ...taiwanScreenerDefaultFilters(), minOperatingCashFlow: '1122637757' };
		const row = screenerSecurityFixture({ operating_cash_flow: 1122637757 });
		const reason = buildTaiwanScreenerMatchReason('operating_cash_flow', applied, row);
		expect(reason?.actual).toContain('億元');
		expect(reason?.condition).toContain('億元');
		expect(reason?.condition).not.toMatch(/仟元/);
	});

	it('低估值觀察-style PE/PB ranges render consistently with the M8E preset contract (0.01 boundary, not 0)', () => {
		const applied = { ...taiwanScreenerDefaultFilters(), minPE: '0.01', maxPE: '20', minPB: '0.01', maxPB: '2' };
		const peRow = screenerSecurityFixture({ pe: 18.5, pb: 1.6 });
		expect(buildTaiwanScreenerMatchReason('pe', applied, peRow)?.condition).toBe('0.01–20');
		expect(buildTaiwanScreenerMatchReason('pb', applied, peRow)?.condition).toBe('0.01–2');
	});

	it('財務穩健-style preset produces two independent reasons (debt_ratio + current_ratio)', () => {
		const applied = { ...taiwanScreenerDefaultFilters(), maxDebtRatio: '50', minCurrentRatio: '100' };
		const row = screenerSecurityFixture({ debt_ratio: 30.94, current_ratio: 245.76 });
		expect(buildTaiwanScreenerMatchReason('debt_ratio', applied, row)?.condition).toBe('≤ 50%');
		expect(buildTaiwanScreenerMatchReason('current_ratio', applied, row)?.condition).toBe('≥ 100%');
	});

	it('現金流健康-style preset produces two independent reasons (operating_cash_flow + cash_flow_to_net_income)', () => {
		const applied = { ...taiwanScreenerDefaultFilters(), minOperatingCashFlow: '1', minCashFlowToNetIncome: '80' };
		const row = screenerSecurityFixture({ operating_cash_flow: 1122637757, cash_flow_to_net_income: 148.06 });
		expect(buildTaiwanScreenerMatchReason('operating_cash_flow', applied, row)).not.toBeNull();
		expect(buildTaiwanScreenerMatchReason('cash_flow_to_net_income', applied, row)?.condition).toBe('≥ 80%');
	});

	it('法人偏多-style preset produces one reason (institutional_net) using the same signed-shares formatter for both sides', () => {
		const applied = { ...taiwanScreenerDefaultFilters(), minInstitutionalNet: '1' };
		const row = screenerSecurityFixture({ institutional_net: 2300000 });
		const reason = buildTaiwanScreenerMatchReason('institutional_net', applied, row);
		expect(reason?.label).toBe('三大法人合計');
		expect(reason?.actual).toContain('+');
	});

	it('manual filters (no preset involved) produce identical reason quality -- the source of truth is `applied`, never activePresetId', () => {
		const applied = { ...taiwanScreenerDefaultFilters(), minPE: '8', maxPE: '15' };
		const row = screenerSecurityFixture({ pe: 12.1 });
		expect(buildTaiwanScreenerMatchReason('pe', applied, row)).toEqual({ label: '本益比（PE）', actual: '12.1', condition: '8–15' });
	});

	it('uses `applied`, not `draft` -- a stale-applied threshold must keep showing until Apply is pressed (verified by only ever accepting one filters argument, the caller\'s job to pass applied)', () => {
		const appliedAtQueryTime = { ...taiwanScreenerDefaultFilters(), minRevenueYoY: '15' };
		const laterDraftEdit = { ...taiwanScreenerDefaultFilters(), minRevenueYoY: '30' };
		const row = screenerSecurityFixture({ revenue_yoy: 20 });
		// The row was produced by appliedAtQueryTime (revenue_yoy >= 15); its own reason must reflect
		// that threshold, never the newer unapplied draft edit.
		expect(buildTaiwanScreenerMatchReason('revenue_yoy', appliedAtQueryTime, row)?.condition).toBe('≥ 15%');
		expect(buildTaiwanScreenerMatchReason('revenue_yoy', laterDraftEdit, row)?.condition).not.toBe('≥ 15%');
	});

	it('base-column price annotation (no unit suffix, matching how ScreenerRow itself displays price — plain toLocaleString)', () => {
		const applied = { ...taiwanScreenerDefaultFilters(), minPrice: '50' };
		const row = screenerSecurityFixture({ price: 65 });
		expect(buildTaiwanScreenerMatchReason('price', applied, row)?.condition).toBe('≥ 50');
	});

	it('base-column change_percent annotation', () => {
		const applied = { ...taiwanScreenerDefaultFilters(), minChangePercent: '5' };
		const row = screenerSecurityFixture({ change_percent: 8.2 });
		expect(buildTaiwanScreenerMatchReason('change_percent', applied, row)?.condition).toBe('≥ 5%');
	});

	it('base-column volume annotation (no unit suffix, matching how ScreenerRow itself displays volume — plain toLocaleString)', () => {
		const applied = { ...taiwanScreenerDefaultFilters(), minVolume: '1000000' };
		const row = screenerSecurityFixture({ volume: 14102018 });
		expect(buildTaiwanScreenerMatchReason('volume', applied, row)?.condition).toBe(`≥ ${(1000000).toLocaleString('zh-TW')}`);
	});

	it('base-column amount annotation', () => {
		const applied = { ...taiwanScreenerDefaultFilters(), minAmount: '10000000000' };
		const row = screenerSecurityFixture({ amount: 33917316870 });
		const reason = buildTaiwanScreenerMatchReason('amount', applied, row);
		expect(reason?.actual).toContain('億元');
		expect(reason?.condition).toContain('億元');
	});

	it('labels reuse the existing descriptor source -- match the exact same Chinese text as buildTaiwanScreenerContext (M8B)', () => {
		const filters = { ...taiwanScreenerDefaultFilters(), minRevenueYoY: '15' };
		const context = buildTaiwanScreenerContext(filters);
		const reason = buildTaiwanScreenerMatchReason('revenue_yoy', filters, screenerSecurityFixture({ revenue_yoy: 28.4 }));
		expect(context.filterLabels[0]).toContain(reason?.label as string);
	});

	it('unknown rowKey (no descriptor) -> null, never throws', () => {
		const applied = taiwanScreenerDefaultFilters();
		expect(buildTaiwanScreenerMatchReason('canonical', applied, screenerSecurityFixture())).toBeNull();
		expect(buildTaiwanScreenerMatchReason('trade_date', applied, screenerSecurityFixture())).toBeNull();
	});
});
