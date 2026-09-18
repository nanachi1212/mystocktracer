import fs from 'node:fs';
import path from 'node:path';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import { LatestDashboardRequestGate, TaiwanDailyDashboard, TaiwanDailyDashboardView, type Dashboard } from './TaiwanDailyDashboard';

const root = path.resolve(__dirname, '../../..');
const source = () => fs.readFileSync(path.join(root, 'frontend/src/components/TaiwanDailyDashboard.tsx'), 'utf8');
const noop = () => {};
const dashboard: Dashboard = {
	generated_at: '2026-09-18T01:00:00Z',
	market: { status: 'stale', as_of: '2026-09-17', indexes: [{ id: 'TAIEX', name: '加權指數', price: 25000, change: 100, change_percent: .4, status: 'stale', as_of: '2026-09-17' }], advancers: 700, decliners: 300, advance_ratio: .7, turnover_twd: 300_000_000_000, emotion: 'positive', industries: [{ id: '24', name: '半導體業', relative_breadth: .2, relative_capital: .3 }] },
	portfolio: { status: 'partial', as_of: '2026-09-17', holdings_count: 2, priced_count: 1, unavailable_count: 1, total_market_value: null, total_unrealized_pl: null, largest_weight_percent: 60, top_3_percent: 100, industries: [{ industry: '半導體業', weight_percent: 60 }], movers: [{ canonical: '2330.TWSE', name: '台積電', price: 1000, change_percent: 3.5, status: 'available', as_of: '2026-09-17' }] },
	watchlist: { status: 'available', as_of: '2026-09-17', count: 1, available_count: 1, unavailable_count: 0, movers: [] },
	alerts: { status: 'unavailable', as_of: null, unread_count: 1, items: [{ id: 7, canonical: '2330.TWSE', security_name: '台積電', title: '董事會重大決議', published_at: '2026-09-18T00:00:00Z', source: 'MOPS', source_url: 'https://mops.twse.com.tw/', stale: true, partial: true }] },
	research: { status: 'available', as_of: '2026-09-18T00:30:00Z', items: [{ run_id: 'run-1', canonical: '2330.TWSE', security_name: '台積電', created_at: '2026-09-18T00:30:00Z', completeness: 'complete', stale: false, partial: false, has_previous: true, comparison: { changed: 1, newly_available: 0, no_longer_available: 0, stale: 0, partial: 0, unavailable: 0, corporate_events_added: 1 } }] },
	attention: [{ reason_code: 'price_unavailable', priority: 90, canonical: '6488.TPEX', title: '環球晶價格無法取得', target: 'taiwan-stock' }],
};

const renderView = (data: Dashboard | null, loading = false, error = '') => renderToStaticMarkup(<TaiwanDailyDashboardView data={data} loading={loading} error={error} onNavigate={noop} onOpenResearch={noop} onOpenResearchHistory={noop} onRefresh={noop} onRetry={noop} onMarkAlertRead={noop} />);

describe('Taiwan daily dashboard', () => {
	it('renders the Taiwan-native shell without fabricating data before loading', () => {
		const html = renderToStaticMarkup(<TaiwanDailyDashboard config={null} refreshKey={0} onNavigate={() => {}} onOpenResearch={() => {}} />);
		expect(html).toContain('台股今日總覽');
		expect(html).toContain('資訊整合與導覽，不構成投資建議');
		expect(html).toContain('重新整理');
		expect(html).not.toContain('值得買');
	});

	it('renders loading, error, partial, unavailable and stale states from data', () => {
		expect(renderView(null, true, '今日總覽暫時無法取得')).toContain('正在整理市場、持股、自選股與研究活動');
		const html = renderView(dashboard, false, '事件同步暫時無法完成');
		for (const text of ['事件同步暫時無法完成', 'taiwan-status stale', 'taiwan-status partial', 'taiwan-status unavailable']) expect(html).toContain(text);
	});

	it('renders all required sections and descriptive portfolio/alert/research facts', () => {
		const html = renderView(dashboard);
		for (const text of ['今日市場', '我的持股', '自選股', '事件提醒', '研究更新', '需要注意', '總市值', '未實現損益', '主要產業', '未讀 1 筆', 'MOPS', '可與前次比較', '價格無法取得']) expect(html).toContain(text);
		expect(html).not.toMatch(/值得買|值得賣|應加碼|應減碼|目標價|停損/);
	});

	it('renders bounded empty states without fabricated values', () => {
		const empty: Dashboard = { ...dashboard, portfolio: { ...dashboard.portfolio, industries: [], movers: [] }, watchlist: { ...dashboard.watchlist, movers: [] }, alerts: { ...dashboard.alerts, unread_count: 0, items: [] }, research: { ...dashboard.research, items: [] }, attention: [] };
		const html = renderView(empty);
		for (const text of ['目前沒有可排序的市場變動', '目前沒有未讀事件', '尚無已完成的 AI 研究', '目前沒有需要注意的資料狀態']) expect(html).toContain(text);
	});

	it('distinguishes unavailable market data from stale market data', () => {
		const unavailable = { ...dashboard, attention: [{ reason_code: 'market_data_unavailable', priority: 100, title: '市場資料目前無法取得', target: 'taiwan-overview' }] };
		const html = renderView(unavailable);
		expect(html).toContain('市場資料無法取得');
		expect(html).not.toContain('市場資料較舊');
	});

	it('uses canonical-symbol navigation and the shared alert read endpoint', () => {
		const value = source();
		expect(value).toContain('onOpenResearch(item.canonical)');
		expect(value).toContain("onNavigate('taiwan-portfolio')");
		expect(value).toContain("onNavigate('taiwan-watchlist')");
		expect(value).toContain("onNavigate('taiwan-alerts')");
		expect(value).toContain('`/api/v1/tw/alerts/${id}/read`');
	});

	it('guards refresh races with a last-request-wins gate and performs no AI research request', () => {
		const gate = new LatestDashboardRequestGate();
		const stale = gate.begin();
		const current = gate.begin();
		expect(gate.isCurrent(stale)).toBe(false);
		expect(gate.isCurrent(current)).toBe(true);
		gate.invalidate();
		expect(gate.isCurrent(current)).toBe(false);
		const value = source();
		expect(value).toContain("'/api/v1/tw/dashboard'");
		expect(value).toContain("'/api/v1/tw/corporate-events/sync'");
		expect(value).not.toContain('/research\'');
	});
});
