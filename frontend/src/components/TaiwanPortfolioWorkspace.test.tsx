import fs from 'node:fs';
import path from 'node:path';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import type { TaiwanPortfolioHolding, TaiwanPortfolioSummary } from '../lib/taiwan-product';
import { formatPercent, formatTWD, PortfolioHoldingRow, PortfolioOverview } from './TaiwanPortfolioWorkspace';

const root = path.resolve(__dirname, '../../..');
const source = () => fs.readFileSync(path.join(root, 'frontend/src/components/TaiwanPortfolioWorkspace.tsx'), 'utf8');
const appSource = () => fs.readFileSync(path.join(root, 'frontend/src/App.tsx'), 'utf8');
const watchlistSource = () => fs.readFileSync(path.join(root, 'frontend/src/components/TaiwanWatchlistWorkspace.tsx'), 'utf8');

const holding = (overrides: Partial<TaiwanPortfolioHolding> = {}): TaiwanPortfolioHolding => ({
	canonical: '2330.TWSE', code: '2330', name: '台積電', exchange: 'TWSE', security_type: 'stock', industry: '24',
	shares: 1000, average_cost: 900, note: '核心持股', created_at: '', updated_at: '', current_price: 1000,
	price_status: 'available', market_value: 1_000_000, total_cost: 900_000, unrealized_pl: 100_000,
	unrealized_pl_percent: 11.111, portfolio_weight_percent: 80,
	...overrides,
});

const summary = (overrides: Partial<TaiwanPortfolioSummary> = {}): TaiwanPortfolioSummary => ({
	holdings: [holding()], holdings_count: 1, priced_holdings_count: 1, total_market_value: 1_000_000,
	available_market_value: 1_000_000, total_cost: 900_000, total_unrealized_pl: 100_000,
	total_unrealized_pl_percent: 11.111, currency: 'TWD', status: 'available',
	concentration: { holdings_count: 1, top_3_percent: 100, top_5_percent: 100, status: 'available', industries: [{ industry: '24', market_value: 1_000_000, weight_percent: 100, holdings_count: 1 }] },
	...overrides,
});

describe('Taiwan portfolio navigation and form', () => {
	it('registers a Taiwan-native portfolio workspace and Watchlist handoff', () => {
		expect(appSource()).toContain("'taiwan-portfolio'");
		expect(appSource()).toContain('<TaiwanPortfolioWorkspace');
		expect(watchlistSource()).toContain('加入持倉');
		expect(watchlistSource()).toContain('onAddPortfolio(item.canonical)');
	});

	it('has a simple canonical-security, shares and average-cost form with edit state', () => {
		const value = source();
		expect(value).toContain('輸入 2330 或台積電');
		expect(value).toContain('股數');
		expect(value).toContain('平均成本（TWD）');
		expect(value).toContain("method: editing ? 'PUT' : 'POST'");
		expect(value).toContain("window.confirm(");
	});

	it('renders loading, empty and API error states', () => {
		const value = source();
		expect(value).toContain('正在讀取持倉');
		expect(value).toContain('目前還沒有持股');
		expect(value).toContain('持倉資料載入失敗');
		expect(value).toContain('重試');
	});
});

describe('Taiwan portfolio presentation', () => {
	it('renders portfolio totals and descriptive concentration', () => {
		const html = renderToStaticMarkup(<PortfolioOverview summary={summary()} />);
		expect(html).toContain('總市值');
		expect(html).toContain('NT$ 1,000,000');
		expect(html).toContain('未實現損益');
		expect(html).toContain('Top 3');
		expect(html).toContain('100.00%');
		expect(html).not.toMatch(/買|賣|加碼|減碼|目標價|停損/);
	});

	it('renders holding list values, P/L, research navigation and Watchlist integration', () => {
		const html = renderToStaticMarkup(<PortfolioHoldingRow holding={holding()} deleting={false} watchlistBusy={false} onEdit={() => {}} onDelete={() => {}} onAddWatchlist={() => {}} onOpenResearch={() => {}} />);
		expect(html).toContain('台積電');
		expect(html).toContain('NT$ 100,000');
		expect(html).toContain('11.11%');
		expect(html).toContain('個股研究 · AI 研究 · 歷史比較');
		expect(html).toContain('加入自選');
		expect(html).toContain('編輯');
		expect(html).toContain('刪除');
	});

	it('renders unavailable price without fake zero P/L or weight', () => {
		const html = renderToStaticMarkup(<PortfolioHoldingRow holding={holding({ current_price: null, market_value: null, unrealized_pl: null, unrealized_pl_percent: null, portfolio_weight_percent: null, price_status: 'unavailable' })} deleting={false} watchlistBusy={false} onEdit={() => {}} onDelete={() => {}} onAddWatchlist={() => {}} onOpenResearch={() => {}} />);
		expect(html).toContain('行情無法取得');
		expect(html).not.toContain('NT$ 0');
	});

	it('suppresses aggregate P/L and concentration when one price is unavailable', () => {
		const html = renderToStaticMarkup(<PortfolioOverview summary={summary({ status: 'partial', priced_holdings_count: 0, total_market_value: null, total_unrealized_pl: null, total_unrealized_pl_percent: null, concentration: { holdings_count: 1, top_3_percent: null, top_5_percent: null, status: 'data_insufficient', industries: [] } })} />);
		expect(html).toContain('無法取得');
		expect(html).toContain('暫不計算權重與產業集中度');
	});

	it('formats TWD and percentages only at presentation time', () => {
		expect(formatTWD(1234.567)).toBe('NT$ 1,234.57');
		expect(formatTWD(null)).toBe('無法取得');
		expect(formatPercent(12.345)).toBe('12.35%');
	});
});

describe('Portfolio event integration', () => {
	it('syncs stock holdings once per load through the existing provider-neutral alert endpoint', () => {
		const value = source();
		expect(value).toContain("item.security_type === 'stock'");
		expect(value).toContain('/api/v1/tw/corporate-events/sync');
		expect(value).toContain('JSON.stringify({ symbols })');
		expect(value).not.toMatch(/setInterval|setTimeout/);
	});
});
