import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import type { Intelligence } from './TaiwanStockResearchWorkspace';
import { SubscriptionAIModal } from './SubscriptionAIModal';

const sampleIntelligence = (): Intelligence => ({
	model_version: 'taiwan_stock_intelligence_v1',
	symbol: '2330.TWSE',
	identity: {
		canonical_symbol: '2330.TWSE',
		code: '2330',
		name: '台積電',
		exchange: 'TWSE',
		currency: 'TWD',
		security_type: 'stock',
		industry_name: '半導體業',
	},
	quote: {
		status: 'available',
		freshness: 'fresh',
		as_of: '2026-09-04',
		data: { price: 1050, change_percent: 1.5 },
	},
	price_history_summary: {
		status: 'available',
		return_5d_percent: 2.34,
		return_20d_percent: 5.67,
		latest_bar_date: '2026-09-04',
	},
	fundamentals: {
		status: 'available',
		data: {
			valuation: { data_date: '2026-09-04', pe: 25.4, pb: 6.8, dividend_yield_percent: 1.8 },
			financial_statement: {
				fiscal_year: 2026,
				fiscal_quarter: 2,
				cumulative_eps: 18.5,
				gross_margin_percent: 53.2,
				operating_margin_percent: 42.1,
				net_margin_percent: 38.0,
				book_value_per_share: 154.2,
				debt_ratio_percent: 32.5,
				debt_to_equity_percent: 48.1,
				current_ratio_percent: 210.5,
				operating_cash_flow: 450000000,
				cash_flow_to_net_income: 125.3,
			},
		},
	},
	institutional: { status: 'available' },
	margin: { status: 'available' },
	market_context: { status: 'available', state: 'positive', confidence: 'high' },
	industry_context: { status: 'available' },
});

describe('SubscriptionAIModal', () => {
	it('renders modal structure with AI provider buttons and analysis types', () => {
		const html = renderToStaticMarkup(
			<SubscriptionAIModal
				intelligence={sampleIntelligence()}
				onClose={() => {}}
			/>
		);

		// Header and Stock info
		expect(html).toContain('使用已訂閱的 AI');
		expect(html).toContain('台積電');
		expect(html).toContain('2330');

		// AI Provider options
		expect(html).toContain('ChatGPT');
		expect(html).toContain('Claude');
		expect(html).toContain('Gemini');

		// Analysis types
		expect(html).toContain('綜合分析');
		expect(html).toContain('基本面分析');
		expect(html).toContain('技術 / 籌碼分析');

		// Evidence preview
		expect(html).toContain('即將提供給 AI 的資料');
		expect(html).toContain('僅包含目前選中證券之公開客觀證據');
		expect(html).toContain('2330.TWSE');
		expect(html).toContain('半導體業');
		expect(html).toContain('1,050 TWD');

		// Actions
		expect(html).toContain('複製並開啟 ChatGPT');
		expect(html).toContain('取消');
	});
});
