import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import { BreadthView, EmotionView, FreshnessHeader, IndustryView, type CommonScope } from './TaiwanMarketWorkspace';

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

describe('EmotionView render resilience', () => {
	const validEmotion: Parameters<typeof EmotionView>[0]['data'] = {
		scope: 'COMBINED',
		status: 'current',
		freshness: 'current',
		as_of: '2026-09-10',
		target_latest_trading_date: '2026-09-10',
		included_exchanges: ['TWSE', 'TPEX'],
		missing_exchanges: [],
		model_version: 'taiwan_emotion_v1',
		confidence: 'high',
		state: 'weak',
		raw: {
			scope: 'COMBINED',
			status: 'current',
			freshness: 'current',
			as_of: '2026-09-10',
			target_latest_trading_date: '2026-09-10',
			included_exchanges: ['TWSE', 'TPEX'],
			missing_exchanges: [],
			advancers: 300,
			decliners: 600,
			unchanged: 100,
			no_trade: 20,
			unknown: 0,
			universe_count: 1020,
			advance_ratio: 0.3,
			advancing_amount_ratio: 0.25,
			total_amount_twd: 350000000000,
			missing_amount_count: 0,
		},
		components: {
			breadth_participation: 'negative',
			capital_participation: 'negative',
			breadth_capital_relationship: 'aligned_negative',
		},
		coverage: {
			direction_coverage: 0.98,
			amount_coverage: 1.0,
		},
	};

	it('renders normal complete emotion data properly', () => {
		const html = renderToStaticMarkup(<EmotionView data={validEmotion} />);
		expect(html).toContain('市場狀態');
		expect(html).toContain('模型 taiwan_emotion_v1');
		expect(html).toContain('家數與成交方向一致偏弱');
		expect(html).toContain('30%');
		expect(html).toContain('25%');
	});

	it('renders safely without crashing when components, raw, or coverage are missing or undefined', () => {
		const corruptedEmotion = {
			...validEmotion,
			model_version: '',
			confidence: '',
			state: '',
			raw: undefined as unknown as typeof validEmotion.raw,
			components: undefined as unknown as typeof validEmotion.components,
			coverage: undefined as unknown as typeof validEmotion.coverage,
		};

		let html = '';
		expect(() => {
			html = renderToStaticMarkup(<EmotionView data={corruptedEmotion} />);
		}).not.toThrow();

		expect(html).toContain('未提供');
		expect(html).toContain('—');
	});

	it('renders safely when passed old Breadth data during view transition (cannot read properties of undefined breadth_participation)', () => {
		const staleBreadthData = {
			scope: 'COMBINED',
			status: 'current',
			freshness: 'current',
			as_of: '2026-09-10',
			target_latest_trading_date: '2026-09-10',
			included_exchanges: ['TWSE', 'TPEX'],
			missing_exchanges: [],
			advancers: 300,
			decliners: 600,
			unchanged: 100,
			no_trade: 20,
			unknown: 0,
			universe_count: 1020,
			advance_ratio: 0.3,
			advancing_amount_ratio: 0.25,
			total_amount_twd: 350000000000,
			missing_amount_count: 0,
		};

		// In React before mounting or transition, data may momentarily be stale Breadth
		expect(() => {
			renderToStaticMarkup(<EmotionView data={staleBreadthData as unknown as typeof validEmotion} />);
		}).not.toThrow();
	});
});

describe('BreadthView and IndustryView render resilience', () => {
	it('BreadthView handles missing and null data without throwing', () => {
		const partialBreadth = {
			scope: 'COMBINED',
			status: 'current',
			freshness: 'current',
			as_of: null,
			target_latest_trading_date: '2026-09-10',
			included_exchanges: [],
			missing_exchanges: [],
			advancers: undefined as unknown as number,
			decliners: undefined as unknown as number,
			unchanged: undefined as unknown as number,
			no_trade: undefined as unknown as number,
			unknown: undefined as unknown as number,
			universe_count: undefined as unknown as number,
			advance_ratio: null,
			advancing_amount_ratio: null,
			total_amount_twd: undefined as unknown as number,
			missing_amount_count: undefined as unknown as number,
		};
		expect(() => {
			renderToStaticMarkup(<BreadthView data={partialBreadth} />);
		}).not.toThrow();
	});

	it('IndustryView handles missing industries array and data_quality without throwing', () => {
		const partialIndustry = {
			scope: 'COMBINED',
			status: 'current',
			freshness: 'current',
			as_of: null,
			target_latest_trading_date: '2026-09-10',
			included_exchanges: [],
			missing_exchanges: [],
			snapshot_status: 'current',
			taxonomy_status: 'current',
			industry_coverage: null,
			classified_count: undefined as unknown as number,
			eligible_universe_count: undefined as unknown as number,
			industries: undefined as unknown as [],
		};
		expect(() => {
			renderToStaticMarkup(<IndustryView data={partialIndustry} />);
		}).not.toThrow();
	});
});
