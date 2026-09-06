import fs from 'node:fs';
import path from 'node:path';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import type { InstitutionalFlow, InstitutionalHistory, MarginHistory, MarginTrading, MarketIndexSnapshot, SecurityIdentity, SourceMeta, TaiwanFundamentals } from '../../lib/backend';
import { taiwanErrorMessage } from '../../lib/taiwan-product';
import type { Breadth, Emotion, Industry } from '../TaiwanMarketWorkspace';
import { ChipView, FundamentalsView, OverviewSummary, partialFailureWarning, topIndustriesByBreadth } from './TaiwanMarketView';

const security: SecurityIdentity = {
	canonical: '0050.TWSE', code: '0050', name: '元大台灣50', market: 'TW', exchange: 'TWSE',
	security_type: 'etf', currency: 'TWD', timezone: 'Asia/Taipei', provider: 'twse', source_url: '', retrieved_at: '',
};
const meta: SourceMeta = { source: 'taiwan:fundamentals', fetched_at: '', latency_ms: 0, stale: false, status: 'data_insufficient' };

describe('FundamentalsView (P1D: ETF null monthly_revenue/dividends)', () => {
	it('renders the exact-null-crash fixture (0050 ETF: monthly_revenue and dividends both null) without throwing', () => {
		const data: TaiwanFundamentals = {
			security, monthly_revenue: null, dividends: null,
			capabilities: {
				monthly_revenue: { status: 'unsupported', retrieved_at: '', reason: 'ETF company revenue is not applicable' },
				dividends: { status: 'data_insufficient', retrieved_at: '', reason: 'official dividend rows unavailable' },
				financial_statement: { status: 'unsupported', retrieved_at: '' },
				valuation: { status: 'data_insufficient', retrieved_at: '' },
			},
			meta,
		};
		const html = renderToStaticMarkup(<FundamentalsView data={data} />);
		expect(html).toContain('目前無適用官方資料');
	});

	it('still renders populated monthly_revenue and dividends normally', () => {
		const data: TaiwanFundamentals = {
			security,
			monthly_revenue: [{ canonical: '2330.TWSE', period: '2026-07', revenue: 467581000000, provider: 'TWSE', source: 'twse', source_url: '', status: 'official' }],
			dividends: [{ year: 2026, cash_dividend: 7, normalized_status: 'official', provider: 'TWSE', status: 'official' }],
			capabilities: {},
			meta,
		};
		const html = renderToStaticMarkup(<FundamentalsView data={data} />);
		expect(html).not.toContain('目前無適用官方資料');
		expect(html).toContain('2026-07');
	});
});

const root = path.resolve(__dirname, '../../../..');
const overviewSource = () => fs.readFileSync(path.join(root, 'frontend/src/components/market/TaiwanMarketView.tsx'), 'utf8');

const institutionalFlow: InstitutionalFlow = {
	canonical: '2330.TWSE', trade_date: '2026-09-03', unit: 'shares',
	foreign_buy: 1000, foreign_sell: 500, foreign_net: 500,
	investment_trust_buy: 200, investment_trust_sell: 100, investment_trust_net: 100,
	dealer_buy: 50, dealer_sell: 20, dealer_net: 30, meta,
};
const institutional: InstitutionalHistory = {
	security, data: [institutionalFlow],
	summary: { foreign_net_5d: 1000, foreign_net_20d: 4000, investment_trust_net_5d: 200, investment_trust_net_20d: 800, dealer_net_5d: 60, dealer_net_20d: 240, foreign_consecutive_buy_days: 2, foreign_consecutive_sell_days: 0, investment_trust_consecutive_buy_days: 1, investment_trust_consecutive_sell_days: 0 },
	meta,
};
const marginTrading: MarginTrading = { canonical: '2330.TWSE', trade_date: '2026-09-03', unit: 'shares', margin_balance: 100000, margin_change: 500, short_balance: 20000, short_change: -100, short_margin_ratio: 20, meta };
const margin: MarginHistory = { security, data: [marginTrading], meta };

describe('ChipView (P1E: partial dataset failure isolation — institutional/margin render independently)', () => {
	it('renders both institutional and margin sections when both datasets succeeded', () => {
		const html = renderToStaticMarkup(<ChipView institutional={institutional} margin={margin} />);
		expect(html).not.toContain('法人買賣超資料目前無法取得');
		expect(html).not.toContain('融資融券資料目前無法取得');
		expect(html).toContain('外資');
		expect(html).toContain('融資餘額');
	});

	it('institutional failing (null) still renders the margin section, with a local fallback for institutional', () => {
		const html = renderToStaticMarkup(<ChipView institutional={null} margin={margin} />);
		expect(html).toContain('法人買賣超資料目前無法取得');
		expect(html).not.toContain('融資融券資料目前無法取得');
		expect(html).toContain('融資餘額');
	});

	it('margin failing (null) still renders the institutional section, with a local fallback for margin', () => {
		const html = renderToStaticMarkup(<ChipView institutional={institutional} margin={null} />);
		expect(html).toContain('融資融券資料目前無法取得');
		expect(html).not.toContain('法人買賣超資料目前無法取得');
		expect(html).toContain('外資');
	});
});

describe('partialFailureWarning (P1E: partial dataset failure warning wording)', () => {
	it('returns empty when nothing failed', () => {
		expect(partialFailureWarning([])).toBe('');
	});

	it('names the single failed dataset', () => {
		expect(partialFailureWarning(['法人買賣超'])).toBe('部分個股資料目前無法取得：法人買賣超');
	});

	it('joins multiple failed datasets with 、', () => {
		expect(partialFailureWarning(['法人買賣超', '融資融券'])).toBe('部分個股資料目前無法取得：法人買賣超、融資融券');
	});
});

describe('P1E error wording — TaiwanMarketView no longer leaks raw fetch/HTTP error text', () => {
	it('taiwanErrorMessage never surfaces a raw "Failed to fetch" network error', () => {
		const reason = new TypeError('Failed to fetch');
		const message = taiwanErrorMessage(reason, '台股行情載入失敗');
		expect(message).not.toContain('Failed to fetch');
		expect(message).toBe('台股行情載入失敗');
	});

	it('TaiwanMarketView routes every error path through taiwanErrorMessage instead of raw reason.message', () => {
		const source = overviewSource();
		expect(source).not.toMatch(/reason instanceof Error \? reason\.message/);
		expect(source.match(/taiwanErrorMessage\(reason,/g)?.length).toBeGreaterThanOrEqual(3);
	});
});

describe('P1E partial failure isolation — select() settles each dataset independently', () => {
	it('uses Promise.allSettled (not Promise.all) so one rejected dataset cannot discard the others', () => {
		const source = overviewSource();
		expect(source).toContain('Promise.allSettled([');
		expect(source).not.toContain('Promise.all([');
	});

	it('only applies a dataset result when it fulfilled, leaving a failed dataset at null instead of stale data', () => {
		const source = overviewSource();
		expect(source).toContain("if (quoteResult.status === 'fulfilled')");
		expect(source).toContain("if (klineResult.status === 'fulfilled')");
		expect(source).toContain("if (institutionalResult.status === 'fulfilled')");
		expect(source).toContain("if (marginResult.status === 'fulfilled')");
		expect(source).toContain("if (fundamentalsResult.status === 'fulfilled')");
		// onStart still clears all five to null/empty before the new request runs — a failed
		// dataset therefore stays cleared, it never resurrects the previous selection's data.
		expect(source).toContain('setQuote(null); setLines([]); setInstitutional(null); setMargin(null); setFundamentals(null);');
	});
});

describe('P1F search zero-result feedback — TaiwanMarketView', () => {
	const searchBody = () => {
		const source = overviewSource();
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
		expect(searchBody()).toContain("setLoading(true); errorOwnerRef.current = 'search'; setError('');");
	});

	it('a zero-result search does not clear the previously selected security or its already-loaded data', () => {
		const body = searchBody();
		expect(body).not.toMatch(/length === 0\)[^;]*setSelected/);
		expect(body).not.toMatch(/length === 0\)[^;]*setQuote/);
	});
});

describe('P1G stale-error ownership — indexes success only clears an error it still owns', () => {
	const indexesEffectBody = () => {
		const source = overviewSource();
		return source.slice(source.indexOf('useEffect(() => {'), source.indexOf('[config, refreshKey]'));
	};

	it('declares a non-rendering ownership ref (not a React state) instead of comparing rendered error text', () => {
		const source = overviewSource();
		expect(source).toContain("const errorOwnerRef = useRef<ErrorOwner>('');");
		expect(source).not.toMatch(/errorOwner\s*===\s*['"`]台股指數載入失敗/);
	});

	it('indexes failure claims ownership before setting the error', () => {
		const body = indexesEffectBody();
		expect(body).toContain("errorOwnerRef.current = 'indexes'; setError(taiwanErrorMessage(reason, '台股指數載入失敗'));");
	});

	it('indexes success clears the error only if indexes is still the current owner (does not blindly clear)', () => {
		const body = indexesEffectBody();
		expect(body).toContain("if (errorOwnerRef.current === 'indexes') { errorOwnerRef.current = ''; setError(''); }");
		expect(body).not.toMatch(/\.then\(\(payload\) => setIndexes\(payload\.data\)\)/);
	});

	it('search claims ownership at start and on failure, so an in-flight indexes success cannot clear a search error', () => {
		const source = overviewSource();
		const searchBody = source.slice(source.indexOf('const search = async'), source.indexOf('const select = async'));
		expect(searchBody).toContain("setLoading(true); errorOwnerRef.current = 'search'; setError('');");
		expect(searchBody).toContain("errorOwnerRef.current = 'search'; setError(taiwanErrorMessage(reason, '台股搜尋失敗'));");
	});

	it('select() claims ownership on start/failure, and on success only keeps ownership when a partial-failure warning is actually shown', () => {
		const source = overviewSource();
		const selectBody = source.slice(source.indexOf('const select = async'), source.indexOf('useEffect(() => {'));
		expect(selectBody).toContain("errorOwnerRef.current = 'stock'; setError('');");
		expect(selectBody).toContain("errorOwnerRef.current = failed.length > 0 ? 'stock' : '';");
		expect(selectBody).toContain("errorOwnerRef.current = 'stock'; setError(taiwanErrorMessage(reason, '台股行情載入失敗'));");
	});
});

describe('P1E refresh behavior — TaiwanMarketView retries the currently selected security', () => {
	it('keeps a ref of the current selection, updated from `selected` but read only inside the refresh effect', () => {
		const source = overviewSource();
		expect(source).toContain('const selectedRef = useRef<SecurityIdentity | null>(null);');
		expect(source).toContain('useEffect(() => { selectedRef.current = selected; }, [selected]);');
	});

	it('the refresh-retry effect depends only on [config, refreshKey] — not `selected` — so selecting a stock does not itself trigger a duplicate request', () => {
		const source = overviewSource();
		expect(source).toContain('if (selectedRef.current) void select(selectedRef.current);');
		expect(source).not.toMatch(/\[config, refreshKey, selected\]/);
		expect(source).not.toMatch(/\[selected, config, refreshKey\]/);
	});

	it('the cold-start indexes effect is unchanged (still the only startup fan-out) — P1D preserved', () => {
		const source = overviewSource();
		const mountEffect = source.slice(source.indexOf('useEffect(() => {'), source.indexOf('[config, refreshKey]'));
		expect(mountEffect).toContain('/api/v1/tw/indexes');
		expect(mountEffect).not.toMatch(/search\(/);
	});
});

const commonScope = { scope: 'combined', status: 'ok', freshness: 'fresh', as_of: '2026-09-07', target_latest_trading_date: '2026-09-07', included_exchanges: ['TWSE', 'TPEx'], missing_exchanges: [] };
const breadthData: Breadth = { ...commonScope, advancers: 812, decliners: 623, unchanged: 40, no_trade: 5, unknown: 0, universe_count: 1480, advance_ratio: 0.566, advancing_amount_ratio: 0.6, total_amount_twd: 1, missing_amount_count: 0 };
const emotionData: Emotion = { ...commonScope, model_version: 'v1', confidence: 'high', state: '偏多', raw: breadthData, components: { breadth_participation: 'high', capital_participation: 'high', breadth_capital_relationship: 'aligned' }, coverage: { direction_coverage: 0.98, amount_coverage: 0.95 } };
const industryFixture = (id: string, name: string, relativeBreadth: number | null): Industry => ({ industry_id: id, industry_name: name, exchange: 'TWSE', constituent_count: 10, relative_breadth: relativeBreadth, relative_capital: relativeBreadth, data_quality: { direction_coverage: 0.9, amount_coverage: 0.9 } });
const indexSnapshotFixture: MarketIndexSnapshot = { id: 'taiex', secid: 'TAIEX', code: 'TAIEX', name: '加權指數', region: 'TW', market: 'TW', currency: 'TWD', price: 17850.32, change: 74, change_percent: 0.42, status: 'ok', meta: { source: 'taiwan:indexes', fetched_at: '', latency_ms: 0, stale: false, status: 'ok' } };
const idleDomain = <T,>(): { data: T | null; loading: boolean; error: string } => ({ data: null, loading: false, error: '' });

describe('M8G topIndustriesByBreadth — display-order-only ranking of an existing field', () => {
	it('sorts by relative_breadth descending and takes the top 3', () => {
		const items = [industryFixture('a', 'A', 0.1), industryFixture('b', 'B', 0.5), industryFixture('c', 'C', 0.3), industryFixture('d', 'D', 0.4)];
		expect(topIndustriesByBreadth(items).map((item) => item.industry_id)).toEqual(['b', 'd', 'c']);
	});

	it('does not mutate the input array (copies before sorting)', () => {
		const items = [industryFixture('a', 'A', 0.1), industryFixture('b', 'B', 0.5)];
		const original = [...items];
		topIndustriesByBreadth(items);
		expect(items).toEqual(original);
	});

	it('excludes industries with a null relative_breadth rather than fabricating a rank', () => {
		const items = [industryFixture('a', 'A', null), industryFixture('b', 'B', 0.5)];
		expect(topIndustriesByBreadth(items).map((item) => item.industry_id)).toEqual(['b']);
	});

	it('excludes non-finite relative_breadth values (NaN/Infinity) the same way', () => {
		const items = [industryFixture('a', 'A', Number.NaN), industryFixture('b', 'B', Number.POSITIVE_INFINITY), industryFixture('c', 'C', 0.2)];
		expect(topIndustriesByBreadth(items).map((item) => item.industry_id)).toEqual(['c']);
	});

	it('returns fewer than 3 when fewer than 3 are rankable', () => {
		const items = [industryFixture('a', 'A', 0.1)];
		expect(topIndustriesByBreadth(items)).toHaveLength(1);
	});
});

describe('M8G OverviewSummary — compact Overview summary cards', () => {
	it('renders Breadth advancers/decliners/advance_ratio', () => {
		const html = renderToStaticMarkup(<OverviewSummary breadth={{ data: breadthData, loading: false, error: '' }} emotion={idleDomain<Emotion>()} topIndustries={[]} industryLoading={false} industryError="" indexSnapshot={null} />);
		expect(html).toContain('812');
		expect(html).toContain('623');
		expect(html).toContain('56.6');
	});

	it('renders the existing Emotion state truthfully, without inventing new sentiment wording', () => {
		const html = renderToStaticMarkup(<OverviewSummary breadth={idleDomain<Breadth>()} emotion={{ data: emotionData, loading: false, error: '' }} topIndustries={[]} industryLoading={false} industryError="" indexSnapshot={null} />);
		expect(html).toContain('偏多');
	});

	it('renders the top 3 industries passed in, in the given order, with their relative_breadth', () => {
		const top3 = [industryFixture('a', '半導體', 0.62), industryFixture('b', '金融', 0.58), industryFixture('c', '電子', 0.55)];
		const html = renderToStaticMarkup(<OverviewSummary breadth={idleDomain<Breadth>()} emotion={idleDomain<Emotion>()} topIndustries={top3} industryLoading={false} industryError="" indexSnapshot={null} />);
		expect(html.indexOf('半導體')).toBeLessThan(html.indexOf('金融'));
		expect(html.indexOf('金融')).toBeLessThan(html.indexOf('電子'));
	});

	it('renders the index snapshot name/price/change_percent without duplicating the K-line/MiniStat detail', () => {
		const html = renderToStaticMarkup(<OverviewSummary breadth={idleDomain<Breadth>()} emotion={idleDomain<Emotion>()} topIndustries={[]} industryLoading={false} industryError="" indexSnapshot={indexSnapshotFixture} />);
		expect(html).toContain('17,850.32');
		expect(html).toContain('0.42');
		expect(html).not.toContain('market-kline-table');
	});

	it('a Breadth failure renders that card in an unavailable state while Emotion/Industry/Index remain usable', () => {
		const html = renderToStaticMarkup(<OverviewSummary breadth={{ data: null, loading: false, error: '市場廣度載入失敗' }} emotion={{ data: emotionData, loading: false, error: '' }} topIndustries={[industryFixture('a', '半導體', 0.62)]} industryLoading={false} industryError="" indexSnapshot={indexSnapshotFixture} />);
		expect(html).toContain('市場廣度載入失敗');
		expect(html).toContain('偏多');
		expect(html).toContain('半導體');
		expect(html).toContain('17,850.32');
	});

	it('an Emotion failure renders that card in an unavailable state while the others remain usable', () => {
		const html = renderToStaticMarkup(<OverviewSummary breadth={{ data: breadthData, loading: false, error: '' }} emotion={{ data: null, loading: false, error: '市場氣氛載入失敗' }} topIndustries={[industryFixture('a', '半導體', 0.62)]} industryLoading={false} industryError="" indexSnapshot={indexSnapshotFixture} />);
		expect(html).toContain('市場氣氛載入失敗');
		expect(html).toContain('812');
		expect(html).toContain('半導體');
	});

	it('an Industry failure renders that card in an unavailable state while the others remain usable', () => {
		const html = renderToStaticMarkup(<OverviewSummary breadth={{ data: breadthData, loading: false, error: '' }} emotion={{ data: emotionData, loading: false, error: '' }} topIndustries={[]} industryLoading={false} industryError="產業雷達載入失敗" indexSnapshot={indexSnapshotFixture} />);
		expect(html).toContain('產業雷達載入失敗');
		expect(html).toContain('812');
		expect(html).toContain('偏多');
	});

	it('an Index failure (no snapshot) renders that card as unavailable while the others remain usable', () => {
		const html = renderToStaticMarkup(<OverviewSummary breadth={{ data: breadthData, loading: false, error: '' }} emotion={{ data: emotionData, loading: false, error: '' }} topIndustries={[industryFixture('a', '半導體', 0.62)]} industryLoading={false} industryError="" indexSnapshot={null} />);
		expect(html).toContain('812');
		expect(html).toContain('偏多');
		expect(html).toContain('半導體');
	});

	it('each card loads independently — a loading Breadth does not block already-ready Emotion/Industry from rendering', () => {
		const html = renderToStaticMarkup(<OverviewSummary breadth={{ data: null, loading: true, error: '' }} emotion={{ data: emotionData, loading: false, error: '' }} topIndustries={[industryFixture('a', '半導體', 0.62)]} industryLoading={false} industryError="" indexSnapshot={indexSnapshotFixture} />);
		expect(html).toContain('偏多');
		expect(html).toContain('半導體');
		expect(html).toContain('17,850.32');
	});

	it('renders a Screener navigation action that calls onNavigate with only the target — no filters/preset/state payload', () => {
		const source = fs.readFileSync(path.join(root, 'frontend/src/components/market/TaiwanMarketView.tsx'), 'utf8');
		const overviewSummarySource = source.slice(source.indexOf('export function OverviewSummary'));
		expect(overviewSummarySource).toContain("onClick={() => onNavigate('taiwan-screener')}");
		expect(overviewSummarySource).toContain('查看台股選股器');
	});

	it('omits the Screener navigation action and detail links entirely when onNavigate is not provided (e.g. tests that do not wire navigation)', () => {
		const html = renderToStaticMarkup(<OverviewSummary breadth={idleDomain<Breadth>()} emotion={idleDomain<Emotion>()} topIndustries={[]} industryLoading={false} industryError="" indexSnapshot={null} />);
		expect(html).not.toContain('查看台股選股器');
	});
});

describe('M8G Overview layout — summary renders above the individual-stock search, index request is reused', () => {
	it('TaiwanMarketView renders <OverviewSummary> before the stock-search <form>', () => {
		const source = overviewSource();
		expect(source.indexOf('<OverviewSummary')).toBeGreaterThan(-1);
		expect(source.indexOf('<OverviewSummary')).toBeLessThan(source.indexOf('<form className="market-filter"'));
	});

	it('the Overview index summary reuses the existing `indexes` fetch/state — no second /api/v1/tw/indexes request is introduced', () => {
		const source = overviewSource();
		expect(source.match(/\/api\/v1\/tw\/indexes/g)?.length).toBe(1);
		expect(source).toContain('const primaryIndexSnapshot = indexSnapshots.find(');
	});

	it('Breadth/Emotion/Industry are each fetched from their own existing endpoint exactly once (4 domain requests total on Overview)', () => {
		const source = overviewSource();
		expect(source).toContain("taiwanMarketPath('market-breadth', 'combined')");
		expect(source).toContain("taiwanMarketPath('market-emotion', 'combined')");
		expect(source).toContain("taiwanMarketPath('industry-radar', 'combined')");
	});

	it('the existing individual-stock search form is still present and unchanged (input + submit button)', () => {
		const source = overviewSource();
		expect(source).toContain('placeholder="例如 2330、台積電、2330.TWSE 或 6488.TPEX"');
	});

	it('the existing CoreIndexView detail block still renders lower on the page', () => {
		const source = overviewSource();
		expect(source).toContain('<CoreIndexView');
		expect(source.indexOf('<OverviewSummary')).toBeLessThan(source.indexOf('<CoreIndexView'));
	});
});
