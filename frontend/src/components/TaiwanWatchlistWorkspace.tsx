import { LoaderCircle, Trash2 } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import type { BackendConfig, Quote } from '../lib/backend';
import { requestJSON } from '../lib/backend';
import { chunkTaiwanSymbols, fetchTaiwanWatchlist, removeTaiwanWatchlistSecurity, runScopedRequest, taiwanErrorMessage, taiwanSecurityTypeLabel, type TaiwanWatchlistSecurity } from '../lib/taiwan-product';

type QuoteLookup = Record<string, Quote>;

// Fetches quotes for every saved security, chunked to the existing /tw/quotes batch limit.
// Each chunk is requested independently (Promise.allSettled): one chunk failing must not
// discard quotes a different, successful chunk already returned. A missing quote (chunk failed,
// or a symbol simply absent from a successful chunk's response) is represented by the symbol
// being absent from the returned map — never a fabricated zero/blank entry.
async function fetchWatchlistQuotes(config: BackendConfig, canonicals: string[]): Promise<QuoteLookup> {
	if (canonicals.length === 0) return {};
	const groups = chunkTaiwanSymbols(canonicals);
	const results = await Promise.allSettled(groups.map((group) => requestJSON<{ data: Quote[] }>(config, `/api/v1/tw/quotes?symbols=${encodeURIComponent(group.join(','))}`)));
	const quotes: QuoteLookup = {};
	for (const result of results) {
		if (result.status === 'fulfilled') {
			for (const quote of result.value.data) quotes[quote.symbol] = quote;
		}
	}
	return quotes;
}

export function TaiwanWatchlistWorkspace({ config, refreshKey }: { config: BackendConfig | null; refreshKey: number }) {
	const [securities, setSecurities] = useState<TaiwanWatchlistSecurity[]>([]);
	const [quotes, setQuotes] = useState<QuoteLookup>({});
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState('');
	const [removingSymbol, setRemovingSymbol] = useState<string | null>(null);
	const loadRequestID = useRef(0);

	// Fetches only while this view is mounted (App unmounts it when the user switches away), and
	// only on mount or an explicit global refresh — never on a timer, never merely because the
	// application opened elsewhere.
	useEffect(() => {
		if (!config) return;
		void runScopedRequest(loadRequestID, async () => {
			const list = await fetchTaiwanWatchlist(config);
			const quoteMap = await fetchWatchlistQuotes(config, list.map((item) => item.canonical));
			return { list, quoteMap };
		}, {
			onStart: () => { setLoading(true); setError(''); },
			onSuccess: ({ list, quoteMap }) => { setSecurities(list); setQuotes(quoteMap); },
			onError: (reason) => { setSecurities([]); setQuotes({}); setError(taiwanErrorMessage(reason, '自選股清單載入失敗')); },
			onSettle: () => setLoading(false),
		});
	}, [config, refreshKey]);

	const remove = async (canonical: string) => {
		if (!config || removingSymbol) return;
		setRemovingSymbol(canonical);
		try {
			await removeTaiwanWatchlistSecurity(config, canonical);
			setSecurities((current) => current.filter((item) => item.canonical !== canonical));
			setQuotes((current) => { const next = { ...current }; delete next[canonical]; return next; });
		} catch (reason) {
			setError(taiwanErrorMessage(reason, '移除自選股失敗'));
		} finally {
			setRemovingSymbol(null);
		}
	};

	return <div className="taiwan-product-workspace taiwan-watchlist-workspace">
		{loading && <div className="taiwan-loading"><LoaderCircle className="spin" size={18} />正在讀取自選股清單</div>}
		{error && <div className="market-partial-warning">{error}</div>}
		{!loading && securities.length === 0 && !error && <div className="taiwan-empty-state"><strong>目前還沒有自選股</strong><p>可從台股總覽或個股研究加入。</p></div>}
		{securities.length > 0 && <div className="taiwan-watchlist-list">{securities.map((item) => (
			<WatchlistRow key={item.canonical} security={item} quote={quotes[item.canonical]} busy={removingSymbol === item.canonical} onRemove={() => void remove(item.canonical)} />
		))}</div>}
	</div>;
}

// Pure/presentational: one saved security's row. A missing `quote` (chunk failed, or the symbol
// was simply absent from a successful chunk) always renders the explicit unavailable state below
// — never a fabricated 0 price/percentage, and never omitting the security itself.
export function WatchlistRow({ security, quote, busy, onRemove }: { security: TaiwanWatchlistSecurity; quote?: Quote; busy: boolean; onRemove: () => void }) {
	return <article className="taiwan-watchlist-row">
		<div className="taiwan-watchlist-identity"><strong>{security.name} {security.code}</strong><span>{security.exchange} · {taiwanSecurityTypeLabel(security.security_type)}</span></div>
		{quote
			? <div className="taiwan-watchlist-quote"><strong>{quote.price.toLocaleString('zh-TW')}</strong><em className={quote.change_percent > 0 ? 'up' : quote.change_percent < 0 ? 'down' : 'flat'}>{quote.change_percent > 0 ? '+' : ''}{quote.change_percent.toFixed(2)}%</em></div>
			: <div className="taiwan-watchlist-quote"><span>報價暫時無法取得</span></div>}
		<button type="button" className="taiwan-watchlist-remove" onClick={onRemove} disabled={busy}>{busy ? <LoaderCircle className="spin" size={14} /> : <Trash2 size={14} />}移除自選</button>
	</article>;
}
