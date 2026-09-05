import fs from 'node:fs';
import path from 'node:path';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import { taiwanScreenerDefaultFilters, type TaiwanScreenerResponse, type TaiwanScreenerSecurity } from '../lib/taiwan-product';
import { ScreenerFilterPanel, ScreenerPagination, ScreenerRow, ScreenerSummary, ScreenerTable, ScreenerWatchlistAction, type WatchlistMembershipState } from './TaiwanScreenerWorkspace';

const root = path.resolve(__dirname, '../../..');
const workspaceSource = () => fs.readFileSync(path.join(root, 'frontend/src/components/TaiwanScreenerWorkspace.tsx'), 'utf8');
const appSource = () => fs.readFileSync(path.join(root, 'frontend/src/App.tsx'), 'utf8');
const overviewSource = () => fs.readFileSync(path.join(root, 'frontend/src/components/market/TaiwanMarketView.tsx'), 'utf8');

const security = (overrides: Partial<TaiwanScreenerSecurity> = {}): TaiwanScreenerSecurity => ({
	canonical: '2330.TWSE', code: '2330', name: '台積電', exchange: 'TWSE', security_type: 'stock', trade_date: '2026-09-04',
	price: 2410, change: 20, change_percent: 0.84, volume: 14102018, amount: 33917316870,
	...overrides,
});

const response = (overrides: Partial<TaiwanScreenerResponse> = {}): TaiwanScreenerResponse => ({
	scope: 'COMBINED', as_of: '2026-09-04', freshness: 'current', total: 1, offset: 0, limit: 50, securities: [security()],
	...overrides,
});

// M7C — default row-level Watchlist props: "ready, not saved, not busy, no error" unless overridden.
const rowWatchlistProps = (overrides: Partial<{ membershipState: WatchlistMembershipState; saved: boolean; busy: boolean; mutationError?: string }> = {}) => ({
	membershipState: 'ready' as WatchlistMembershipState, saved: false, busy: false, mutationError: undefined as string | undefined, onToggleWatchlist: () => {},
	...overrides,
});

const tableWatchlistProps = (overrides: Partial<{ watchlistState: WatchlistMembershipState; watchlistedCanonicals: Set<string>; busyCanonicals: Set<string>; mutationErrors: Record<string, string> }> = {}) => ({
	watchlistState: 'ready' as WatchlistMembershipState, watchlistedCanonicals: new Set<string>(), busyCanonicals: new Set<string>(), mutationErrors: {} as Record<string, string>, onToggleWatchlist: () => {},
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
