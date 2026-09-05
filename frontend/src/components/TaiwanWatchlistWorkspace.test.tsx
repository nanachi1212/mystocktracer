import fs from 'node:fs';
import path from 'node:path';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import type { Quote, SourceMeta } from '../lib/backend';
import { chunkTaiwanSymbols, type TaiwanWatchlistSecurity } from '../lib/taiwan-product';
import { TaiwanWatchlistWorkspace, WatchlistRow } from './TaiwanWatchlistWorkspace';

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
		expect(app).toContain("if (window.location.hash === '#taiwan-watchlist') return 'taiwan-watchlist';");
		expect(app).toMatch(/switchWorkspace\('taiwan-watchlist'\)/);
		expect(app).toContain('自選股');
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
		expect(html).toContain('可從台股總覽或個股研究加入');
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
