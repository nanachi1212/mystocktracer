import { AlertTriangle } from 'lucide-react';
import type { MarketIndexSeries, MarketIndexSnapshot, SourceMeta } from '../../lib/backend';

// Phase B1 trimmed this module to the market-neutral index views the Taiwan market workspace
// consumes. The A-share billboard, fund-flow, margin-balance, industry-momentum and research
// views were removed with the rest of the China-market surface.

export function SourceNotice({ meta, locale = 'zh-CN' }: { meta: SourceMeta | null; locale?: 'zh-CN' | 'zh-TW' }) {
	if (!meta) return null;
	return <div className={`market-source-notice ${meta.stale ? 'stale' : ''}`}>
		<span>{locale === 'zh-TW' ? '來源' : '来源'} {meta.source} · {locale === 'zh-TW' ? '擷取' : '抓取'} {formatDateTime(meta.fetched_at, locale)}</span>
		{meta.fallback_reason && <em>{locale === 'zh-TW' ? '部分資料改用備援來源' : meta.fallback_reason}</em>}
	</div>;
}

export function CoreIndexView({ indexes, selectedID, onSelect, series, seriesLoading, meta, locale = 'zh-CN' }: {
	indexes: MarketIndexSnapshot[];
	selectedID: string;
	onSelect: (id: string) => void;
	series: MarketIndexSeries | null;
	seriesLoading: boolean;
	meta: SourceMeta | null;
	locale?: 'zh-CN' | 'zh-TW';
}) {
	const selected = indexes.find((item) => item.id === selectedID) || indexes[0];
	const lines = series?.lines || [];
	const first = lines[0]?.close || 0;
	const latest = lines.at(-1)?.close || selected?.price || 0;
	const returnPercent = first ? (latest / first - 1) * 100 : 0;
	const high = lines.length ? Math.max(...lines.map((line) => line.high)) : 0;
	const low = lines.length ? Math.min(...lines.map((line) => line.low)) : 0;
	return <div className="market-data-view">
		<SourceNotice meta={meta} locale={locale} />
		<div className="market-index-selector">{indexes.map((item) => <button type="button" className={item.id === selected?.id ? 'active' : ''} key={item.id} onClick={() => onSelect(item.id)}>
			<span>{item.name}</span><strong>{formatPrice(item.price)}</strong><em className={toneClass(item.change_percent)}>{formatPercent(item.change_percent)}</em>
		</button>)}</div>
		{selected ? <section className="market-index-detail">
			<header><div><span>{selected.region} · {selected.market}</span><h3>{selected.name}</h3><small>{locale === 'zh-TW' ? `最近 ${lines.length || '--'} 個交易日 · ${statusLabelTW(selected.status)}` : `最近 ${lines.length || '--'} 个交易周期 · ${statusLabel(selected.status)}`}</small></div><div><strong>{formatPrice(selected.price)}</strong><em className={toneClass(selected.change_percent)}>{formatPercent(selected.change_percent)}</em></div></header>
			<div className="market-index-chart-wrap">
				{seriesLoading ? <div className="market-chart-loading">{locale === 'zh-TW' ? '走勢圖載入中…' : '走势图加载中…'}</div> : <IndexLineChart lines={lines} />}
				<aside>
					<MiniStat label={locale === 'zh-TW' ? '區間報酬' : '区间收益'} value={formatPercent(returnPercent)} tone={toneClass(returnPercent)} />
					<MiniStat label={locale === 'zh-TW' ? '區間高點' : '区间高点'} value={formatPrice(high)} />
					<MiniStat label={locale === 'zh-TW' ? '區間低點' : '区间低点'} value={formatPrice(low)} />
					<MiniStat label={locale === 'zh-TW' ? '行情時間' : '行情时间'} value={formatDateTime(series?.index.trade_time || selected.trade_time, locale)} />
				</aside>
			</div>
			<div className="market-kline-table"><header><span>日期</span><span>{locale === 'zh-TW' ? '開盤' : '开盘'}</span><span>最高</span><span>最低</span><span>{locale === 'zh-TW' ? '收盤' : '收盘'}</span><span>{locale === 'zh-TW' ? '漲跌' : '涨跌'}</span></header>{lines.slice(-8).reverse().map((line) => <article key={line.time}><span>{formatDate(line.time, locale)}</span><span>{formatPrice(line.open)}</span><span>{formatPrice(line.high)}</span><span>{formatPrice(line.low)}</span><strong>{formatPrice(line.close)}</strong><em className={toneClass(line.change_percent || 0)}>{formatPercent(line.change_percent || 0)}</em></article>)}</div>
		</section> : <EmptyData title={locale === 'zh-TW' ? '目前沒有指數資料' : '暂无核心指数'} detail={locale === 'zh-TW' ? '等待官方指數資料恢復。' : '等待指数目录恢复。'} />}
	</div>;
}

function MiniStat({ label, value, tone = '' }: { label: string; value: string; tone?: string }) {
	return <article><small>{label}</small><strong className={tone}>{value}</strong></article>;
}

function IndexLineChart({ lines }: { lines: MarketIndexSeries['lines'] }) {
	if (lines.length < 2) return <div className="market-chart-loading">暂无足够走势数据</div>;
	const values = lines.map((line) => line.close);
	const minValue = Math.min(...values);
	const maxValue = Math.max(...values);
	const range = maxValue - minValue || 1;
	const points = values.map((value, index) => `${(index / (values.length - 1)) * 100},${92 - ((value - minValue) / range) * 80}`).join(' ');
	const area = `0,100 ${points} 100,100`;
	const up = values.at(-1)! >= values[0];
	return <div className={`market-index-chart ${up ? 'up' : 'down'}`}><svg viewBox="0 0 100 100" preserveAspectRatio="none" role="img" aria-label="指数收盘走势"><defs><linearGradient id="marketArea" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stopColor="currentColor" stopOpacity=".22" /><stop offset="1" stopColor="currentColor" stopOpacity="0" /></linearGradient></defs><polygon points={area} fill="url(#marketArea)" /><polyline points={points} fill="none" stroke="currentColor" strokeWidth="1.8" vectorEffect="non-scaling-stroke" /></svg><span>{formatPrice(maxValue)}</span><span>{formatPrice(minValue)}</span></div>;
}

function EmptyData({ title, detail }: { title: string; detail: string }) {
	return <div className="market-module-empty"><AlertTriangle size={22} /><strong>{title}</strong><span>{detail}</span></div>;
}

function formatPrice(value: number) {
	if (!Number.isFinite(value) || value === 0) return '--';
	return value >= 10_000 ? value.toLocaleString('zh-CN', { maximumFractionDigits: 1 }) : value.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

function formatPercent(value: number) {
	if (!Number.isFinite(value)) return '--';
	const fractionDigits = value !== 0 && Math.abs(value) < 0.01 ? 4 : 2;
	return `${value > 0 ? '+' : ''}${value.toFixed(fractionDigits)}%`;
}

function formatDateTime(value?: string, locale: 'zh-CN' | 'zh-TW' = 'zh-CN') {
	if (!value) return '--';
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) return value;
	return date.toLocaleString(locale, { hour12: false });
}

function formatDate(value?: string, locale: 'zh-CN' | 'zh-TW' = 'zh-CN') {
	if (!value) return '--';
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) return value.slice(0, 10);
	return date.toLocaleDateString(locale, { month: '2-digit', day: '2-digit' });
}

function statusLabelTW(status?: string) {
	return status === 'open' ? '交易中' : status === 'closed' ? '已收盤' : '狀態未知';
}

function statusLabel(status: string) {
	return status === 'open' ? '交易中' : status === 'closed' ? '已收盘' : '状态未知';
}

function toneClass(value: number) {
	return value > 0 ? 'up' : value < 0 ? 'down' : 'flat';
}
