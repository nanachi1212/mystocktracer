import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import {
	formatTaiwanPercent, formatTaiwanPlainNumber, formatTaiwanRatio, formatTaiwanRevenueTWD, formatTaiwanTWD, resolveTaiwanWorkspace, runScopedRequest,
	taiwanComponentList, taiwanDefaultWorkspace, taiwanIntelligencePath, taiwanMarketPath, taiwanPrimaryNavigation, taiwanResearchPath,
	taiwanScreenerDefaultFilters, taiwanScreenerHasDividendCriteria, taiwanScreenerHasRevenueCriteria, taiwanScreenerHasValuationCriteria,
	taiwanScreenerPath, taiwanScreenerSortOptions, taiwanStatusLabel, validateTaiwanScreenerFilters, type TaiwanScreenerFilters,
} from './taiwan-product';

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
		expect(html).toContain('台灣股票研究工作台');
	});

	it('uses the Taiwan overview for empty and unknown hashes', () => {
		expect(resolveTaiwanWorkspace('')).toBe(taiwanDefaultWorkspace);
		expect(resolveTaiwanWorkspace('#unknown')).toBe(taiwanDefaultWorkspace);
		expect(resolveTaiwanWorkspace('#taiwan-emotion')).toBe('taiwan-emotion');
	});

	it('keeps only Taiwan-safe primary navigation labels', () => {
		expect(taiwanPrimaryNavigation.map((item) => item[1])).toEqual(['台股總覽', '市場廣度', '市場情緒', '產業雷達', '台股選股器', '個股研究', 'AI 研究', '自選股']);
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
	it('16. sort dropdown offers exactly 21 options (M7A 4 + M7D 9 + M7E-A 8)', () => {
		expect(taiwanScreenerSortOptions.length).toBe(21);
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
