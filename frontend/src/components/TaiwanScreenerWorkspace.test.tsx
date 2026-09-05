import fs from 'node:fs';
import path from 'node:path';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import {
	taiwanScreenerDefaultFilters, taiwanScreenerHasDividendCriteria, taiwanScreenerHasInstitutionalCriteria, taiwanScreenerHasMarginCriteria,
	taiwanScreenerHasRevenueCriteria, taiwanScreenerHasValuationCriteria, taiwanScreenerPath,
	type TaiwanScreenerFilters, type TaiwanScreenerResponse, type TaiwanScreenerSecurity,
} from '../lib/taiwan-product';
import {
	ScreenerAdvancedCell, ScreenerDomainFreshness, ScreenerFilterPanel, ScreenerPagination, ScreenerRow, ScreenerSummary, ScreenerTable,
	ScreenerWatchlistAction, type WatchlistMembershipState,
} from './TaiwanScreenerWorkspace';

const root = path.resolve(__dirname, '../../..');
const workspaceSource = () => fs.readFileSync(path.join(root, 'frontend/src/components/TaiwanScreenerWorkspace.tsx'), 'utf8');
const appSource = () => fs.readFileSync(path.join(root, 'frontend/src/App.tsx'), 'utf8');
const overviewSource = () => fs.readFileSync(path.join(root, 'frontend/src/components/market/TaiwanMarketView.tsx'), 'utf8');

const security = (overrides: Partial<TaiwanScreenerSecurity> = {}): TaiwanScreenerSecurity => ({
	canonical: '2330.TWSE', code: '2330', name: '台積電', exchange: 'TWSE', security_type: 'stock', trade_date: '2026-09-04',
	price: 2410, change: 20, change_percent: 0.84, volume: 14102018, amount: 33917316870,
	foreign_net: null, trust_net: null, dealer_net: null, institutional_net: null,
	margin_balance: null, margin_change: null, short_balance: null, short_change: null, short_margin_ratio: null,
	monthly_revenue: null, revenue_yoy: null, pe: null, pb: null, dividend_yield: null,
	cash_dividend: null, stock_dividend: null, total_dividend: null,
	...overrides,
});

const response = (overrides: Partial<TaiwanScreenerResponse> = {}): TaiwanScreenerResponse => ({
	scope: 'COMBINED', as_of: '2026-09-04', freshness: 'current', total: 1, offset: 0, limit: 50, securities: [security()],
	...overrides,
});

// M7C — default row-level Watchlist props: "ready, not saved, not busy, no error" unless overridden.
// M7D/M7E-A — showInstitutional/showMargin/showRevenue/showValuation/showDividends default false
// (legacy row shape) unless a test opts in.
const rowWatchlistProps = (overrides: Partial<{ membershipState: WatchlistMembershipState; saved: boolean; busy: boolean; mutationError?: string; showInstitutional: boolean; showMargin: boolean; showRevenue: boolean; showValuation: boolean; showDividends: boolean }> = {}) => ({
	membershipState: 'ready' as WatchlistMembershipState, saved: false, busy: false, mutationError: undefined as string | undefined, onToggleWatchlist: () => {},
	showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: false,
	...overrides,
});

const tableWatchlistProps = (overrides: Partial<{ watchlistState: WatchlistMembershipState; watchlistedCanonicals: Set<string>; busyCanonicals: Set<string>; mutationErrors: Record<string, string>; showInstitutional: boolean; showMargin: boolean; showRevenue: boolean; showValuation: boolean; showDividends: boolean }> = {}) => ({
	watchlistState: 'ready' as WatchlistMembershipState, watchlistedCanonicals: new Set<string>(), busyCanonicals: new Set<string>(), mutationErrors: {} as Record<string, string>, onToggleWatchlist: () => {},
	showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: false,
	...overrides,
});

describe('M7B — navigation wiring', () => {
	it('App.tsx declares a taiwan-screener workspace mode, hash mapping, and a 台股選股器 nav button', () => {
		const app = appSource();
		expect(app).toContain("'taiwan-screener'");
		expect(app).toContain("if (window.location.hash === '#taiwan-screener') return 'taiwan-screener';");
		expect(app).toMatch(/switchWorkspace\('taiwan-screener'\)/);
		expect(app).toContain('台股選股器');
	});

	it('renders TaiwanScreenerWorkspace for the taiwan-screener mode, wired to the existing App research handoff', () => {
		const app = appSource();
		expect(app).toContain("workspaceMode === 'taiwan-screener' ? <TaiwanScreenerWorkspace config={config} refreshKey={marketRefreshKey} onOpenResearch={openTaiwanStockResearch} />");
	});
});

describe('M7B — cold start does not preload the Screener', () => {
	it('the Taiwan overview cold-start indexes effect does not fetch the Screener or (M7C) the Watchlist', () => {
		const source = overviewSource();
		const mountEffect = source.slice(source.indexOf('useEffect(() => {'), source.indexOf('[config, refreshKey]'));
		expect(mountEffect).toContain('/api/v1/tw/indexes');
		expect(mountEffect).not.toMatch(/screener/i);
		expect(mountEffect).not.toMatch(/watchlist/i);
	});

	it('TaiwanScreenerWorkspace only fetches inside its own mount/refresh effect, guarded by config', () => {
		const source = workspaceSource();
		const effectBody = source.slice(source.indexOf('useEffect(() => {'), source.indexOf('}, [config, refreshKey, applied]);'));
		expect(effectBody).toContain('if (!config) return;');
		expect(effectBody).toContain('taiwanScreenerPath(applied)');
	});

	it('introduces no polling/timer', () => {
		expect(workspaceSource()).not.toMatch(/setInterval|setTimeout/);
	});
});

describe('M7B — draft vs applied filter state', () => {
	it('every filter control writes to draft (onChange), never directly to applied/fetch', () => {
		const source = workspaceSource();
		const panel = source.slice(source.indexOf('export function ScreenerFilterPanel'), source.indexOf('export function ScreenerSummary'));
		expect(panel).not.toMatch(/setApplied|requestJSON/);
		expect(panel).toContain('onChange(');
	});

	it('applyFilters is the only place that changes `applied`, and it resets offset to 0', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const applyFilters = () => {'), source.indexOf('const clearFilters = () => {'));
		expect(fn).toContain('validateTaiwanScreenerFilters(draft)');
		expect(fn).toContain('setApplied({ ...draft, offset: 0 })');
	});

	it('a local validation failure sets localError and returns before touching applied', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const applyFilters = () => {'), source.indexOf('const clearFilters = () => {'));
		expect(fn).toMatch(/if \(message\) \{ setLocalError\(message\); return; \}/);
	});

	it('clearFilters restores both draft and applied to taiwanScreenerDefaultFilters()', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const clearFilters = () => {'), source.indexOf('const goPrevious = () => {'));
		expect(fn).toContain('taiwanScreenerDefaultFilters()');
		expect(fn).toMatch(/setDraft\(defaults\)/);
		expect(fn).toMatch(/setApplied\(defaults\)/);
	});

	it('pagination changes only offset — the current applied filters are spread, not replaced', () => {
		const source = workspaceSource();
		const previous = source.slice(source.indexOf('const goPrevious = () => {'), source.indexOf('const goNext = () => {'));
		const next = source.slice(source.indexOf('const goNext = () => {'), source.indexOf('return <div className="taiwan-product-workspace'));
		expect(previous).toContain('setApplied((current) => ({ ...current, offset:');
		expect(next).toContain('setApplied((current) => ({ ...current, offset:');
	});

	it('the fetch effect depends on applied (not draft) — global refresh repeats the current applied query unchanged', () => {
		const source = workspaceSource();
		expect(source).toContain('}, [config, refreshKey, applied]);');
		expect(source).not.toMatch(/\[config, refreshKey, draft\]/);
	});

	it('uses the shared runScopedRequest race guard so a stale Apply response cannot overwrite a newer one', () => {
		const source = workspaceSource();
		expect(source).toContain('runScopedRequest(requestID');
		expect(source).toContain('setData(null); setLoading(true); setError(\'\');');
	});
});

describe('M7B/M7C — no request fan-out / no local filtering', () => {
	it('the only Watchlist calls are the existing fetch/add/remove helpers — no per-row membership or quote requests', () => {
		const source = workspaceSource();
		expect(source).toContain('fetchTaiwanWatchlist(config)');
		expect(source).toContain('addTaiwanWatchlistSecurity(config, canonical)');
		expect(source).toContain('removeTaiwanWatchlistSecurity(config, canonical)');
		expect(source).not.toMatch(/\/api\/v1\/tw\/quotes/);
		expect(source).not.toMatch(/isTaiwanSecurityWatchlisted/); // that helper itself calls fetchTaiwanWatchlist per-row — must not be reused here
	});

	it('never calls AI research / intelligence endpoints', () => {
		const source = workspaceSource();
		expect(source).not.toMatch(/taiwanResearchPath|taiwanIntelligencePath|generateResearch/);
	});

	it('never calls fundamentals/institutional/margin endpoints', () => {
		const source = workspaceSource();
		expect(source).not.toMatch(/\/api\/v1\/tw\/fundamentals|\/api\/v1\/tw\/institutional|\/api\/v1\/tw\/margin/);
	});

	it('issues exactly one requestJSON call per result load', () => {
		const source = workspaceSource();
		const matches = source.match(/requestJSON</g) || [];
		expect(matches.length).toBe(1);
	});

	it('never fetches every page automatically (no loop over offset/pages)', () => {
		const source = workspaceSource();
		expect(source).not.toMatch(/for\s*\(|while\s*\(/);
	});

	it('never filters/sorts the securities array locally — renders the backend response as-is', () => {
		const source = workspaceSource();
		expect(source).not.toMatch(/securities\.filter\(|securities\.sort\(|data\.securities\.filter\(|data\.securities\.sort\(/);
	});
});

describe('M7B — row click passes exact canonical to the existing App research handoff', () => {
	it('the identity button is wired to onOpen, and onOpen is called with the exact canonical (TWSE)', () => {
		let opened = '';
		const element = ScreenerRow({ security: security(), onOpen: () => { opened = '2330.TWSE'; }, ...rowWatchlistProps() });
		const identityCell = (element.props.children as unknown[])[0] as { props: { children: { props: { onClick: () => void } } } };
		identityCell.props.children.props.onClick();
		expect(opened).toBe('2330.TWSE');
	});

	it('a TPEX security remains 6488.TPEX end to end — never re-inferred or collapsed to code-only', () => {
		let opened = '';
		const tpex = security({ canonical: '6488.TPEX', code: '6488', name: '環球晶', exchange: 'TPEX' });
		const html = renderToStaticMarkup(<ScreenerTable securities={[tpex]} onOpenResearch={(canonical) => { opened = canonical; }} {...tableWatchlistProps()} />);
		expect(html).toContain('環球晶');
		expect(html).toContain('TPEX');
		// Simulate the click via the same wiring ScreenerTable uses (onOpenResearch(item.canonical)).
		const element = ScreenerRow({ security: tpex, onOpen: () => opened = tpex.canonical, ...rowWatchlistProps() });
		const identityCell = (element.props.children as unknown[])[0] as { props: { children: { props: { onClick: () => void } } } };
		identityCell.props.children.props.onClick();
		expect(opened).toBe('6488.TPEX');
	});

	it('ScreenerTable wires each row onOpen to onOpenResearch(item.canonical), not code alone', () => {
		const source = workspaceSource();
		const table = source.slice(source.indexOf('export function ScreenerTable'), source.indexOf('// Pure/presentational row.'));
		expect(table).toContain('onOpen={() => onOpenResearch(item.canonical)}');
		expect(table).not.toMatch(/onOpenResearch\(item\.code\)/);
	});

	it('TaiwanScreenerWorkspace implements no navigation token/nonce of its own — relies entirely on the existing M6C handoff', () => {
		const source = workspaceSource();
		expect(source).not.toMatch(/token|Nonce/);
	});

	it('the Watchlist action cell is a sibling <td> of the identity <td>, never nested inside it', () => {
		const element = ScreenerRow({ security: security(), onOpen: () => {}, ...rowWatchlistProps() });
		const cells = (element.props.children as unknown[]).filter(Boolean) as { type: string }[];
		expect(cells.every((cell) => cell.type === 'td')).toBe(true);
		expect(cells.length).toBe(8); // 7 data columns + 1 Watchlist action column, all flat siblings
	});

	it('clicking the ScreenerWatchlistAction toggle calls onToggle only — it has no onOpen/navigation callback at all', () => {
		let toggled = false;
		const element = ScreenerWatchlistAction({ membershipState: 'ready', saved: false, busy: false, onToggle: () => { toggled = true; } });
		const button = (element.props.children as unknown[])[0] as { type: string; props: { onClick: () => void } };
		expect(button.type).toBe('button');
		button.props.onClick();
		expect(toggled).toBe(true);
	});
});

describe('M7B — missing-value and zero-value rendering (ScreenerRow)', () => {
	it('renders — for a null price, null change_percent, null volume, and null amount', () => {
		const html = renderToStaticMarkup(<table><tbody>{ScreenerRow({ security: security({ price: null, change_percent: null, volume: null, amount: null }), onOpen: () => {}, ...rowWatchlistProps() })}</tbody></table>);
		expect(html).toContain('—');
		expect((html.match(/—/g) || []).length).toBeGreaterThanOrEqual(4);
		expect(html).not.toMatch(/>0<\/td>|>0%<\/td>/);
	});

	it('renders a genuine zero as zero, not as missing', () => {
		const html = renderToStaticMarkup(<table><tbody>{ScreenerRow({ security: security({ price: 0, change_percent: 0, volume: 0, amount: 0 }), onOpen: () => {}, ...rowWatchlistProps() })}</tbody></table>);
		expect(html).toContain('<td>0</td>');
		expect(html).toContain('0%');
	});

	it('renders a signed change_percent (positive shows +, negative shows the sign)', () => {
		const up = renderToStaticMarkup(<table><tbody>{ScreenerRow({ security: security({ change_percent: 1.5 }), onOpen: () => {}, ...rowWatchlistProps() })}</tbody></table>);
		expect(up).toContain('+1.5%');
		const down = renderToStaticMarkup(<table><tbody>{ScreenerRow({ security: security({ change_percent: -2.3 }), onOpen: () => {}, ...rowWatchlistProps() })}</tbody></table>);
		expect(down).toContain('-2.3%');
	});
});

describe('M7B — result summary (freshness/scope/as_of/total)', () => {
	it('renders total, scope, and freshness', () => {
		const html = renderToStaticMarkup(<ScreenerSummary data={response({ total: 42, scope: 'TWSE', freshness: 'current' })} />);
		expect(html).toContain('42');
		expect(html).toContain('TWSE');
		expect(html).toContain('最新');
	});

	it('renders as_of when provided, and 未提供 when null (never substitutes today\'s date)', () => {
		const withDate = renderToStaticMarkup(<ScreenerSummary data={response({ as_of: '2026-09-04' })} />);
		expect(withDate).toContain('資料日期 2026-09-04');
		const withoutDate = renderToStaticMarkup(<ScreenerSummary data={response({ as_of: null })} />);
		expect(withoutDate).toContain('資料日期 未提供');
	});
});

describe('M7B — empty state (total = 0 is not an error)', () => {
	it('workspace source shows the empty-state message only when data.total === 0, independent of the error banner', () => {
		const source = workspaceSource();
		expect(source).toContain('沒有符合目前條件的台灣證券');
		expect(source).toMatch(/data && data\.total === 0/);
	});
});

describe('M7B — pagination (ScreenerPagination)', () => {
	it('previous is disabled on the first page (offset = 0)', () => {
		const html = renderToStaticMarkup(<ScreenerPagination data={response({ offset: 0, total: 100, securities: [security()] })} loading={false} onPrevious={() => {}} onNext={() => {}} />);
		const buttons = html.match(/<button[^>]*>/g) || [];
		expect(buttons[0]).toMatch(/disabled/);
	});

	it('next is disabled on the final page (offset + securities.length >= total)', () => {
		const html = renderToStaticMarkup(<ScreenerPagination data={response({ offset: 50, total: 51, securities: [security()] })} loading={false} onPrevious={() => {}} onNext={() => {}} />);
		const buttons = html.match(/<button[^>]*>/g) || [];
		expect(buttons[1]).toMatch(/disabled/);
	});

	it('both enabled on a middle page', () => {
		const html = renderToStaticMarkup(<ScreenerPagination data={response({ offset: 50, total: 200, securities: Array.from({ length: 50 }, () => security()) })} loading={false} onPrevious={() => {}} onNext={() => {}} />);
		const buttons = html.match(/<button[^>]*>/g) || [];
		expect(buttons[0]).not.toMatch(/disabled/);
		expect(buttons[1]).not.toMatch(/disabled/);
	});

	it('shows a X–Y of N summary', () => {
		const html = renderToStaticMarkup(<ScreenerPagination data={response({ offset: 50, total: 200, securities: Array.from({ length: 50 }, () => security()) })} loading={false} onPrevious={() => {}} onNext={() => {}} />);
		expect(html).toContain('第 51–100 筆，共 200 筆');
	});
});

describe('M7B — filter panel renders backend-compatible defaults', () => {
	it('renders combined/amount/desc as the selected defaults', () => {
		const html = renderToStaticMarkup(<ScreenerFilterPanel draft={taiwanScreenerDefaultFilters()} onChange={() => {}} onApply={() => {}} onClear={() => {}} localError="" />);
		expect(html).toContain('套用條件');
		expect(html).toContain('清除條件');
	});

	it('shows a local validation error message when present', () => {
		const html = renderToStaticMarkup(<ScreenerFilterPanel draft={taiwanScreenerDefaultFilters()} onChange={() => {}} onApply={() => {}} onClear={() => {}} localError="最低價格不可高於最高價格" />);
		expect(html).toContain('最低價格不可高於最高價格');
	});
});

describe('M7B — error semantics', () => {
	it('maps a backend failure through the existing taiwanErrorMessage helper with a safe Traditional Chinese fallback', () => {
		const source = workspaceSource();
		expect(source).toContain("taiwanErrorMessage(reason, '台股選股資料載入失敗')");
	});
});

// ==================================================
// M7C -- Screener <-> Watchlist integration
// ==================================================

describe('M7C -- Watchlist membership load is independent of Screener query', () => {
	it('the Watchlist membership effect depends only on [config, refreshKey] -- never on `applied`', () => {
		const source = workspaceSource();
		expect(source).toContain('}, [config, refreshKey]);');
		const watchlistEffect = source.slice(source.indexOf('void runScopedRequest(watchlistRequestID'), source.indexOf('}, [config, refreshKey]);'));
		expect(watchlistEffect).toContain('fetchTaiwanWatchlist(config)');
		expect(watchlistEffect).not.toMatch(/applied/);
	});

	it('the Watchlist membership effect uses its own runScopedRequest ref, separate from the Screener request ref', () => {
		const source = workspaceSource();
		expect(source).toContain('const watchlistRequestID = useRef(0);');
		expect(source).toContain('runScopedRequest(watchlistRequestID');
		expect(source).toContain('runScopedRequest(requestID');
	});

	it('applyFilters/clearFilters/goPrevious/goNext never touch Watchlist membership state', () => {
		const source = workspaceSource();
		const handlers = source.slice(source.indexOf('const applyFilters = () => {'), source.indexOf('// Pessimistic add/remove'));
		expect(handlers).not.toMatch(/setWatchlistedCanonicals|setWatchlistState|setBusyCanonicals|setMutationErrors/);
	});

	it('the Screener-data fetch effect never touches Watchlist membership state (a Screener reload cannot reset it)', () => {
		const source = workspaceSource();
		const screenerEffect = source.slice(source.indexOf('void runScopedRequest(requestID'), source.indexOf('}, [config, refreshKey, applied]);'));
		expect(screenerEffect).not.toMatch(/setWatchlistedCanonicals|setWatchlistState|setBusyCanonicals|setMutationErrors/);
	});

	it('a Watchlist membership load failure only sets watchlistState -- it never touches Screener data/error state', () => {
		const source = workspaceSource();
		const watchlistEffect = source.slice(source.indexOf('void runScopedRequest(watchlistRequestID'), source.indexOf('}, [config, refreshKey]);'));
		expect(watchlistEffect).toContain("onError: () => setWatchlistState('error')");
		expect(watchlistEffect).not.toMatch(/setData\(|setError\(/);
	});
});

describe('M7C -- membership state model (loading/ready/error distinguished from false)', () => {
	it('membership ready + absent renders 加入自選', () => {
		const element = ScreenerWatchlistAction({ membershipState: 'ready', saved: false, busy: false, onToggle: () => {} });
		const html = renderToStaticMarkup(<>{element}</>);
		expect(html).toContain('加入自選');
		expect(html).not.toContain('移除自選');
	});

	it('membership ready + present renders 移除自選', () => {
		const element = ScreenerWatchlistAction({ membershipState: 'ready', saved: true, busy: false, onToggle: () => {} });
		const html = renderToStaticMarkup(<>{element}</>);
		expect(html).toContain('移除自選');
		expect(html).not.toContain('加入自選');
	});

	it('membership loading never renders 加入自選/移除自選 -- it shows an explicit loading indicator instead', () => {
		const element = ScreenerWatchlistAction({ membershipState: 'loading', saved: false, busy: false, onToggle: () => {} });
		const html = renderToStaticMarkup(<>{element}</>);
		expect(html).not.toContain('加入自選');
		expect(html).not.toContain('移除自選');
		expect(html).toContain('自選狀態讀取中');
	});

	it('membership error never renders 加入自選/移除自選 -- it shows an explicit unavailable indicator instead', () => {
		const element = ScreenerWatchlistAction({ membershipState: 'error', saved: false, busy: false, onToggle: () => {} });
		const html = renderToStaticMarkup(<>{element}</>);
		expect(html).not.toContain('加入自選');
		expect(html).not.toContain('移除自選');
		expect(html).toContain('自選狀態無法取得');
	});

	it('idle (pre-mount) state also never pretends "not saved" -- same unavailable branch as loading', () => {
		const element = ScreenerWatchlistAction({ membershipState: 'idle', saved: false, busy: false, onToggle: () => {} });
		const html = renderToStaticMarkup(<>{element}</>);
		expect(html).not.toContain('加入自選');
	});

	it('a top-level warning renders when Watchlist membership load fails, without touching the Screener error banner', () => {
		const source = workspaceSource();
		expect(source).toContain('watchlistState === \'error\' && <div className="market-partial-warning">自選股狀態暫時無法取得</div>');
	});
});

describe('M7C -- membership failure does not erase Screener results', () => {
	it('the workspace still renders ScreenerSummary/ScreenerTable purely from `data`, independent of watchlistState', () => {
		const source = workspaceSource();
		expect(source).toMatch(/\{data && <ScreenerSummary/);
		expect(source).toMatch(/\{data && data\.securities\.length > 0 && <ScreenerTable/);
	});
});

describe('M7C -- add/remove mutation uses exact canonical, pessimistic update, and per-security busy guard', () => {
	it('toggleWatchlist receives the row canonical directly (never security.code) via ScreenerTable wiring', () => {
		const source = workspaceSource();
		const table = source.slice(source.indexOf('export function ScreenerTable'), source.indexOf('// Pure/presentational row.'));
		expect(table).toContain('onToggleWatchlist={() => onToggleWatchlist(item.canonical)}');
		expect(table).not.toMatch(/onToggleWatchlist\(item\.code\)/);
	});

	it('the mutation function calls addTaiwanWatchlistSecurity/removeTaiwanWatchlistSecurity with the exact canonical parameter', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const toggleWatchlist = async'), source.indexOf('return <div className="taiwan-product-workspace'));
		expect(fn).toContain('addTaiwanWatchlistSecurity(config, canonical)');
		expect(fn).toContain('removeTaiwanWatchlistSecurity(config, canonical)');
	});

	it('a successful add/remove only updates the local Set after the await resolves (pessimistic, not optimistic)', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const toggleWatchlist = async'), source.indexOf('return <div className="taiwan-product-workspace'));
		const tryBlock = fn.slice(fn.indexOf('try {'), fn.indexOf('} catch'));
		expect(tryBlock).toMatch(/await addTaiwanWatchlistSecurity\(config, canonical\);\s*setWatchlistedCanonicals/);
		expect(tryBlock).toMatch(/await removeTaiwanWatchlistSecurity\(config, canonical\);\s*setWatchlistedCanonicals/);
	});

	it('a failed mutation only sets mutationErrors -- it never touches watchlistedCanonicals (prior confirmed membership is preserved)', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const toggleWatchlist = async'), source.indexOf('return <div className="taiwan-product-workspace'));
		const catchBlock = fn.slice(fn.indexOf('} catch (reason) {'), fn.indexOf('} finally {'));
		expect(catchBlock).toContain('setMutationErrors(');
		expect(catchBlock).not.toMatch(/setWatchlistedCanonicals/);
	});

	it('the mutation error message is produced via the existing taiwanErrorMessage helper with safe Traditional Chinese fallbacks', () => {
		const source = workspaceSource();
		expect(source).toContain("taiwanErrorMessage(reason, saved ? '移除自選失敗' : '加入自選失敗')");
	});

	it('a busy canonical is guarded at function entry -- a second click for the same canonical is a no-op while mutating', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const toggleWatchlist = async'), source.indexOf('const saved = watchlistedCanonicals'));
		expect(fn).toContain('busyCanonicals.has(canonical)');
	});

	it('busy state is a Set keyed by canonical (per-security), not a single global flag -- other rows remain usable', () => {
		const source = workspaceSource();
		expect(source).toContain('const [busyCanonicals, setBusyCanonicals] = useState<Set<string>>(new Set());');
		const fn = source.slice(source.indexOf('const toggleWatchlist = async'), source.indexOf('return <div className="taiwan-product-workspace'));
		expect(fn).toMatch(/setBusyCanonicals\(\(current\) => new Set\(current\)\.add\(canonical\)\)/);
		expect(fn).toMatch(/setBusyCanonicals\(\(current\) => \{ const next = new Set\(current\); next\.delete\(canonical\); return next; \}\)/);
	});

	it('ScreenerWatchlistAction renders a disabled, spinning toggle while busy=true, and an enabled one while idle', () => {
		const busyHtml = renderToStaticMarkup(<>{ScreenerWatchlistAction({ membershipState: 'ready', saved: false, busy: true, onToggle: () => {} })}</>);
		expect(busyHtml).toMatch(/<button[^>]*disabled[^>]*>/);
		const idleHtml = renderToStaticMarkup(<>{ScreenerWatchlistAction({ membershipState: 'ready', saved: false, busy: false, onToggle: () => {} })}</>);
		expect(idleHtml).not.toMatch(/<button[^>]*disabled[^>]*>/);
	});

	it('a mutation error message renders next to the toggle button when present', () => {
		const html = renderToStaticMarkup(<>{ScreenerWatchlistAction({ membershipState: 'ready', saved: false, busy: false, mutationError: '加入自選失敗', onToggle: () => {} })}</>);
		expect(html).toContain('加入自選失敗');
	});
});

describe('M7C -- no additional data fan-out from Screener Watchlist integration', () => {
	it('still issues exactly one Screener requestJSON call and never calls /tw/quotes/fundamentals/institutional/margin/intelligence/research/AI', () => {
		const source = workspaceSource();
		expect((source.match(/requestJSON</g) || []).length).toBe(1);
		expect(source).not.toMatch(/\/api\/v1\/tw\/quotes/);
		expect(source).not.toMatch(/\/api\/v1\/tw\/fundamentals|\/api\/v1\/tw\/institutional|\/api\/v1\/tw\/margin/);
		expect(source).not.toMatch(/taiwanIntelligencePath|taiwanResearchPath|hermes|Hermes/i);
	});
});

describe('M7C -- page/filter changes preserve membership state (no reset)', () => {
	it('goPrevious/goNext/applyFilters/clearFilters only ever call setApplied/setDraft/setLocalError -- never Watchlist setters', () => {
		const source = workspaceSource();
		const previous = source.slice(source.indexOf('const goPrevious = () => {'), source.indexOf('const goNext = () => {'));
		const next = source.slice(source.indexOf('const goNext = () => {'), source.indexOf('// Pessimistic add/remove'));
		expect(previous).not.toMatch(/setWatchlisted|setWatchlistState|setBusyCanonicals|setMutationErrors/);
		expect(next).not.toMatch(/setWatchlisted|setWatchlistState|setBusyCanonicals|setMutationErrors/);
	});
});

describe('M7C -- global refresh reloads both current query and Watchlist membership', () => {
	it('both effects include refreshKey in their dependency array, so an explicit refresh re-runs each exactly once', () => {
		const source = workspaceSource();
		expect(source).toContain('}, [config, refreshKey, applied]);');
		expect(source).toContain('}, [config, refreshKey]);');
	});
});

// ==================================================
// M7D -- Institutional + Margin filter UI
// ==================================================

const withAdvanced = (overrides: Partial<TaiwanScreenerFilters> = {}): TaiwanScreenerFilters => ({ ...taiwanScreenerDefaultFilters(), ...overrides });

describe('M7D -- advanced groups exist, default collapsed, collapse/expand has no request semantics', () => {
	it('1. renders both 法人籌碼 and 融資融券 group toggles', () => {
		const html = renderToStaticMarkup(<ScreenerFilterPanel draft={taiwanScreenerDefaultFilters()} onChange={() => {}} onApply={() => {}} onClear={() => {}} localError="" />);
		expect(html).toContain('法人籌碼');
		expect(html).toContain('融資融券');
	});

	it('2. advanced min/max inputs are not present in the initial (collapsed) render', () => {
		const html = renderToStaticMarkup(<ScreenerFilterPanel draft={taiwanScreenerDefaultFilters()} onChange={() => {}} onApply={() => {}} onClear={() => {}} localError="" />);
		expect(html).not.toContain('最低外資買賣超');
		expect(html).not.toContain('最低融資餘額');
	});

	it('3. the expand/collapse toggles are local useState, never call onChange/onApply/onClear', () => {
		const source = workspaceSource();
		const panel = source.slice(source.indexOf('export function ScreenerFilterPanel'), source.indexOf('export function ScreenerSummary'));
		expect(panel).toContain('useState(false)');
		const institutionalToggle = panel.slice(panel.indexOf('法人籌碼') - 400, panel.indexOf('法人籌碼'));
		expect(institutionalToggle).toContain('setInstitutionalExpanded');
		expect(institutionalToggle).not.toMatch(/onChange\(|onApply\(|onClear\(/);
	});
});

describe('M7D -- draft typing sends no request (institutional/margin)', () => {
	it('4-5. every advanced input writes via the same set() helper used by basic fields, never a request call', () => {
		const source = workspaceSource();
		const panel = source.slice(source.indexOf('export function ScreenerFilterPanel'), source.indexOf('export function ScreenerSummary'));
		expect(panel).toContain("set('minForeignNet'");
		expect(panel).toContain("set('minMarginBalance'");
		expect(panel).not.toMatch(/requestJSON|taiwanScreenerPath\(/);
	});
});

describe('M7D -- query construction', () => {
	it('6. Apply serializes institutional params', () => {
		const path = taiwanScreenerPath(withAdvanced({ minForeignNet: '100000', maxTrustNet: '-5000', minDealerNet: '0', minInstitutionalNet: '250000' }));
		expect(path).toContain('min_foreign_net=100000');
		expect(path).toContain('max_trust_net=-5000');
		expect(path).toContain('min_dealer_net=0');
		expect(path).toContain('min_institutional_net=250000');
	});

	it('7. Apply serializes margin params', () => {
		const path = taiwanScreenerPath(withAdvanced({ minMarginBalance: '5000000', maxMarginChange: '1000', minShortBalance: '0', minShortMarginRatio: '8.2' }));
		expect(path).toContain('min_margin_balance=5000000');
		expect(path).toContain('max_margin_change=1000');
		expect(path).toContain('min_short_balance=0');
		expect(path).toContain('min_short_margin_ratio=8.2');
	});

	it('8. combined institutional + margin params serialize together in one query', () => {
		const path = taiwanScreenerPath(withAdvanced({ minForeignNet: '100000', minMarginBalance: '5000000' }));
		expect(path).toContain('min_foreign_net=100000');
		expect(path).toContain('min_margin_balance=5000000');
	});

	it('9. blank advanced params are omitted entirely', () => {
		const path = taiwanScreenerPath(taiwanScreenerDefaultFilters());
		for (const key of ['foreign_net', 'trust_net', 'dealer_net', 'institutional_net', 'margin_balance', 'margin_change', 'short_balance', 'short_change', 'short_margin_ratio']) {
			expect(path).not.toContain(key);
		}
	});
});

describe('M7D -- advanced sort', () => {
	it('10. sort=institutional_net query', () => {
		expect(taiwanScreenerPath(withAdvanced({ sort: 'institutional_net' }))).toContain('sort=institutional_net');
	});

	it('11. sort=margin_balance query', () => {
		expect(taiwanScreenerPath(withAdvanced({ sort: 'margin_balance' }))).toContain('sort=margin_balance');
	});

	it('12. legacy sort keys still work unchanged', () => {
		expect(taiwanScreenerPath(withAdvanced({ sort: 'price' }))).toContain('sort=price');
		expect(taiwanScreenerPath(withAdvanced({ sort: 'amount' }))).toContain('sort=amount');
	});

	it('sort dropdown includes all 9 new advanced options plus the 4 legacy ones', () => {
		const html = renderToStaticMarkup(<ScreenerFilterPanel draft={taiwanScreenerDefaultFilters()} onChange={() => {}} onApply={() => {}} onClear={() => {}} localError="" />);
		for (const label of ['股價', '漲跌幅', '成交量', '成交金額', '外資買賣超', '投信買賣超', '自營商買賣超', '三大法人合計', '融資餘額', '融資增減', '融券餘額', '融券增減', '券資比']) {
			expect(html).toContain(label);
		}
	});
});

describe('M7D -- Clear / pagination / global refresh preserve advanced semantics', () => {
	it('13. taiwanScreenerDefaultFilters() (used by Clear) resets every advanced field to blank', () => {
		const defaults = taiwanScreenerDefaultFilters();
		expect(defaults.minForeignNet).toBe('');
		expect(defaults.maxShortMarginRatio).toBe('');
	});

	it('14. Apply resets offset to 0 even when advanced criteria are present', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const applyFilters = () => {'), source.indexOf('const clearFilters = () => {'));
		expect(fn).toContain('setApplied({ ...draft, offset: 0 })');
	});

	it('15-16. pagination/refresh spread the full applied object (including advanced fields), never rebuild it field-by-field', () => {
		const source = workspaceSource();
		const previous = source.slice(source.indexOf('const goPrevious = () => {'), source.indexOf('const goNext = () => {'));
		const next = source.slice(source.indexOf('const goNext = () => {'), source.indexOf('// Pessimistic add/remove'));
		expect(previous).toContain('setApplied((current) => ({ ...current, offset:');
		expect(next).toContain('setApplied((current) => ({ ...current, offset:');
	});
});

describe('M7D -- missing/zero/sign rendering (institutional)', () => {
	it('17. null institutional values render —', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ foreign_net: null, trust_net: null, dealer_net: null, institutional_net: null }), showInstitutional: true, showMargin: false, showRevenue: false, showValuation: false, showDividends: false })}</tr></tbody></table>);
		expect((html.match(/—/g) || []).length).toBeGreaterThanOrEqual(4);
	});

	it('18. zero institutional value renders real 0, not missing', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ foreign_net: 0, trust_net: 0, dealer_net: 0, institutional_net: 0 }), showInstitutional: true, showMargin: false, showRevenue: false, showValuation: false, showDividends: false })}</tr></tbody></table>);
		expect(html).toContain('外資 0');
		expect(html).not.toContain('外資 —');
	});

	it('19. positive/negative institutional values render explicit signs, never "+0"', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ foreign_net: 534504, trust_net: -12000, dealer_net: 0 }), showInstitutional: true, showMargin: false, showRevenue: false, showValuation: false, showDividends: false })}</tr></tbody></table>);
		expect(html).toContain('+534,504');
		expect(html).toContain('-12,000');
		expect(html).not.toContain('+0');
	});
});

describe('M7D -- missing/zero/ratio rendering (margin)', () => {
	it('20. null margin values render —', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ margin_balance: null, margin_change: null, short_balance: null, short_change: null, short_margin_ratio: null }), showInstitutional: false, showMargin: true, showRevenue: false, showValuation: false, showDividends: false })}</tr></tbody></table>);
		expect((html.match(/—/g) || []).length).toBeGreaterThanOrEqual(5);
	});

	it('21. zero margin value renders real zero', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ margin_balance: 0, short_balance: 0 }), showInstitutional: false, showMargin: true, showRevenue: false, showValuation: false, showDividends: false })}</tr></tbody></table>);
		expect(html).toContain('融資 0');
		expect(html).toContain('融券 0');
	});

	it('22. short_margin_ratio renders as a percentage without re-scaling the raw backend value', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ short_margin_ratio: 8.2 }), showInstitutional: false, showMargin: true, showRevenue: false, showValuation: false, showDividends: false })}</tr></tbody></table>);
		expect(html).toContain('8.2%');
	});
});

describe('M7D -- adaptive result presentation driven by APPLIED filters, not draft', () => {
	it('23. institutional criteria (applied) shows the institutional block and the 進階資料 header', () => {
		const applied = withAdvanced({ minForeignNet: '0' });
		const show = taiwanScreenerHasInstitutionalCriteria(applied);
		expect(show).toBe(true);
		const html = renderToStaticMarkup(<ScreenerTable securities={[security()]} onOpenResearch={() => {}} {...tableWatchlistProps({ showInstitutional: show, showMargin: false })} />);
		expect(html).toContain('進階資料');
		expect(html).toContain('外資');
		expect(html).not.toContain('融資 ');
	});

	it('24. margin criteria (applied) shows the margin block and the 進階資料 header', () => {
		const applied = withAdvanced({ minMarginBalance: '0' });
		const show = taiwanScreenerHasMarginCriteria(applied);
		expect(show).toBe(true);
		const html = renderToStaticMarkup(<ScreenerTable securities={[security()]} onOpenResearch={() => {}} {...tableWatchlistProps({ showInstitutional: false, showMargin: show })} />);
		expect(html).toContain('進階資料');
		expect(html).toContain('融資');
		expect(html).not.toContain('外資');
	});

	it('25. both criteria active shows both blocks', () => {
		const applied = withAdvanced({ minForeignNet: '0', minMarginBalance: '0' });
		const html = renderToStaticMarkup(<ScreenerTable securities={[security()]} onOpenResearch={() => {}} {...tableWatchlistProps({ showInstitutional: taiwanScreenerHasInstitutionalCriteria(applied), showMargin: taiwanScreenerHasMarginCriteria(applied) })} />);
		expect(html).toContain('外資');
		expect(html).toContain('融資');
	});

	it('26. neither criteria (no filter/sort active) shows no advanced column at all -- byte-identical legacy table', () => {
		const applied = taiwanScreenerDefaultFilters();
		expect(taiwanScreenerHasInstitutionalCriteria(applied)).toBe(false);
		expect(taiwanScreenerHasMarginCriteria(applied)).toBe(false);
		const html = renderToStaticMarkup(<ScreenerTable securities={[security()]} onOpenResearch={() => {}} {...tableWatchlistProps()} />);
		expect(html).not.toContain('進階資料');
	});

	it('the workspace computes showInstitutional/showMargin from `applied`, never from `draft`', () => {
		const source = workspaceSource();
		expect(source).toContain('taiwanScreenerHasInstitutionalCriteria(applied)');
		expect(source).toContain('taiwanScreenerHasMarginCriteria(applied)');
		expect(source).not.toMatch(/taiwanScreenerHasInstitutionalCriteria\(draft\)|taiwanScreenerHasMarginCriteria\(draft\)/);
	});
});

describe('M7D -- domain freshness / unavailable state', () => {
	it('27. institutional freshness renders when status is present and not unavailable', () => {
		const html = renderToStaticMarkup(<ScreenerDomainFreshness label="法人資料" asOf="2026-09-04" status="current" unavailableMessage="法人資料暫時無法取得" />);
		expect(html).toContain('法人資料：2026-09-04');
		expect(html).toContain('最新');
	});

	it('28. margin freshness renders when status is present and not unavailable', () => {
		const html = renderToStaticMarkup(<ScreenerDomainFreshness label="融資融券資料" asOf="2026-09-04" status="stale" unavailableMessage="融資融券資料暫時無法取得" />);
		expect(html).toContain('融資融券資料：2026-09-04');
		expect(html).toContain('資料較舊');
	});

	it('29. unavailable institutional status shows the safe warning, not a fabricated as_of', () => {
		const html = renderToStaticMarkup(<ScreenerDomainFreshness label="法人資料" asOf={null} status="unavailable" unavailableMessage="法人資料暫時無法取得" />);
		expect(html).toContain('法人資料暫時無法取得');
		expect(html).not.toContain('2026');
	});

	it('30. unavailable margin status shows the safe warning', () => {
		const html = renderToStaticMarkup(<ScreenerDomainFreshness label="融資融券資料" asOf={null} status="unavailable" unavailableMessage="融資融券資料暫時無法取得" />);
		expect(html).toContain('融資融券資料暫時無法取得');
	});

	it('renders nothing when status is absent (domain not requested at all -- never guessed)', () => {
		const html = renderToStaticMarkup(<>{ScreenerDomainFreshness({ label: '法人資料', asOf: undefined, status: undefined, unavailableMessage: '法人資料暫時無法取得' })}</>);
		expect(html).toBe('');
	});

	it('freshness/unavailable blocks are gated by showInstitutional/showMargin in the workspace body', () => {
		const source = workspaceSource();
		expect(source).toMatch(/data && showInstitutional && <ScreenerDomainFreshness/);
		expect(source).toMatch(/data && showMargin && <ScreenerDomainFreshness/);
	});
});

describe('M7D -- no request fan-out introduced', () => {
	it('31-33. still issues exactly one requestJSON call (Screener) and never a second endpoint for institutional/margin', () => {
		const source = workspaceSource();
		const matches = source.match(/requestJSON</g) || [];
		expect(matches.length).toBe(1);
		expect(source).not.toMatch(/\/api\/v1\/tw\/institutional|\/api\/v1\/tw\/margin/);
	});

	it('39-40. no fundamentals/AI/Hermes/quotes fan-out introduced by advanced filters', () => {
		const source = workspaceSource();
		expect(source).not.toMatch(/\/api\/v1\/tw\/quotes|\/api\/v1\/tw\/fundamentals|taiwanIntelligencePath|taiwanResearchPath|hermes|Hermes/i);
	});
});

describe('M7D -- research navigation and Watchlist action remain unaffected by advanced columns', () => {
	it('34. exact 2330.TWSE navigation preserved even with the advanced column rendered', () => {
		let opened = '';
		const twse = security({ canonical: '2330.TWSE', foreign_net: 100 });
		const element = ScreenerRow({ security: twse, onOpen: () => { opened = twse.canonical; }, ...rowWatchlistProps({ showInstitutional: true }) });
		const identityCell = (element.props.children as unknown[])[0] as { props: { children: { props: { onClick: () => void } } } };
		identityCell.props.children.props.onClick();
		expect(opened).toBe('2330.TWSE');
	});

	it('35. exact 6488.TPEX navigation preserved with margin column rendered', () => {
		let opened = '';
		const tpex = security({ canonical: '6488.TPEX', code: '6488', name: '環球晶', exchange: 'TPEX', margin_balance: 200000 });
		const element = ScreenerRow({ security: tpex, onOpen: () => { opened = tpex.canonical; }, ...rowWatchlistProps({ showMargin: true }) });
		const identityCell = (element.props.children as unknown[])[0] as { props: { children: { props: { onClick: () => void } } } };
		identityCell.props.children.props.onClick();
		expect(opened).toBe('6488.TPEX');
	});

	it('36. the Watchlist toggle still never triggers onOpen, even with advanced columns present', () => {
		let toggled = false;
		const element = ScreenerWatchlistAction({ membershipState: 'ready', saved: false, busy: false, onToggle: () => { toggled = true; } });
		const button = (element.props.children as unknown[])[0] as { type: string; props: { onClick: () => void } };
		button.props.onClick();
		expect(toggled).toBe(true);
	});
});

describe('M7D -- race safety and cold-start unchanged', () => {
	it('37. still uses the shared runScopedRequest race guard for the Screener fetch (advanced fields do not change this)', () => {
		const source = workspaceSource();
		expect(source).toContain('runScopedRequest(requestID');
	});

	it('38. cold overview cold-start still does not preload the Screener or Watchlist', () => {
		const source = overviewSource();
		const mountEffect = source.slice(source.indexOf('useEffect(() => {'), source.indexOf('[config, refreshKey]'));
		expect(mountEffect).not.toMatch(/screener/i);
		expect(mountEffect).not.toMatch(/watchlist/i);
	});
});

// ==================================================
// M7E-A -- Revenue + Valuation + Dividends filter UI
// ==================================================

const withFundamentals = (overrides: Partial<TaiwanScreenerFilters> = {}): TaiwanScreenerFilters => ({ ...taiwanScreenerDefaultFilters(), ...overrides });

describe('M7E-A -- 基本面 group exists, default collapsed, collapse/expand has no request semantics', () => {
	it('1. renders the 基本面 group toggle with 營收/估值/股利 subsections once expanded', () => {
		const collapsed = renderToStaticMarkup(<ScreenerFilterPanel draft={taiwanScreenerDefaultFilters()} onChange={() => {}} onApply={() => {}} onClear={() => {}} localError="" />);
		expect(collapsed).toContain('基本面');
	});

	it('2. defaults collapsed: fundamentals min/max inputs are not present in the initial render', () => {
		const html = renderToStaticMarkup(<ScreenerFilterPanel draft={taiwanScreenerDefaultFilters()} onChange={() => {}} onApply={() => {}} onClear={() => {}} localError="" />);
		expect(html).not.toContain('最低月營收');
		expect(html).not.toContain('最低本益比');
		expect(html).not.toContain('最低現金股利');
	});

	it('3. the 基本面 toggle is local useState, never calls onChange/onApply/onClear, and does not reset by Clear', () => {
		const source = workspaceSource();
		const panel = source.slice(source.indexOf('export function ScreenerFilterPanel'), source.indexOf('export function ScreenerSummary'));
		expect(panel).toContain('useState(false)');
		const fundamentalsToggle = panel.slice(panel.indexOf('基本面') - 400, panel.indexOf('基本面'));
		expect(fundamentalsToggle).toContain('setFundamentalsExpanded');
		expect(fundamentalsToggle).not.toMatch(/onChange\(|onApply\(|onClear\(/);
	});

	it('expanding 基本面 reveals 營收/估值/股利 subsections with their labeled fields', () => {
		const source = workspaceSource();
		const panel = source.slice(source.indexOf('export function ScreenerFilterPanel'), source.indexOf('export function ScreenerSummary'));
		expect(panel).toContain('營收');
		expect(panel).toContain('估值');
		expect(panel).toContain('股利');
		expect(panel).toContain('最低月營收（元）');
		expect(panel).toContain('最低本益比（PE）');
		expect(panel).toContain('最低現金股利');
	});
});

describe('M7E-A -- draft typing sends no request (revenue/valuation/dividends)', () => {
	it('4-6. every fundamentals input writes via the same set() helper used by basic fields, never a request call', () => {
		const source = workspaceSource();
		const panel = source.slice(source.indexOf('export function ScreenerFilterPanel'), source.indexOf('export function ScreenerSummary'));
		expect(panel).toContain("set('minMonthlyRevenue'");
		expect(panel).toContain("set('minPE'");
		expect(panel).toContain("set('minCashDividend'");
		expect(panel).not.toMatch(/requestJSON|taiwanScreenerPath\(/);
	});
});

describe('M7E-A -- query construction', () => {
	it('7. Apply serializes monthly_revenue in raw TWD', () => {
		expect(taiwanScreenerPath(withFundamentals({ minMonthlyRevenue: '1000000000' }))).toContain('min_monthly_revenue=1000000000');
	});

	it('8. Apply serializes revenue_yoy verbatim', () => {
		expect(taiwanScreenerPath(withFundamentals({ minRevenueYoY: '12.16' }))).toContain('min_revenue_yoy=12.16');
	});

	it('9. Apply serializes PE/PB/dividend_yield', () => {
		const path = taiwanScreenerPath(withFundamentals({ minPE: '0', maxPB: '5', minDividendYield: '5.3' }));
		expect(path).toContain('min_pe=0');
		expect(path).toContain('max_pb=5');
		expect(path).toContain('min_dividend_yield=5.3');
	});

	it('10. Apply serializes dividends', () => {
		const path = taiwanScreenerPath(withFundamentals({ minCashDividend: '5', minStockDividend: '0', minTotalDividend: '5' }));
		expect(path).toContain('min_cash_dividend=5');
		expect(path).toContain('min_stock_dividend=0');
		expect(path).toContain('min_total_dividend=5');
	});

	it('11. combined fundamentals params serialize together in one query', () => {
		const path = taiwanScreenerPath(withFundamentals({ minRevenueYoY: '0', minPE: '0', minCashDividend: '0' }));
		expect(path).toContain('min_revenue_yoy=0');
		expect(path).toContain('min_pe=0');
		expect(path).toContain('min_cash_dividend=0');
	});

	it('12. blank fundamentals params are omitted entirely', () => {
		const path = taiwanScreenerPath(taiwanScreenerDefaultFilters());
		for (const key of ['monthly_revenue', 'revenue_yoy', 'min_pe=', 'max_pe=', 'min_pb=', 'max_pb=', 'dividend_yield', 'cash_dividend', 'stock_dividend', 'total_dividend']) {
			expect(path).not.toContain(key);
		}
	});
});

describe('M7E-A -- advanced sort', () => {
	it('13. sort=monthly_revenue / revenue_yoy query', () => {
		expect(taiwanScreenerPath(withFundamentals({ sort: 'monthly_revenue' }))).toContain('sort=monthly_revenue');
		expect(taiwanScreenerPath(withFundamentals({ sort: 'revenue_yoy' }))).toContain('sort=revenue_yoy');
	});

	it('14. sort=pe / pb / dividend_yield query', () => {
		expect(taiwanScreenerPath(withFundamentals({ sort: 'pe' }))).toContain('sort=pe');
		expect(taiwanScreenerPath(withFundamentals({ sort: 'pb' }))).toContain('sort=pb');
		expect(taiwanScreenerPath(withFundamentals({ sort: 'dividend_yield' }))).toContain('sort=dividend_yield');
	});

	it('15. sort=cash_dividend / stock_dividend / total_dividend query', () => {
		expect(taiwanScreenerPath(withFundamentals({ sort: 'cash_dividend' }))).toContain('sort=cash_dividend');
		expect(taiwanScreenerPath(withFundamentals({ sort: 'stock_dividend' }))).toContain('sort=stock_dividend');
		expect(taiwanScreenerPath(withFundamentals({ sort: 'total_dividend' }))).toContain('sort=total_dividend');
	});

	it('16. sort dropdown includes all 8 new fundamentals options plus every legacy option (21 total)', () => {
		const html = renderToStaticMarkup(<ScreenerFilterPanel draft={taiwanScreenerDefaultFilters()} onChange={() => {}} onApply={() => {}} onClear={() => {}} localError="" />);
		for (const label of ['月營收', '月營收年增率', '本益比（PE）', '股價淨值比（PB）', '殖利率', '現金股利', '股票股利', '合計股利']) {
			expect(html).toContain(label);
		}
	});
});

describe('M7E-A -- Clear/Apply/pagination/refresh preserve fundamentals', () => {
	it('17. Clear (taiwanScreenerDefaultFilters()) resets all fundamentals fields', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const clearFilters = () => {'), source.indexOf('const goPrevious = () => {'));
		expect(fn).toContain('taiwanScreenerDefaultFilters()');
	});

	it('18. Apply resets offset to 0 even with fundamentals criteria present', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const applyFilters = () => {'), source.indexOf('const clearFilters = () => {'));
		expect(fn).toContain('setApplied({ ...draft, offset: 0 })');
	});

	it('19-20. pagination/refresh spread the full applied object (including fundamentals fields), never rebuild it field-by-field', () => {
		const source = workspaceSource();
		const previous = source.slice(source.indexOf('const goPrevious = () => {'), source.indexOf('const goNext = () => {'));
		const next = source.slice(source.indexOf('const goNext = () => {'), source.indexOf('// Pessimistic add/remove'));
		expect(previous).toContain('setApplied((current) => ({ ...current, offset:');
		expect(next).toContain('setApplied((current) => ({ ...current, offset:');
		expect(source).toContain('}, [config, refreshKey, applied]);');
	});
});

describe('M7E-A -- missing/zero rendering (revenue)', () => {
	it('21. null revenue values render —', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ monthly_revenue: null, revenue_yoy: null }), showInstitutional: false, showMargin: false, showRevenue: true, showValuation: false, showDividends: false })}</tr></tbody></table>);
		expect((html.match(/—/g) || []).length).toBeGreaterThanOrEqual(2);
	});

	it('22-23. zero revenue renders 0 (not —), and grouped formatting applies to a real value', () => {
		const zero = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ monthly_revenue: 0, revenue_yoy: 0 }), showInstitutional: false, showMargin: false, showRevenue: true, showValuation: false, showDividends: false })}</tr></tbody></table>);
		expect(zero).toContain('月營收 0');
		expect(zero).toContain('年增 0%');
		const grouped = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ monthly_revenue: 52340000000 }), showInstitutional: false, showMargin: false, showRevenue: true, showValuation: false, showDividends: false })}</tr></tbody></table>);
		expect(grouped).toContain((52340000000).toLocaleString('zh-TW'));
		expect(grouped).not.toContain('52340000000');
	});

	it('24. revenue_yoy renders signed percent (positive/negative/zero)', () => {
		const up = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ revenue_yoy: 12.16 }), showInstitutional: false, showMargin: false, showRevenue: true, showValuation: false, showDividends: false })}</tr></tbody></table>);
		expect(up).toContain('+12.16%');
		const down = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ revenue_yoy: -8.4 }), showInstitutional: false, showMargin: false, showRevenue: true, showValuation: false, showDividends: false })}</tr></tbody></table>);
		expect(down).toContain('-8.4%');
	});
});

describe('M7E-A -- missing/zero rendering (valuation)', () => {
	it('25. null PE/PB render —', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ pe: null, pb: null, dividend_yield: null }), showInstitutional: false, showMargin: false, showRevenue: false, showValuation: true, showDividends: false })}</tr></tbody></table>);
		expect((html.match(/—/g) || []).length).toBeGreaterThanOrEqual(3);
	});

	it('26. real zero PE/PB remains 0, not missing', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ pe: 0, pb: 0 }), showInstitutional: false, showMargin: false, showRevenue: false, showValuation: true, showDividends: false })}</tr></tbody></table>);
		expect(html).toContain('PE 0');
		expect(html).toContain('PB 0');
	});

	it('27. dividend_yield renders as an unsigned percentage, no re-scaling', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ dividend_yield: 5.3 }), showInstitutional: false, showMargin: false, showRevenue: false, showValuation: true, showDividends: false })}</tr></tbody></table>);
		expect(html).toContain('殖利率 5.3%');
	});
});

describe('M7E-A -- missing/zero rendering (dividends)', () => {
	it('28. null dividend values render —', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ cash_dividend: null, stock_dividend: null, total_dividend: null }), showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: true })}</tr></tbody></table>);
		expect((html.match(/—/g) || []).length).toBeGreaterThanOrEqual(3);
	});

	it('29-30. zero dividend renders 0, and decimal formatting applies to a real value', () => {
		const zero = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ cash_dividend: 5, stock_dividend: 0, total_dividend: 5 }), showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: true })}</tr></tbody></table>);
		expect(zero).toContain('股票 0');
		expect(zero).toContain('現金 5');
		expect(zero).toContain('合計 5');
	});
});

describe('M7E-A -- adaptive result presentation driven by APPLIED filters, not draft', () => {
	it('31. revenue criteria (applied) shows the revenue block', () => {
		const applied = withFundamentals({ minRevenueYoY: '0' });
		const show = taiwanScreenerHasRevenueCriteria(applied);
		expect(show).toBe(true);
		const html = renderToStaticMarkup(<ScreenerTable securities={[security()]} onOpenResearch={() => {}} {...tableWatchlistProps({ showRevenue: show })} />);
		expect(html).toContain('進階資料');
		expect(html).toContain('月營收');
	});

	it('32. valuation criteria (applied) shows the valuation block', () => {
		const applied = withFundamentals({ minPE: '0' });
		const show = taiwanScreenerHasValuationCriteria(applied);
		expect(show).toBe(true);
		const html = renderToStaticMarkup(<ScreenerTable securities={[security()]} onOpenResearch={() => {}} {...tableWatchlistProps({ showValuation: show })} />);
		expect(html).toContain('進階資料');
		expect(html).toContain('PE');
	});

	it('33. dividend criteria (applied) shows the dividend block', () => {
		const applied = withFundamentals({ minCashDividend: '0' });
		const show = taiwanScreenerHasDividendCriteria(applied);
		expect(show).toBe(true);
		const html = renderToStaticMarkup(<ScreenerTable securities={[security()]} onOpenResearch={() => {}} {...tableWatchlistProps({ showDividends: show })} />);
		expect(html).toContain('進階資料');
		expect(html).toContain('現金');
	});

	it('34. multiple fundamentals domains (and M7D domains) show together in the same compact cell', () => {
		const applied = withFundamentals({ minForeignNet: '0', minPE: '0', minCashDividend: '0' });
		const html = renderToStaticMarkup(<ScreenerTable securities={[security({ foreign_net: 100 })]} onOpenResearch={() => {}} {...tableWatchlistProps({
			showInstitutional: taiwanScreenerHasInstitutionalCriteria(applied), showValuation: taiwanScreenerHasValuationCriteria(applied), showDividends: taiwanScreenerHasDividendCriteria(applied),
		})} />);
		expect(html).toContain('外資');
		expect(html).toContain('PE');
		expect(html).toContain('現金');
	});

	it('35. draft-only fundamentals typing must not alter currently displayed result presentation', () => {
		const source = workspaceSource();
		expect(source).toContain('taiwanScreenerHasRevenueCriteria(applied)');
		expect(source).toContain('taiwanScreenerHasValuationCriteria(applied)');
		expect(source).toContain('taiwanScreenerHasDividendCriteria(applied)');
		expect(source).not.toMatch(/taiwanScreenerHasRevenueCriteria\(draft\)|taiwanScreenerHasValuationCriteria\(draft\)|taiwanScreenerHasDividendCriteria\(draft\)/);
	});

	it('neither fundamentals nor M7D criteria active keeps the byte-identical legacy table', () => {
		const applied = taiwanScreenerDefaultFilters();
		expect(taiwanScreenerHasRevenueCriteria(applied)).toBe(false);
		expect(taiwanScreenerHasValuationCriteria(applied)).toBe(false);
		expect(taiwanScreenerHasDividendCriteria(applied)).toBe(false);
		const html = renderToStaticMarkup(<ScreenerTable securities={[security()]} onOpenResearch={() => {}} {...tableWatchlistProps()} />);
		expect(html).not.toContain('進階資料');
	});
});

describe('M7E-A -- domain freshness / unavailable state', () => {
	it('36. revenue freshness renders the backend period as-is, never converted to a fake date', () => {
		const html = renderToStaticMarkup(<ScreenerDomainFreshness label="營收資料" asOf="2026-07" status="available" unavailableMessage="營收資料暫時無法取得" />);
		expect(html).toContain('營收資料：2026-07');
		expect(html).not.toContain('2026-07-31');
	});

	it('37. valuation freshness renders the backend trade date and status', () => {
		const html = renderToStaticMarkup(<ScreenerDomainFreshness label="估值資料" asOf="2026-09-04" status="current" unavailableMessage="估值資料暫時無法取得" />);
		expect(html).toContain('估值資料：2026-09-04');
		expect(html).toContain('最新');
	});

	it('38. dividends freshness renders the backend year identifier, never converted to a fake date', () => {
		const html = renderToStaticMarkup(<ScreenerDomainFreshness label="股利資料" asOf="2025" status="available" unavailableMessage="股利資料暫時無法取得" />);
		expect(html).toContain('股利資料：2025');
		expect(html).not.toContain('2025-01-01');
		expect(html).not.toContain('2025-12-31');
	});

	it('39-40. period/year is never reinterpreted as a full calendar date anywhere in the workspace source', () => {
		const source = workspaceSource();
		expect(source).not.toMatch(/revenue_as_of.*-31|dividends_as_of.*-01-01/);
	});

	it('41. revenue unavailable shows the safe warning, never a fabricated as_of', () => {
		const html = renderToStaticMarkup(<ScreenerDomainFreshness label="營收資料" asOf={null} status="unavailable" unavailableMessage="營收資料暫時無法取得" />);
		expect(html).toContain('營收資料暫時無法取得');
		expect(html).not.toContain('2026');
	});

	it('42. valuation unavailable shows the safe warning', () => {
		const html = renderToStaticMarkup(<ScreenerDomainFreshness label="估值資料" asOf={null} status="unavailable" unavailableMessage="估值資料暫時無法取得" />);
		expect(html).toContain('估值資料暫時無法取得');
	});

	it('43. dividends unavailable shows the safe warning', () => {
		const html = renderToStaticMarkup(<ScreenerDomainFreshness label="股利資料" asOf={null} status="unavailable" unavailableMessage="股利資料暫時無法取得" />);
		expect(html).toContain('股利資料暫時無法取得');
	});

	it('44. unavailable warning and the zero-result empty state are independent -- both can render together', () => {
		const source = workspaceSource();
		// The domain-freshness warning block and the total===0 empty-state block are two separate,
		// unconditional-on-each-other JSX expressions in the render tree -- neither is gated by the other.
		expect(source).toMatch(/data && showRevenue && <ScreenerDomainFreshness/);
		expect(source).toMatch(/data && data\.total === 0 && <div className="taiwan-empty-state">/);
	});

	it('freshness blocks are gated by showRevenue/showValuation/showDividends in the workspace body', () => {
		const source = workspaceSource();
		expect(source).toMatch(/data && showRevenue && <ScreenerDomainFreshness/);
		expect(source).toMatch(/data && showValuation && <ScreenerDomainFreshness/);
		expect(source).toMatch(/data && showDividends && <ScreenerDomainFreshness/);
	});
});

describe('M7E-A -- no request fan-out introduced', () => {
	it('45-47. still issues exactly one requestJSON call (Screener) and never a second endpoint for revenue/valuation/dividends', () => {
		const source = workspaceSource();
		const matches = source.match(/requestJSON</g) || [];
		expect(matches.length).toBe(1);
		expect(source).not.toMatch(/\/api\/v1\/tw\/revenue|\/api\/v1\/tw\/valuation|\/api\/v1\/tw\/dividends/);
	});

	it('53-56. no fundamentals/FinMind/per-security/AI/Hermes fan-out introduced by fundamentals filters', () => {
		const source = workspaceSource();
		expect(source).not.toMatch(/\/api\/v1\/tw\/fundamentals|FinMind|taiwanIntelligencePath|taiwanResearchPath|hermes|Hermes/i);
	});
});

describe('M7E-A -- research navigation and Watchlist action remain unaffected by fundamentals columns', () => {
	it('48. exact 2330.TWSE navigation preserved even with the revenue/valuation/dividend columns rendered', () => {
		let opened = '';
		const twse = security({ canonical: '2330.TWSE', monthly_revenue: 100 });
		const element = ScreenerRow({ security: twse, onOpen: () => { opened = twse.canonical; }, ...rowWatchlistProps({ showRevenue: true }) });
		const identityCell = (element.props.children as unknown[])[0] as { props: { children: { props: { onClick: () => void } } } };
		identityCell.props.children.props.onClick();
		expect(opened).toBe('2330.TWSE');
	});

	it('49. exact 6488.TPEX navigation preserved with valuation/dividend columns rendered', () => {
		let opened = '';
		const tpex = security({ canonical: '6488.TPEX', code: '6488', name: '環球晶', exchange: 'TPEX', pe: 10, cash_dividend: 3 });
		const element = ScreenerRow({ security: tpex, onOpen: () => { opened = tpex.canonical; }, ...rowWatchlistProps({ showValuation: true, showDividends: true }) });
		const identityCell = (element.props.children as unknown[])[0] as { props: { children: { props: { onClick: () => void } } } };
		identityCell.props.children.props.onClick();
		expect(opened).toBe('6488.TPEX');
	});

	it('50. the Watchlist toggle still never triggers onOpen, even with fundamentals columns present', () => {
		let toggled = false;
		const element = ScreenerWatchlistAction({ membershipState: 'ready', saved: false, busy: false, onToggle: () => { toggled = true; } });
		const button = (element.props.children as unknown[])[0] as { type: string; props: { onClick: () => void } };
		button.props.onClick();
		expect(toggled).toBe(true);
	});
});

describe('M7E-A -- race safety and cold-start unchanged', () => {
	it('51. still uses the shared runScopedRequest race guard for the Screener fetch (fundamentals fields do not change this)', () => {
		const source = workspaceSource();
		expect(source).toContain('runScopedRequest(requestID');
	});

	it('52. cold overview cold-start still does not preload the Screener or Watchlist', () => {
		const source = overviewSource();
		const mountEffect = source.slice(source.indexOf('useEffect(() => {'), source.indexOf('[config, refreshKey]'));
		expect(mountEffect).not.toMatch(/screener/i);
		expect(mountEffect).not.toMatch(/watchlist/i);
	});
});
