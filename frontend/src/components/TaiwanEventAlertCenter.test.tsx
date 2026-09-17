import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import { safeAlertSourceURL, TaiwanEventAlertPanel, TaiwanEventAlertTrigger, type AlertPayload, type TaiwanEventAlert } from './TaiwanEventAlertCenter';

const alert = (overrides: Partial<TaiwanEventAlert> = {}): TaiwanEventAlert => ({
	id: 1, canonical: '2330.TWSE', security_name: '台積電', provider: 'provider-a', event_id: 'event-1',
	title: '董事會重大決議', category: '重大訊息', published_at: '2026-09-17T01:00:00Z', source: 'mops',
	source_url: 'https://example.com/event-1', status: 'available', stale: false, partial: false,
	created_at: '2026-09-17T01:01:00Z', read: false, ...overrides,
});

const payload = (overrides: Partial<AlertPayload> = {}): AlertPayload => ({
	alerts: [alert()], unread_count: 1, total: 1, limit: 50, offset: 0, corporate_events_enabled: true, ...overrides,
});

describe('Taiwan event alert center', () => {
	it('renders an unread badge with a bounded count', () => {
		const html = renderToStaticMarkup(<TaiwanEventAlertTrigger open={false} unreadCount={120} onToggle={() => {}} />);
		expect(html).toContain('事件提醒');
		expect(html).toContain('99+');
	});

	it('renders stock, symbol, title, time, category, provenance, source and unread action', () => {
		const html = renderToStaticMarkup(<TaiwanEventAlertPanel payload={payload()} loading={false} error="" busyID={null} onRetry={() => {}} onMarkRead={() => {}} onMarkAllRead={() => {}} />);
		for (const text of ['台積電', '2330.TWSE', '董事會重大決議', '重大訊息', 'provider-a · mops', '查看來源', '標記已讀']) expect(html).toContain(text);
	});

	it('renders read, stale and partial states explicitly', () => {
		const html = renderToStaticMarkup(<TaiwanEventAlertPanel payload={payload({ alerts: [alert({ read: true, stale: true, partial: true })], unread_count: 0 })} loading={false} error="" busyID={null} onRetry={() => {}} onMarkRead={() => {}} onMarkAllRead={() => {}} />);
		expect(html).toContain('已讀');
		expect(html).toContain('資料較舊');
		expect(html).toContain('部分資料');
	});

	it('distinguishes loading, error, empty and disabled states', () => {
		const empty = payload({ alerts: [], unread_count: 0, total: 0, corporate_events_enabled: false });
		expect(renderToStaticMarkup(<TaiwanEventAlertPanel payload={empty} loading={true} error="" busyID={null} onRetry={() => {}} onMarkRead={() => {}} onMarkAllRead={() => {}} />)).toContain('正在讀取事件提醒');
		expect(renderToStaticMarkup(<TaiwanEventAlertPanel payload={empty} loading={false} error="服務中斷" busyID={null} onRetry={() => {}} onMarkRead={() => {}} onMarkAllRead={() => {}} />)).toContain('既有自選股與公告資料不受影響');
		const emptyHTML = renderToStaticMarkup(<TaiwanEventAlertPanel payload={empty} loading={false} error="" busyID={null} onRetry={() => {}} onMarkRead={() => {}} onMarkAllRead={() => {}} />);
		expect(emptyHTML).toContain('目前沒有事件提醒');
		expect(emptyHTML).toContain('事件提醒已在系統設定中關閉');
	});

	it('rejects unsafe source URLs', () => {
		expect(safeAlertSourceURL('javascript:alert(1)')).toBeNull();
		expect(safeAlertSourceURL('not a url')).toBeNull();
		expect(safeAlertSourceURL('https://mops.twse.com.tw/example')).toContain('https://');
	});
});
