import { AlertTriangle, BarChart3, Edit3, ExternalLink, LoaderCircle, Plus, RefreshCw, Search, Star, Trash2, WalletCards, X } from 'lucide-react';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { FormEvent } from 'react';
import { requestJSON, type BackendConfig, type SecurityIdentity } from '../lib/backend';
import {
	addTaiwanWatchlistSecurity,
	taiwanErrorMessage,
	taiwanPortfolioHoldingPath,
	taiwanPortfolioPath,
	taiwanPortfolioSummaryPath,
	type TaiwanPortfolioHolding,
	type TaiwanPortfolioSummary,
} from '../lib/taiwan-product';

type Props = {
	config: BackendConfig | null;
	refreshKey: number;
	onOpenResearch: (canonical: string) => void;
	externalSymbolRequest?: { canonical: string; token: number } | null;
};

type Draft = { security: SecurityIdentity | null; shares: string; averageCost: string; note: string };
const emptyDraft: Draft = { security: null, shares: '', averageCost: '', note: '' };

export function TaiwanPortfolioWorkspace({ config, refreshKey, onOpenResearch, externalSymbolRequest }: Props) {
	const [summary, setSummary] = useState<TaiwanPortfolioSummary | null>(null);
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState('');
	const [notice, setNotice] = useState('');
	const [query, setQuery] = useState('');
	const [matches, setMatches] = useState<SecurityIdentity[]>([]);
	const [searching, setSearching] = useState(false);
	const [draft, setDraft] = useState<Draft>(emptyDraft);
	const [editing, setEditing] = useState<string | null>(null);
	const [saving, setSaving] = useState(false);
	const [deleting, setDeleting] = useState<string | null>(null);
	const [watchlistBusy, setWatchlistBusy] = useState<string | null>(null);
	const handledExternalToken = useRef<number | null>(null);

	const syncPortfolioEvents = useCallback(async (holdings: TaiwanPortfolioHolding[]) => {
		if (!config) return;
		const symbols = holdings.filter((item) => item.security_type === 'stock').map((item) => item.canonical).slice(0, 20);
		if (!symbols.length) return;
		try {
			await requestJSON(config, '/api/v1/tw/corporate-events/sync', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ symbols }) });
		} catch { /* event enrichment is independent from persisted portfolio data */ }
	}, [config]);

	const load = useCallback(async () => {
		if (!config) return;
		setLoading(true); setError('');
		try {
			const payload = await requestJSON<{ data: TaiwanPortfolioSummary }>(config, taiwanPortfolioSummaryPath());
			setSummary(payload.data);
			void syncPortfolioEvents(payload.data.holdings);
		} catch (reason) { setError(taiwanErrorMessage(reason, '持倉資料載入失敗')); }
		finally { setLoading(false); }
	}, [config, syncPortfolioEvents]);

	useEffect(() => { void load(); }, [load, refreshKey]);

	useEffect(() => {
		if (!config || !externalSymbolRequest || handledExternalToken.current === externalSymbolRequest.token) return;
		handledExternalToken.current = externalSymbolRequest.token;
		void (async () => {
			try {
				const [payload, portfolio] = await Promise.all([
					requestJSON<{ data: { securities: SecurityIdentity[] } }>(config, `/api/v1/tw/securities?query=${encodeURIComponent(externalSymbolRequest.canonical)}`),
					requestJSON<{ data: { holdings: TaiwanPortfolioHolding[] } }>(config, taiwanPortfolioPath()),
				]);
				if (payload.data.securities.length === 1) {
					const security = payload.data.securities[0];
					const existing = portfolio.data.holdings.find((item) => item.canonical === security.canonical);
					setDraft(existing ? { security, shares: String(existing.shares), averageCost: String(existing.average_cost), note: existing.note || '' } : { ...emptyDraft, security });
					setEditing(existing ? existing.canonical : null);
					setQuery(security.name);
					setNotice(existing ? '此標的已在持倉中，已開啟編輯狀態。' : '已從自選股帶入標的，請填寫股數與平均成本。');
				}
			} catch (reason) { setError(taiwanErrorMessage(reason, '無法帶入自選股')); }
		})();
	}, [config, externalSymbolRequest]);

	const search = async (event: FormEvent) => {
		event.preventDefault();
		if (!config || !query.trim()) return;
		setSearching(true); setError('');
		try {
			const payload = await requestJSON<{ data: { securities: SecurityIdentity[] } }>(config, `/api/v1/tw/securities?query=${encodeURIComponent(query.trim())}`);
			setMatches(payload.data.securities);
			if (payload.data.securities.length === 1) setDraft((current) => ({ ...current, security: payload.data.securities[0] }));
			if (!payload.data.securities.length) setError(`找不到符合「${query.trim()}」的台灣證券`);
		} catch (reason) { setError(taiwanErrorMessage(reason, '台灣證券搜尋失敗')); }
		finally { setSearching(false); }
	};

	const save = async (event: FormEvent) => {
		event.preventDefault();
		if (!config || !draft.security || saving) return;
		const shares = Number(draft.shares);
		const averageCost = Number(draft.averageCost);
		if (!Number.isFinite(shares) || shares <= 0 || !Number.isFinite(averageCost) || averageCost < 0) {
			setError('股數必須大於 0，平均成本必須是非負的有限數字。'); return;
		}
		setSaving(true); setError(''); setNotice('');
		try {
			const path = editing ? taiwanPortfolioHoldingPath(editing) : taiwanPortfolioPath();
			await requestJSON(config, path, { method: editing ? 'PUT' : 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ symbol: draft.security.canonical, shares, average_cost: averageCost, note: draft.note }) });
			setDraft(emptyDraft); setEditing(null); setQuery(''); setMatches([]);
			setNotice(editing ? '持股已更新。' : '持股已加入。');
			await load();
		} catch (reason) { setError(taiwanErrorMessage(reason, editing ? '更新持股失敗' : '新增持股失敗')); }
		finally { setSaving(false); }
	};

	const edit = (holding: TaiwanPortfolioHolding) => {
		setEditing(holding.canonical);
		setDraft({ security: { canonical: holding.canonical, code: holding.code || holding.canonical.split('.')[0], name: holding.name, market: 'TW', exchange: holding.exchange || '', security_type: holding.security_type || 'stock', currency: 'TWD', timezone: 'Asia/Taipei', provider: '', source_url: '', retrieved_at: '' }, shares: String(holding.shares), averageCost: String(holding.average_cost), note: holding.note || '' });
		setQuery(holding.name); setMatches([]); setNotice(''); setError('');
	};

	const remove = async (holding: TaiwanPortfolioHolding) => {
		if (!config || deleting || !window.confirm(`確定要刪除 ${holding.name} 的持倉資料嗎？`)) return;
		setDeleting(holding.canonical); setError('');
		try { await requestJSON(config, taiwanPortfolioHoldingPath(holding.canonical), { method: 'DELETE' }); setNotice('持股已刪除，自選股與提醒歷史不受影響。'); await load(); }
		catch (reason) { setError(taiwanErrorMessage(reason, '刪除持股失敗')); }
		finally { setDeleting(null); }
	};

	const addToWatchlist = async (holding: TaiwanPortfolioHolding) => {
		if (!config || watchlistBusy) return;
		setWatchlistBusy(holding.canonical); setError('');
		try { await addTaiwanWatchlistSecurity(config, holding.canonical); setNotice(`${holding.name} 已加入自選股。`); }
		catch (reason) { setError(taiwanErrorMessage(reason, '加入自選股失敗')); }
		finally { setWatchlistBusy(null); }
	};

	const formValid = Boolean(draft.security && Number(draft.shares) > 0 && Number(draft.averageCost) >= 0);
	return <div className="taiwan-portfolio-workspace">
		{error && <div className="taiwan-portfolio-message error"><AlertTriangle size={16} />{error}<button type="button" onClick={() => void load()}><RefreshCw size={14} />重試</button></div>}
		{notice && <div className="taiwan-portfolio-message success">{notice}</div>}
		{loading && !summary ? <div className="taiwan-loading"><LoaderCircle className="spin" size={18} />正在讀取持倉</div> : null}
		{summary && <PortfolioOverview summary={summary} />}
		<section className="taiwan-portfolio-layout">
			<form className="taiwan-portfolio-form" onSubmit={save}>
				<header><div><Plus size={17} /><strong>{editing ? '編輯持股' : '新增持股'}</strong></div>{editing && <button type="button" onClick={() => { setEditing(null); setDraft(emptyDraft); setQuery(''); }}><X size={14} />取消</button>}</header>
				<label><span>股票</span><div className="taiwan-portfolio-search"><input value={query} onChange={(event) => { setQuery(event.target.value); setDraft((current) => ({ ...current, security: null })); }} placeholder="輸入 2330 或台積電" disabled={Boolean(editing)} /><button type="button" onClick={(event) => void search(event as unknown as FormEvent)} disabled={searching || Boolean(editing)}>{searching ? <LoaderCircle className="spin" size={14} /> : <Search size={14} />}搜尋</button></div></label>
				{matches.length > 1 && <div className="taiwan-portfolio-matches">{matches.map((item) => <button type="button" key={item.canonical} onClick={() => { setDraft((current) => ({ ...current, security: item })); setQuery(`${item.name} ${item.code}`); setMatches([]); }}><strong>{item.name} {item.code}</strong><span>{item.exchange} · {item.security_type.toUpperCase()}</span></button>)}</div>}
				{draft.security && <div className="taiwan-portfolio-selected"><strong>{draft.security.name} {draft.security.code}</strong><span>{draft.security.canonical}</span></div>}
				<label><span>股數</span><input type="number" min="0" step="any" value={draft.shares} onChange={(event) => setDraft((current) => ({ ...current, shares: event.target.value }))} required /></label>
				<label><span>平均成本（TWD）</span><input type="number" min="0" step="any" value={draft.averageCost} onChange={(event) => setDraft((current) => ({ ...current, averageCost: event.target.value }))} required /></label>
				<label><span>備註（選填）</span><textarea maxLength={500} value={draft.note} onChange={(event) => setDraft((current) => ({ ...current, note: event.target.value }))} /></label>
				<button className="taiwan-portfolio-save" type="submit" disabled={!formValid || saving}>{saving ? <LoaderCircle className="spin" size={15} /> : <WalletCards size={15} />}{editing ? '儲存變更' : '儲存持股'}</button>
			</form>
			<section className="taiwan-portfolio-holdings">
				<header><div><WalletCards size={17} /><strong>持股明細</strong></div><span>{summary?.holdings_count || 0} 檔</span></header>
				{summary && summary.holdings.length === 0 && <div className="taiwan-empty-state"><strong>目前還沒有持股</strong><p>搜尋台灣證券，填入股數與平均成本後即可建立持倉。</p></div>}
				{summary?.holdings.map((holding) => <PortfolioHoldingRow key={holding.canonical} holding={holding} deleting={deleting === holding.canonical} watchlistBusy={watchlistBusy === holding.canonical} onEdit={() => edit(holding)} onDelete={() => void remove(holding)} onAddWatchlist={() => void addToWatchlist(holding)} onOpenResearch={() => onOpenResearch(holding.canonical)} />)}
			</section>
		</section>
	</div>;
}

export function PortfolioOverview({ summary }: { summary: TaiwanPortfolioSummary }) {
	return <>
		<section className="taiwan-portfolio-overview">
			<PortfolioMetric label="總市值" value={formatTWD(summary.total_market_value)} sub={summary.status === 'partial' ? `已取得 ${summary.priced_holdings_count}/${summary.holdings_count} 檔行情` : '新台幣'} />
			<PortfolioMetric label="總成本" value={formatTWD(summary.total_cost)} sub="新台幣" />
			<PortfolioMetric label="未實現損益" value={formatTWD(summary.total_unrealized_pl)} sub={formatPercent(summary.total_unrealized_pl_percent)} tone={tone(summary.total_unrealized_pl)} />
			<PortfolioMetric label="持股檔數" value={String(summary.holdings_count)} sub={summary.status === 'stale' ? '部分行情資料較舊' : summary.status === 'partial' ? '部分行情無法取得' : '行情已取得'} />
		</section>
		<section className="taiwan-portfolio-concentration"><header><BarChart3 size={16} /><strong>集中度</strong></header>{summary.concentration.status === 'data_insufficient' ? <p>部分持股缺少行情，暫不計算權重與產業集中度。</p> : summary.concentration.status === 'empty' ? <p>建立持倉後將顯示描述性集中度。</p> : <div><span>Top 3 <strong>{formatPercent(summary.concentration.top_3_percent)}</strong></span><span>Top 5 <strong>{formatPercent(summary.concentration.top_5_percent)}</strong></span>{summary.concentration.industries.map((item) => <span key={item.industry}>{item.industry} <strong>{formatPercent(item.weight_percent)}</strong></span>)}</div>}</section>
	</>;
}

function PortfolioMetric({ label, value, sub, tone: valueTone = '' }: { label: string; value: string; sub: string; tone?: string }) { return <article><span>{label}</span><strong className={valueTone}>{value}</strong><small>{sub}</small></article>; }

export function PortfolioHoldingRow({ holding, deleting, watchlistBusy, onEdit, onDelete, onAddWatchlist, onOpenResearch }: { holding: TaiwanPortfolioHolding; deleting: boolean; watchlistBusy: boolean; onEdit: () => void; onDelete: () => void; onAddWatchlist: () => void; onOpenResearch: () => void }) {
	return <article className="taiwan-portfolio-row">
		<header><div><strong>{holding.name} {holding.code}</strong><span>{holding.canonical}{holding.industry ? ` · ${holding.industry}` : ''}</span></div><em className={holding.price_status}>{holding.price_status === 'available' ? '行情可用' : holding.price_status === 'stale' ? '行情資料較舊' : '行情無法取得'}</em></header>
		<div className="taiwan-portfolio-values"><span><small>股數</small><strong>{holding.shares.toLocaleString('zh-TW')}</strong></span><span><small>平均成本</small><strong>{formatTWD(holding.average_cost)}</strong></span><span><small>現價</small><strong>{formatTWD(holding.current_price)}</strong></span><span><small>市值</small><strong>{formatTWD(holding.market_value)}</strong></span><span><small>損益</small><strong className={tone(holding.unrealized_pl)}>{formatTWD(holding.unrealized_pl)} <i>{formatPercent(holding.unrealized_pl_percent)}</i></strong></span><span><small>權重</small><strong>{formatPercent(holding.portfolio_weight_percent)}</strong></span></div>
		{holding.note && <p>{holding.note}</p>}
		<footer><button type="button" onClick={onOpenResearch}><ExternalLink size={13} />個股研究 · AI 研究 · 歷史比較</button><button type="button" onClick={onAddWatchlist} disabled={watchlistBusy}>{watchlistBusy ? <LoaderCircle className="spin" size={13} /> : <Star size={13} />}加入自選</button><button type="button" onClick={onEdit}><Edit3 size={13} />編輯</button><button type="button" className="danger" onClick={onDelete} disabled={deleting}>{deleting ? <LoaderCircle className="spin" size={13} /> : <Trash2 size={13} />}刪除</button></footer>
	</article>;
}

export function formatTWD(value: number | null | undefined) { return value == null ? '無法取得' : `NT$ ${value.toLocaleString('zh-TW', { maximumFractionDigits: 2 })}`; }
export function formatPercent(value: number | null | undefined) { return value == null ? '—' : `${value.toLocaleString('zh-TW', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}%`; }
function tone(value: number | null | undefined) { return value == null ? '' : value > 0 ? 'up' : value < 0 ? 'down' : 'flat'; }
