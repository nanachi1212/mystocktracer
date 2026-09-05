import { LoaderCircle } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import type { BackendConfig } from '../lib/backend';
import { requestJSON } from '../lib/backend';
import {
	formatTaiwanPercent, formatTaiwanTWD, runScopedRequest, taiwanErrorMessage, taiwanScopes, taiwanScreenerDefaultFilters,
	taiwanScreenerOrderOptions, taiwanScreenerPath, taiwanScreenerSortOptions, taiwanSecurityTypeLabel, taiwanStatusLabel,
	validateTaiwanScreenerFilters, type TaiwanScreenerFilters, type TaiwanScreenerResponse, type TaiwanScreenerSecurity,
} from '../lib/taiwan-product';

// M7B — Taiwan Screener. Loads/screens/sorts/paginates entirely through the existing M7A backend
// contract (GET /api/v1/tw/screener); this component never fetches the full market and filters it
// locally. All form controls are draft state — nothing here issues a request until 套用條件 is
// clicked, so typing alone can never fan out requests. `applied` is the only state the fetch effect
// depends on, so Apply, Clear, pagination, and the global refresh all funnel through one request
// path, and runScopedRequest (the same guard used across every other Taiwan view) guarantees a
// slower stale response can never overwrite a newer one.
export function TaiwanScreenerWorkspace({ config, refreshKey, onOpenResearch }: { config: BackendConfig | null; refreshKey: number; onOpenResearch: (canonical: string) => void }) {
	const [draft, setDraft] = useState<TaiwanScreenerFilters>(taiwanScreenerDefaultFilters);
	const [applied, setApplied] = useState<TaiwanScreenerFilters>(taiwanScreenerDefaultFilters);
	const [localError, setLocalError] = useState('');
	const [data, setData] = useState<TaiwanScreenerResponse | null>(null);
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState('');
	const requestID = useRef(0);

	// Fetches only while this workspace is mounted, and only in reaction to `applied` changing (Apply,
	// Clear, pagination) or an explicit global refresh — never on a timer, never merely because the
	// filter panel re-rendered.
	useEffect(() => {
		if (!config) return;
		void runScopedRequest(requestID, () => requestJSON<{ data: TaiwanScreenerResponse }>(config, taiwanScreenerPath(applied)), {
			onStart: () => { setData(null); setLoading(true); setError(''); },
			onSuccess: (payload) => setData(payload.data),
			onError: (reason) => { setData(null); setError(taiwanErrorMessage(reason, '台股選股資料載入失敗')); },
			onSettle: () => setLoading(false),
		});
	}, [config, refreshKey, applied]);

	const applyFilters = () => {
		const message = validateTaiwanScreenerFilters(draft);
		if (message) { setLocalError(message); return; }
		setLocalError('');
		setApplied({ ...draft, offset: 0 });
	};

	const clearFilters = () => {
		const defaults = taiwanScreenerDefaultFilters();
		setDraft(defaults);
		setLocalError('');
		setApplied(defaults);
	};

	const goPrevious = () => {
		if (applied.offset <= 0) return;
		setApplied((current) => ({ ...current, offset: Math.max(0, current.offset - current.limit) }));
	};

	const goNext = () => {
		if (!data || applied.offset + data.securities.length >= data.total) return;
		setApplied((current) => ({ ...current, offset: current.offset + current.limit }));
	};

	return <div className="taiwan-product-workspace taiwan-screener-workspace">
		<ScreenerFilterPanel draft={draft} onChange={setDraft} onApply={applyFilters} onClear={clearFilters} localError={localError} />
		{error && <div className="market-partial-warning">{error}</div>}
		{loading && <div className="taiwan-loading"><LoaderCircle className="spin" size={18} />正在讀取台股選股資料</div>}
		{data && <ScreenerSummary data={data} />}
		{data && data.total === 0 && <div className="taiwan-empty-state"><strong>沒有符合目前條件的台灣證券</strong><p>可放寬篩選條件或按下「清除條件」查看全部結果。</p></div>}
		{data && data.securities.length > 0 && <ScreenerTable securities={data.securities} onOpenResearch={onOpenResearch} />}
		{data && <ScreenerPagination data={data} loading={loading} onPrevious={goPrevious} onNext={goNext} />}
	</div>;
}

// Pure/presentational: previous/next disabled purely from `data` (offset/total/securities.length) —
// no hidden state of its own, so its enabled/disabled logic is directly testable without hooks.
export function ScreenerPagination({ data, loading, onPrevious, onNext }: { data: TaiwanScreenerResponse; loading: boolean; onPrevious: () => void; onNext: () => void }) {
	const hasPrevious = data.offset > 0;
	const hasNext = data.offset + data.securities.length < data.total;
	return <div className="taiwan-screener-pagination" aria-label="選股結果分頁">
		<span>{data.total > 0 ? `第 ${data.offset + 1}–${data.offset + data.securities.length} 筆，共 ${data.total.toLocaleString('zh-TW')} 筆` : '共 0 筆'}</span>
		<div>
			<button type="button" disabled={!hasPrevious || loading} onClick={onPrevious}>上一頁</button>
			<button type="button" disabled={!hasNext || loading} onClick={onNext}>下一頁</button>
		</div>
	</div>;
}

export function ScreenerFilterPanel({ draft, onChange, onApply, onClear, localError }: {
	draft: TaiwanScreenerFilters;
	onChange: (next: TaiwanScreenerFilters) => void;
	onApply: () => void;
	onClear: () => void;
	localError: string;
}) {
	const set = <K extends keyof TaiwanScreenerFilters>(key: K, value: TaiwanScreenerFilters[K]) => onChange({ ...draft, [key]: value });
	return <section className="taiwan-screener-filters">
		<div className="taiwan-screener-filter-grid">
			<label><span>市場</span><select value={draft.scope} onChange={(event) => set('scope', event.target.value as TaiwanScreenerFilters['scope'])}>{taiwanScopes.map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}</select></label>
			<label><span>最低價格</span><input type="number" inputMode="decimal" value={draft.minPrice} onChange={(event) => set('minPrice', event.target.value)} placeholder="不限" /></label>
			<label><span>最高價格</span><input type="number" inputMode="decimal" value={draft.maxPrice} onChange={(event) => set('maxPrice', event.target.value)} placeholder="不限" /></label>
			<label><span>最低漲跌幅 %</span><input type="number" inputMode="decimal" value={draft.minChangePercent} onChange={(event) => set('minChangePercent', event.target.value)} placeholder="不限" /></label>
			<label><span>最高漲跌幅 %</span><input type="number" inputMode="decimal" value={draft.maxChangePercent} onChange={(event) => set('maxChangePercent', event.target.value)} placeholder="不限" /></label>
			<label><span>最低成交量（股）</span><input type="number" inputMode="numeric" value={draft.minVolume} onChange={(event) => set('minVolume', event.target.value)} placeholder="不限" /></label>
			<label><span>最高成交量（股）</span><input type="number" inputMode="numeric" value={draft.maxVolume} onChange={(event) => set('maxVolume', event.target.value)} placeholder="不限" /></label>
			<label><span>最低成交金額（元）</span><input type="number" inputMode="numeric" value={draft.minAmount} onChange={(event) => set('minAmount', event.target.value)} placeholder="不限" /></label>
			<label><span>最高成交金額（元）</span><input type="number" inputMode="numeric" value={draft.maxAmount} onChange={(event) => set('maxAmount', event.target.value)} placeholder="不限" /></label>
			<label><span>排序欄位</span><select value={draft.sort} onChange={(event) => set('sort', event.target.value as TaiwanScreenerFilters['sort'])}>{taiwanScreenerSortOptions.map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}</select></label>
			<label><span>排序方向</span><select value={draft.order} onChange={(event) => set('order', event.target.value as TaiwanScreenerFilters['order'])}>{taiwanScreenerOrderOptions.map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}</select></label>
		</div>
		{localError && <p className="taiwan-screener-filter-error">{localError}</p>}
		<div className="taiwan-screener-filter-actions">
			<button type="button" className="taiwan-screener-apply" onClick={onApply}>套用條件</button>
			<button type="button" className="taiwan-screener-clear" onClick={onClear}>清除條件</button>
		</div>
	</section>;
}

export function ScreenerSummary({ data }: { data: TaiwanScreenerResponse }) {
	return <section className="taiwan-status-card"><div><strong>{data.scope}</strong><span className={`taiwan-status ${data.freshness}`}>{taiwanStatusLabel(data.freshness)}</span><span>{data.total.toLocaleString('zh-TW')} 檔符合條件</span></div><small>資料日期 {data.as_of || '未提供'}</small></section>;
}

export function ScreenerTable({ securities, onOpenResearch }: { securities: TaiwanScreenerSecurity[]; onOpenResearch: (canonical: string) => void }) {
	return <div className="taiwan-screener-table-wrap"><table className="taiwan-screener-table">
		<thead><tr><th>證券</th><th>市場</th><th>價格</th><th>漲跌幅</th><th>成交量</th><th>成交金額</th><th>資料日期</th></tr></thead>
		<tbody>{securities.map((item) => <ScreenerRow key={item.canonical} security={item} onOpen={() => onOpenResearch(item.canonical)} />)}</tbody>
	</table></div>;
}

// Pure/presentational row. Clicking the identity area hands the exact backend `canonical` (never
// code alone, never re-inferred) to the caller's existing stock-research handoff — this component
// owns no navigation state of its own.
export function ScreenerRow({ security, onOpen }: { security: TaiwanScreenerSecurity; onOpen: () => void }) {
	const tone = security.change_percent == null ? '' : security.change_percent > 0 ? 'up' : security.change_percent < 0 ? 'down' : 'flat';
	return <tr>
		<td><button type="button" className="taiwan-screener-identity" onClick={onOpen}><strong>{security.name} {security.code}</strong></button></td>
		<td>{security.exchange} · {taiwanSecurityTypeLabel(security.security_type)}</td>
		<td>{security.price == null ? '—' : security.price.toLocaleString('zh-TW')}</td>
		<td className={tone}>{formatTaiwanPercent(security.change_percent, true)}</td>
		<td>{security.volume == null ? '—' : security.volume.toLocaleString('zh-TW')}</td>
		<td>{formatTaiwanTWD(security.amount)}</td>
		<td>{security.trade_date || '—'}</td>
	</tr>;
}
