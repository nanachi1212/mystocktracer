import { Bell, Check, CheckCheck, ExternalLink, LoaderCircle, RefreshCw } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import type { BackendConfig } from '../lib/backend';
import { requestJSON } from '../lib/backend';
import { taiwanErrorMessage } from '../lib/taiwan-product';

export type TaiwanEventAlert = {
	id: number;
	canonical: string;
	security_name: string;
	provider: string;
	event_id: string;
	title: string;
	category?: string;
	published_at?: string;
	source: string;
	source_url?: string;
	retrieved_at?: string;
	status: string;
	stale: boolean;
	partial: boolean;
	created_at: string;
	read: boolean;
	read_at?: string;
};

export type AlertPayload = {
	alerts: TaiwanEventAlert[];
	unread_count: number;
	total: number;
	limit: number;
	offset: number;
	corporate_events_enabled: boolean;
};

export function safeAlertSourceURL(value?: string): string | null {
	if (!value) return null;
	try {
		const parsed = new URL(value);
		return parsed.protocol === 'https:' || parsed.protocol === 'http:' ? parsed.toString() : null;
	} catch {
		return null;
	}
}

function formatAlertTime(value?: string) {
	if (!value) return '時間未提供';
	const parsed = new Date(value);
	return Number.isNaN(parsed.getTime()) ? '時間未提供' : parsed.toLocaleString('zh-TW', { hour12: false });
}

export function TaiwanEventAlertCenter({ config, refreshKey, defaultOpen = false }: { config: BackendConfig | null; refreshKey: number; defaultOpen?: boolean }) {
	const [open, setOpen] = useState(defaultOpen);
	const [payload, setPayload] = useState<AlertPayload>({ alerts: [], unread_count: 0, total: 0, limit: 50, offset: 0, corporate_events_enabled: true });
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState('');
	const [busyID, setBusyID] = useState<number | 'all' | null>(null);
	const requestID = useRef(0);

	const load = async () => {
		if (!config) return;
		const current = ++requestID.current;
		setLoading(true);
		setError('');
		try {
			const response = await requestJSON<{ data: AlertPayload }>(config, '/api/v1/tw/alerts?status=all&limit=50&offset=0');
			if (current === requestID.current) setPayload(response.data);
		} catch (reason) {
			if (current === requestID.current) setError(taiwanErrorMessage(reason, '事件提醒暫時無法取得'));
		} finally {
			if (current === requestID.current) setLoading(false);
		}
	};

	useEffect(() => {
		void load();
		return () => { requestID.current += 1; };
	// load is deliberately scoped to backend identity and successful Watchlist sync refreshes.
	// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [config, refreshKey]);

	const markRead = async (id: number) => {
		if (!config || busyID !== null) return;
		requestID.current += 1;
		setLoading(false);
		setBusyID(id); setError('');
		try {
			await requestJSON(config, `/api/v1/tw/alerts/${id}/read`, { method: 'PUT' });
			setPayload((current) => ({ ...current, unread_count: Math.max(0, current.unread_count - (current.alerts.find((item) => item.id === id)?.read ? 0 : 1)), alerts: current.alerts.map((item) => item.id === id ? { ...item, read: true, read_at: new Date().toISOString() } : item) }));
		} catch (reason) {
			setError(taiwanErrorMessage(reason, '標記已讀失敗'));
		} finally { setBusyID(null); }
	};

	const markAllRead = async () => {
		if (!config || busyID !== null || payload.unread_count === 0) return;
		requestID.current += 1;
		setLoading(false);
		setBusyID('all'); setError('');
		try {
			await requestJSON(config, '/api/v1/tw/alerts/read-all', { method: 'PUT' });
			const readAt = new Date().toISOString();
			setPayload((current) => ({ ...current, unread_count: 0, alerts: current.alerts.map((item) => ({ ...item, read: true, read_at: item.read_at || readAt })) }));
		} catch (reason) {
			setError(taiwanErrorMessage(reason, '全部標記已讀失敗'));
		} finally { setBusyID(null); }
	};

	return <section className="taiwan-alert-center">
		<TaiwanEventAlertTrigger open={open} unreadCount={payload.unread_count} onToggle={() => setOpen((current) => !current)} />
		{open && <TaiwanEventAlertPanel payload={payload} loading={loading} error={error} busyID={busyID} onRetry={() => void load()} onMarkRead={(id) => void markRead(id)} onMarkAllRead={() => void markAllRead()} />}
	</section>;
}

export function TaiwanEventAlertTrigger({ open, unreadCount, onToggle }: { open: boolean; unreadCount: number; onToggle: () => void }) {
	return <button type="button" className="taiwan-alert-trigger" aria-expanded={open} onClick={onToggle}>
		<Bell size={16} />事件提醒{unreadCount > 0 && <span className="taiwan-alert-badge">{unreadCount > 99 ? '99+' : unreadCount}</span>}
	</button>;
}

export function TaiwanEventAlertPanel({ payload, loading, error, busyID, onRetry, onMarkRead, onMarkAllRead }: {
	payload: AlertPayload; loading: boolean; error: string; busyID: number | 'all' | null;
	onRetry: () => void; onMarkRead: (id: number) => void; onMarkAllRead: () => void;
}) {
	return <div className="taiwan-alert-panel">
		<header><div><strong>事件提醒中心</strong><small>最近 {payload.total} 則 · 未讀 {payload.unread_count} 則</small></div><div><button type="button" onClick={onRetry} disabled={loading}><RefreshCw className={loading ? 'spin' : ''} size={14} />重新整理</button><button type="button" onClick={onMarkAllRead} disabled={busyID !== null || payload.unread_count === 0}>{busyID === 'all' ? <LoaderCircle className="spin" size={14} /> : <CheckCheck size={14} />}全部已讀</button></div></header>
		{!payload.corporate_events_enabled && <div className="market-partial-warning">事件提醒已在系統設定中關閉；既有提醒會保留，但不會新增 inbox 項目。</div>}
		{loading && payload.alerts.length === 0 && <div className="taiwan-loading"><LoaderCircle className="spin" size={18} />正在讀取事件提醒</div>}
		{error && <div className="market-partial-warning">{error}。既有自選股與公告資料不受影響。</div>}
		{!loading && !error && payload.alerts.length === 0 && <div className="taiwan-empty-state"><strong>目前沒有事件提醒</strong><p>第一次同步只建立基準，不會補發舊公告；之後的新事件會顯示在這裡。</p></div>}
		{payload.alerts.length > 0 && <div className="taiwan-alert-list">{payload.alerts.map((alert) => {
			const sourceURL = safeAlertSourceURL(alert.source_url);
			return <article key={alert.id} className={`taiwan-alert-item ${alert.read ? 'read' : 'unread'}`}>
				<div className="taiwan-alert-heading"><div><strong>{alert.security_name || alert.canonical}</strong><span>{alert.canonical}</span></div><time>{formatAlertTime(alert.published_at || alert.created_at)}</time></div>
				<h3>{alert.title}</h3>
				<div className="taiwan-alert-meta">{alert.category && <span>{alert.category}</span>}<span>{alert.provider} · {alert.source}</span>{alert.stale && <em>資料較舊</em>}{alert.partial && <em>部分資料</em>}</div>
				<footer>{sourceURL ? <a href={sourceURL} target="_blank" rel="noreferrer">查看來源 <ExternalLink size={13} /></a> : <span>來源連結未提供</span>}{alert.read ? <span><Check size={13} />已讀</span> : <button type="button" onClick={() => onMarkRead(alert.id)} disabled={busyID !== null}>{busyID === alert.id ? <LoaderCircle className="spin" size={13} /> : <Check size={13} />}標記已讀</button>}</footer>
			</article>;
		})}</div>}
	</div>;
}
