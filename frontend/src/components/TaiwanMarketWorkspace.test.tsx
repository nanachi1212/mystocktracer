import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import { FreshnessHeader, type CommonScope } from './TaiwanMarketWorkspace';

const baseScope = (overrides: Partial<CommonScope> = {}): CommonScope => ({
	scope: 'combined', status: 'available', freshness: 'current', as_of: '2026-09-04',
	target_latest_trading_date: '2026-09-04', included_exchanges: ['twse', 'tpex'], missing_exchanges: [],
	...overrides,
});

describe('FreshnessHeader (backend connected vs Taiwan data available)', () => {
	it('shows an explicit unavailable state instead of implying the request succeeded', () => {
		const html = renderToStaticMarkup(<FreshnessHeader data={baseScope({ status: 'unavailable', freshness: 'unavailable', as_of: null })} />);
		expect(html).toContain('無法取得');
		expect(html).not.toContain('已連線');
	});

	it('shows partial and stale as their own distinct labels, not folded into available', () => {
		const partial = renderToStaticMarkup(<FreshnessHeader data={baseScope({ status: 'partial' })} />);
		expect(partial).toContain('部分資料');
		const stale = renderToStaticMarkup(<FreshnessHeader data={baseScope({ freshness: 'stale' })} />);
		expect(stale).toContain('資料較舊');
	});

	it('keeps 資料日期 (as_of) and 最新完成交易日 (target_latest_trading_date) as two distinct values when they differ', () => {
		const html = renderToStaticMarkup(<FreshnessHeader data={baseScope({ as_of: '2026-09-03', target_latest_trading_date: '2026-09-04' })} />);
		expect(html).toContain('資料日期 2026-09-03');
		expect(html).toContain('最新完成交易日 2026-09-04');
	});

	it('labels a missing as_of as not provided rather than silently substituting the trading day', () => {
		const html = renderToStaticMarkup(<FreshnessHeader data={baseScope({ as_of: null, target_latest_trading_date: '2026-09-04' })} />);
		expect(html).toContain('資料日期 未提供');
		expect(html).toContain('最新完成交易日 2026-09-04');
	});
});
