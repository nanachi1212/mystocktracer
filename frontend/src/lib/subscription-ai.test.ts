import { describe, expect, it } from 'vitest';
import type { Intelligence } from '../components/TaiwanStockResearchWorkspace';
import {
	SUBSCRIPTION_AI_ANALYSIS_TYPES,
	SUBSCRIPTION_AI_PROVIDERS,
	buildEvidenceSummary,
	buildSubscriptionAIPrompt,
} from './subscription-ai';

const mockStockIntelligence = (): Intelligence => ({
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
		target_latest_completed_trading_date: '2026-09-04',
		data: {
			price: 1050,
			change_percent: 1.5,
			meta: { trade_date: '2026-09-04', is_realtime: false },
		},
	},
	price_history_summary: {
		status: 'available',
		freshness: 'fresh',
		return_5d_percent: 2.34,
		return_20d_percent: 5.67,
		latest_bar_date: '2026-09-04',
	},
	fundamentals: {
		status: 'available',
		freshness: 'fresh',
		as_of: '2026-08',
		data: {
			valuation: {
				data_date: '2026-09-04',
				pe: 25.4,
				pb: 6.8,
				dividend_yield_percent: 1.8,
			},
			financial_statement: {
				fiscal_year: 2026,
				fiscal_quarter: 2,
				accounting_category: 'ci',
				cumulative_eps: 18.5,
				gross_margin_percent: 53.2,
				operating_margin_percent: 42.1,
				net_margin_percent: 38.0,
				book_value_per_share: 154.2,
				balance_fiscal_year: 2026,
				balance_fiscal_quarter: 2,
				debt_ratio_percent: 32.5,
				debt_to_equity_percent: 48.1,
				current_ratio_percent: 210.5,
				cashflow_fiscal_year: 2026,
				cashflow_fiscal_quarter: 2,
				operating_cash_flow: 450000000,
				cash_flow_to_net_income: 125.3,
			},
		},
	},
	institutional: {
		status: 'available',
		freshness: 'fresh',
		as_of: '2026-09-04',
		reason: '三大法人同買',
	},
	margin: {
		status: 'available',
		freshness: 'fresh',
		as_of: '2026-09-04',
		reason: '資減券增',
	},
	market_context: {
		status: 'available',
		freshness: 'fresh',
		state: 'positive',
		confidence: 'high',
		advance_ratio: 0.62,
		advancing_amount_ratio: 0.71,
	},
	industry_context: {
		status: 'available',
		freshness: 'fresh',
		industry: {
			industry_name: '半導體業',
			relative_breadth: 0.15,
			relative_capital: 0.35,
		},
	},
	interpretation: {
		model_version: 'taiwan_stock_interpretation_v1',
		components: {
			price: { state: 'positive', status: 'available', freshness: 'fresh', as_of: '2026-09-04', reasons: ['短期均線多頭排列'] },
			fundamentals: { state: 'positive', status: 'available', freshness: 'fresh', as_of: '2026-08', reasons: ['營收創同期新高'] },
		},
		data_quality: {
			available_components: ['price', 'fundamentals'],
			indeterminate_components: null,
			unavailable_components: null,
			stale_components: null,
			partial_components: null,
		},
	},
});

describe('Subscription AI Providers & URLs', () => {
	it('defines the three official AI providers with exact allowed URLs', () => {
		expect(SUBSCRIPTION_AI_PROVIDERS).toHaveLength(3);
		const map = Object.fromEntries(SUBSCRIPTION_AI_PROVIDERS.map((p) => [p.id, p.url]));
		expect(map.chatgpt).toBe('https://chatgpt.com/');
		expect(map.claude).toBe('https://claude.ai/');
		expect(map.gemini).toBe('https://gemini.google.com/');
	});

	it('defines the three analysis types with comprehensive as default', () => {
		expect(SUBSCRIPTION_AI_ANALYSIS_TYPES.map((t) => t.id)).toEqual([
			'comprehensive',
			'fundamental',
			'technical_chips',
		]);
	});
});

describe('buildSubscriptionAIPrompt & buildEvidenceSummary', () => {
	it('builds comprehensive prompt with all expected sections and stock evidence', () => {
		const stock = mockStockIntelligence();
		const prompt = buildSubscriptionAIPrompt(stock, 'comprehensive');

		expect(prompt).toContain('你是台灣股票研究助理');
		expect(prompt).toContain('使用繁體中文');
		expect(prompt).toContain('區分已知事實與推論');
		expect(prompt).toContain('null / unavailable / partial / stale 必須保留原意');
		expect(prompt).toContain('不捏造缺失資料');
		expect(prompt).toContain('不保證收益');
		expect(prompt).toContain('不寫成確定的買進或賣出指令');
		expect(prompt).toContain('【分析方式：綜合分析】');
		expect(prompt).toContain('2330');
		expect(prompt).toContain('台積電');
		expect(prompt).toContain('2330.TWSE');
		expect(prompt).toContain('1,050 TWD');
		expect(prompt).toContain('半導體業');
	});

	it('builds fundamental analysis prompt with fundamental instructions', () => {
		const stock = mockStockIntelligence();
		const prompt = buildSubscriptionAIPrompt(stock, 'fundamental');

		expect(prompt).toContain('【分析方式：基本面分析】');
		expect(prompt).toContain('營收 / 獲利');
		expect(prompt).toContain('財務品質');
		expect(prompt).toContain('本益比 (PE): 25.4');
		expect(prompt).toContain('累計 EPS: 18.5');
	});

	it('builds technical/chips analysis prompt with technical instructions', () => {
		const stock = mockStockIntelligence();
		const prompt = buildSubscriptionAIPrompt(stock, 'technical_chips');

		expect(prompt).toContain('【分析方式：技術 / 籌碼分析】');
		expect(prompt).toContain('價格趨勢');
		expect(prompt).toContain('量價');
		expect(prompt).toContain('法人');
		expect(prompt).toContain('融資融券');
		expect(prompt).toContain('5 日收盤報酬率: +2.34%');
	});

	it('preserves null / unavailable without fabricating 0', () => {
		const stock = mockStockIntelligence();
		// Set some metrics to null
		stock.quote.data = undefined;
		stock.price_history_summary.return_5d_percent = null;
		stock.fundamentals.data = {
			valuation: { pe: null, pb: null, dividend_yield_percent: null },
			financial_statement: null,
		};
		stock.fundamentals.status = 'unavailable';

		const prompt = buildSubscriptionAIPrompt(stock, 'comprehensive');

		expect(prompt).toContain('報價數據: 未提供 (unavailable)');
		expect(prompt).toContain('5 日收盤報酬率: 未提供 (unavailable)');
		expect(prompt).toContain('本益比 (PE): 未提供 (unavailable)');
		expect(prompt).toContain('股價淨值比 (PB): 未提供 (unavailable)');
		expect(prompt).toContain('獲利能力資料: 未提供 (unavailable)');

		// Must NOT contain "0.00" or "0" for missing values
		expect(prompt).not.toMatch(/本益比 \(PE\): 0/);
		expect(prompt).not.toMatch(/5 日收盤報酬率: \+?0\.00%/);
	});

	it('properly notes ETF security_type, marks company statements as not_applicable, and does not output company metrics', () => {
		const etf = mockStockIntelligence();
		etf.identity.canonical_symbol = '0050.TWSE';
		etf.identity.code = '0050';
		etf.identity.name = '元大台灣50';
		etf.identity.security_type = 'etf';
		// Even if fundamentals.data somehow had financial_statement attached
		etf.fundamentals.data = {
			valuation: {
				data_date: '2026-09-04',
				pe: null,
				pb: null,
				dividend_yield_percent: 3.5,
			},
			financial_statement: {
				fiscal_year: 2026,
				fiscal_quarter: 2,
				accounting_category: 'ci',
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
		};

		const prompt = buildSubscriptionAIPrompt(etf, 'fundamental');

		expect(prompt).toContain('0050');
		expect(prompt).toContain('元大台灣50');
		expect(prompt).toContain('ETF (指數股票型基金)，不適用一般公司財務比率與損益資產表');
		expect(prompt).toContain('獲利能力 (EPS/毛利/營益/淨利): 不適用 (not_applicable)');
		expect(prompt).toContain('資產負債表 (負債比/流動比/每股淨值): 不適用 (not_applicable)');
		expect(prompt).toContain('現金流量表 (營業活動現金流): 不適用 (not_applicable)');

		// Must NOT output any company financial numbers for ETF
		expect(prompt).not.toContain('累計 EPS: 18.5');
		expect(prompt).not.toContain('毛利率 (%): +53.20%');
		expect(prompt).not.toContain('負債比率 (%): 32.50%');
		expect(prompt).not.toContain('營業活動現金流量');
		// But permitted valuation if present is preserved
		expect(prompt).toContain('殖利率 (%): 3.5%');
	});

	it('strictly isolates selected stock evidence without secrets, credentials or other stocks', () => {
		const stock = mockStockIntelligence();
		const prompt = buildSubscriptionAIPrompt(stock, 'comprehensive');

		// Privacy & security check: must NOT leak credentials, secrets, or unrelated stocks
		expect(prompt).not.toContain('2317');
		expect(prompt).not.toContain('鴻海');
		expect(prompt).not.toContain('api_key');
		expect(prompt).not.toContain('token');
		expect(prompt).not.toContain('cookie');
		expect(prompt).not.toContain('password');
		expect(prompt).not.toContain('portfolio');
		expect(prompt).not.toContain('holding');
		expect(prompt).not.toContain('cost_price');
		expect(prompt).not.toContain('C:\\');
		expect(prompt).not.toContain('/Users/');
	});
});
