import { ArrowLeft, ChevronDown, GitCompareArrows, History, LoaderCircle, RefreshCw } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import type { BackendConfig } from '../lib/backend';
import { requestJSON } from '../lib/backend';
import { taiwanErrorMessage } from '../lib/taiwan-product';
import type { Research } from './TaiwanStockResearchWorkspace';

export type ResearchHistorySummary = {
	run_id: string; canonical: string; security_name: string; created_at: string; evidence_as_of?: string;
	research_version: string; payload_version: string; model_provider?: string; model_name?: string;
	completeness: string; stale: boolean; partial: boolean; has_previous: boolean;
};

export type ResearchHistoryRecord = Omit<ResearchHistorySummary, 'has_previous'> & {
	has_previous?: boolean;
	evidence_snapshot: Record<string, unknown>; research_result: Research;
	provenance: Record<string, unknown>; validity: Record<string, unknown>;
};

export type ResearchFieldChange = { path: string; state: string; before?: unknown; after?: unknown; delta?: number; delta_percent?: number };
export type ResearchComparison = {
	comparable: boolean; reason?: string; current_run_id: string; previous_run_id?: string;
	summary: { unchanged: number; changed: number; newly_available: number; no_longer_available: number; stale: number; partial: number; unavailable: number; corporate_events_added: number; corporate_events_removed: number };
	domains: { domain: string; state: string; changed_fields: ResearchFieldChange[] }[];
	assumptions: { kind: string; section?: string; text: string; evidence_keys: string[]; status: string }[];
	corporate_events_added: { event_id: string; title?: string; category?: string; published_at?: string }[];
	corporate_events_removed: { event_id: string; title?: string; category?: string; published_at?: string }[];
};

export type ComparisonPayload = { available: boolean; reason?: string; current_run_id?: string; comparison?: ResearchComparison };

export class LatestRequestGate {
	private revision = 0;
	begin() { this.revision += 1; return this.revision; }
	invalidate() { this.revision += 1; }
	isCurrent(token: number) { return token === this.revision; }
}

export const fetchResearchHistoryPage = (config: BackendConfig, canonical: string, offset = 0) => requestJSON<{ data: { runs: ResearchHistorySummary[]; total: number; limit: number; offset: number } }>(config, `/api/v1/tw/stocks/${encodeURIComponent(canonical)}/research-history?limit=20&offset=${offset}`);
export const fetchResearchHistoryRecord = (config: BackendConfig, canonical: string, runID: string) => requestJSON<{ data: ResearchHistoryRecord }>(config, `/api/v1/tw/stocks/${encodeURIComponent(canonical)}/research-history/${encodeURIComponent(runID)}`);
export const fetchResearchComparison = (config: BackendConfig, canonical: string, runID: string) => requestJSON<{ data: ComparisonPayload }>(config, `/api/v1/tw/stocks/${encodeURIComponent(canonical)}/research-history/${encodeURIComponent(runID)}/comparison`);

const comparisonLabels: Record<string, string> = {
	unchanged: '未改變', changed: '證據已改變', newly_available: '新增可用證據', no_longer_available: '證據不再可用',
	stale: '資料較舊', partial: '部分資料', unavailable: '無法取得', still_supported: '仍有證據支持', weakened: '支持減弱',
	contradicted_by_new_evidence: '新證據與先前假設衝突', insufficient_evidence: '資料不足', not_comparable: '無法比較',
};
const domainLabels: Record<string, string> = { price: '市場價格', market: '市場狀態', industry: '產業', institutional: '法人籌碼', margin: '融資融券', fundamentals: '基本面／營收／估值', corporate_events: '公司事件' };
const completenessLabels: Record<string, string> = { complete: '證據完整', limited: '證據有限', partial: '部分證據' };

function displayValue(value: unknown): string {
	if (value === null || value === undefined || value === '') return '未提供';
	if (typeof value === 'number') return value.toLocaleString('zh-TW', { maximumFractionDigits: 4 });
	if (typeof value === 'boolean') return value ? '是' : '否';
	return String(value);
}

export function ResearchHistoryList({ runs, activeRunID, onOpen, onCompare }: { runs: ResearchHistorySummary[]; activeRunID?: string; onOpen: (runID: string) => void; onCompare: (runID: string) => void }) {
	if (runs.length === 0) return <div className="taiwan-empty-state"><strong>尚無研究歷史</strong><p>成功完成第一份 AI 研究後，結構化證據與結果會保存在目前裝置。</p></div>;
	return <div className="taiwan-research-history-list">{runs.map((run, index) => <article key={run.run_id} className={activeRunID === run.run_id ? 'active' : ''}>
		<header><div><strong>{index === 0 ? '最新研究' : '歷史研究'}</strong><time>{new Date(run.created_at).toLocaleString('zh-TW', { hour12: false })}</time></div><span>{run.research_version}</span></header>
		<p>證據截至 {run.evidence_as_of || '未提供'} · {completenessLabels[run.completeness] || run.completeness}{run.stale ? ' · 資料較舊' : ''}{run.partial ? ' · 部分資料' : ''}</p>
		<small>{[run.model_provider, run.model_name].filter(Boolean).join(' · ') || '模型資訊未提供'}</small>
		<footer><button type="button" onClick={() => onOpen(run.run_id)}>開啟研究</button>{run.has_previous ? <button type="button" onClick={() => onCompare(run.run_id)}><GitCompareArrows size={13} />與前次研究比較</button> : <span>沒有更早的成功研究</span>}</footer>
	</article>)}</div>;
}

export function ResearchComparisonView({ payload }: { payload: ComparisonPayload }) {
	if (!payload.available) return <div className="taiwan-empty-state"><strong>沒有可比較的前次研究</strong><p>{payload.reason || '完成第二份成功研究後即可比較。'}</p></div>;
	const comparison = payload.comparison;
	if (!comparison) return <div className="market-partial-warning">比較資料格式不完整。</div>;
	if (!comparison.comparable) return <div className="market-partial-warning"><strong>此歷史版本無法完整比較</strong><p>{comparison.reason || '版本不相容，系統不會猜測舊資料語意。'}</p></div>;
	return <div className="taiwan-research-comparison">
		<section className="taiwan-comparison-summary" aria-label="研究比較摘要"><strong>證據變化摘要</strong><div><span>改變 {comparison.summary.changed}</span><span>新增 {comparison.summary.newly_available}</span><span>不再可用 {comparison.summary.no_longer_available}</span><span>新事件 {comparison.summary.corporate_events_added}</span></div></section>
		<div className="taiwan-comparison-domains">{comparison.domains.map((domain) => <article key={domain.domain}>
			<header><strong>{domainLabels[domain.domain] || domain.domain}</strong><span>{comparisonLabels[domain.state] || domain.state}</span></header>
			{domain.changed_fields.length > 0 && <ul>{domain.changed_fields.map((field) => <li key={field.path}><code>{field.path}</code><span>{displayValue(field.before)} → {displayValue(field.after)}</span>{field.delta !== undefined && <small>變化 {displayValue(field.delta)}{field.delta_percent !== undefined ? `（${displayValue(field.delta_percent)}%）` : ''}</small>}</li>)}</ul>}
		</article>)}</div>
		<section className="taiwan-comparison-assumptions"><strong>先前研究依據</strong>{comparison.assumptions.length === 0 ? <p>舊版研究沒有足夠的結構化依據可比較。</p> : <ul>{comparison.assumptions.map((item, index) => <li key={`${item.kind}-${item.section || index}`}><span>{item.text}</span><strong>{comparisonLabels[item.status] || item.status}</strong></li>)}</ul>}</section>
		{(comparison.corporate_events_added.length > 0 || comparison.corporate_events_removed.length > 0) && <section className="taiwan-comparison-events"><strong>公司事件變化</strong>{comparison.corporate_events_added.map((event) => <p key={`added-${event.event_id}`}>新增：{event.title || event.event_id}{event.category ? ` · ${event.category}` : ''}</p>)}{comparison.corporate_events_removed.map((event) => <p key={`removed-${event.event_id}`}>不再出現在快照：{event.title || event.event_id}{event.category ? ` · ${event.category}` : ''}</p>)}</section>}
	</div>;
}

export function TaiwanResearchHistory({ config, canonical, refreshKey, openRequest = 0, onOpenResearch }: { config: BackendConfig | null; canonical: string; refreshKey: number; openRequest?: number; onOpenResearch: (record: ResearchHistoryRecord) => void }) {
	const [open, setOpen] = useState(false);
	const [runs, setRuns] = useState<ResearchHistorySummary[]>([]);
	const [total, setTotal] = useState(0);
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState('');
	const [activeRunID, setActiveRunID] = useState('');
	const [comparison, setComparison] = useState<ComparisonPayload | null>(null);
	const [comparisonLoading, setComparisonLoading] = useState(false);
	const requestGate = useRef(new LatestRequestGate());
	const comparisonRequestGate = useRef(new LatestRequestGate());

	const load = async (offset = 0, append = false) => {
		if (!config || !canonical) return;
		const current = requestGate.current.begin();
		setLoading(true); setError('');
		try {
			const response = await fetchResearchHistoryPage(config, canonical, offset);
			if (requestGate.current.isCurrent(current)) { setRuns((existing) => append ? [...existing, ...response.data.runs] : response.data.runs); setTotal(response.data.total); }
		} catch (reason) { if (requestGate.current.isCurrent(current)) setError(taiwanErrorMessage(reason, '研究歷史暫時無法取得')); }
		finally { if (requestGate.current.isCurrent(current)) setLoading(false); }
	};

	useEffect(() => {
		setRuns([]); setTotal(0); setActiveRunID(''); setComparison(null); setError('');
		void load();
		return () => { requestGate.current.invalidate(); comparisonRequestGate.current.invalidate(); };
	// load is scoped to the selected canonical symbol and successful research refreshes.
	// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [config, canonical, refreshKey]);
	useEffect(() => { if (openRequest > 0) setOpen(true); }, [openRequest]);

	const openRun = async (runID: string) => {
		if (!config) return;
		comparisonRequestGate.current.invalidate(); setComparison(null); setComparisonLoading(false);
		const current = requestGate.current.begin();
		setLoading(true); setError('');
		try {
			const response = await fetchResearchHistoryRecord(config, canonical, runID);
			if (!requestGate.current.isCurrent(current)) return;
			setActiveRunID(runID); onOpenResearch(response.data);
		} catch (reason) { if (requestGate.current.isCurrent(current)) setError(taiwanErrorMessage(reason, '無法開啟這份研究')); }
		finally { if (requestGate.current.isCurrent(current)) setLoading(false); }
	};

	const compare = async (runID: string) => {
		if (!config) return;
		requestGate.current.invalidate();
		const current = comparisonRequestGate.current.begin();
		setComparisonLoading(true); setError('');
		try {
			const [record, response] = await Promise.all([fetchResearchHistoryRecord(config, canonical, runID), fetchResearchComparison(config, canonical, runID)]);
			if (comparisonRequestGate.current.isCurrent(current)) { setActiveRunID(runID); onOpenResearch(record.data); setComparison(response.data); }
		} catch (reason) { if (comparisonRequestGate.current.isCurrent(current)) setError(taiwanErrorMessage(reason, '研究比較暫時無法取得')); }
		finally { if (comparisonRequestGate.current.isCurrent(current)) setComparisonLoading(false); }
	};

	const returnLatest = () => { if (runs[0]) void openRun(runs[0].run_id); };
	return <section className="taiwan-research-history">
		<button type="button" className="taiwan-research-history-trigger" aria-expanded={open} onClick={() => setOpen((value) => !value)}><History size={15} />研究歷史 {runs.length > 0 && <span>{runs.length}</span>}<ChevronDown size={14} /></button>
		{open && <div className="taiwan-research-history-panel" aria-live="polite">
			<header><div><strong>研究歷史</strong><small>保存當時的研究與 bounded evidence 快照，不代表 canonical market fact。</small></div><div>{activeRunID && runs[0]?.run_id !== activeRunID && <button type="button" onClick={returnLatest}><ArrowLeft size={13} />返回最新研究</button>}<button type="button" onClick={() => void load()} disabled={loading}><RefreshCw className={loading ? 'spin' : ''} size={13} />重新整理</button></div></header>
			{loading && runs.length === 0 && <div className="taiwan-loading"><LoaderCircle className="spin" size={16} />正在讀取研究歷史</div>}
			{error && <div className="market-partial-warning" role="alert">{error}</div>}
			{(runs.length > 0 || (!loading && !error)) && <ResearchHistoryList runs={runs} activeRunID={activeRunID} onOpen={(runID) => void openRun(runID)} onCompare={(runID) => void compare(runID)} />}
			{runs.length < total && <button type="button" className="taiwan-research-history-more" onClick={() => void load(runs.length, true)} disabled={loading}>{loading ? <LoaderCircle className="spin" size={13} /> : null}載入更多較舊研究</button>}
			{comparisonLoading && <div className="taiwan-loading"><LoaderCircle className="spin" size={16} />正在比較結構化證據</div>}
			{comparison && !comparisonLoading && <ResearchComparisonView payload={comparison} />}
		</div>}
	</section>;
}
