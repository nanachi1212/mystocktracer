import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { formatTaiwanRatio, formatTaiwanTWD, resolveTaiwanWorkspace, runScopedRequest, taiwanComponentList, taiwanDefaultWorkspace, taiwanIntelligencePath, taiwanMarketPath, taiwanPrimaryNavigation, taiwanResearchPath, taiwanStatusLabel } from './taiwan-product';

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
		expect(taiwanPrimaryNavigation.map((item) => item[1])).toEqual(['台股總覽', '市場廣度', '市場情緒', '產業雷達', '個股研究', 'AI 研究']);
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
		// whether TWSE/TPEx data actually loaded. The topbar wording must not conflate the two;
		// per-scope status (available/partial/stale/unavailable) is shown by each Taiwan view
		// itself, sourced from backend status/freshness fields.
		expect(app).not.toContain('台股官方資料服務已連線');
		expect(app).toContain('後端服務已連線');
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
