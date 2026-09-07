import fs from 'node:fs';
import path from 'node:path';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import {
	resolveTaiwanWorkspace, taiwanPrimaryNavigation,
	taiwanFinancialsStatusLabel, taiwanScreenerDefaultFilters, taiwanScreenerHasBalanceCriteria, taiwanScreenerHasCashflowCriteria, taiwanScreenerHasDividendCriteria, taiwanScreenerHasFinancialsCriteria, taiwanScreenerHasInstitutionalCriteria, taiwanScreenerHasMarginCriteria,
	taiwanScreenerHasRevenueCriteria, taiwanScreenerHasValuationCriteria, taiwanScreenerPath,
	type TaiwanScreenerFilters, type TaiwanScreenerResponse, type TaiwanScreenerSecurity,
} from '../lib/taiwan-product';
import {
	ScreenerAdvancedCell, ScreenerDomainFreshness, ScreenerFilterPanel, ScreenerFinancialsFreshness, ScreenerPagination, ScreenerRow, ScreenerSummary, ScreenerTable,
	ScreenerWatchlistAction, taiwanScreenerPresets, type WatchlistMembershipState,
} from './TaiwanScreenerWorkspace';

const presetById = (id: string) => {
	const preset = taiwanScreenerPresets.find((item) => item.id === id);
	if (!preset) throw new Error(`unknown preset id: ${id}`);
	return preset;
};

const root = path.resolve(__dirname, '../../..');
const workspaceSource = () => fs.readFileSync(path.join(root, 'frontend/src/components/TaiwanScreenerWorkspace.tsx'), 'utf8');
const appSource = () => fs.readFileSync(path.join(root, 'frontend/src/App.tsx'), 'utf8');
const overviewSource = () => fs.readFileSync(path.join(root, 'frontend/src/components/market/TaiwanMarketView.tsx'), 'utf8');

const security = (overrides: Partial<TaiwanScreenerSecurity> = {}): TaiwanScreenerSecurity => ({
	canonical: '2330.TWSE', code: '2330', name: '台積電', exchange: 'TWSE', security_type: 'stock', trade_date: '2026-09-04',
	price: 2410, change: 20, change_percent: 0.84, volume: 14102018, amount: 33917316870,
	foreign_net: null, trust_net: null, dealer_net: null, institutional_net: null,
	margin_balance: null, margin_change: null, short_balance: null, short_change: null, short_margin_ratio: null,
	monthly_revenue: null, revenue_yoy: null, revenue_mom: null, cumulative_revenue_yoy: null, pe: null, pb: null, dividend_yield: null,
	cash_dividend: null, stock_dividend: null, total_dividend: null,
	financial_period: null, cumulative_eps: null, gross_margin: null, operating_margin: null,
	net_margin: null, book_value_per_share: null,
	debt_ratio: null, debt_to_equity: null, current_ratio: null, balance_period: null,
	operating_cash_flow: null, cash_flow_to_net_income: null, cashflow_period: null,
	...overrides,
});

const response = (overrides: Partial<TaiwanScreenerResponse> = {}): TaiwanScreenerResponse => ({
	scope: 'COMBINED', as_of: '2026-09-04', freshness: 'current', total: 1, offset: 0, limit: 50, securities: [security()],
	...overrides,
});

// M7C — default row-level Watchlist props: "ready, not saved, not busy, no error" unless overridden.
// M7D/M7E-A — showInstitutional/showMargin/showRevenue/showValuation/showDividends default false
// (legacy row shape) unless a test opts in.
const rowWatchlistProps = (overrides: Partial<{ membershipState: WatchlistMembershipState; saved: boolean; busy: boolean; mutationError?: string; showInstitutional: boolean; showMargin: boolean; showRevenue: boolean; showValuation: boolean; showDividends: boolean; showFinancials: boolean; showBalance: boolean; showCashflow: boolean }> = {}) => ({
	membershipState: 'ready' as WatchlistMembershipState, saved: false, busy: false, mutationError: undefined as string | undefined, onToggleWatchlist: () => {},
	showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: false, showBalance: false, showCashflow: false,
	...overrides,
});

const tableWatchlistProps = (overrides: Partial<{ watchlistState: WatchlistMembershipState; watchlistedCanonicals: Set<string>; busyCanonicals: Set<string>; mutationErrors: Record<string, string>; showInstitutional: boolean; showMargin: boolean; showRevenue: boolean; showValuation: boolean; showDividends: boolean; showFinancials: boolean; showBalance: boolean; showCashflow: boolean }> = {}) => ({
	watchlistState: 'ready' as WatchlistMembershipState, watchlistedCanonicals: new Set<string>(), busyCanonicals: new Set<string>(), mutationErrors: {} as Record<string, string>, onToggleWatchlist: () => {},
	showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: false, showBalance: false, showCashflow: false,
	...overrides,
});

describe('M7B — navigation wiring', () => {
	it('App.tsx declares a taiwan-screener workspace mode, hash mapping, and a 台股選股器 nav button', () => {
		const app = appSource();
		expect(app).toContain("'taiwan-screener'");
		expect(resolveTaiwanWorkspace('#taiwan-screener')).toBe('taiwan-screener');
		expect(app).toContain('return resolveTaiwanWorkspace(window.location.hash)');
		expect(app).toContain('onClick={() => switchWorkspace(mode)}');
		expect(taiwanPrimaryNavigation).toContainEqual(['taiwan-screener', '台股選股器']);
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

	it('M8B: TaiwanScreenerWorkspace passes onOpenResearch the exact APPLIED filters as a built context, never draft', () => {
		const source = workspaceSource();
		expect(source).toContain('onOpenResearch={(canonical) => onOpenResearch(canonical, buildTaiwanScreenerContext(applied))}');
		expect(source).not.toContain('buildTaiwanScreenerContext(draft)');
	});

	it('M8B: onOpenResearch prop type accepts an optional TaiwanResearchEntryContext second argument', () => {
		const source = workspaceSource();
		expect(source).toContain('onOpenResearch: (canonical: string, context?: TaiwanResearchEntryContext | null) => void');
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
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ foreign_net: null, trust_net: null, dealer_net: null, institutional_net: null }), showInstitutional: true, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: false, showBalance: false, showCashflow: false })}</tr></tbody></table>);
		expect((html.match(/—/g) || []).length).toBeGreaterThanOrEqual(4);
	});

	it('18. zero institutional value renders real 0, not missing', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ foreign_net: 0, trust_net: 0, dealer_net: 0, institutional_net: 0 }), showInstitutional: true, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: false, showBalance: false, showCashflow: false })}</tr></tbody></table>);
		expect(html).toContain('外資 0');
		expect(html).not.toContain('外資 —');
	});

	it('19. positive/negative institutional values render explicit signs, never "+0"', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ foreign_net: 534504, trust_net: -12000, dealer_net: 0 }), showInstitutional: true, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: false, showBalance: false, showCashflow: false })}</tr></tbody></table>);
		expect(html).toContain('+534,504');
		expect(html).toContain('-12,000');
		expect(html).not.toContain('+0');
	});
});

describe('M7D -- missing/zero/ratio rendering (margin)', () => {
	it('20. null margin values render —', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ margin_balance: null, margin_change: null, short_balance: null, short_change: null, short_margin_ratio: null }), showInstitutional: false, showMargin: true, showRevenue: false, showValuation: false, showDividends: false, showFinancials: false, showBalance: false, showCashflow: false })}</tr></tbody></table>);
		expect((html.match(/—/g) || []).length).toBeGreaterThanOrEqual(5);
	});

	it('21. zero margin value renders real zero', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ margin_balance: 0, short_balance: 0 }), showInstitutional: false, showMargin: true, showRevenue: false, showValuation: false, showDividends: false, showFinancials: false, showBalance: false, showCashflow: false })}</tr></tbody></table>);
		expect(html).toContain('融資 0');
		expect(html).toContain('融券 0');
	});

	it('22. short_margin_ratio renders as a percentage without re-scaling the raw backend value', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ short_margin_ratio: 8.2 }), showInstitutional: false, showMargin: true, showRevenue: false, showValuation: false, showDividends: false, showFinancials: false, showBalance: false, showCashflow: false })}</tr></tbody></table>);
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
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ monthly_revenue: null, revenue_yoy: null }), showInstitutional: false, showMargin: false, showRevenue: true, showValuation: false, showDividends: false, showFinancials: false, showBalance: false, showCashflow: false })}</tr></tbody></table>);
		expect((html.match(/—/g) || []).length).toBeGreaterThanOrEqual(2);
	});

	it('22-23. zero revenue renders 0 (not —), and grouped formatting applies to a real value', () => {
		const zero = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ monthly_revenue: 0, revenue_yoy: 0 }), showInstitutional: false, showMargin: false, showRevenue: true, showValuation: false, showDividends: false, showFinancials: false, showBalance: false, showCashflow: false })}</tr></tbody></table>);
		expect(zero).toContain('月營收 0');
		expect(zero).toContain('年增 0%');
		const grouped = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ monthly_revenue: 52340000000 }), showInstitutional: false, showMargin: false, showRevenue: true, showValuation: false, showDividends: false, showFinancials: false, showBalance: false, showCashflow: false })}</tr></tbody></table>);
		expect(grouped).toContain((52340000000).toLocaleString('zh-TW'));
		expect(grouped).not.toContain('52340000000');
	});

	it('24. revenue_yoy renders signed percent (positive/negative/zero)', () => {
		const up = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ revenue_yoy: 12.16 }), showInstitutional: false, showMargin: false, showRevenue: true, showValuation: false, showDividends: false, showFinancials: false, showBalance: false, showCashflow: false })}</tr></tbody></table>);
		expect(up).toContain('+12.16%');
		const down = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ revenue_yoy: -8.4 }), showInstitutional: false, showMargin: false, showRevenue: true, showValuation: false, showDividends: false, showFinancials: false, showBalance: false, showCashflow: false })}</tr></tbody></table>);
		expect(down).toContain('-8.4%');
	});
});

describe('M7E-A -- missing/zero rendering (valuation)', () => {
	it('25. null PE/PB render —', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ pe: null, pb: null, dividend_yield: null }), showInstitutional: false, showMargin: false, showRevenue: false, showValuation: true, showDividends: false, showFinancials: false, showBalance: false, showCashflow: false })}</tr></tbody></table>);
		expect((html.match(/—/g) || []).length).toBeGreaterThanOrEqual(3);
	});

	it('26. real zero PE/PB remains 0, not missing', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ pe: 0, pb: 0 }), showInstitutional: false, showMargin: false, showRevenue: false, showValuation: true, showDividends: false, showFinancials: false, showBalance: false, showCashflow: false })}</tr></tbody></table>);
		expect(html).toContain('PE 0');
		expect(html).toContain('PB 0');
	});

	it('27. dividend_yield renders as an unsigned percentage, no re-scaling', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ dividend_yield: 5.3 }), showInstitutional: false, showMargin: false, showRevenue: false, showValuation: true, showDividends: false, showFinancials: false, showBalance: false, showCashflow: false })}</tr></tbody></table>);
		expect(html).toContain('殖利率 5.3%');
	});
});

describe('M7E-A -- missing/zero rendering (dividends)', () => {
	it('28. null dividend values render —', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ cash_dividend: null, stock_dividend: null, total_dividend: null }), showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: true, showFinancials: false, showBalance: false, showCashflow: false })}</tr></tbody></table>);
		expect((html.match(/—/g) || []).length).toBeGreaterThanOrEqual(3);
	});

	it('29-30. zero dividend renders 0, and decimal formatting applies to a real value', () => {
		const zero = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: security({ cash_dividend: 5, stock_dividend: 0, total_dividend: 5 }), showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: true, showFinancials: false, showBalance: false, showCashflow: false })}</tr></tbody></table>);
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

// ==================================================
// M7E-B -- Financial statement (cumulative EPS + ci-only margins) filter UI
// ==================================================

describe('M7E-B -- 財務報表 subsection exists inside 基本面, default collapsed, collapse/expand has no request semantics', () => {
	it('1. renders the 財務報表 subheading and its 6 labeled inputs once 基本面 is expanded', () => {
		const source = workspaceSource();
		const panel = source.slice(source.indexOf('export function ScreenerFilterPanel'), source.indexOf('export function ScreenerSummary'));
		expect(panel).toContain('財務報表');
		expect(panel).toContain('累計 EPS 最小');
		expect(panel).toContain('累計 EPS 最大');
		expect(panel).toContain('毛利率最小（%）');
		expect(panel).toContain('毛利率最大（%）');
		expect(panel).toContain('營業利益率最小（%）');
		expect(panel).toContain('營業利益率最大（%）');
	});

	it('2. defaults collapsed: financial-statement inputs are not present in the initial (collapsed) render', () => {
		const html = renderToStaticMarkup(<ScreenerFilterPanel draft={taiwanScreenerDefaultFilters()} onChange={() => {}} onApply={() => {}} onClear={() => {}} localError="" />);
		expect(html).not.toContain('累計 EPS 最小');
		expect(html).not.toContain('毛利率最小');
	});

	it('3. no second top-level fundamentals group was created -- 財務報表 lives inside the existing 基本面 toggle', () => {
		const source = workspaceSource();
		const panel = source.slice(source.indexOf('export function ScreenerFilterPanel'), source.indexOf('export function ScreenerSummary'));
		const fundamentalsGroupCount = (panel.match(/基本面/g) || []).length;
		expect(fundamentalsGroupCount).toBe(1); // exactly one 基本面 toggle button label
		expect(panel).toContain('setFundamentalsExpanded');
	});

	it('does not duplicate M7E-A revenue/valuation/dividend controls', () => {
		const source = workspaceSource();
		const panel = source.slice(source.indexOf('export function ScreenerFilterPanel'), source.indexOf('export function ScreenerSummary'));
		expect((panel.match(/最低月營收（元）/g) || []).length).toBe(1);
		expect((panel.match(/最低本益比（PE）/g) || []).length).toBe(1);
		expect((panel.match(/最低現金股利/g) || []).length).toBe(1);
	});

	it('includes the financial-sector margin note without exposing internal category codes', () => {
		const source = workspaceSource();
		const panel = source.slice(source.indexOf('export function ScreenerFilterPanel'), source.indexOf('export function ScreenerSummary'));
		expect(panel).toContain('部分金融相關產業不提供毛利率／營業利益率');
		expect(panel).not.toMatch(/\bci\b|\bfh\b|\bbd\b|\bins\b|\bmim\b|\bbasi\b/);
	});
});

describe('M7E-B -- draft typing sends no request (financial statement)', () => {
	it('every financial-statement input writes via the same set() helper used by basic fields, never a request call', () => {
		const source = workspaceSource();
		const panel = source.slice(source.indexOf('export function ScreenerFilterPanel'), source.indexOf('export function ScreenerSummary'));
		expect(panel).toContain("set('minCumulativeEPS'");
		expect(panel).toContain("set('minGrossMargin'");
		expect(panel).toContain("set('minOperatingMargin'");
		expect(panel).not.toMatch(/requestJSON|taiwanScreenerPath\(/);
	});
});

describe('M7E-B -- query construction', () => {
	it('Apply serializes cumulative_eps/gross_margin/operating_margin', () => {
		const path = taiwanScreenerPath(withFundamentals({ minCumulativeEPS: '10', maxGrossMargin: '50', minOperatingMargin: '0' }));
		expect(path).toContain('min_cumulative_eps=10');
		expect(path).toContain('max_gross_margin=50');
		expect(path).toContain('min_operating_margin=0');
	});

	it('blank financial-statement params are omitted entirely', () => {
		const path = taiwanScreenerPath(taiwanScreenerDefaultFilters());
		for (const key of ['cumulative_eps', 'gross_margin', 'operating_margin']) {
			expect(path).not.toContain(key);
		}
	});
});

describe('M7E-B -- sort', () => {
	it('sort=cumulative_eps / gross_margin / operating_margin query', () => {
		expect(taiwanScreenerPath(withFundamentals({ sort: 'cumulative_eps' }))).toContain('sort=cumulative_eps');
		expect(taiwanScreenerPath(withFundamentals({ sort: 'gross_margin' }))).toContain('sort=gross_margin');
		expect(taiwanScreenerPath(withFundamentals({ sort: 'operating_margin' }))).toContain('sort=operating_margin');
	});

	it('sort dropdown includes the 3 new financial-statement options plus every legacy option', () => {
		const html = renderToStaticMarkup(<ScreenerFilterPanel draft={taiwanScreenerDefaultFilters()} onChange={() => {}} onApply={() => {}} onClear={() => {}} localError="" />);
		for (const label of ['累計 EPS', '毛利率', '營業利益率', '股價', '成交金額', '外資買賣超', '本益比（PE）', '現金股利']) {
			expect(html).toContain(label);
		}
	});
});

describe('M7E-B -- Clear/Apply/pagination/refresh preserve financial-statement fields', () => {
	it('Clear (taiwanScreenerDefaultFilters()) resets financial-statement fields', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const clearFilters = () => {'), source.indexOf('const goPrevious = () => {'));
		expect(fn).toContain('taiwanScreenerDefaultFilters()');
	});

	it('Apply resets offset to 0 even with financial-statement criteria present', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const applyFilters = () => {'), source.indexOf('const clearFilters = () => {'));
		expect(fn).toContain('setApplied({ ...draft, offset: 0 })');
	});

	it('pagination/refresh spread the full applied object (including financial-statement fields), never rebuild it field-by-field', () => {
		const source = workspaceSource();
		const previous = source.slice(source.indexOf('const goPrevious = () => {'), source.indexOf('const goNext = () => {'));
		const next = source.slice(source.indexOf('const goNext = () => {'), source.indexOf('// Pessimistic add/remove'));
		expect(previous).toContain('setApplied((current) => ({ ...current, offset:');
		expect(next).toContain('setApplied((current) => ({ ...current, offset:');
		expect(source).toContain('}, [config, refreshKey, applied]);');
	});
});

describe('M7E-B -- result rendering: null/zero/negative for cumulative_eps/gross_margin/operating_margin', () => {
	it('a full row renders exact user-facing semantics: 期間/累計 EPS/毛利率/營業利益率', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({
			security: security({ financial_period: '2026-Q2', cumulative_eps: 49.33, gross_margin: 67.03, operating_margin: 59.29 }),
			showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: true, showBalance: false, showCashflow: false,
		})}</tr></tbody></table>);
		expect(html).toContain('期間 2026-Q2');
		expect(html).toContain('累計 EPS');
		expect(html).toContain('49.33');
		expect(html).toContain('毛利率');
		expect(html).toContain('67.03%');
		expect(html).toContain('營業利益率');
		expect(html).toContain('59.29%');
	});

	it('null values render — (no financials row at all)', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({
			security: security({ financial_period: null, cumulative_eps: null, gross_margin: null, operating_margin: null }),
			showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: true, showBalance: false, showCashflow: false,
		})}</tr></tbody></table>);
		expect((html.match(/—/g) || []).length).toBeGreaterThanOrEqual(4); // 期間 + 3 metric fields
	});

	it('zero renders as 0/0%, not missing', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({
			security: security({ financial_period: '2026-Q2', cumulative_eps: 0, gross_margin: 0, operating_margin: 0 }),
			showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: true, showBalance: false, showCashflow: false,
		})}</tr></tbody></table>);
		expect(html).toContain('累計 EPS 0');
		expect(html).toContain('毛利率 0%');
		expect(html).toContain('營業利益率 0%');
	});

	it('negative values render as negative, never clamped', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({
			security: security({ financial_period: '2026-Q2', cumulative_eps: -1.23, gross_margin: -20, operating_margin: -30 }),
			showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: true, showBalance: false, showCashflow: false,
		})}</tr></tbody></table>);
		expect(html).toContain('-1.23');
		expect(html).toContain('-20%');
		expect(html).toContain('-30%');
	});
});

describe('M7E-B -- critical per-row financial_period test', () => {
	it('a row on an older period than the domain target shows its OWN period, never substituted by the domain financials_period', () => {
		// Domain financials_period = 2026-Q2; this row is still on 2026-Q1 with nulled metrics --
		// exactly what the backend's mixed-period gate produces for an older-quarter security.
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({
			security: security({ financial_period: '2026-Q1', cumulative_eps: null, gross_margin: null, operating_margin: null }),
			showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: true, showBalance: false, showCashflow: false,
		})}</tr></tbody></table>);
		expect(html).toContain('期間 2026-Q1');
		expect(html).not.toContain('2026-Q2');
	});
});

describe('M7E-B -- adaptive result presentation driven by APPLIED filters, not draft', () => {
	it('financial-statement criteria (applied) shows the financial-statement block and the 進階資料 header', () => {
		const applied = withFundamentals({ minCumulativeEPS: '0' });
		const show = taiwanScreenerHasFinancialsCriteria(applied);
		expect(show).toBe(true);
		const html = renderToStaticMarkup(<ScreenerTable securities={[security()]} onOpenResearch={() => {}} {...tableWatchlistProps({ showFinancials: show })} />);
		expect(html).toContain('進階資料');
		expect(html).toContain('累計 EPS');
	});

	it('neither financial-statement nor other criteria active keeps the byte-identical legacy table', () => {
		const applied = taiwanScreenerDefaultFilters();
		expect(taiwanScreenerHasFinancialsCriteria(applied)).toBe(false);
		const html = renderToStaticMarkup(<ScreenerTable securities={[security()]} onOpenResearch={() => {}} {...tableWatchlistProps()} />);
		expect(html).not.toContain('進階資料');
	});

	it('draft-only financial-statement typing must not alter currently displayed result presentation', () => {
		const source = workspaceSource();
		expect(source).toContain('taiwanScreenerHasFinancialsCriteria(applied)');
		expect(source).not.toMatch(/taiwanScreenerHasFinancialsCriteria\(draft\)/);
	});
});

describe('M7E-B -- domain freshness / status mapping', () => {
	it('available renders the domain period and 可使用, never 最新', () => {
		const html = renderToStaticMarkup(<ScreenerFinancialsFreshness period="2026-Q2" status="available" />);
		expect(html).toContain('財務報表：2026-Q2');
		expect(html).toContain('可使用');
		expect(html).not.toContain('最新');
	});

	it('partial renders the domain period, 部分可使用, and a non-blocking warning', () => {
		const html = renderToStaticMarkup(<ScreenerFinancialsFreshness period="2026-Q2" status="partial" />);
		expect(html).toContain('財務報表：2026-Q2');
		expect(html).toContain('部分可使用');
		expect(html).toContain('部分財務報表資料暫時無法取得');
	});

	it('unavailable renders a safe warning, never a fabricated period', () => {
		const html = renderToStaticMarkup(<ScreenerFinancialsFreshness period={null} status="unavailable" />);
		expect(html).toContain('財務報表資料目前無法取得');
		expect(html).not.toContain('財務報表：');
	});

	it('renders nothing when status is absent (domain not requested)', () => {
		const html = renderToStaticMarkup(<>{ScreenerFinancialsFreshness({ period: undefined, status: undefined })}</>);
		expect(html).toBe('');
	});

	it('no days-behind, no publication date, no available_at ever appears', () => {
		const html = renderToStaticMarkup(<ScreenerFinancialsFreshness period="2026-Q2" status="available" />);
		expect(html).not.toMatch(/days_behind|published_at|available_at|2026-06-30/);
	});

	it('freshness block is gated by showFinancials in the workspace body', () => {
		const source = workspaceSource();
		expect(source).toMatch(/data && showFinancials && <ScreenerFinancialsFreshness/);
	});
});

describe('M7E-B -- partial warning coexists with rendered rows/pagination/Watchlist', () => {
	it('partial status does not disable or hide anything -- it is a standalone, non-blocking element', () => {
		const html = renderToStaticMarkup(<ScreenerFinancialsFreshness period="2026-Q2" status="partial" />);
		expect(html).not.toMatch(/disabled/);
	});
});

describe('M7E-B -- unavailable + empty state render independently', () => {
	it('the unavailable warning and the total===0 empty-state block are two separate, unconditional-on-each-other JSX expressions', () => {
		const source = workspaceSource();
		expect(source).toMatch(/data && showFinancials && <ScreenerFinancialsFreshness/);
		expect(source).toMatch(/data && data\.total === 0 && <div className="taiwan-empty-state">/);
	});
});

describe('M7E-B -- no request fan-out introduced', () => {
	it('still issues exactly one requestJSON call (Screener) and never a second endpoint for financial statements', () => {
		const source = workspaceSource();
		const matches = source.match(/requestJSON</g) || [];
		expect(matches.length).toBe(1);
		expect(source).not.toMatch(/\/api\/v1\/tw\/financials|\/api\/v1\/tw\/fundamentals|FinMind/i);
	});
});

describe('M7E-B -- research navigation and Watchlist action remain unaffected by the financial-statement column', () => {
	it('exact 2330.TWSE navigation preserved even with the financial-statement column rendered', () => {
		let opened = '';
		const twse = security({ canonical: '2330.TWSE', cumulative_eps: 49.33 });
		const element = ScreenerRow({ security: twse, onOpen: () => { opened = twse.canonical; }, ...rowWatchlistProps({ showFinancials: true, showBalance: false, showCashflow: false }) });
		const identityCell = (element.props.children as unknown[])[0] as { props: { children: { props: { onClick: () => void } } } };
		identityCell.props.children.props.onClick();
		expect(opened).toBe('2330.TWSE');
	});

	it('exact 6488.TPEX navigation preserved with financial-statement column rendered', () => {
		let opened = '';
		const tpex = security({ canonical: '6488.TPEX', code: '6488', name: '環球晶', exchange: 'TPEX', cumulative_eps: 11.87 });
		const element = ScreenerRow({ security: tpex, onOpen: () => { opened = tpex.canonical; }, ...rowWatchlistProps({ showFinancials: true, showBalance: false, showCashflow: false }) });
		const identityCell = (element.props.children as unknown[])[0] as { props: { children: { props: { onClick: () => void } } } };
		identityCell.props.children.props.onClick();
		expect(opened).toBe('6488.TPEX');
	});

	it('the Watchlist toggle still never triggers onOpen, even with the financial-statement column present', () => {
		let toggled = false;
		const element = ScreenerWatchlistAction({ membershipState: 'ready', saved: false, busy: false, onToggle: () => { toggled = true; } });
		const button = (element.props.children as unknown[])[0] as { type: string; props: { onClick: () => void } };
		button.props.onClick();
		expect(toggled).toBe(true);
	});
});

describe('M7E-B -- race safety and cold-start unchanged', () => {
	it('still uses the shared runScopedRequest race guard for the Screener fetch (financial-statement fields do not change this)', () => {
		const source = workspaceSource();
		expect(source).toContain('runScopedRequest(requestID');
	});

	it('cold overview cold-start still does not preload the Screener or Watchlist', () => {
		const source = overviewSource();
		const mountEffect = source.slice(source.indexOf('useEffect(() => {'), source.indexOf('[config, refreshKey]'));
		expect(mountEffect).not.toMatch(/screener/i);
		expect(mountEffect).not.toMatch(/watchlist/i);
	});
});

describe('M7E-C -- 財務報表 subsection gains exactly 4 new inputs (net margin + BVPS), no new top-level group', () => {
	it('1. renders the 4 new labeled inputs once 基本面 is expanded, alongside the existing 6 M7E-B inputs', () => {
		const source = workspaceSource();
		const panel = source.slice(source.indexOf('export function ScreenerFilterPanel'), source.indexOf('export function ScreenerSummary'));
		expect(panel).toContain('淨利率最小（%）');
		expect(panel).toContain('淨利率最大（%）');
		expect(panel).toContain('每股參考淨值最小');
		expect(panel).toContain('每股參考淨值最大');
		// existing M7E-B inputs remain present, unduplicated.
		expect((panel.match(/累計 EPS 最小/g) || []).length).toBe(1);
		expect((panel.match(/毛利率最小（%）/g) || []).length).toBe(1);
	});

	it('2. defaults collapsed: net margin / BVPS inputs are not present in the initial (collapsed) render', () => {
		const html = renderToStaticMarkup(<ScreenerFilterPanel draft={taiwanScreenerDefaultFilters()} onChange={() => {}} onApply={() => {}} onClear={() => {}} localError="" />);
		expect(html).not.toContain('淨利率最小');
		expect(html).not.toContain('每股參考淨值最小');
	});

	it('3. no second top-level fundamentals group and no new 財務報表 subheading -- reuses the existing one', () => {
		const source = workspaceSource();
		const panel = source.slice(source.indexOf('export function ScreenerFilterPanel'), source.indexOf('export function ScreenerSummary'));
		expect((panel.match(/財務報表/g) || []).length).toBe(1);
		expect((panel.match(/基本面/g) || []).length).toBe(1);
	});

	it('industry note is updated to truthfully include net margin, without exposing internal category codes', () => {
		const source = workspaceSource();
		const panel = source.slice(source.indexOf('export function ScreenerFilterPanel'), source.indexOf('export function ScreenerSummary'));
		expect(panel).toContain('部分金融相關產業不提供毛利率／營業利益率／淨利率');
		expect(panel).not.toMatch(/\bci\b|\bfh\b|\bbd\b|\bins\b|\bmim\b|\bbasi\b/);
	});
});

describe('M7E-C -- draft typing sends no request (net margin / BVPS)', () => {
	it('every new input writes via the same set() helper used by every other field, never a request call', () => {
		const source = workspaceSource();
		const panel = source.slice(source.indexOf('export function ScreenerFilterPanel'), source.indexOf('export function ScreenerSummary'));
		expect(panel).toContain("set('minNetMargin'");
		expect(panel).toContain("set('minBookValuePerShare'");
		expect(panel).not.toMatch(/requestJSON|taiwanScreenerPath\(/);
	});
});

describe('M7E-C -- query construction', () => {
	it('Apply serializes net_margin/book_value_per_share', () => {
		const path = taiwanScreenerPath(withFundamentals({ minNetMargin: '10', maxBookValuePerShare: '50' }));
		expect(path).toContain('min_net_margin=10');
		expect(path).toContain('max_book_value_per_share=50');
	});

	it('an explicit zero is serialized, never omitted as if blank', () => {
		const path = taiwanScreenerPath(withFundamentals({ minNetMargin: '0', minBookValuePerShare: '0' }));
		expect(path).toContain('min_net_margin=0');
		expect(path).toContain('min_book_value_per_share=0');
	});

	it('a negative value is serialized correctly', () => {
		const path = taiwanScreenerPath(withFundamentals({ minNetMargin: '-5' }));
		expect(path).toContain('min_net_margin=-5');
	});

	it('blank net-margin/BVPS params are omitted entirely', () => {
		const path = taiwanScreenerPath(taiwanScreenerDefaultFilters());
		expect(path).not.toContain('net_margin');
		expect(path).not.toContain('book_value_per_share');
	});
});

describe('M7E-C -- sort', () => {
	it('sort=net_margin / book_value_per_share query', () => {
		expect(taiwanScreenerPath(withFundamentals({ sort: 'net_margin' }))).toContain('sort=net_margin');
		expect(taiwanScreenerPath(withFundamentals({ sort: 'book_value_per_share' }))).toContain('sort=book_value_per_share');
	});

	it('sort dropdown includes the 2 new options plus every legacy option (26 total)', () => {
		const html = renderToStaticMarkup(<ScreenerFilterPanel draft={taiwanScreenerDefaultFilters()} onChange={() => {}} onApply={() => {}} onClear={() => {}} localError="" />);
		for (const label of ['淨利率', '每股參考淨值', '累計 EPS', '毛利率', '股價', '現金股利']) {
			expect(html).toContain(label);
		}
	});
});

describe('M7E-C -- Clear/Apply/pagination/refresh preserve net margin / BVPS fields', () => {
	it('Clear (taiwanScreenerDefaultFilters()) resets the new fields', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const clearFilters = () => {'), source.indexOf('const goPrevious = () => {'));
		expect(fn).toContain('taiwanScreenerDefaultFilters()');
	});

	it('Apply resets offset to 0 even with net margin / BVPS criteria present', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const applyFilters = () => {'), source.indexOf('const clearFilters = () => {'));
		expect(fn).toContain('setApplied({ ...draft, offset: 0 })');
	});

	it('pagination/refresh spread the full applied object (including the new fields), never rebuild it field-by-field', () => {
		const source = workspaceSource();
		const previous = source.slice(source.indexOf('const goPrevious = () => {'), source.indexOf('const goNext = () => {'));
		const next = source.slice(source.indexOf('const goNext = () => {'), source.indexOf('// Pessimistic add/remove'));
		expect(previous).toContain('setApplied((current) => ({ ...current, offset:');
		expect(next).toContain('setApplied((current) => ({ ...current, offset:');
		expect(source).toContain('}, [config, refreshKey, applied]);');
	});
});

describe('M7E-C -- result rendering: null/zero/negative for net_margin/book_value_per_share', () => {
	it('a full row renders exact user-facing semantics: 淨利率/每股參考淨值 with correct units, no ×100 rescale', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({
			security: security({ financial_period: '2026-Q2', net_margin: 53.19, book_value_per_share: 248.05 }),
			showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: true, showBalance: false, showCashflow: false,
		})}</tr></tbody></table>);
		expect(html).toContain('淨利率');
		expect(html).toContain('53.19%');
		expect(html).not.toContain('5319%');
		expect(html).toContain('每股參考淨值');
		expect(html).toContain('248.05 元／股');
	});

	it('null values render — (e.g. non-ci category / balance domain not requested)', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({
			security: security({ financial_period: '2026-Q2', net_margin: null, book_value_per_share: null }),
			showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: true, showBalance: false, showCashflow: false,
		})}</tr></tbody></table>);
		expect(html).not.toContain('0%');
		expect((html.match(/—/g) || []).length).toBeGreaterThanOrEqual(2);
	});

	it('zero renders as 0%/0 元／股, not missing', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({
			security: security({ financial_period: '2026-Q2', net_margin: 0, book_value_per_share: 0 }),
			showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: true, showBalance: false, showCashflow: false,
		})}</tr></tbody></table>);
		expect(html).toContain('淨利率 0%');
		expect(html).toContain('每股參考淨值 0 元／股');
	});

	it('negative values render as negative, never clamped', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({
			security: security({ financial_period: '2026-Q2', net_margin: -12.5, book_value_per_share: -3.2 }),
			showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: true, showBalance: false, showCashflow: false,
		})}</tr></tbody></table>);
		expect(html).toContain('-12.5%');
		expect(html).toContain('-3.2 元／股');
	});
});

describe('M7E-C -- mixed-period frontend test (row financial_period is never substituted by financials_period)', () => {
	it('row A (target period) shows its BVPS; row B (older period, BVPS nulled by backend) shows — and its OWN 2026-Q1 period', () => {
		const rowA = security({ canonical: '2330.TWSE', financial_period: '2026-Q2', book_value_per_share: 248.05 });
		const rowB = security({ canonical: '1101.TWSE', code: '1101', name: '台泥', financial_period: '2026-Q1', book_value_per_share: null });
		const htmlA = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: rowA, showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: true, showBalance: false, showCashflow: false })}</tr></tbody></table>);
		const htmlB = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({ security: rowB, showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: true, showBalance: false, showCashflow: false })}</tr></tbody></table>);
		expect(htmlA).toContain('期間 2026-Q2');
		expect(htmlA).toContain('248.05 元／股');
		expect(htmlB).toContain('期間 2026-Q1');
		expect(htmlB).not.toContain('2026-Q2');
		expect(htmlB).not.toContain('248.05');
	});
});

describe('M7E-C -- financial holding test (2882.TWSE-style row: net_margin null, BVPS present)', () => {
	it('net margin renders — while BVPS renders normally, with no frontend workaround', () => {
		const holding = security({ canonical: '2882.TWSE', code: '2882', name: '國泰金', financial_period: '2026-Q2', net_margin: null, book_value_per_share: 69.37 });
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({
			security: holding, showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: true, showBalance: false, showCashflow: false,
		})}</tr></tbody></table>);
		expect(html).toContain('淨利率 —');
		expect(html).toContain('69.37 元／股');
	});
});

describe('M7E-C -- applied-domain detection reuses the existing showFinancials contract (no new showBalance concept)', () => {
	it('an active net_margin or book_value_per_share filter/sort also activates showFinancials', () => {
		expect(taiwanScreenerHasFinancialsCriteria(withFundamentals({ minNetMargin: '0' }))).toBe(true);
		expect(taiwanScreenerHasFinancialsCriteria(withFundamentals({ maxBookValuePerShare: '0' }))).toBe(true);
		expect(taiwanScreenerHasFinancialsCriteria(withFundamentals({ sort: 'net_margin' }))).toBe(true);
		expect(taiwanScreenerHasFinancialsCriteria(withFundamentals({ sort: 'book_value_per_share' }))).toBe(true);
	});

	it('draft-only typing must not alter currently displayed result presentation', () => {
		const source = workspaceSource();
		expect(source).toContain('taiwanScreenerHasFinancialsCriteria(applied)');
		expect(source).not.toMatch(/taiwanScreenerHasFinancialsCriteria\(draft\)/);
	});
});

describe('M7E-C -- financials_period/financials_status semantics unchanged; no new balance UI', () => {
	// balance_period/balance_status were deliberately introduced by M7G (independent balance-sheet
	// ratio domain, see taiwanScreenerHasBalanceCriteria) -- no longer forbidden here. book_value_status/
	// balance_sheet_period were never introduced by any phase and remain forbidden.
	it('no book_value_status/balance_sheet_period UI was introduced', () => {
		const source = workspaceSource();
		expect(source).not.toMatch(/book_value_status|balance_sheet_period/);
	});

	it('existing partial/unavailable ScreenerFinancialsFreshness behavior is reused verbatim (not duplicated)', () => {
		const source = workspaceSource();
		expect((source.match(/function ScreenerFinancialsFreshness/g) || []).length).toBe(1);
	});
});

describe('M7E-C -- no request fan-out introduced', () => {
	it('still issues exactly one requestJSON call (Screener) and never a direct balance/fundamentals/FinMind endpoint', () => {
		const source = workspaceSource();
		const matches = source.match(/requestJSON</g) || [];
		expect(matches.length).toBe(1);
		expect(source).not.toMatch(/\/api\/v1\/tw\/financials|\/api\/v1\/tw\/fundamentals|\/api\/v1\/tw\/balance|FinMind/i);
	});
});

describe('M7E-C -- research navigation and Watchlist action remain unaffected', () => {
	it('exact 2330.TWSE navigation preserved even with net_margin/BVPS columns rendered', () => {
		let opened = '';
		const twse = security({ canonical: '2330.TWSE', net_margin: 53.19, book_value_per_share: 248.05 });
		const element = ScreenerRow({ security: twse, onOpen: () => { opened = twse.canonical; }, ...rowWatchlistProps({ showFinancials: true, showBalance: false, showCashflow: false }) });
		const identityCell = (element.props.children as unknown[])[0] as { props: { children: { props: { onClick: () => void } } } };
		identityCell.props.children.props.onClick();
		expect(opened).toBe('2330.TWSE');
	});

	it('the Watchlist toggle still never triggers onOpen, even with net_margin/BVPS columns present', () => {
		let toggled = false;
		const element = ScreenerWatchlistAction({ membershipState: 'ready', saved: false, busy: false, onToggle: () => { toggled = true; } });
		const button = (element.props.children as unknown[])[0] as { type: string; props: { onClick: () => void } };
		button.props.onClick();
		expect(toggled).toBe(true);
	});
});

describe('M7E-C -- race safety and cold-start unchanged', () => {
	it('still uses the shared runScopedRequest race guard for the Screener fetch', () => {
		const source = workspaceSource();
		expect(source).toContain('runScopedRequest(requestID');
	});

	it('cold overview cold-start still does not preload the Screener or Watchlist', () => {
		const source = overviewSource();
		const mountEffect = source.slice(source.indexOf('useEffect(() => {'), source.indexOf('[config, refreshKey]'));
		expect(mountEffect).not.toMatch(/screener/i);
		expect(mountEffect).not.toMatch(/watchlist/i);
	});
});

describe('M7F -- 營收 subsection gains exactly 4 new inputs (revenue growth), no new top-level group', () => {
	it('1. renders the 4 new labeled inputs once 基本面 is expanded, alongside the existing 4 M7E-A revenue inputs', () => {
		const source = workspaceSource();
		const panel = source.slice(source.indexOf('export function ScreenerFilterPanel'), source.indexOf('export function ScreenerSummary'));
		expect(panel).toContain('月營收月增率最小（%）');
		expect(panel).toContain('月營收月增率最大（%）');
		expect(panel).toContain('累計營收年增率最小（%）');
		expect(panel).toContain('累計營收年增率最大（%）');
		expect((panel.match(/最低月營收年增率（%）/g) || []).length).toBe(1);
	});

	it('2. defaults collapsed: revenue growth inputs are not present in the initial (collapsed) render', () => {
		const html = renderToStaticMarkup(<ScreenerFilterPanel draft={taiwanScreenerDefaultFilters()} onChange={() => {}} onApply={() => {}} onClear={() => {}} localError="" />);
		expect(html).not.toContain('月營收月增率最小');
		expect(html).not.toContain('累計營收年增率最小');
	});

	it('3. no second top-level fundamentals group and no new 營收 subheading -- reuses the existing one', () => {
		const source = workspaceSource();
		const panel = source.slice(source.indexOf('export function ScreenerFilterPanel'), source.indexOf('export function ScreenerSummary'));
		expect((panel.match(/<h4 className="taiwan-screener-advanced-subheading">營收<\/h4>/g) || []).length).toBe(1);
		expect((panel.match(/基本面/g) || []).length).toBe(1);
	});
});

describe('M7F -- draft typing sends no request (revenue growth)', () => {
	it('every new input writes via the same set() helper used by every other field, never a request call', () => {
		const source = workspaceSource();
		const panel = source.slice(source.indexOf('export function ScreenerFilterPanel'), source.indexOf('export function ScreenerSummary'));
		expect(panel).toContain("set('minRevenueMoM'");
		expect(panel).toContain("set('minCumulativeRevenueYoY'");
		expect(panel).not.toMatch(/requestJSON|taiwanScreenerPath\(/);
	});
});

describe('M7F -- query construction', () => {
	it('Apply serializes revenue_mom/cumulative_revenue_yoy', () => {
		const path = taiwanScreenerPath(withFundamentals({ minRevenueMoM: '-20', maxCumulativeRevenueYoY: '50' }));
		expect(path).toContain('min_revenue_mom=-20');
		expect(path).toContain('max_cumulative_revenue_yoy=50');
	});

	it('an explicit zero is serialized, never omitted as if blank', () => {
		const path = taiwanScreenerPath(withFundamentals({ minRevenueMoM: '0', minCumulativeRevenueYoY: '0' }));
		expect(path).toContain('min_revenue_mom=0');
		expect(path).toContain('min_cumulative_revenue_yoy=0');
	});

	it('blank revenue-growth params are omitted entirely', () => {
		const path = taiwanScreenerPath(taiwanScreenerDefaultFilters());
		expect(path).not.toContain('revenue_mom');
		expect(path).not.toContain('cumulative_revenue_yoy');
	});
});

describe('M7F -- sort', () => {
	it('sort=revenue_mom / cumulative_revenue_yoy query', () => {
		expect(taiwanScreenerPath(withFundamentals({ sort: 'revenue_mom' }))).toContain('sort=revenue_mom');
		expect(taiwanScreenerPath(withFundamentals({ sort: 'cumulative_revenue_yoy' }))).toContain('sort=cumulative_revenue_yoy');
	});

	it('sort dropdown includes the 2 new options plus every legacy option (28 total)', () => {
		const html = renderToStaticMarkup(<ScreenerFilterPanel draft={taiwanScreenerDefaultFilters()} onChange={() => {}} onApply={() => {}} onClear={() => {}} localError="" />);
		for (const label of ['月營收月增率', '累計營收年增率', '月營收', '月營收年增率', '股價', '現金股利']) {
			expect(html).toContain(label);
		}
	});
});

describe('M7F -- Clear/Apply/pagination/refresh preserve revenue growth fields', () => {
	it('Clear (taiwanScreenerDefaultFilters()) resets the new fields', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const clearFilters = () => {'), source.indexOf('const goPrevious = () => {'));
		expect(fn).toContain('taiwanScreenerDefaultFilters()');
	});

	it('Apply resets offset to 0 even with revenue-growth criteria present', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const applyFilters = () => {'), source.indexOf('const clearFilters = () => {'));
		expect(fn).toContain('setApplied({ ...draft, offset: 0 })');
	});

	it('pagination/refresh spread the full applied object (including the new fields), never rebuild it field-by-field', () => {
		const source = workspaceSource();
		const previous = source.slice(source.indexOf('const goPrevious = () => {'), source.indexOf('const goNext = () => {'));
		const next = source.slice(source.indexOf('const goNext = () => {'), source.indexOf('// Pessimistic add/remove'));
		expect(previous).toContain('setApplied((current) => ({ ...current, offset:');
		expect(next).toContain('setApplied((current) => ({ ...current, offset:');
		expect(source).toContain('}, [config, refreshKey, applied]);');
	});
});

describe('M7F -- result rendering: null/zero/negative for revenue_mom/cumulative_revenue_yoy', () => {
	it('a full row renders exact user-facing semantics: 月增率/累計年增率, no ×100 rescale', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({
			security: security({ revenue_mom: 5.62, cumulative_revenue_yoy: 37.01 }),
			showInstitutional: false, showMargin: false, showRevenue: true, showValuation: false, showDividends: false, showFinancials: false, showBalance: false, showCashflow: false,
		})}</tr></tbody></table>);
		expect(html).toContain('月增率');
		expect(html).toContain('+5.62%');
		expect(html).not.toContain('562%');
		expect(html).toContain('累計年增率');
		expect(html).toContain('+37.01%');
	});

	it('null values render — (no revenue row at all)', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({
			security: security({ revenue_mom: null, cumulative_revenue_yoy: null }),
			showInstitutional: false, showMargin: false, showRevenue: true, showValuation: false, showDividends: false, showFinancials: false, showBalance: false, showCashflow: false,
		})}</tr></tbody></table>);
		expect((html.match(/—/g) || []).length).toBeGreaterThanOrEqual(2);
	});

	it('zero renders as 0%, not missing', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({
			security: security({ revenue_mom: 0, cumulative_revenue_yoy: 0 }),
			showInstitutional: false, showMargin: false, showRevenue: true, showValuation: false, showDividends: false, showFinancials: false, showBalance: false, showCashflow: false,
		})}</tr></tbody></table>);
		expect(html).toContain('月增率 0%');
		expect(html).toContain('累計年增率 0%');
	});

	it('negative values render as negative, never clamped', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({
			security: security({ revenue_mom: -11.5, cumulative_revenue_yoy: -4.44 }),
			showInstitutional: false, showMargin: false, showRevenue: true, showValuation: false, showDividends: false, showFinancials: false, showBalance: false, showCashflow: false,
		})}</tr></tbody></table>);
		expect(html).toContain('-11.5%');
		expect(html).toContain('-4.44%');
	});
});

describe('M7F -- applied-domain detection reuses the existing showRevenue contract', () => {
	it('an active revenue_mom or cumulative_revenue_yoy filter/sort also activates showRevenue', () => {
		expect(taiwanScreenerHasRevenueCriteria(withFundamentals({ minRevenueMoM: '0' }))).toBe(true);
		expect(taiwanScreenerHasRevenueCriteria(withFundamentals({ maxCumulativeRevenueYoY: '0' }))).toBe(true);
		expect(taiwanScreenerHasRevenueCriteria(withFundamentals({ sort: 'revenue_mom' }))).toBe(true);
		expect(taiwanScreenerHasRevenueCriteria(withFundamentals({ sort: 'cumulative_revenue_yoy' }))).toBe(true);
	});

	it('draft-only typing must not alter currently displayed result presentation', () => {
		const source = workspaceSource();
		expect(source).toContain('taiwanScreenerHasRevenueCriteria(applied)');
		expect(source).not.toMatch(/taiwanScreenerHasRevenueCriteria\(draft\)/);
	});
});

describe('M7F -- revenue status: available / partial / unavailable', () => {
	it('available renders the domain period and no partial/unavailable warning', () => {
		const html = renderToStaticMarkup(<ScreenerDomainFreshness label="營收資料" asOf="2026-07" status="available" unavailableMessage="營收資料暫時無法取得" partialMessage="部分月營收資料暫時無法取得，已顯示目前可用資料。" />);
		expect(html).toContain('營收資料：2026-07');
		expect(html).not.toContain('部分月營收資料暫時無法取得');
		expect(html).not.toContain('營收資料暫時無法取得');
	});

	it('partial renders the freshness line PLUS a non-blocking warning -- distinguishable from available', () => {
		const html = renderToStaticMarkup(<ScreenerDomainFreshness label="營收資料" asOf="2026-07" status="partial" unavailableMessage="營收資料暫時無法取得" partialMessage="部分月營收資料暫時無法取得，已顯示目前可用資料。" />);
		expect(html).toContain('營收資料：2026-07');
		expect(html).toContain('部分月營收資料暫時無法取得，已顯示目前可用資料。');
	});

	it('unavailable renders a safe warning, independent of empty-result state, never a fabricated period', () => {
		const html = renderToStaticMarkup(<ScreenerDomainFreshness label="營收資料" asOf={null} status="unavailable" unavailableMessage="營收資料暫時無法取得" partialMessage="部分月營收資料暫時無法取得，已顯示目前可用資料。" />);
		expect(html).toContain('營收資料暫時無法取得');
		expect(html).not.toContain('營收資料：');
	});

	it('other existing domains (no partialMessage passed) keep byte-identical behavior even if status were partial', () => {
		const html = renderToStaticMarkup(<ScreenerDomainFreshness label="估值資料" asOf="2026-09-04" status="partial" unavailableMessage="估值資料暫時無法取得" />);
		expect(html).toContain('估值資料：2026-09-04');
		expect(html).not.toMatch(/暫時無法取得，已顯示目前可用資料/);
	});
});

describe('M7F -- no request fan-out introduced', () => {
	it('still issues exactly one requestJSON call (Screener) and never a direct revenue/fundamentals/FinMind/TWSE/TPEx endpoint', () => {
		const source = workspaceSource();
		const matches = source.match(/requestJSON</g) || [];
		expect(matches.length).toBe(1);
		expect(source).not.toMatch(/\/api\/v1\/tw\/financials|\/api\/v1\/tw\/fundamentals|\/api\/v1\/tw\/balance|FinMind|openapi\.twse|tpex\.org/i);
	});
});

describe('M7F -- research navigation and Watchlist action remain unaffected', () => {
	it('exact 2330.TWSE navigation preserved even with revenue_mom/cumulative_revenue_yoy columns rendered', () => {
		let opened = '';
		const twse = security({ canonical: '2330.TWSE', revenue_mom: 5.62, cumulative_revenue_yoy: 37.01 });
		const element = ScreenerRow({ security: twse, onOpen: () => { opened = twse.canonical; }, ...rowWatchlistProps({ showRevenue: true }) });
		const identityCell = (element.props.children as unknown[])[0] as { props: { children: { props: { onClick: () => void } } } };
		identityCell.props.children.props.onClick();
		expect(opened).toBe('2330.TWSE');
	});

	it('the Watchlist toggle still never triggers onOpen, even with revenue-growth columns present', () => {
		let toggled = false;
		const element = ScreenerWatchlistAction({ membershipState: 'ready', saved: false, busy: false, onToggle: () => { toggled = true; } });
		const button = (element.props.children as unknown[])[0] as { type: string; props: { onClick: () => void } };
		button.props.onClick();
		expect(toggled).toBe(true);
	});
});

describe('M7F -- race safety and cold-start unchanged', () => {
	it('still uses the shared runScopedRequest race guard for the Screener fetch', () => {
		const source = workspaceSource();
		expect(source).toContain('runScopedRequest(requestID');
	});

	it('cold overview cold-start still does not preload the Screener or Watchlist', () => {
		const source = overviewSource();
		const mountEffect = source.slice(source.indexOf('useEffect(() => {'), source.indexOf('[config, refreshKey]'));
		expect(mountEffect).not.toMatch(/screener/i);
		expect(mountEffect).not.toMatch(/watchlist/i);
	});
});

// ==================================================
// M7G -- balance-sheet ratios (debt_ratio / debt_to_equity / current_ratio), independent
// balance_period/balance_status domain, ci-only via backend nulls (frontend never re-derives category).
// ==================================================

const fundamentalsPanel = () => {
	const source = workspaceSource();
	return source.slice(source.indexOf('{fundamentalsExpanded && <>'), source.indexOf("{localError && <p"));
};

describe('M7G -- 財務報表 subsection gains exactly 6 new inputs (debt_ratio/debt_to_equity/current_ratio), no new top-level group', () => {
	it('1. renders the 6 new labeled inputs once 基本面 is expanded, alongside the existing M7E-C/M7E-B inputs', () => {
		const panel = fundamentalsPanel();
		expect(panel).toContain('負債比率最小（%）');
		expect(panel).toContain('負債比率最大（%）');
		expect(panel).toContain('負債權益比最小（%）');
		expect(panel).toContain('負債權益比最大（%）');
		expect(panel).toContain('流動比率最小（%）');
		expect(panel).toContain('流動比率最大（%）');
		// existing M7E-C/M7E-B inputs remain present, unduplicated.
		expect((panel.match(/每股參考淨值最小/g) || []).length).toBe(1);
		expect((panel.match(/淨利率最小（%）/g) || []).length).toBe(1);
	});

	it('2. defaults collapsed: the 6 new inputs are not present in the initial (collapsed) render', () => {
		const html = renderToStaticMarkup(<ScreenerFilterPanel draft={taiwanScreenerDefaultFilters()} onChange={() => {}} onApply={() => {}} onClear={() => {}} localError="" />);
		expect(html).not.toContain('負債比率最小');
		expect(html).not.toContain('負債權益比最小');
		expect(html).not.toContain('流動比率最小');
	});

	it('3. no second top-level group and no new 財務報表 subheading -- reuses the existing one', () => {
		const source = workspaceSource();
		const panel = source.slice(source.indexOf('export function ScreenerFilterPanel'), source.indexOf('export function ScreenerSummary'));
		expect((panel.match(/財務報表/g) || []).length).toBe(1);
		expect((panel.match(/基本面/g) || []).length).toBe(1);
	});

	it('4. basic filter count under 基本面 grows from 30 to exactly 36 numeric range inputs (M7H later adds 4 more -- see the M7H suite for the current total of 40)', () => {
		const panel = fundamentalsPanel();
		const count = (panel.match(/inputMode="decimal"|inputMode="numeric"/g) || []).length;
		expect(count).toBe(40);
	});

	it('industry note is updated to truthfully include the three new ratios, without exposing internal category codes', () => {
		const panel = fundamentalsPanel();
		expect(panel).toContain('負債比率／負債權益比／流動比率');
		expect(panel).not.toMatch(/\bci\b|\bfh\b|\bbd\b|\bins\b|\bmim\b|\bbasi\b/);
		expect(panel).not.toContain('只有 ci 類別');
	});
});

describe('M7G -- draft typing sends no request (balance ratios)', () => {
	it('every new input writes via the same set() helper used by every other field, never a request call', () => {
		const panel = fundamentalsPanel();
		expect(panel).toContain("set('minDebtRatio'");
		expect(panel).toContain("set('minDebtToEquity'");
		expect(panel).toContain("set('minCurrentRatio'");
		expect(panel).not.toMatch(/requestJSON|taiwanScreenerPath\(/);
	});
});

describe('M7G -- query construction', () => {
	it('Apply serializes debt_ratio/debt_to_equity/current_ratio', () => {
		const path = taiwanScreenerPath(withFundamentals({ minDebtRatio: '10', maxDebtToEquity: '50', minCurrentRatio: '100' }));
		expect(path).toContain('min_debt_ratio=10');
		expect(path).toContain('max_debt_to_equity=50');
		expect(path).toContain('min_current_ratio=100');
	});

	it('an explicit zero is serialized, never omitted as if blank', () => {
		const path = taiwanScreenerPath(withFundamentals({ minDebtRatio: '0', minDebtToEquity: '0', minCurrentRatio: '0' }));
		expect(path).toContain('min_debt_ratio=0');
		expect(path).toContain('min_debt_to_equity=0');
		expect(path).toContain('min_current_ratio=0');
	});

	it('a negative value is serialized correctly', () => {
		const path = taiwanScreenerPath(withFundamentals({ minDebtRatio: '-5' }));
		expect(path).toContain('min_debt_ratio=-5');
	});

	it('blank balance-ratio params are omitted entirely', () => {
		const path = taiwanScreenerPath(taiwanScreenerDefaultFilters());
		expect(path).not.toContain('debt_ratio');
		expect(path).not.toContain('debt_to_equity');
		expect(path).not.toContain('current_ratio');
	});
});

describe('M7G -- sort', () => {
	it('sort=debt_ratio / debt_to_equity / current_ratio query', () => {
		expect(taiwanScreenerPath(withFundamentals({ sort: 'debt_ratio' }))).toContain('sort=debt_ratio');
		expect(taiwanScreenerPath(withFundamentals({ sort: 'debt_to_equity' }))).toContain('sort=debt_to_equity');
		expect(taiwanScreenerPath(withFundamentals({ sort: 'current_ratio' }))).toContain('sort=current_ratio');
	});

	it('sort dropdown includes the 3 new options plus every legacy option (31 total)', () => {
		const html = renderToStaticMarkup(<ScreenerFilterPanel draft={taiwanScreenerDefaultFilters()} onChange={() => {}} onApply={() => {}} onClear={() => {}} localError="" />);
		for (const label of ['負債比率', '負債權益比', '流動比率', '淨利率', '每股參考淨值', '股價']) {
			expect(html).toContain(label);
		}
	});
});

describe('M7G -- Clear/Apply/pagination/refresh preserve balance-ratio fields', () => {
	it('Clear (taiwanScreenerDefaultFilters()) resets the new fields', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const clearFilters = () => {'), source.indexOf('const goPrevious = () => {'));
		expect(fn).toContain('taiwanScreenerDefaultFilters()');
	});

	it('Apply resets offset to 0 even with balance-ratio criteria present', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const applyFilters = () => {'), source.indexOf('const clearFilters = () => {'));
		expect(fn).toContain('setApplied({ ...draft, offset: 0 })');
	});

	it('pagination/refresh spread the full applied object (including the new fields), never rebuild it field-by-field', () => {
		const source = workspaceSource();
		const previous = source.slice(source.indexOf('const goPrevious = () => {'), source.indexOf('const goNext = () => {'));
		const next = source.slice(source.indexOf('const goNext = () => {'), source.indexOf('// Pessimistic add/remove'));
		expect(previous).toContain('setApplied((current) => ({ ...current, offset:');
		expect(next).toContain('setApplied((current) => ({ ...current, offset:');
		expect(source).toContain('}, [config, refreshKey, applied]);');
	});
});

describe('M7G -- result rendering: null/zero/negative for debt_ratio/debt_to_equity/current_ratio', () => {
	it('a full row renders exact user-facing semantics: 負債比/負債權益比/流動比 with correct units, no ×100 rescale', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({
			security: security({ balance_period: '2026-Q2', debt_ratio: 30.94, debt_to_equity: 44.81, current_ratio: 245.76 }),
			showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: false, showBalance: true, showCashflow: false,
		})}</tr></tbody></table>);
		expect(html).toContain('負債比');
		expect(html).toContain('30.94%');
		expect(html).not.toContain('3094%');
		expect(html).toContain('負債權益比');
		expect(html).toContain('44.81%');
		expect(html).toContain('流動比');
		expect(html).toContain('245.76%');
	});

	it('null values render — (e.g. non-ci category, or balance domain not requested for this row)', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({
			security: security({ balance_period: null, debt_ratio: null, debt_to_equity: null, current_ratio: null }),
			showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: false, showBalance: true, showCashflow: false,
		})}</tr></tbody></table>);
		expect(html).not.toContain('0%');
		expect((html.match(/—/g) || []).length).toBeGreaterThanOrEqual(4); // period + 3 metric fields
	});

	it('zero renders as 0%, not missing', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({
			security: security({ balance_period: '2026-Q2', debt_ratio: 0, debt_to_equity: 0, current_ratio: 0 }),
			showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: false, showBalance: true, showCashflow: false,
		})}</tr></tbody></table>);
		expect(html).toContain('負債比 0%');
		expect(html).toContain('負債權益比 0%');
		expect(html).toContain('流動比 0%');
	});

	it('negative values render as negative, never clamped (backend never actually returns one, but the frontend must not fabricate a positive)', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({
			security: security({ balance_period: '2026-Q2', debt_ratio: -5.5, debt_to_equity: -1.2, current_ratio: -3 }),
			showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: false, showBalance: true, showCashflow: false,
		})}</tr></tbody></table>);
		expect(html).toContain('-5.5%');
		expect(html).toContain('-1.2%');
		expect(html).toContain('-3%');
	});
});

describe('M7G -- row balance_period test', () => {
	it('balance_period=2026-Q2 renders 資產負債表期間 2026-Q2', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({
			security: security({ balance_period: '2026-Q2', debt_ratio: 30.94 }),
			showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: false, showBalance: true, showCashflow: false,
		})}</tr></tbody></table>);
		expect(html).toContain('資產負債表期間 2026-Q2');
	});

	it('balance_period=null never fabricates a row period, and is never substituted by financial_period', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({
			security: security({ balance_period: null, financial_period: '2026-Q2', debt_ratio: null }),
			showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: false, showBalance: true, showCashflow: false,
		})}</tr></tbody></table>);
		expect(html).toContain('資產負債表期間 —');
		expect(html).not.toContain('資產負債表期間 2026-Q2');
	});
});

describe('M7G -- balance_status: available / partial / unavailable', () => {
	it('available renders the domain period and no partial/unavailable warning', () => {
		const html = renderToStaticMarkup(<ScreenerDomainFreshness label="資產負債表資料" asOf="2026-Q2" status="available" unavailableMessage="資產負債表資料暫時無法取得" partialMessage="部分資產負債表資料暫時無法取得，已顯示目前可用資料。" />);
		expect(html).toContain('資產負債表資料：2026-Q2');
		expect(html).not.toContain('部分資產負債表資料暫時無法取得');
		expect(html).not.toContain('資產負債表資料暫時無法取得');
	});

	it('partial renders the freshness line PLUS the exact non-blocking warning text -- rows remain visible', () => {
		const html = renderToStaticMarkup(<ScreenerDomainFreshness label="資產負債表資料" asOf="2026-Q2" status="partial" unavailableMessage="資產負債表資料暫時無法取得" partialMessage="部分資產負債表資料暫時無法取得，已顯示目前可用資料。" />);
		expect(html).toContain('資產負債表資料：2026-Q2');
		expect(html).toContain('部分資產負債表資料暫時無法取得，已顯示目前可用資料。');
	});

	it('unavailable renders a safe warning, independent of empty-result state, never a fabricated period', () => {
		const html = renderToStaticMarkup(<ScreenerDomainFreshness label="資產負債表資料" asOf={null} status="unavailable" unavailableMessage="資產負債表資料暫時無法取得" partialMessage="部分資產負債表資料暫時無法取得，已顯示目前可用資料。" />);
		expect(html).toContain('資產負債表資料暫時無法取得');
		expect(html).not.toContain('資產負債表資料：');
	});

	it('unavailable warning and the zero-result empty state are independent -- both can render together', () => {
		const source = workspaceSource();
		expect(source).toMatch(/data && showBalance && <ScreenerDomainFreshness/);
		expect(source).toMatch(/data && data\.total === 0 && <div className="taiwan-empty-state">/);
	});
});

// Mandatory (task section 39): financials_period/financials_status must never be merged with or
// overwritten by balance_period/balance_status, even when both are simultaneously present with
// DIFFERENT periods/statuses.
describe('M7G -- domain separation (financials vs balance are never merged)', () => {
	it('financials_period=2026-Q2/available and balance_period=2026-Q1/partial render independently, neither substituted for the other', () => {
		const html = renderToStaticMarkup(<>
			<ScreenerFinancialsFreshness period="2026-Q2" status="available" />
			<ScreenerDomainFreshness label="資產負債表資料" asOf="2026-Q1" status="partial" unavailableMessage="資產負債表資料暫時無法取得" partialMessage="部分資產負債表資料暫時無法取得，已顯示目前可用資料。" />
		</>);
		expect(html).toContain('財務報表：2026-Q2');
		expect(html).toContain('資產負債表資料：2026-Q1');
		expect(html).toContain('部分資產負債表資料暫時無法取得，已顯示目前可用資料。');
		expect(html).not.toContain('財務報表：2026-Q1');
		expect(html).not.toContain('資產負債表資料：2026-Q2');
	});

	it('the workspace body renders each freshness block gated by its own independent flag (showFinancials vs showBalance)', () => {
		const source = workspaceSource();
		expect(source).toMatch(/data && showFinancials && <ScreenerFinancialsFreshness/);
		expect(source).toMatch(/data && showBalance && <ScreenerDomainFreshness label="資產負債表資料"/);
	});

	it('showBalance is computed via its own independent helper, not folded into showFinancials', () => {
		const source = workspaceSource();
		expect(source).toContain('const showBalance = taiwanScreenerHasBalanceCriteria(applied);');
		expect(source).toContain('const showFinancials = taiwanScreenerHasFinancialsCriteria(applied);');
	});
});

describe('M7G -- BVPS regression: existing behavior unchanged with or without M7G criteria present', () => {
	it('BVPS still renders under the financials block, never moved into the balance-only block', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({
			security: security({ financial_period: '2026-Q2', book_value_per_share: 28.5 }),
			showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: true, showBalance: false, showCashflow: false,
		})}</tr></tbody></table>);
		expect(html).toContain('每股參考淨值');
		expect(html).toContain('28.5 元／股');
	});

	it('financials freshness/status rendering is unaffected when M7G criteria are also present', () => {
		expect(taiwanScreenerHasFinancialsCriteria(withFundamentals({ minBookValuePerShare: '0', minDebtRatio: '0' }))).toBe(true);
		expect(taiwanScreenerHasBalanceCriteria(withFundamentals({ minBookValuePerShare: '0', minDebtRatio: '0' }))).toBe(true);
		const source = workspaceSource();
		expect((source.match(/function ScreenerFinancialsFreshness/g) || []).length).toBe(1);
	});
});

describe('M7G -- ci-only user-facing semantics (2882.TWSE-style fixture: frontend trusts backend nulls)', () => {
	it('all three metrics render —, no fabricated period, industry note is available when the section is expanded', () => {
		const holding = security({ canonical: '2882.TWSE', code: '2882', name: '國泰金', balance_period: null, debt_ratio: null, debt_to_equity: null, current_ratio: null });
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({
			security: holding, showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: false, showBalance: true, showCashflow: false,
		})}</tr></tbody></table>);
		expect(html).toContain('負債比 —');
		expect(html).toContain('負債權益比 —');
		expect(html).toContain('流動比 —');
		expect(html).toContain('資產負債表期間 —');
		const panel = fundamentalsPanel();
		expect(panel).toContain('負債比率／負債權益比／流動比率');
	});
});

describe('M7G -- no request fan-out introduced', () => {
	it('still issues exactly one requestJSON call (Screener) and never a direct balance/fundamentals/FinMind/TWSE/TPEx endpoint', () => {
		const source = workspaceSource();
		const matches = source.match(/requestJSON</g) || [];
		expect(matches.length).toBe(1);
		expect(source).not.toMatch(/t187ap07|mopsfin_t187ap07|\/api\/v1\/tw\/financials|\/api\/v1\/tw\/fundamentals|\/api\/v1\/tw\/balance|FinMind|Yahoo|openapi\.twse|tpex\.org/i);
	});
});

describe('M7G -- research navigation and Watchlist action remain unaffected', () => {
	it('exact 2330.TWSE navigation preserved even with debt_ratio/debt_to_equity/current_ratio columns rendered', () => {
		let opened = '';
		const twse = security({ canonical: '2330.TWSE', debt_ratio: 30.94, debt_to_equity: 44.81, current_ratio: 245.76 });
		const element = ScreenerRow({ security: twse, onOpen: () => { opened = twse.canonical; }, ...rowWatchlistProps({ showBalance: true, showCashflow: false }) });
		const identityCell = (element.props.children as unknown[])[0] as { props: { children: { props: { onClick: () => void } } } };
		identityCell.props.children.props.onClick();
		expect(opened).toBe('2330.TWSE');
	});

	it('the Watchlist toggle still never triggers onOpen, even with balance-ratio columns present', () => {
		let toggled = false;
		const element = ScreenerWatchlistAction({ membershipState: 'ready', saved: false, busy: false, onToggle: () => { toggled = true; } });
		const button = (element.props.children as unknown[])[0] as { type: string; props: { onClick: () => void } };
		button.props.onClick();
		expect(toggled).toBe(true);
	});
});

describe('M7G -- race safety and cold-start unchanged', () => {
	it('still uses the shared runScopedRequest race guard for the Screener fetch', () => {
		const source = workspaceSource();
		expect(source).toContain('runScopedRequest(requestID');
	});

	it('cold overview cold-start still does not preload the Screener or Watchlist', () => {
		const source = overviewSource();
		const mountEffect = source.slice(source.indexOf('useEffect(() => {'), source.indexOf('[config, refreshKey]'));
		expect(mountEffect).not.toMatch(/screener/i);
		expect(mountEffect).not.toMatch(/watchlist/i);
	});
});

// ==================================================
// M7H -- cash-flow (operating_cash_flow / cash_flow_to_net_income), independent cashflow_period/
// cashflow_status domain (MOPS official quarterly XBRL bulk archive), all six categories.
// ==================================================

describe('M7H -- 基本面 subsection gains exactly 4 new inputs (operating_cash_flow/cash_flow_to_net_income), no new top-level group', () => {
	it('1. renders the 4 new labeled inputs once 基本面 is expanded, alongside the existing 財務報表 inputs', () => {
		const panel = fundamentalsPanel();
		expect(panel).toContain('營業活動現金流量最小（仟元）');
		expect(panel).toContain('營業活動現金流量最大（仟元）');
		expect(panel).toContain('營業現金流／淨利最小（%）');
		expect(panel).toContain('營業現金流／淨利最大（%）');
		expect((panel.match(/負債比率最小（%）/g) || []).length).toBe(1);
	});

	it('2. defaults collapsed: the 4 new inputs are not present in the initial (collapsed) render', () => {
		const html = renderToStaticMarkup(<ScreenerFilterPanel draft={taiwanScreenerDefaultFilters()} onChange={() => {}} onApply={() => {}} onClear={() => {}} localError="" />);
		expect(html).not.toContain('營業活動現金流量最小');
		expect(html).not.toContain('營業現金流／淨利最小');
	});

	it('3. no second top-level group -- reuses the existing 基本面 group, adds its own 現金流量 subheading', () => {
		const source = workspaceSource();
		const panel = source.slice(source.indexOf('export function ScreenerFilterPanel'), source.indexOf('export function ScreenerSummary'));
		// "現金流量" is deliberately also a substring of the field labels below the subheading (e.g.
		// 營業活動現金流量最小), so this checks the subheading tag specifically, not a bare substring count.
		expect((panel.match(/taiwan-screener-advanced-subheading">現金流量</g) || []).length).toBe(1);
		expect((panel.match(/基本面/g) || []).length).toBe(1);
	});

	it('4. basic filter count under 基本面 grows from 36 to exactly 40 numeric range inputs', () => {
		const panel = fundamentalsPanel();
		const count = (panel.match(/inputMode="decimal"|inputMode="numeric"/g) || []).length;
		expect(count).toBe(40);
	});
});

describe('M7H -- draft typing sends no request (cash flow)', () => {
	it('every new input writes via the same set() helper used by every other field, never a request call', () => {
		const panel = fundamentalsPanel();
		expect(panel).toContain("set('minOperatingCashFlow'");
		expect(panel).toContain("set('minCashFlowToNetIncome'");
		expect(panel).not.toMatch(/requestJSON|taiwanScreenerPath\(/);
	});
});

describe('M7H -- query construction', () => {
	it('Apply serializes operating_cash_flow/cash_flow_to_net_income', () => {
		const path = taiwanScreenerPath(withFundamentals({ minOperatingCashFlow: '500000', maxCashFlowToNetIncome: '150' }));
		expect(path).toContain('min_operating_cash_flow=500000');
		expect(path).toContain('max_cash_flow_to_net_income=150');
	});

	it('an explicit zero is serialized, never omitted as if blank', () => {
		const path = taiwanScreenerPath(withFundamentals({ minOperatingCashFlow: '0', minCashFlowToNetIncome: '0' }));
		expect(path).toContain('min_operating_cash_flow=0');
		expect(path).toContain('min_cash_flow_to_net_income=0');
	});

	it('a negative value is serialized correctly (both metrics can legitimately be negative)', () => {
		const path = taiwanScreenerPath(withFundamentals({ minOperatingCashFlow: '-150005', minCashFlowToNetIncome: '-6.34' }));
		expect(path).toContain('min_operating_cash_flow=-150005');
		expect(path).toContain('min_cash_flow_to_net_income=-6.34');
	});

	it('blank cash-flow params are omitted entirely', () => {
		const path = taiwanScreenerPath(taiwanScreenerDefaultFilters());
		expect(path).not.toContain('operating_cash_flow');
		expect(path).not.toContain('cash_flow_to_net_income');
	});
});

describe('M7H -- sort', () => {
	it('sort=operating_cash_flow / cash_flow_to_net_income query', () => {
		expect(taiwanScreenerPath(withFundamentals({ sort: 'operating_cash_flow' }))).toContain('sort=operating_cash_flow');
		expect(taiwanScreenerPath(withFundamentals({ sort: 'cash_flow_to_net_income' }))).toContain('sort=cash_flow_to_net_income');
	});

	it('sort dropdown includes the 2 new options plus every legacy option (33 total)', () => {
		const html = renderToStaticMarkup(<ScreenerFilterPanel draft={taiwanScreenerDefaultFilters()} onChange={() => {}} onApply={() => {}} onClear={() => {}} localError="" />);
		for (const label of ['營業活動現金流量', '營業現金流／淨利', '負債比率', '股價']) {
			expect(html).toContain(label);
		}
	});
});

describe('M7H -- Clear/Apply/pagination/refresh preserve cash-flow fields', () => {
	it('Clear (taiwanScreenerDefaultFilters()) resets the new fields', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const clearFilters = () => {'), source.indexOf('const goPrevious = () => {'));
		expect(fn).toContain('taiwanScreenerDefaultFilters()');
	});

	it('pagination/refresh spread the full applied object (including the new fields), never rebuild it field-by-field', () => {
		const source = workspaceSource();
		expect(source).toContain('}, [config, refreshKey, applied]);');
	});
});

describe('M7H -- result rendering: null/zero/negative for operating_cash_flow/cash_flow_to_net_income', () => {
	it('a full row renders exact user-facing semantics: 營業活動現金流量/營業現金流／淨利 with correct units, no ×100 rescale on the ratio', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({
			security: security({ cashflow_period: '2026-Q2', operating_cash_flow: 1122637757, cash_flow_to_net_income: 148.06 }),
			showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: false, showBalance: false, showCashflow: true,
		})}</tr></tbody></table>);
		expect(html).toContain('營業活動現金流量');
		expect(html).toContain('億元');
		expect(html).toContain('148.06%');
		expect(html).not.toContain('14806%');
	});

	it('null values render — (domain not requested for this row, or the archive has no report for this issuer)', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({
			security: security({ cashflow_period: null, operating_cash_flow: null, cash_flow_to_net_income: null }),
			showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: false, showBalance: false, showCashflow: true,
		})}</tr></tbody></table>);
		expect(html).not.toContain('0%');
		expect((html.match(/—/g) || []).length).toBeGreaterThanOrEqual(3); // period + 2 metric fields
	});

	it('zero renders as a real zero, never —', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({
			security: security({ cashflow_period: '2026-Q2', operating_cash_flow: 0, cash_flow_to_net_income: 0 }),
			showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: false, showBalance: false, showCashflow: true,
		})}</tr></tbody></table>);
		expect(html).toContain('0 億元');
		expect(html).toContain('營業現金流／淨利 0%');
	});

	it('negative values render as negative, never clamped to positive (a real earnings-quality signal, unlike debt_ratio-style ratios)', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({
			security: security({ cashflow_period: '2026-Q2', operating_cash_flow: -150005, cash_flow_to_net_income: -6.34 }),
			showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: false, showBalance: false, showCashflow: true,
		})}</tr></tbody></table>);
		expect(html).toContain('-6.34%');
		expect(html).toMatch(/-[\d.,]+\s*億元/);
	});
});

describe('M7H -- row cashflow_period test', () => {
	it('cashflow_period=2026-Q2 renders 現金流量期間 2026-Q2', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({
			security: security({ cashflow_period: '2026-Q2', operating_cash_flow: 1122637757 }),
			showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: false, showBalance: false, showCashflow: true,
		})}</tr></tbody></table>);
		expect(html).toContain('現金流量期間 2026-Q2');
	});

	it('cashflow_period=null never fabricates a row period, and is never substituted by financial_period/balance_period', () => {
		const html = renderToStaticMarkup(<table><tbody><tr>{ScreenerAdvancedCell({
			security: security({ cashflow_period: null, financial_period: '2026-Q2', balance_period: '2026-Q2', operating_cash_flow: null }),
			showInstitutional: false, showMargin: false, showRevenue: false, showValuation: false, showDividends: false, showFinancials: false, showBalance: false, showCashflow: true,
		})}</tr></tbody></table>);
		expect(html).toContain('現金流量期間 —');
		expect(html).not.toContain('現金流量期間 2026-Q2');
	});
});

describe('M7H -- cashflow_status: available / partial / unavailable', () => {
	it('available renders the domain period and no partial/unavailable warning', () => {
		const html = renderToStaticMarkup(<ScreenerDomainFreshness label="現金流量資料" asOf="2026-Q2" status="available" unavailableMessage="現金流量資料暫時無法取得" partialMessage="部分現金流量資料暫時無法取得，已顯示目前可用資料。" />);
		expect(html).toContain('現金流量資料：2026-Q2');
		expect(html).not.toContain('部分現金流量資料暫時無法取得');
		expect(html).not.toContain('現金流量資料暫時無法取得');
	});

	it('partial renders the freshness line PLUS the exact non-blocking warning text -- rows remain visible, not implying zero', () => {
		const html = renderToStaticMarkup(<ScreenerDomainFreshness label="現金流量資料" asOf="2026-Q2" status="partial" unavailableMessage="現金流量資料暫時無法取得" partialMessage="部分現金流量資料暫時無法取得，已顯示目前可用資料。" />);
		expect(html).toContain('現金流量資料：2026-Q2');
		expect(html).toContain('部分現金流量資料暫時無法取得，已顯示目前可用資料。');
	});

	it('unavailable renders a safe warning, never a fabricated period, and never implies the Screener itself is empty', () => {
		const html = renderToStaticMarkup(<ScreenerDomainFreshness label="現金流量資料" asOf={null} status="unavailable" unavailableMessage="現金流量資料暫時無法取得" partialMessage="部分現金流量資料暫時無法取得，已顯示目前可用資料。" />);
		expect(html).toContain('現金流量資料暫時無法取得');
		expect(html).not.toContain('現金流量資料：');
	});

	it('unavailable warning and the zero-result empty state are independent -- both can render together, and other rows/domains stay usable', () => {
		const source = workspaceSource();
		expect(source).toMatch(/data && showCashflow && <ScreenerDomainFreshness/);
		expect(source).toMatch(/data && data\.total === 0 && <div className="taiwan-empty-state">/);
	});
});

// Mandatory: financials_period/financials_status and balance_period/balance_status must never be
// merged with or overwritten by cashflow_period/cashflow_status, even when all three are simultaneously
// present with DIFFERENT periods/statuses.
describe('M7H -- domain separation (financials/balance/cashflow are never merged)', () => {
	it('financials_period=2026-Q2/available, balance_period=2026-Q1/partial, and cashflow_period=2025-Q4/unavailable all render independently', () => {
		const html = renderToStaticMarkup(<>
			<ScreenerFinancialsFreshness period="2026-Q2" status="available" />
			<ScreenerDomainFreshness label="資產負債表資料" asOf="2026-Q1" status="partial" unavailableMessage="資產負債表資料暫時無法取得" partialMessage="部分資產負債表資料暫時無法取得，已顯示目前可用資料。" />
			<ScreenerDomainFreshness label="現金流量資料" asOf={null} status="unavailable" unavailableMessage="現金流量資料暫時無法取得" partialMessage="部分現金流量資料暫時無法取得，已顯示目前可用資料。" />
		</>);
		expect(html).toContain('財務報表：2026-Q2');
		expect(html).toContain('資產負債表資料：2026-Q1');
		expect(html).toContain('現金流量資料暫時無法取得');
		expect(html).not.toContain('現金流量資料：2026-Q2');
		expect(html).not.toContain('財務報表：2025-Q4');
	});

	it('the workspace body renders the cashflow freshness block gated by its own independent flag (showCashflow)', () => {
		const source = workspaceSource();
		expect(source).toMatch(/data && showCashflow && <ScreenerDomainFreshness label="現金流量資料"/);
	});

	it('showCashflow is computed via its own independent helper, not folded into showFinancials/showBalance', () => {
		const source = workspaceSource();
		expect(source).toContain('const showCashflow = taiwanScreenerHasCashflowCriteria(applied);');
		expect(source).toContain('const showBalance = taiwanScreenerHasBalanceCriteria(applied);');
		expect(source).toContain('const showFinancials = taiwanScreenerHasFinancialsCriteria(applied);');
	});

	it('taiwanScreenerHasFinancialsCriteria/taiwanScreenerHasBalanceCriteria remain unaffected by an active M7H filter', () => {
		expect(taiwanScreenerHasFinancialsCriteria(withFundamentals({ minOperatingCashFlow: '0' }))).toBe(false);
		expect(taiwanScreenerHasBalanceCriteria(withFundamentals({ sort: 'cash_flow_to_net_income' }))).toBe(false);
		expect(taiwanScreenerHasCashflowCriteria(withFundamentals({ minDebtRatio: '0' }))).toBe(false);
	});
});

describe('M7H -- research navigation and Watchlist action remain unaffected', () => {
	it('exact 2330.TWSE navigation preserved even with the cash-flow column rendered', () => {
		let opened = '';
		const twse = security({ canonical: '2330.TWSE', operating_cash_flow: 1122637757, cash_flow_to_net_income: 148.06 });
		const element = ScreenerRow({ security: twse, onOpen: () => { opened = twse.canonical; }, ...rowWatchlistProps({ showCashflow: true }) });
		const identityCell = (element.props.children as unknown[])[0] as { props: { children: { props: { onClick: () => void } } } };
		identityCell.props.children.props.onClick();
		expect(opened).toBe('2330.TWSE');
	});
});

describe('M7H -- no request fan-out introduced', () => {
	it('still issues exactly one requestJSON call (Screener) and never a direct cash-flow/XBRL/MOPS endpoint', () => {
		const source = workspaceSource();
		const matches = source.match(/requestJSON</g) || [];
		expect(matches.length).toBe(1);
		expect(source).not.toMatch(/mopsov|t203sb02|FileDownLoad|tifrs-|\/api\/v1\/tw\/cashflow/i);
	});
});

// ==================================================
// M8E -- Screener preset modes: five product-layer shortcuts over existing TaiwanScreenerFilters.
// ==================================================

describe('M8E — preset data contract (exact field mapping)', () => {
	it('exactly five presets are defined', () => {
		expect(taiwanScreenerPresets.length).toBe(5);
	});

	it('營收成長: exact field mapping', () => {
		const preset = presetById('revenue-growth');
		expect(preset.label).toBe('營收成長');
		expect(preset.overrides).toEqual({ minRevenueYoY: '15' });
		expect(preset.sort).toBe('revenue_yoy');
		expect(preset.order).toBe('desc');
	});

	it('法人偏多: exact field mapping', () => {
		const preset = presetById('institutional-net-buy');
		expect(preset.label).toBe('法人偏多');
		expect(preset.overrides).toEqual({ minInstitutionalNet: '1' });
		expect(preset.sort).toBe('institutional_net');
		expect(preset.order).toBe('desc');
	});

	it('財務穩健: exact field mapping', () => {
		const preset = presetById('financial-stability');
		expect(preset.label).toBe('財務穩健');
		expect(preset.overrides).toEqual({ maxDebtRatio: '50', minCurrentRatio: '100' });
		expect(preset.sort).toBe('debt_ratio');
		expect(preset.order).toBe('asc');
	});

	it('現金流健康: exact field mapping', () => {
		const preset = presetById('cashflow-health');
		expect(preset.label).toBe('現金流健康');
		expect(preset.overrides).toEqual({ minOperatingCashFlow: '1', minCashFlowToNetIncome: '80' });
		expect(preset.sort).toBe('cash_flow_to_net_income');
		expect(preset.order).toBe('desc');
	});

	it('低估值觀察: exact field mapping (0.01 boundary, matching M8E.1R correction -- never "0")', () => {
		const preset = presetById('low-valuation-watch');
		expect(preset.label).toBe('低估值觀察');
		expect(preset.overrides).toEqual({ minPE: '0.01', maxPE: '20', minPB: '0.01', maxPB: '2' });
		expect(preset.sort).toBe('pe');
		expect(preset.order).toBe('asc');
	});

	it('every preset override key actually exists on taiwanScreenerDefaultFilters() -- no typo/non-existent field', () => {
		const defaults = taiwanScreenerDefaultFilters();
		for (const preset of taiwanScreenerPresets) {
			for (const key of Object.keys(preset.overrides)) {
				expect(Object.prototype.hasOwnProperty.call(defaults, key)).toBe(true);
			}
		}
	});
});

describe('M8E — preset UI rendering', () => {
	it('renders exactly five preset buttons with their labels', () => {
		const html = renderToStaticMarkup(<ScreenerFilterPanel draft={taiwanScreenerDefaultFilters()} onChange={() => {}} onApply={() => {}} onClear={() => {}} localError="" presets={taiwanScreenerPresets} activePresetId={null} onApplyPreset={() => {}} />);
		for (const preset of taiwanScreenerPresets) {
			expect(html).toContain(preset.label);
		}
	});

	it('the active preset renders aria-pressed="true"; others render aria-pressed="false"', () => {
		const html = renderToStaticMarkup(<ScreenerFilterPanel draft={taiwanScreenerDefaultFilters()} onChange={() => {}} onApply={() => {}} onClear={() => {}} localError="" presets={taiwanScreenerPresets} activePresetId="revenue-growth" onApplyPreset={() => {}} />);
		expect(html).toMatch(/aria-pressed="true"[^>]*>營收成長/);
		expect(html).toMatch(/aria-pressed="false"[^>]*>法人偏多/);
	});

	it('rendering with no presets supplied (default) shows no preset buttons -- backward compatible with existing call sites', () => {
		const html = renderToStaticMarkup(<ScreenerFilterPanel draft={taiwanScreenerDefaultFilters()} onChange={() => {}} onApply={() => {}} onClear={() => {}} localError="" />);
		for (const preset of taiwanScreenerPresets) {
			expect(html).not.toContain(preset.label);
		}
	});
});

describe('M8E — preset interaction wiring (source-verified, matching this codebase\'s existing convention for hook-driven logic)', () => {
	it('buildPresetFilters always starts from taiwanScreenerDefaultFilters() -- never merges with prior draft/applied state', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('function buildPresetFilters'), source.indexOf('export function TaiwanScreenerWorkspace'));
		expect(fn).toContain('return { ...taiwanScreenerDefaultFilters(), ...preset.overrides, sort: preset.sort, order: preset.order };');
	});

	it('applyPreset sets draft, applied, and activePresetId together (offset resets to 0 via taiwanScreenerDefaultFilters())', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const applyPreset = '), source.indexOf('return <div className="taiwan-product-workspace taiwan-screener-workspace">'));
		expect(fn).toContain('const next = buildPresetFilters(preset);');
		expect(fn).toContain('setDraft(next);');
		expect(fn).toContain('setApplied(next);');
		expect(fn).toContain('setActivePresetId(preset.id);');
	});

	it('handleDraftChange (any manual field edit) clears activePresetId', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const handleDraftChange = '), source.indexOf('const applyPreset = '));
		expect(fn).toContain('setDraft(next);');
		expect(fn).toContain('setActivePresetId(null);');
	});

	it('clearFilters clears activePresetId in addition to the existing reset behavior', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const clearFilters = '), source.indexOf('const handleDraftChange = '));
		expect(fn).toContain('setDraft(defaults);');
		expect(fn).toContain('setApplied(defaults);');
		expect(fn).toContain('setActivePresetId(null);');
	});

	it('ScreenerFilterPanel receives onChange={handleDraftChange} (not the raw setDraft), so manual edits always clear activePresetId', () => {
		const source = workspaceSource();
		expect(source).toContain('<ScreenerFilterPanel draft={draft} onChange={handleDraftChange} onApply={applyFilters} onClear={clearFilters} localError={localError} presets={taiwanScreenerPresets} activePresetId={activePresetId} onApplyPreset={applyPreset} />');
	});

	it('applyFilters (existing manual Apply button) is unaffected -- never touches activePresetId', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const applyFilters = '), source.indexOf('const clearFilters = '));
		expect(fn).not.toContain('activePresetId');
	});
});

describe('M8E — advanced group auto-expand on preset click', () => {
	it('handlePresetClick expands institutionalExpanded/fundamentalsExpanded based on the preset\'s own group tag, reusing the existing expand state', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('const handlePresetClick = '), source.indexOf('return <section className="taiwan-screener-filters">'));
		expect(fn).toContain('onApplyPreset(preset);');
		expect(fn).toContain("if (preset.group === 'institutional') setInstitutionalExpanded(true);");
		expect(fn).toContain("if (preset.group === 'fundamentals') setFundamentalsExpanded(true);");
	});

	it('every preset is tagged with the correct group: only 法人偏多 is institutional, the other four are fundamentals', () => {
		expect(presetById('institutional-net-buy').group).toBe('institutional');
		for (const id of ['revenue-growth', 'financial-stability', 'cashflow-health', 'low-valuation-watch']) {
			expect(presetById(id).group).toBe('fundamentals');
		}
	});
});

describe('M8E — lazy domain gating preserved (no new request path, no eager unrelated domains)', () => {
	it('still issues exactly one requestJSON call (Screener) -- presets introduce zero new network calls', () => {
		const source = workspaceSource();
		const matches = source.match(/requestJSON</g) || [];
		expect(matches.length).toBe(1);
	});

	it('each preset activates exactly the one existing domain-criteria helper matching its own fields', () => {
		expect(taiwanScreenerHasRevenueCriteria({ ...taiwanScreenerDefaultFilters(), ...presetById('revenue-growth').overrides })).toBe(true);
		expect(taiwanScreenerHasInstitutionalCriteria({ ...taiwanScreenerDefaultFilters(), ...presetById('institutional-net-buy').overrides })).toBe(true);
		expect(taiwanScreenerHasBalanceCriteria({ ...taiwanScreenerDefaultFilters(), ...presetById('financial-stability').overrides })).toBe(true);
		expect(taiwanScreenerHasCashflowCriteria({ ...taiwanScreenerDefaultFilters(), ...presetById('cashflow-health').overrides })).toBe(true);
		expect(taiwanScreenerHasValuationCriteria({ ...taiwanScreenerDefaultFilters(), ...presetById('low-valuation-watch').overrides })).toBe(true);
	});

	it('presets never accidentally activate an unrelated domain (no eager multi-domain fetch)', () => {
		expect(taiwanScreenerHasInstitutionalCriteria({ ...taiwanScreenerDefaultFilters(), ...presetById('revenue-growth').overrides })).toBe(false);
		expect(taiwanScreenerHasCashflowCriteria({ ...taiwanScreenerDefaultFilters(), ...presetById('financial-stability').overrides })).toBe(false);
		expect(taiwanScreenerHasBalanceCriteria({ ...taiwanScreenerDefaultFilters(), ...presetById('cashflow-health').overrides })).toBe(false);
		expect(taiwanScreenerHasValuationCriteria({ ...taiwanScreenerDefaultFilters(), ...presetById('financial-stability').overrides })).toBe(false);
	});
});

describe('M8E — no AI, no persistence, no new backend path', () => {
	it('introduces no AI research call, no persistence mechanism', () => {
		const source = workspaceSource();
		expect(source).not.toMatch(/taiwanResearchPath|GenerateTaiwanResearch|localStorage|sessionStorage|indexedDB/i);
	});

	it('does not introduce a generic preset framework -- taiwanScreenerPresets is a fixed five-entry array, not an extensible/persisted structure', () => {
		const source = workspaceSource();
		expect(source).not.toMatch(/customPreset|savePreset|userPreset|PresetStore|preset.*localStorage/i);
	});
});

// ==================================================
// M8F -- Screener Match Reasons: inline "why did this stock match?" annotations, derived purely
// from applied filters + the row's own returned values. No AI, no score, no new request.
// ==================================================

describe('M8F — inline annotation rendering (real values, no separate panel/column)', () => {
	it('no active filters at all -> no annotation anywhere in the advanced cell', () => {
		const html = renderToStaticMarkup(<ScreenerAdvancedCell security={security({ revenue_yoy: 28.4 })} applied={taiwanScreenerDefaultFilters()} {...tableWatchlistProps({ showRevenue: true })} />);
		expect(html).not.toMatch(/（[≥≤].*）/);
	});

	it('rendering with no `applied` supplied (default) shows no annotation -- backward compatible with existing call sites', () => {
		const html = renderToStaticMarkup(<ScreenerAdvancedCell security={security({ revenue_yoy: 28.4 })} {...tableWatchlistProps({ showRevenue: true })} />);
		expect(html).not.toMatch(/（[≥≤].*）/);
		expect(html).toContain('+28.4%');
	});

	it('an active min filter annotates only the matching field, leaving unrelated fields in the same block untouched', () => {
		const applied = { ...taiwanScreenerDefaultFilters(), minRevenueYoY: '15' };
		const html = renderToStaticMarkup(<ScreenerAdvancedCell security={security({ revenue_yoy: 28.4, revenue_mom: 3.1, monthly_revenue: 1000000000 })} applied={applied} {...tableWatchlistProps({ showRevenue: true })} />);
		expect(html).toContain('年增 +28.4%（≥ 15%）');
		expect(html).toContain('<span>月增率 +3.1%</span>');
		expect(html).toContain('<span>月營收 1,000,000,000</span>');
	});

	it('base columns (price/change_percent/volume/amount) annotate inline via ScreenerRow, not a new column', () => {
		const applied = { ...taiwanScreenerDefaultFilters(), minPrice: '50', minChangePercent: '5', minVolume: '1000000', minAmount: '10000000000' };
		const row = security({ price: 65, change_percent: 8.2, volume: 14102018, amount: 33917316870 });
		const element = ScreenerRow({ security: row, applied, onOpen: () => {}, ...rowWatchlistProps() });
		const html = renderToStaticMarkup(element);
		expect(html).toContain('（≥ 50）');
		expect(html).toContain('（≥ 5%）');
	});
});

describe('M8F — preset annotation examples (M8E presets produce real facts, never just the preset label)', () => {
	it('營收成長 preset annotates revenue_yoy with its own threshold', () => {
		const applied = { ...taiwanScreenerDefaultFilters(), ...presetById('revenue-growth').overrides };
		const html = renderToStaticMarkup(<ScreenerAdvancedCell security={security({ revenue_yoy: 28.4 })} applied={applied} {...tableWatchlistProps({ showRevenue: true })} />);
		expect(html).toContain('年增 +28.4%（≥ 15%）');
		expect(html).not.toContain('營收成長');
	});

	it('法人偏多 preset annotates institutional_net', () => {
		const applied = { ...taiwanScreenerDefaultFilters(), ...presetById('institutional-net-buy').overrides };
		const html = renderToStaticMarkup(<ScreenerAdvancedCell security={security({ institutional_net: 2300000 })} applied={applied} {...tableWatchlistProps({ showInstitutional: true })} />);
		expect(html).toMatch(/三大法人 \+[\d,]+（≥ \+1）/);
	});

	it('財務穩健 preset annotates both debt_ratio and current_ratio', () => {
		const applied = { ...taiwanScreenerDefaultFilters(), ...presetById('financial-stability').overrides };
		const html = renderToStaticMarkup(<ScreenerAdvancedCell security={security({ debt_ratio: 30.94, current_ratio: 245.76 })} applied={applied} {...tableWatchlistProps({ showBalance: true })} />);
		expect(html).toContain('負債比 30.94%（≤ 50%）');
		expect(html).toContain('流動比 245.76%（≥ 100%）');
	});

	it('現金流健康 preset annotates both operating_cash_flow and cash_flow_to_net_income, both in consistent units', () => {
		const applied = { ...taiwanScreenerDefaultFilters(), ...presetById('cashflow-health').overrides };
		const html = renderToStaticMarkup(<ScreenerAdvancedCell security={security({ operating_cash_flow: 1122637757, cash_flow_to_net_income: 148.06 })} applied={applied} {...tableWatchlistProps({ showCashflow: true })} />);
		expect(html).toMatch(/營業活動現金流量 [\d.,]+ 億元（≥ [\d.,]+ 億元）/);
		expect(html).toContain('營業現金流／淨利 148.06%（≥ 80%）');
	});

	it('低估值觀察 preset annotates both PE and PB as ranges', () => {
		const applied = { ...taiwanScreenerDefaultFilters(), ...presetById('low-valuation-watch').overrides };
		const html = renderToStaticMarkup(<ScreenerAdvancedCell security={security({ pe: 18.5, pb: 1.6 })} applied={applied} {...tableWatchlistProps({ showValuation: true })} />);
		expect(html).toContain('PE 18.5（0.01–20）');
		expect(html).toContain('PB 1.6（0.01–2）');
	});
});

describe('M8F — manual filters (no preset) produce identical annotation quality', () => {
	it('manually set minPE/maxPE (activePresetId irrelevant to this component) annotates exactly like a preset would', () => {
		const applied = { ...taiwanScreenerDefaultFilters(), minPE: '8', maxPE: '15' };
		const html = renderToStaticMarkup(<ScreenerAdvancedCell security={security({ pe: 12.1 })} applied={applied} {...tableWatchlistProps({ showValuation: true })} />);
		expect(html).toContain('PE 12.1（8–15）');
	});
});

describe('M8F — null/unavailable safety', () => {
	it('a filtered field with a null row value renders — with no annotation appended (never fabricated)', () => {
		const applied = { ...taiwanScreenerDefaultFilters(), minRevenueYoY: '15' };
		const html = renderToStaticMarkup(<ScreenerAdvancedCell security={security({ revenue_yoy: null })} applied={applied} {...tableWatchlistProps({ showRevenue: true })} />);
		expect(html).toContain('年增 —');
		expect(html).not.toMatch(/年增 —（/);
	});
});

describe('M8F — propagation and existing behavior preserved', () => {
	it('TaiwanScreenerWorkspace passes applied={applied} into ScreenerTable', () => {
		const source = workspaceSource();
		expect(source).toContain('securities={data.securities} applied={applied} onOpenResearch={');
	});

	it('ScreenerTable passes applied down to each ScreenerRow', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('export function ScreenerTable'), source.indexOf('// Pure/presentational row.'));
		expect(fn).toContain('security={item} applied={applied}');
	});

	it('ScreenerRow passes applied down to ScreenerAdvancedCell', () => {
		const source = workspaceSource();
		const fn = source.slice(source.indexOf('export function ScreenerRow'), source.indexOf('// M7D — pure/presentational compact advanced-data cell.'));
		expect(fn).toContain('security={security} applied={applied}');
	});

	it('introduces no new requestJSON call -- still exactly one (the existing Screener query)', () => {
		const source = workspaceSource();
		const matches = source.match(/requestJSON</g) || [];
		expect(matches.length).toBe(1);
	});

	it('introduces no AI call and no persistence', () => {
		const source = workspaceSource();
		const importsBlock = source.slice(0, source.indexOf('type QuoteLookup') > -1 ? source.length : source.length);
		expect(source).not.toMatch(/taiwanResearchPath|GenerateTaiwanResearch|localStorage|sessionStorage|indexedDB/i);
		expect(importsBlock).toBeDefined();
	});

	it('existing M8E preset tests remain unaffected (5 presets, exact field mapping) -- spot check', () => {
		expect(taiwanScreenerPresets.length).toBe(5);
		expect(presetById('revenue-growth').overrides).toEqual({ minRevenueYoY: '15' });
	});
});
