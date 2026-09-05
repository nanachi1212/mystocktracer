import { LoaderCircle, Search, Star } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import type { FormEvent } from 'react';
import type { BackendConfig, InstitutionalHistory, KLine, MarginHistory, MarketIndexSeries, Quote, SecurityIdentity, TaiwanFundamentals } from '../../lib/backend';
import { requestJSON } from '../../lib/backend';
import { addTaiwanWatchlistSecurity, isTaiwanSecurityWatchlisted, removeTaiwanWatchlistSecurity, runScopedRequest, taiwanErrorMessage } from '../../lib/taiwan-product';
import { CoreIndexView, SourceNotice } from './MarketDataViews';

const DATASET_LABELS = { quote: '報價', kline: 'K 線', institutional: '法人買賣超', margin: '融資融券', fundamentals: '基本面' } as const;

export function partialFailureWarning(failed: string[]): string {
	return failed.length > 0 ? `部分個股資料目前無法取得：${failed.join('、')}` : '';
}

// `error` is a single shared state (indexes / search / selected-stock all write to it), so a
// source can only safely clear it on success if no other source has taken ownership since it
// started. This ref tracks who last set `error`, without triggering a render on its own.
type ErrorOwner = 'indexes' | 'search' | 'stock' | '';

export function TaiwanMarketView({ config, refreshKey }: { config: BackendConfig | null; refreshKey: number }) {
	const [query, setQuery] = useState('');
	const [matches, setMatches] = useState<SecurityIdentity[]>([]);
	const [selected, setSelected] = useState<SecurityIdentity | null>(null);
	const [quote, setQuote] = useState<Quote | null>(null);
	const [lines, setLines] = useState<KLine[]>([]);
	const [indexes, setIndexes] = useState<MarketIndexSeries[]>([]);
	const [institutional, setInstitutional] = useState<InstitutionalHistory | null>(null);
	const [margin, setMargin] = useState<MarginHistory | null>(null);
	const [fundamentals, setFundamentals] = useState<TaiwanFundamentals | null>(null);
	const [selectedIndex, setSelectedIndex] = useState('taiex');
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState('');
	const selectRequestID = useRef(0);
	const selectedRef = useRef<SecurityIdentity | null>(null);
	const errorOwnerRef = useRef<ErrorOwner>('');
	// Watchlist membership/mutation state is deliberately separate from `error`/`errorOwnerRef`
	// above (P1G) — a failed membership check or toggle must never interact with that ownership
	// model, so it gets its own small, independent state instead of reusing the shared banner.
	const [inWatchlist, setInWatchlist] = useState<boolean | null>(null);
	const [watchlistBusy, setWatchlistBusy] = useState(false);
	const [watchlistError, setWatchlistError] = useState('');
	const watchlistCheckID = useRef(0);

	const search = async (value = query) => {
		if (!config || !value.trim()) return;
		setLoading(true); errorOwnerRef.current = 'search'; setError('');
		try {
			const payload = await requestJSON<{ data: { securities: SecurityIdentity[] } }>(config, `/api/v1/tw/securities?query=${encodeURIComponent(value.trim())}`);
			setMatches(payload.data.securities);
			if (payload.data.securities.length === 1) await select(payload.data.securities[0]);
			else if (payload.data.securities.length === 0) setError(`找不到符合「${value.trim()}」的台灣證券`);
		} catch (reason) { errorOwnerRef.current = 'search'; setError(taiwanErrorMessage(reason, '台股搜尋失敗')); }
		finally { setLoading(false); }
	};

	const select = async (security: SecurityIdentity) => {
		if (!config) return;
		setSelected(security);
		// Independent of the main stock-data fetch below: check Watchlist membership for the
		// newly selected security so the toggle button can render the correct state. Scoped so a
		// stale check for a since-abandoned selection can never overwrite the current one. A
		// failure here is silent (button simply stays hidden) rather than surfacing through the
		// shared `error`/`errorOwnerRef` — this is not the P1G-owned overview error.
		setInWatchlist(null); setWatchlistError('');
		void runScopedRequest(watchlistCheckID, () => isTaiwanSecurityWatchlisted(config, security.canonical), {
			onSuccess: (saved) => setInWatchlist(saved),
		});
		// Clear the previous selection's data immediately so a still-loading new symbol never
		// renders under the old symbol's heading, and drop a stale response if the user has
		// since selected another symbol. Each dataset is settled independently (Promise.allSettled)
		// so one dataset failing does not discard the other datasets that succeeded.
		await runScopedRequest(selectRequestID, () => Promise.allSettled([
			requestJSON<{ data: Quote[] }>(config, `/api/v1/tw/quotes?symbols=${encodeURIComponent(security.canonical)}`),
			requestJSON<{ data: KLine[] }>(config, `/api/v1/tw/kline?symbol=${encodeURIComponent(security.canonical)}&limit=120`),
			requestJSON<{ data: InstitutionalHistory }>(config, `/api/v1/tw/institutional?symbol=${encodeURIComponent(security.canonical)}&limit=20`),
			requestJSON<{ data: MarginHistory }>(config, `/api/v1/tw/margin?symbol=${encodeURIComponent(security.canonical)}&limit=20`),
			requestJSON<{ data: TaiwanFundamentals }>(config, `/api/v1/tw/fundamentals?symbol=${encodeURIComponent(security.canonical)}&months=24`),
		]), {
			onStart: () => { setQuote(null); setLines([]); setInstitutional(null); setMargin(null); setFundamentals(null); setLoading(true); errorOwnerRef.current = 'stock'; setError(''); },
			onSuccess: ([quoteResult, klineResult, institutionalResult, marginResult, fundamentalsResult]) => {
				const failed: string[] = [];
				if (quoteResult.status === 'fulfilled') setQuote(quoteResult.value.data[0] || null); else failed.push(DATASET_LABELS.quote);
				if (klineResult.status === 'fulfilled') setLines(klineResult.value.data); else failed.push(DATASET_LABELS.kline);
				if (institutionalResult.status === 'fulfilled') setInstitutional(institutionalResult.value.data); else failed.push(DATASET_LABELS.institutional);
				if (marginResult.status === 'fulfilled') setMargin(marginResult.value.data); else failed.push(DATASET_LABELS.margin);
				if (fundamentalsResult.status === 'fulfilled') setFundamentals(fundamentalsResult.value.data); else failed.push(DATASET_LABELS.fundamentals);
				errorOwnerRef.current = failed.length > 0 ? 'stock' : '';
				setError(partialFailureWarning(failed));
			},
			onError: (reason) => { errorOwnerRef.current = 'stock'; setError(taiwanErrorMessage(reason, '台股行情載入失敗')); },
			onSettle: () => setLoading(false),
		});
	};

	// Adds/removes the currently selected security from the Watchlist. Deliberately does not
	// touch `selected`/quote/kline/etc. — Watchlist membership is independent of the currently
	// displayed stock data, so a toggle (success or failure) never reloads or clears it.
	const toggleWatchlist = async () => {
		if (!config || !selected || watchlistBusy || inWatchlist === null) return;
		setWatchlistBusy(true); setWatchlistError('');
		try {
			if (inWatchlist) { await removeTaiwanWatchlistSecurity(config, selected.canonical); setInWatchlist(false); }
			else { await addTaiwanWatchlistSecurity(config, selected.canonical); setInWatchlist(true); }
		} catch (reason) {
			setWatchlistError(taiwanErrorMessage(reason, inWatchlist ? '移除自選股失敗' : '加入自選股失敗'));
		} finally {
			setWatchlistBusy(false);
		}
	};

	useEffect(() => {
		if (!config) return;
		requestJSON<{ data: MarketIndexSeries[] }>(config, '/api/v1/tw/indexes')
			.then((payload) => {
				setIndexes(payload.data);
				// Only clear `error` if indexes is still its owner — a search/stock error that
				// took ownership while this request was in flight must survive indexes recovering.
				if (errorOwnerRef.current === 'indexes') { errorOwnerRef.current = ''; setError(''); }
			})
			.catch((reason) => { errorOwnerRef.current = 'indexes'; setError(taiwanErrorMessage(reason, '台股指數載入失敗')); });
	}, [config, refreshKey]);

	// Refresh retries the currently selected security (if any) in addition to the indexes effect
	// above. This effect only depends on [config, refreshKey] — not `selected` — so selecting a
	// stock does not itself re-trigger a fetch; only a refreshKey change (or config resolving) does.
	useEffect(() => {
		if (!config) return;
		if (selectedRef.current) void select(selectedRef.current);
	}, [config, refreshKey]);

	// Kept in sync with `selected` so the refresh effect above can read the current selection
	// without depending on `selected` itself (which would re-trigger a fetch on every selection).
	useEffect(() => { selectedRef.current = selected; }, [selected]);

	const submit = (event: FormEvent) => { event.preventDefault(); void search(); };
	const indexSnapshots = indexes.map((item) => item.index);
	const indexSeries = indexes.find((item) => item.index.id === selectedIndex) || null;
	return <div className="market-data-view taiwan-market-view">
		<form className="market-filter" onSubmit={submit}><label><Search size={15} /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="例如 2330、台積電、2330.TWSE 或 6488.TPEX" /></label><button type="submit" disabled={loading}>{loading ? <LoaderCircle className="spin" size={14} /> : '搜尋'}</button></form>
		{error && <div className="market-partial-warning">{error}</div>}
		{matches.length > 1 && <div className="taiwan-search-results">{matches.map((item) => <button type="button" key={item.canonical} onClick={() => void select(item)}><strong>{item.code} {item.name}</strong><span>{item.exchange} · {item.security_type.toUpperCase()} · {item.currency}</span></button>)}</div>}
		{selected && inWatchlist !== null && <div><button type="button" className={`taiwan-watchlist-toggle${inWatchlist ? ' saved' : ''}`} onClick={() => void toggleWatchlist()} disabled={watchlistBusy}>{watchlistBusy ? <LoaderCircle className="spin" size={14} /> : <Star size={14} />}{inWatchlist ? '移除自選' : '加入自選'}</button>{watchlistError && <span className="taiwan-watchlist-toggle-error">{watchlistError}</span>}</div>}
		{loading && selected && <div className="taiwan-loading"><LoaderCircle className="spin" size={18} />正在讀取官方個股資料</div>}
		{!selected && !loading && <div className="taiwan-empty-state"><strong>請選擇一檔證券查看台股資料</strong><p>搜尋上市或上櫃股票、ETF，選擇後即可查看報價、K 線、籌碼與基本面資料。</p></div>}
		{selected && quote && <section className="market-index-detail"><header><div><span>{selected.exchange} · {selected.security_type.toUpperCase()} · {selected.currency}</span><h3>{selected.name} {selected.code}</h3><small>{quote.meta.is_realtime ? '即時行情' : '官方收盤資料'} · {quote.meta.trade_date} · {quote.meta.stale ? '資料較舊' : '最新完成交易資料'}</small></div><div><strong>{quote.price.toLocaleString('zh-TW')}</strong><em className={quote.change_percent > 0 ? 'up' : quote.change_percent < 0 ? 'down' : 'flat'}>{quote.change_percent > 0 ? '+' : ''}{quote.change_percent.toFixed(2)}%</em></div></header><SourceNotice meta={quote.meta} locale="zh-TW" /><div className="market-kline-table"><header><span>日期</span><span>開盤</span><span>最高</span><span>最低</span><span>收盤</span><span>漲跌</span></header>{lines.slice(-10).reverse().map((line) => <article key={line.time}><span>{new Date(line.time).toLocaleDateString('zh-TW')}</span><span>{line.open}</span><span>{line.high}</span><span>{line.low}</span><strong>{line.close}</strong><em>{(line.change_percent || 0).toFixed(2)}%</em></article>)}</div></section>}
		{(institutional || margin) && <ChipView institutional={institutional} margin={margin} />}
		{fundamentals && <FundamentalsView data={fundamentals} />}
		<CoreIndexView indexes={indexSnapshots} selectedID={selectedIndex} onSelect={setSelectedIndex} series={indexSeries} seriesLoading={false} meta={indexSeries?.meta || null} locale="zh-TW" />
	</div>;
}

export function FundamentalsView({ data }: { data: TaiwanFundamentals }) {
	// monthly_revenue/dividends are backend lists with no null-vs-empty distinction (e.g. ETFs
	// like 0050 report them as null via `unavailable`/`data_insufficient` capabilities below);
	// guard the lookup so an empty/absent list renders "目前無適用官方資料" instead of crashing.
	const latestRevenue = data.monthly_revenue?.at(-1); const statement = data.financial_statement; const valuation = data.valuation; const dividend = data.dividends?.at(0);
	const money = (value?: number) => value == null ? '—' : `${(value / 100000000).toLocaleString('zh-TW', { maximumFractionDigits: 2 })} 億元`;
	const number = (value?: number, suffix = '') => value == null ? '—' : `${value.toLocaleString('zh-TW', { maximumFractionDigits: 2 })}${suffix}`;
	const unavailable = (key: string) => ['unsupported','data_insufficient'].includes(data.capabilities[key]?.status || '');
	return <section className="taiwan-fundamentals-panel"><header><div><span>FUNDAMENTALS</span><h3>基本面</h3></div><small>所有金額已統一為 TWD</small></header><div className="taiwan-fundamentals-grid">
		<article><strong>月營收</strong>{unavailable('monthly_revenue') ? <p>目前無適用官方資料</p> : <><b>{money(latestRevenue?.revenue)}</b><span>{latestRevenue?.period || '—'} · MoM {number(latestRevenue?.computed_mom_percent, '%')} · YoY {number(latestRevenue?.computed_yoy_percent, '%')}</span><small>{latestRevenue?.provider || '—'} · {latestRevenue?.status || '—'}</small></>}</article>
		<article><strong>財報</strong>{unavailable('financial_statement') ? <p>目前無適用官方資料</p> : <><b>累計 EPS {number(statement?.cumulative_eps)}</b><span>營收 {money(statement?.revenue)} · 毛利 {money(statement?.gross_profit)}</span><span>營業利益 {money(statement?.operating_income)} · 稅後淨利 {money(statement?.net_income_attributable_to_parent)}</span><small>{statement?.fiscal_year} Q{statement?.fiscal_quarter} · {statement?.period_end} · 可用日 {statement?.available_at || '官方未提供'}</small></>}</article>
		<article><strong>估值</strong>{unavailable('valuation') ? <p>目前無適用官方資料</p> : <><b>PE {number(valuation?.pe)}</b><span>PB {number(valuation?.pb)} · 殖利率 {number(valuation?.dividend_yield_percent, '%')}</span><small>{valuation?.data_date || '—'} · {valuation?.provider || '—'}</small></>}</article>
		<article><strong>股利</strong>{unavailable('dividends') ? <p>目前無適用官方資料</p> : <><b>現金 {number(dividend?.cash_dividend, ' 元／股')}</b><span>股票 {number(dividend?.stock_dividend, ' 元／股')} · 除權息日 {dividend?.ex_dividend_date || '—'}</span><small>{dividend?.year || '—'} · {dividend?.raw_status || dividend?.normalized_status || '狀態未提供'}</small></>}</article>
	</div></section>;
}

export function ChipView({ institutional, margin }: { institutional: InstitutionalHistory | null; margin: MarginHistory | null }) {
	const latest = institutional?.data.at(-1); const latestMargin = margin?.data.at(-1); const summary = institutional?.summary;
	const lots = (value?: number) => value == null ? '—' : `${(value / 1000).toLocaleString('zh-TW')} 張`;
	const signedLots = (value?: number) => value == null ? '—' : `${value > 0 ? '+' : ''}${(value / 1000).toLocaleString('zh-TW')} 張`;
	return <section className="taiwan-chip-panel"><header><div><span>CHIP DATA</span><h3>籌碼</h3></div><small>{latest?.trade_date || '資料不足'} · 單位顯示為張</small></header>
		{institutional ? <div className="taiwan-chip-grid">{[
			['外資', latest?.foreign_buy, latest?.foreign_sell, latest?.foreign_net, summary?.foreign_net_5d, summary?.foreign_net_20d],
			['投信', latest?.investment_trust_buy, latest?.investment_trust_sell, latest?.investment_trust_net, summary?.investment_trust_net_5d, summary?.investment_trust_net_20d],
			['自營商', latest?.dealer_buy, latest?.dealer_sell, latest?.dealer_net, summary?.dealer_net_5d, summary?.dealer_net_20d],
		].map(([label,buy,sell,net,five,twenty]) => <article key={String(label)}><strong>{label}</strong><span>買入 {lots(buy as number)}</span><span>賣出 {lots(sell as number)}</span><em>買賣超 {signedLots(net as number)}</em><small>5日 {signedLots(five as number)} · 20日 {signedLots(twenty as number)}</small></article>)}</div> : <p>法人買賣超資料目前無法取得</p>}
		{margin ? <div className="taiwan-margin-grid"><article><span>融資餘額</span><strong>{lots(latestMargin?.margin_balance)}</strong></article><article><span>融資增減</span><strong>{signedLots(latestMargin?.margin_change)}</strong></article><article><span>融券餘額</span><strong>{lots(latestMargin?.short_balance)}</strong></article><article><span>融券增減</span><strong>{signedLots(latestMargin?.short_change)}</strong></article><article><span>券資比</span><strong>{latestMargin?.short_margin_ratio == null ? '—' : `${latestMargin.short_margin_ratio.toFixed(2)}%`}</strong></article></div> : <p>融資融券資料目前無法取得</p>}
		<SourceNotice meta={(institutional || margin)!.meta} locale="zh-TW" />
	</section>;
}
