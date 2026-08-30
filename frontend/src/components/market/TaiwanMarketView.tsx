import { LoaderCircle, Search } from 'lucide-react';
import { useEffect, useState } from 'react';
import type { FormEvent } from 'react';
import type { BackendConfig, InstitutionalHistory, KLine, MarginHistory, MarketIndexSeries, Quote, SecurityIdentity } from '../../lib/backend';
import { requestJSON } from '../../lib/backend';
import { CoreIndexView, SourceNotice } from './MarketDataViews';

export function TaiwanMarketView({ config, refreshKey }: { config: BackendConfig | null; refreshKey: number }) {
	const [query, setQuery] = useState('2330');
	const [matches, setMatches] = useState<SecurityIdentity[]>([]);
	const [selected, setSelected] = useState<SecurityIdentity | null>(null);
	const [quote, setQuote] = useState<Quote | null>(null);
	const [lines, setLines] = useState<KLine[]>([]);
	const [indexes, setIndexes] = useState<MarketIndexSeries[]>([]);
	const [institutional, setInstitutional] = useState<InstitutionalHistory | null>(null);
	const [margin, setMargin] = useState<MarginHistory | null>(null);
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
			const [quotePayload, linePayload, institutionalPayload, marginPayload] = await Promise.all([
				requestJSON<{ data: Quote[] }>(config, `/api/v1/tw/quotes?symbols=${encodeURIComponent(security.canonical)}`),
				requestJSON<{ data: KLine[] }>(config, `/api/v1/tw/kline?symbol=${encodeURIComponent(security.canonical)}&limit=120`),
				requestJSON<{ data: InstitutionalHistory }>(config, `/api/v1/tw/institutional?symbol=${encodeURIComponent(security.canonical)}&limit=20`),
				requestJSON<{ data: MarginHistory }>(config, `/api/v1/tw/margin?symbol=${encodeURIComponent(security.canonical)}&limit=20`),
			]);
			setQuote(quotePayload.data[0] || null); setLines(linePayload.data);
			setInstitutional(institutionalPayload.data); setMargin(marginPayload.data);
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
		{institutional && margin && <ChipView institutional={institutional} margin={margin} />}
		<CoreIndexView indexes={indexSnapshots} selectedID={selectedIndex} onSelect={setSelectedIndex} series={indexSeries} seriesLoading={false} meta={indexSeries?.meta || null} />
	</div>;
}

function ChipView({ institutional, margin }: { institutional: InstitutionalHistory; margin: MarginHistory }) {
	const latest = institutional.data.at(-1); const latestMargin = margin.data.at(-1); const summary = institutional.summary;
	const lots = (value?: number) => value == null ? '—' : `${(value / 1000).toLocaleString('zh-TW')} 張`;
	const signedLots = (value?: number) => value == null ? '—' : `${value > 0 ? '+' : ''}${(value / 1000).toLocaleString('zh-TW')} 張`;
	return <section className="taiwan-chip-panel"><header><div><span>CHIP DATA</span><h3>籌碼</h3></div><small>{latest?.trade_date || '資料不足'} · 單位顯示為張</small></header>
		<div className="taiwan-chip-grid">{[
			['外資', latest?.foreign_buy, latest?.foreign_sell, latest?.foreign_net, summary.foreign_net_5d, summary.foreign_net_20d],
			['投信', latest?.investment_trust_buy, latest?.investment_trust_sell, latest?.investment_trust_net, summary.investment_trust_net_5d, summary.investment_trust_net_20d],
			['自營商', latest?.dealer_buy, latest?.dealer_sell, latest?.dealer_net, summary.dealer_net_5d, summary.dealer_net_20d],
		].map(([label,buy,sell,net,five,twenty]) => <article key={String(label)}><strong>{label}</strong><span>買入 {lots(buy as number)}</span><span>賣出 {lots(sell as number)}</span><em>買賣超 {signedLots(net as number)}</em><small>5日 {signedLots(five as number)} · 20日 {signedLots(twenty as number)}</small></article>)}</div>
		<div className="taiwan-margin-grid"><article><span>融資餘額</span><strong>{lots(latestMargin?.margin_balance)}</strong></article><article><span>融資增減</span><strong>{signedLots(latestMargin?.margin_change)}</strong></article><article><span>融券餘額</span><strong>{lots(latestMargin?.short_balance)}</strong></article><article><span>融券增減</span><strong>{signedLots(latestMargin?.short_change)}</strong></article><article><span>券資比</span><strong>{latestMargin?.short_margin_ratio == null ? '—' : `${latestMargin.short_margin_ratio.toFixed(2)}%`}</strong></article></div>
		<SourceNotice meta={institutional.meta} />
	</section>;
}
