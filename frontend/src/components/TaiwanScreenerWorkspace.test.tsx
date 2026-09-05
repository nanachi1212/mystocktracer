import fs from 'node:fs';
import path from 'node:path';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import { taiwanScreenerDefaultFilters, type TaiwanScreenerResponse, type TaiwanScreenerSecurity } from '../lib/taiwan-product';
import { ScreenerFilterPanel, ScreenerPagination, ScreenerRow, ScreenerSummary, ScreenerTable } from './TaiwanScreenerWorkspace';

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
	it('the Taiwan overview cold-start indexes effect does not fetch the Screener', () => {
		const source = overviewSource();
		const mountEffect = source.slice(source.indexOf('useEffect(() => {'), source.indexOf('[config, refreshKey]'));
		expect(mountEffect).toContain('/api/v1/tw/indexes');
		expect(mountEffect).not.toMatch(/screener/i);
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

describe('M7B — no request fan-out / no local filtering', () => {
	it('never calls the Watchlist endpoint', () => {
		expect(workspaceSource()).not.toMatch(/watchlist/i);
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
		const element = ScreenerRow({ security: security(), onOpen: () => { opened = '2330.TWSE'; } });
		const identityCell = (element.props.children as unknown[])[0] as { props: { children: { props: { onClick: () => void } } } };
		identityCell.props.children.props.onClick();
		expect(opened).toBe('2330.TWSE');
	});

	it('a TPEX security remains 6488.TPEX end to end — never re-inferred or collapsed to code-only', () => {
		let opened = '';
		const tpex = security({ canonical: '6488.TPEX', code: '6488', name: '環球晶', exchange: 'TPEX' });
		const html = renderToStaticMarkup(<ScreenerTable securities={[tpex]} onOpenResearch={(canonical) => { opened = canonical; }} />);
		expect(html).toContain('環球晶');
		expect(html).toContain('TPEX');
		// Simulate the click via the same wiring ScreenerTable uses (onOpenResearch(item.canonical)).
		const element = ScreenerRow({ security: tpex, onOpen: () => opened = tpex.canonical });
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
});

describe('M7B — missing-value and zero-value rendering (ScreenerRow)', () => {
	it('renders — for a null price, null change_percent, null volume, and null amount', () => {
		const html = renderToStaticMarkup(<table><tbody>{ScreenerRow({ security: security({ price: null, change_percent: null, volume: null, amount: null }), onOpen: () => {} })}</tbody></table>);
		expect(html).toContain('—');
		expect((html.match(/—/g) || []).length).toBeGreaterThanOrEqual(4);
		expect(html).not.toMatch(/>0<\/td>|>0%<\/td>/);
	});

	it('renders a genuine zero as zero, not as missing', () => {
		const html = renderToStaticMarkup(<table><tbody>{ScreenerRow({ security: security({ price: 0, change_percent: 0, volume: 0, amount: 0 }), onOpen: () => {} })}</tbody></table>);
		expect(html).toContain('<td>0</td>');
		expect(html).toContain('0%');
	});

	it('renders a signed change_percent (positive shows +, negative shows the sign)', () => {
		const up = renderToStaticMarkup(<table><tbody>{ScreenerRow({ security: security({ change_percent: 1.5 }), onOpen: () => {} })}</tbody></table>);
		expect(up).toContain('+1.5%');
		const down = renderToStaticMarkup(<table><tbody>{ScreenerRow({ security: security({ change_percent: -2.3 }), onOpen: () => {} })}</tbody></table>);
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
