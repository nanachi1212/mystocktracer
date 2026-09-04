import fs from 'node:fs';
import path from 'node:path';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import type { InstitutionalFlow, InstitutionalHistory, MarginHistory, MarginTrading, SecurityIdentity, SourceMeta, TaiwanFundamentals } from '../../lib/backend';
import { taiwanErrorMessage } from '../../lib/taiwan-product';
import { ChipView, FundamentalsView, partialFailureWarning } from './TaiwanMarketView';

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
