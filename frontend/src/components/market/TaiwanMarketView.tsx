import { LoaderCircle, Search } from 'lucide-react';
import { useEffect, useState } from 'react';
import type { FormEvent } from 'react';
import type { BackendConfig, KLine, MarketIndexSeries, Quote, SecurityIdentity } from '../../lib/backend';
import { requestJSON } from '../../lib/backend';
import { CoreIndexView, SourceNotice } from './MarketDataViews';

export function TaiwanMarketView({ config, refreshKey }: { config: BackendConfig | null; refreshKey: number }) {
	const [query, setQuery] = useState('2330');
	const [matches, setMatches] = useState<SecurityIdentity[]>([]);
	const [selected, setSelected] = useState<SecurityIdentity | null>(null);
	const [quote, setQuote] = useState<Quote | null>(null);
	const [lines, setLines] = useState<KLine[]>([]);
	const [indexes, setIndexes] = useState<MarketIndexSeries[]>([]);
	const [selectedIndex, setSelectedIndex] = useState('taiex');
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState('');

	const search = async (value = query) => {
		if (!config || !value.trim()) return;
		setLoading(true); setError('');
		try {
			const payload = await requestJSON<{ data: { securities: SecurityIdentity[] } }>(config, `/api/v1/tw/securities?query=${encodeURIComponent(value.trim())}`);
			setMatches(payload.data.securities);
			if (payload.data.securities.length === 1) await select(payload.data.securities[0]);
		} catch (reason) { setError(reason instanceof Error ? reason.message : '台股搜尋失敗'); }
		finally { setLoading(false); }
	};

	const select = async (security: SecurityIdentity) => {
		if (!config) return;
		setSelected(security); setLoading(true); setError('');
		try {
			const [quotePayload, linePayload] = await Promise.all([
				requestJSON<{ data: Quote[] }>(config, `/api/v1/tw/quotes?symbols=${encodeURIComponent(security.canonical)}`),
				requestJSON<{ data: KLine[] }>(config, `/api/v1/tw/kline?symbol=${encodeURIComponent(security.canonical)}&limit=120`),
			]);
			setQuote(quotePayload.data[0] || null); setLines(linePayload.data);
		} catch (reason) { setError(reason instanceof Error ? reason.message : '台股行情載入失敗'); }
		finally { setLoading(false); }
	};

	useEffect(() => {
		if (!config) return;
		requestJSON<{ data: MarketIndexSeries[] }>(config, '/api/v1/tw/indexes').then((payload) => setIndexes(payload.data)).catch((reason) => setError(reason instanceof Error ? reason.message : '台股指數載入失敗'));
		void search('2330');
	}, [config, refreshKey]);

	const submit = (event: FormEvent) => { event.preventDefault(); void search(); };
	const indexSnapshots = indexes.map((item) => item.index);
	const indexSeries = indexes.find((item) => item.index.id === selectedIndex) || null;
	return <div className="market-data-view taiwan-market-view">
		<form className="market-filter" onSubmit={submit}><label><Search size={15} /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="輸入 2330、台積電或 canonical symbol" /></label><button type="submit" disabled={loading}>{loading ? <LoaderCircle className="spin" size={14} /> : '搜尋'}</button></form>
		{error && <div className="market-partial-warning">{error}</div>}
		{matches.length > 1 && <div className="taiwan-search-results">{matches.map((item) => <button type="button" key={item.canonical} onClick={() => void select(item)}><strong>{item.code} {item.name}</strong><span>{item.exchange} · {item.security_type.toUpperCase()} · {item.currency}</span></button>)}</div>}
		{selected && quote && <section className="market-index-detail"><header><div><span>{selected.exchange} · {selected.security_type.toUpperCase()} · {selected.currency}</span><h3>{selected.name} {selected.code}</h3><small>{quote.meta.is_realtime ? '即時行情' : '官方收盤資料'} · {quote.meta.trade_date}</small></div><div><strong>{quote.price.toLocaleString('zh-TW')}</strong><em className={quote.change_percent > 0 ? 'up' : quote.change_percent < 0 ? 'down' : 'flat'}>{quote.change_percent > 0 ? '+' : ''}{quote.change_percent.toFixed(2)}%</em></div></header><SourceNotice meta={quote.meta} /><div className="market-kline-table"><header><span>日期</span><span>開盤</span><span>最高</span><span>最低</span><span>收盤</span><span>漲跌</span></header>{lines.slice(-10).reverse().map((line) => <article key={line.time}><span>{new Date(line.time).toLocaleDateString('zh-TW')}</span><span>{line.open}</span><span>{line.high}</span><span>{line.low}</span><strong>{line.close}</strong><em>{(line.change_percent || 0).toFixed(2)}%</em></article>)}</div></section>}
		<CoreIndexView indexes={indexSnapshots} selectedID={selectedIndex} onSelect={setSelectedIndex} series={indexSeries} seriesLoading={false} meta={indexSeries?.meta || null} />
	</div>;
}
