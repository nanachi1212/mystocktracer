import { Activity, BarChart3, Building2, LoaderCircle } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import type { BackendConfig } from '../lib/backend';
import { requestJSON } from '../lib/backend';
import { formatTaiwanPercent, formatTaiwanRatio, formatTaiwanTWD, runScopedRequest, taiwanErrorMessage, taiwanMarketPath, taiwanScopes, taiwanStatusLabel, type TaiwanScope } from '../lib/taiwan-product';
import { TaiwanMarketView } from './market/TaiwanMarketView';

type View = 'overview' | 'breadth' | 'emotion' | 'industry';
export type CommonScope = { scope: string; status: string; freshness: string; as_of: string | null; target_latest_trading_date: string; included_exchanges: string[]; missing_exchanges: string[] };
type Breadth = CommonScope & { advancers: number; decliners: number; unchanged: number; no_trade: number; unknown: number; universe_count: number; advance_ratio: number | null; advancing_amount_ratio: number | null; total_amount_twd: number; missing_amount_count: number };
type Emotion = CommonScope & { model_version: string; confidence: string; state: string; raw: Breadth; components: { breadth_participation: string; capital_participation: string; breadth_capital_relationship: string }; coverage: { direction_coverage: number | null; amount_coverage: number | null } };
type Industry = { industry_id: string; industry_name: string; exchange: string; constituent_count: number; relative_breadth: number | null; relative_capital: number | null; data_quality: { direction_coverage: number | null; amount_coverage: number | null } };
type IndustryScope = CommonScope & { snapshot_status: string; taxonomy_status: string; industry_coverage: number | null; classified_count: number; eligible_universe_count: number; industries: Industry[] };

export function TaiwanMarketWorkspace({ config, refreshKey, view }: { config: BackendConfig | null; refreshKey: number; view: View }) {
	const [scope, setScope] = useState<TaiwanScope>('combined');
	const [data, setData] = useState<Breadth | Emotion | IndustryScope | null>(null);
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState('');
	const scopeRequestID = useRef(0);
	useEffect(() => {
		if (!config || view === 'overview') return;
		const endpoint = view === 'breadth' ? 'market-breadth' : view === 'emotion' ? 'market-emotion' : 'industry-radar';
		void runScopedRequest(scopeRequestID, () => requestJSON<{ data: Breadth | Emotion | IndustryScope }>(config, taiwanMarketPath(endpoint, scope)), {
			// Clear the previous scope's data immediately: the scope tabs already show the new
			// selection, so the data below must never keep rendering the old scope unlabeled.
			onStart: () => { setData(null); setLoading(true); setError(''); },
			onSuccess: (payload) => setData(payload.data),
			onError: (reason) => { setData(null); setError(taiwanErrorMessage(reason, '台灣市場資料載入失敗')); },
			onSettle: () => setLoading(false),
		});
	}, [config, refreshKey, scope, view]);
	if (view === 'overview') return <TaiwanMarketView config={config} refreshKey={refreshKey} />;
	return <div className="taiwan-product-workspace">
		<div className="taiwan-scope-tabs" aria-label="市場範圍">{taiwanScopes.map((item) => <button type="button" key={item.id} className={scope === item.id ? 'active' : ''} onClick={() => setScope(item.id)}>{item.label}</button>)}</div>
		{loading && <div className="taiwan-loading"><LoaderCircle className="spin" size={18} />正在讀取官方市場資料</div>}
		{error && <div className="market-partial-warning">{error}</div>}
		{data && <FreshnessHeader data={data} />}
		{data && view === 'breadth' && <BreadthView data={data as Breadth} />}
		{data && view === 'emotion' && <EmotionView data={data as Emotion} />}
		{data && view === 'industry' && <IndustryView data={data as IndustryScope} />}
	</div>;
}

export function FreshnessHeader({ data }: { data: CommonScope }) {
	return <section className="taiwan-status-card"><div><strong>{data.scope}</strong><span className={`taiwan-status ${data.status}`}>{taiwanStatusLabel(data.status)}</span><span>{taiwanStatusLabel(data.freshness)}</span></div><small>資料日期 {data.as_of || '未提供'} · 最新完成交易日 {data.target_latest_trading_date || '未提供'}</small>{data.missing_exchanges?.length > 0 && <p>缺少市場：{data.missing_exchanges.join('、')}；已納入：{data.included_exchanges.join('、') || '無'}</p>}</section>;
}

function BreadthView({ data }: { data: Breadth }) {
	return <><section className="taiwan-metric-grid">{[
		['上漲家數', data.advancers], ['下跌家數', data.decliners], ['平盤', data.unchanged], ['無成交', data.no_trade], ['未知方向', data.unknown], ['市場母體', data.universe_count],
	].map(([label, value]) => <article key={String(label)}><span>{label}</span><strong>{Number(value).toLocaleString('zh-TW')}</strong></article>)}</section>
	<section className="taiwan-detail-grid"><article><Activity /><span>上漲家數比率</span><strong>{formatTaiwanRatio(data.advance_ratio)}</strong></article><article><BarChart3 /><span>上漲成交占比</span><strong>{formatTaiwanRatio(data.advancing_amount_ratio)}</strong></article><article><span>總成交額</span><strong>{formatTaiwanTWD(data.total_amount_twd)}</strong><small>缺少成交額 {data.missing_amount_count.toLocaleString('zh-TW')} 筆</small></article></section></>;
}

function EmotionView({ data }: { data: Emotion }) {
	return <><section className="taiwan-emotion-summary"><div><span>市場狀態</span><strong>{taiwanStatusLabel(data.state)}</strong></div><div><span>資料信心</span><strong>{taiwanStatusLabel(data.confidence)}</strong></div><small>模型 {data.model_version}</small></section><section className="taiwan-detail-grid"><article><span>市場參與</span><strong>{taiwanStatusLabel(data.components.breadth_participation)}</strong><small>上漲家數比率 {formatTaiwanRatio(data.raw.advance_ratio)}</small></article><article><span>成交方向</span><strong>{taiwanStatusLabel(data.components.capital_participation)}</strong><small>上漲成交占比 {formatTaiwanRatio(data.raw.advancing_amount_ratio)}</small></article><article><span>家數與成交關係</span><strong>{taiwanStatusLabel(data.components.breadth_capital_relationship)}</strong><small>方向涵蓋 {formatTaiwanRatio(data.coverage.direction_coverage)} · 成交涵蓋 {formatTaiwanRatio(data.coverage.amount_coverage)}</small></article></section></>;
}

function IndustryView({ data }: { data: IndustryScope }) {
	return <><section className="taiwan-detail-grid"><article><Building2 /><span>官方分類涵蓋</span><strong>{formatTaiwanRatio(data.industry_coverage)}</strong><small>{data.classified_count.toLocaleString('zh-TW')} / {data.eligible_universe_count.toLocaleString('zh-TW')} 檔</small></article><article><span>分類狀態</span><strong>{taiwanStatusLabel(data.taxonomy_status)}</strong></article><article><span>市場快照</span><strong>{taiwanStatusLabel(data.snapshot_status)}</strong></article></section><div className="taiwan-industry-list">{data.industries.map((item) => <article key={item.industry_id}><header><strong>{item.industry_name}</strong><span>{item.exchange} · {item.constituent_count} 檔</span></header><div><span>相對市場廣度 <b>{formatTaiwanPercent(item.relative_breadth == null ? null : item.relative_breadth * 100, true)}</b></span><span>相對成交方向 <b>{formatTaiwanPercent(item.relative_capital == null ? null : item.relative_capital * 100, true)}</b></span></div><small>方向涵蓋 {formatTaiwanRatio(item.data_quality.direction_coverage)} · 成交涵蓋 {formatTaiwanRatio(item.data_quality.amount_coverage)}</small></article>)}</div></>;
}
