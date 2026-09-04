import { Bot, LoaderCircle, Search } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import type { FormEvent } from 'react';
import type { BackendConfig, SecurityIdentity } from '../lib/backend';
import { requestJSON } from '../lib/backend';
import { formatTaiwanPercent, runScopedRequest, taiwanComponentList, taiwanErrorMessage, taiwanIntelligencePath, taiwanReasonLabel, taiwanResearchPath, taiwanSecurityTypeLabel, taiwanStatusLabel } from '../lib/taiwan-product';

export type Evidence = { status: string; freshness?: string; as_of?: string; reason?: string; data?: Record<string, unknown> };
export type Component = { state: string; status: string; freshness?: string; as_of?: string; reasons: string[] };
export type Intelligence = {
	model_version: string; symbol: string;
	identity: { canonical_symbol: string; code: string; name: string; exchange: string; currency: string; security_type: string; industry_name?: string };
	quote: Evidence & { target_latest_completed_trading_date?: string; data?: { price: number; change_percent: number; meta?: { trade_date?: string; is_realtime?: boolean } } };
	price_history_summary: Evidence & { return_5d_percent: number | null; return_20d_percent: number | null; latest_bar_date?: string };
	fundamentals: Evidence; institutional: Evidence; margin: Evidence;
	market_context: Evidence & { state?: string; confidence?: string; advance_ratio?: number | null; advancing_amount_ratio?: number | null };
	industry_context: Evidence & { taxonomy_status?: string; industry?: { industry_name: string; relative_breadth: number | null; relative_capital: number | null } };
	interpretation?: { model_version: string; components: Record<string, Component>; data_quality: { available_components: string[] | null; indeterminate_components: string[] | null; unavailable_components: string[] | null; stale_components: string[] | null; partial_components: string[] | null } };
};
type ResearchSection = { text: string; evidence_keys: string[] };
type Research = { status: string; model_version: string; generated_at?: string; headline?: string; summary?: string; sections: Record<string, ResearchSection>; conflicts: string[]; data_limitations: string[]; research_notes: string[]; reason?: string };

const componentLabels: Record<string, string> = { price: '價格', market: '市場', price_market_relationship: '個股與市場關係', industry: '產業', institutional: '法人', margin: '融資融券', fundamentals: '基本面' };
const researchLabels: Record<string, string> = { price: '價格', market: '市場', industry: '產業', institutional: '法人', margin: '融資融券', fundamentals: '基本面' };

export function TaiwanStockResearchWorkspace({ config, refreshKey }: { config: BackendConfig | null; refreshKey: number }) {
	const [query, setQuery] = useState('2330');
	const [matches, setMatches] = useState<SecurityIdentity[]>([]);
	const [selected, setSelected] = useState<SecurityIdentity | null>(null);
	const [intelligence, setIntelligence] = useState<Intelligence | null>(null);
	const [research, setResearch] = useState<Research | null>(null);
	const [loading, setLoading] = useState(false);
	const [researching, setResearching] = useState(false);
	const [error, setError] = useState('');
	const selectRequestID = useRef(0);

	const search = async (value = query) => {
		if (!config || !value.trim()) return;
		setLoading(true); setError('');
		try {
			const payload = await requestJSON<{ data: { securities: SecurityIdentity[] } }>(config, `/api/v1/tw/securities?query=${encodeURIComponent(value.trim())}`);
			setMatches(payload.data.securities);
			if (payload.data.securities.length === 1) await select(payload.data.securities[0]);
		} catch (reason) { setError(taiwanErrorMessage(reason, '台灣證券搜尋失敗')); }
		finally { setLoading(false); }
	};
	const select = async (security: SecurityIdentity) => {
		if (!config) return;
		setSelected(security);
		// Clear the previous selection's data immediately so a still-loading new symbol never
		// renders under the old symbol's heading/evidence — and drop a stale response if the
		// user has since selected another symbol (last-wins by selection order, not arrival order).
		await runScopedRequest(selectRequestID, () => requestJSON<{ data: Intelligence }>(config, taiwanIntelligencePath(security.canonical)), {
			onStart: () => { setIntelligence(null); setResearch(null); setLoading(true); setError(''); },
			onSuccess: (payload) => setIntelligence(payload.data),
			onError: (reason) => { setIntelligence(null); setError(taiwanErrorMessage(reason, '台灣個股研究資料載入失敗')); },
			onSettle: () => setLoading(false),
		});
	};
	const generateResearch = async () => {
		if (!config || !selected) return;
		setResearching(true); setError('');
		try {
			const payload = await requestJSON<{ data: { intelligence: Intelligence; ai_research: Research } }>(config, taiwanResearchPath(selected.canonical), { method: 'POST' });
			setIntelligence(payload.data.intelligence); setResearch(payload.data.ai_research);
		} catch (reason) { setError(taiwanErrorMessage(reason, 'AI 研究目前無法使用')); }
		finally { setResearching(false); }
	};
	useEffect(() => { if (config) void search('2330'); }, [config, refreshKey]);
	const submit = (event: FormEvent) => { event.preventDefault(); void search(); };
	return <div className="taiwan-product-workspace taiwan-stock-research">
		<form className="market-filter" onSubmit={submit}><label><Search size={15} /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="輸入 2330、台積電、2330.TWSE 或 6488.TPEX" aria-label="台灣證券名稱或代碼" /></label><button type="submit" disabled={loading}>{loading ? <LoaderCircle className="spin" size={14} /> : '搜尋'}</button></form>
		{error && <div className="market-partial-warning">{error}</div>}
		{matches.length > 1 && <div className="taiwan-search-results">{matches.map((item) => <button type="button" key={item.canonical} onClick={() => void select(item)}><strong>{item.code} {item.name}</strong><span>{item.exchange} · {taiwanSecurityTypeLabel(item.security_type)}</span></button>)}</div>}
		{loading && <div className="taiwan-loading"><LoaderCircle className="spin" size={18} />正在讀取官方個股資料</div>}
		{intelligence && <>
			<section className="taiwan-stock-heading"><div><span>{intelligence.identity.exchange} · {taiwanSecurityTypeLabel(intelligence.identity.security_type)} · {intelligence.identity.currency}</span><h2>{intelligence.identity.name} {intelligence.identity.code}</h2><small>{intelligence.identity.canonical_symbol}{intelligence.identity.industry_name ? ` · ${intelligence.identity.industry_name}` : ''}</small></div>{intelligence.quote.data && <div><strong>{intelligence.quote.data.price.toLocaleString('zh-TW')}</strong><em className={intelligence.quote.data.change_percent > 0 ? 'up' : intelligence.quote.data.change_percent < 0 ? 'down' : 'flat'}>{formatTaiwanPercent(intelligence.quote.data.change_percent, true)}</em></div>}</section>
			<section className="taiwan-status-card"><div><span className={`taiwan-status ${intelligence.quote.status}`}>{taiwanStatusLabel(intelligence.quote.status)}</span><span>{taiwanStatusLabel(intelligence.quote.freshness)}</span></div><small>資料日期 {intelligence.quote.as_of || intelligence.quote.data?.meta?.trade_date || '未提供'} · 最新完成交易日 {intelligence.quote.target_latest_completed_trading_date || '未提供'}</small></section>
			<section className="taiwan-detail-grid"><article><span>5 日收盤報酬</span><strong>{formatTaiwanPercent(intelligence.price_history_summary.return_5d_percent, true)}</strong><small>{intelligence.price_history_summary.latest_bar_date || '資料不足'}</small></article><article><span>20 日收盤報酬</span><strong>{formatTaiwanPercent(intelligence.price_history_summary.return_20d_percent, true)}</strong></article><article><span>市場狀態</span><strong>{taiwanStatusLabel(intelligence.market_context.state)}</strong><small>資料信心 {taiwanStatusLabel(intelligence.market_context.confidence)}</small></article></section>
			<EvidenceOverview intelligence={intelligence} />
			{intelligence.interpretation && <InterpretationView intelligence={intelligence} />}
			<section className="taiwan-ai-action"><div><strong>AI 研究</strong><p>AI 只會在你按下按鈕後，依上方官方資料與確定性解讀產生摘要。</p></div><button type="button" onClick={() => void generateResearch()} disabled={researching}>{researching ? <><LoaderCircle className="spin" size={14} />產生中</> : <><Bot size={14} />產生 AI 研究摘要</>}</button></section>
			{research && <ResearchView research={research} />}
		</>}
		{!intelligence && !loading && <div className="taiwan-empty-state"><strong>選擇台灣證券開始研究</strong><p>可搜尋上市或上櫃股票；原始資料載入不會呼叫 AI。</p></div>}
	</div>;
}

function EvidenceOverview({ intelligence }: { intelligence: Intelligence }) {
	const industry = intelligence.industry_context.industry;
	const items: [string, Evidence, string][] = [
		['基本面', intelligence.fundamentals, intelligence.fundamentals.reason || '官方財務與營收資料'],
		['法人', intelligence.institutional, intelligence.institutional.reason || '外資、投信與自營商資料'],
		['融資融券', intelligence.margin, intelligence.margin.reason || '融資與融券餘額資料'],
		['產業', intelligence.industry_context, industry ? `${industry.industry_name} · 相對市場廣度 ${formatTaiwanPercent(industry.relative_breadth == null ? null : industry.relative_breadth * 100, true)} · 相對成交方向 ${formatTaiwanPercent(industry.relative_capital == null ? null : industry.relative_capital * 100, true)}` : intelligence.industry_context.reason || '官方產業分類資料'],
	];
	return <section className="taiwan-evidence-overview"><header><span>官方證據</span><h3>資料涵蓋與狀態</h3></header><div className="taiwan-detail-grid">{items.map(([label, item, detail]) => <article key={label}><span>{label}</span><strong>{taiwanStatusLabel(item.status)}</strong><small>{item.as_of ? `資料日期 ${item.as_of}` : taiwanStatusLabel(item.freshness)}</small><p>{taiwanReasonLabel(detail)}</p></article>)}</div></section>;
}

export function InterpretationView({ intelligence }: { intelligence: Intelligence }) {
	const interpretation = intelligence.interpretation!;
	const availableComponents = taiwanComponentList(interpretation.data_quality.available_components);
	const indeterminateComponents = taiwanComponentList(interpretation.data_quality.indeterminate_components);
	const unavailableComponents = taiwanComponentList(interpretation.data_quality.unavailable_components);
	const staleComponents = taiwanComponentList(interpretation.data_quality.stale_components);
	const partialComponents = taiwanComponentList(interpretation.data_quality.partial_components);
	return <section className="taiwan-interpretation"><header><div><span>確定性解讀</span><h3>各項證據狀態</h3></div><small>{interpretation.model_version}</small></header><div>{Object.entries(interpretation.components).map(([key, item]) => <article key={key}><header><strong>{componentLabels[key] || key}</strong><span>{taiwanStatusLabel(item.state)} · {taiwanStatusLabel(item.status)}</span></header><small>{item.as_of ? `資料日期 ${item.as_of}` : taiwanStatusLabel(item.freshness)}</small><ul>{item.reasons.map((reason) => <li key={reason}>{taiwanReasonLabel(reason)}</li>)}</ul></article>)}</div><footer>可用 {availableComponents.length} 項 · 無法判定 {indeterminateComponents.length} 項 · 無法取得／不適用 {unavailableComponents.length} 項{staleComponents.length > 0 && ` · 資料較舊 ${staleComponents.length} 項`}{partialComponents.length > 0 && ` · 部分資料 ${partialComponents.length} 項`}</footer></section>;
}

function ResearchView({ research }: { research: Research }) {
	if (research.status !== 'available') return <section className="taiwan-research-unavailable"><strong>AI 研究目前無法使用</strong><p>下方官方資料與確定性解讀仍可正常查看。</p></section>;
	return <section className="taiwan-research-result"><header><span>AI 研究</span><h3>{research.headline}</h3><p>{research.summary}</p><small>{research.generated_at ? new Date(research.generated_at).toLocaleString('zh-TW') : ''} · {research.model_version}</small></header><div>{Object.entries(research.sections).map(([key, section]) => <article key={key}><strong>{researchLabels[key] || key}</strong><p>{section.text}</p><small>證據：{section.evidence_keys.join('、')}</small></article>)}</div>{[['證據衝突', research.conflicts], ['資料限制', research.data_limitations], ['研究備註', research.research_notes]].map(([label, items]) => Array.isArray(items) && items.length > 0 && <section key={String(label)}><strong>{label}</strong><ul>{items.map((item) => <li key={item}>{item}</li>)}</ul></section>)}</section>;
}
