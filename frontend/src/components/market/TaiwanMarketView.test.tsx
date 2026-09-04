import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import type { SecurityIdentity, SourceMeta, TaiwanFundamentals } from '../../lib/backend';
import { FundamentalsView } from './TaiwanMarketView';

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
