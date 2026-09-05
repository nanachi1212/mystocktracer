import { requestJSON, type BackendConfig } from './backend';

export type TaiwanScope = 'twse' | 'tpex' | 'combined';

export const taiwanPrimaryNavigation = [
	['taiwan-overview', '台股總覽'], ['taiwan-breadth', '市場廣度'], ['taiwan-emotion', '市場情緒'],
	['taiwan-industry', '產業雷達'], ['taiwan-screener', '台股選股器'], ['taiwan-stock', '個股研究'],
	['taiwan-research', 'AI 研究'], ['taiwan-watchlist', '自選股'],
] as const;

export const taiwanDefaultWorkspace = 'taiwan-overview';

export function resolveTaiwanWorkspace(hash: string) {
	const value = hash.replace(/^#/, '');
	return taiwanPrimaryNavigation.some(([id]) => id === value) ? value : taiwanDefaultWorkspace;
}

export const taiwanMarketPath = (kind: 'market-breadth' | 'market-emotion' | 'industry-radar', scope: TaiwanScope) => `/api/v1/tw/${kind}?scope=${scope}`;
export const taiwanIntelligencePath = (symbol: string) => `/api/v1/tw/stocks/${encodeURIComponent(symbol)}/intelligence`;
export const taiwanResearchPath = (symbol: string) => `/api/v1/tw/stocks/${encodeURIComponent(symbol)}/research`;
export const taiwanWatchlistPath = () => '/api/v1/tw/watchlist';
export const taiwanWatchlistRemovePath = (canonical: string) => `/api/v1/tw/watchlist/${encodeURIComponent(canonical)}`;

export const taiwanScopes: { id: TaiwanScope; label: string }[] = [
	{ id: 'combined', label: '台灣市場' },
	{ id: 'twse', label: '上市（TWSE）' },
	{ id: 'tpex', label: '上櫃（TPEx）' },
];

// M7B — Taiwan Screener. Wire shape of one row from the existing M7A backend contract
// (GET /api/v1/tw/screener). Every numeric field is nullable and must stay that way end-to-end —
// a missing quote/volume/amount/change_percent is never normalized to 0.
export type TaiwanScreenerSecurity = {
	canonical: string; code: string; name: string; exchange: string; security_type: string; trade_date: string;
	price: number | null; change: number | null; change_percent: number | null; volume: number | null; amount: number | null;
};

export type TaiwanScreenerResponse = {
	scope: string; as_of: string | null; freshness: string; total: number; offset: number; limit: number;
	securities: TaiwanScreenerSecurity[];
};

export type TaiwanScreenerSort = 'price' | 'change_percent' | 'volume' | 'amount';
export type TaiwanScreenerOrder = 'asc' | 'desc';

export const taiwanScreenerSortOptions: { id: TaiwanScreenerSort; label: string }[] = [
	{ id: 'price', label: '股價' },
	{ id: 'change_percent', label: '漲跌幅' },
	{ id: 'volume', label: '成交量' },
	{ id: 'amount', label: '成交金額' },
];

export const taiwanScreenerOrderOptions: { id: TaiwanScreenerOrder; label: string }[] = [
	{ id: 'desc', label: '高到低' },
	{ id: 'asc', label: '低到高' },
];

export const TAIWAN_SCREENER_PAGE_SIZE = 50;

// Range fields stay as raw strings (not numbers) so a blank input, a bare "-", or a mid-edit value
// can all be represented without forcing a premature 0 — the query builder below is the only place
// that turns a filled-in field into a number.
export type TaiwanScreenerFilters = {
	scope: TaiwanScope;
	minPrice: string; maxPrice: string;
	minChangePercent: string; maxChangePercent: string;
	minVolume: string; maxVolume: string;
	minAmount: string; maxAmount: string;
	sort: TaiwanScreenerSort;
	order: TaiwanScreenerOrder;
	limit: number;
	offset: number;
};

// A fresh object every call — callers hold this in React state, so a shared mutable literal here
// would let one workspace instance's edits leak into another's "defaults".
export function taiwanScreenerDefaultFilters(): TaiwanScreenerFilters {
	return {
		scope: 'combined',
		minPrice: '', maxPrice: '',
		minChangePercent: '', maxChangePercent: '',
		minVolume: '', maxVolume: '',
		minAmount: '', maxAmount: '',
		sort: 'amount', order: 'desc',
		limit: TAIWAN_SCREENER_PAGE_SIZE, offset: 0,
	};
}

function taiwanScreenerRangeValue(raw: string): number | null {
	if (raw.trim() === '') return null;
	const value = Number(raw);
	return Number.isFinite(value) ? value : null;
}

// Builds the exact M7A query contract. Blank optional fields are omitted; an explicitly entered 0
// is preserved (Number("0") is finite, so it is not treated as blank); a non-numeric or
// non-finite (NaN/Infinity) entry is silently omitted rather than sent upstream. scope/sort/order
// always come from validated UI choices, so they are always included.
export function taiwanScreenerPath(filters: TaiwanScreenerFilters): string {
	const params = new URLSearchParams();
	params.set('scope', filters.scope);
	params.set('sort', filters.sort);
	params.set('order', filters.order);
	params.set('limit', String(filters.limit));
	params.set('offset', String(Math.max(0, filters.offset)));
	const setRange = (key: string, raw: string) => {
		const value = taiwanScreenerRangeValue(raw);
		if (value != null) params.set(key, String(value));
	};
	setRange('min_price', filters.minPrice);
	setRange('max_price', filters.maxPrice);
	setRange('min_change_percent', filters.minChangePercent);
	setRange('max_change_percent', filters.maxChangePercent);
	setRange('min_volume', filters.minVolume);
	setRange('max_volume', filters.maxVolume);
	setRange('min_amount', filters.minAmount);
	setRange('max_amount', filters.maxAmount);
	return `/api/v1/tw/screener?${params.toString()}`;
}

// Local, best-effort validation only — the backend remains authoritative. Checks each min/max pair
// independently and returns the first violation found (blank/non-numeric fields never trigger it).
export function validateTaiwanScreenerFilters(filters: TaiwanScreenerFilters): string {
	const pairs: [string, string, string][] = [
		['minPrice', 'maxPrice', '最低價格不可高於最高價格'],
		['minChangePercent', 'maxChangePercent', '最低漲跌幅不可高於最高漲跌幅'],
		['minVolume', 'maxVolume', '最低成交量不可高於最高成交量'],
		['minAmount', 'maxAmount', '最低成交金額不可高於最高成交金額'],
	];
	for (const [minKey, maxKey, message] of pairs) {
		const min = taiwanScreenerRangeValue(filters[minKey as keyof TaiwanScreenerFilters] as string);
		const max = taiwanScreenerRangeValue(filters[maxKey as keyof TaiwanScreenerFilters] as string);
		if (min != null && max != null && min > max) return message;
	}
	return '';
}

// Wire shape of one saved Taiwan security (M6A backend). Quote/price data is
// intentionally absent here — the watchlist persists identity only; latest
// prices are composed separately from the existing /tw/quotes endpoint (M6B).
export type TaiwanWatchlistSecurity = { canonical: string; code: string; name: string; exchange: string; security_type: string; created_at: string };

export async function fetchTaiwanWatchlist(config: BackendConfig): Promise<TaiwanWatchlistSecurity[]> {
	const payload = await requestJSON<{ data: { securities: TaiwanWatchlistSecurity[] } }>(config, taiwanWatchlistPath());
	return payload.data.securities;
}

export async function isTaiwanSecurityWatchlisted(config: BackendConfig, canonical: string): Promise<boolean> {
	const securities = await fetchTaiwanWatchlist(config);
	return securities.some((item) => item.canonical === canonical);
}

export async function addTaiwanWatchlistSecurity(config: BackendConfig, canonical: string): Promise<TaiwanWatchlistSecurity> {
	const payload = await requestJSON<{ data: { security: TaiwanWatchlistSecurity } }>(config, taiwanWatchlistPath(), {
		method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ symbol: canonical }),
	});
	return payload.data.security;
}

export async function removeTaiwanWatchlistSecurity(config: BackendConfig, canonical: string): Promise<void> {
	await requestJSON(config, taiwanWatchlistRemovePath(canonical), { method: 'DELETE' });
}

// The backend /tw/quotes endpoint supports at most 10 symbols per request (all-or-nothing per
// call, not per-symbol) — chunk a larger Watchlist into deterministic groups so each group can be
// fetched (and can fail) independently via Promise.allSettled, without ever exceeding that limit.
export const TAIWAN_QUOTES_BATCH_LIMIT = 10;

export function chunkTaiwanSymbols(symbols: string[], size: number = TAIWAN_QUOTES_BATCH_LIMIT): string[][] {
	const groups: string[][] = [];
	for (let i = 0; i < symbols.length; i += size) groups.push(symbols.slice(i, i + size));
	return groups;
}

const labels: Record<string, string> = {
	current: '最新', stale: '資料較舊', partial: '部分資料', unavailable: '無法取得',
	available: '可使用', data_insufficient: '資料不足', not_applicable: '不適用', indeterminate: '無法判定',
	'official close': '官方收盤', official: '官方資料', realtime: '即時', delayed: '延遲',
	positive: '偏正向', negative: '偏弱', weak: '偏弱', mixed: '訊號分歧', balanced: '多空相當',
	high: '高', medium: '中', low: '低', supportive: '相對有支撐', reported: '已揭露', flat: '持平',
	aligned_positive: '家數與成交方向一致偏正向', aligned_negative: '家數與成交方向一致偏弱',
	breadth_positive_capital_negative: '上漲家數偏多、上漲成交占比偏低',
	breadth_negative_capital_positive: '上漲家數偏少、上漲成交占比偏高',
};

export function taiwanStatusLabel(value?: string) {
	return value ? labels[value] || value.replaceAll('_', ' ') : '未提供';
}

export function taiwanSecurityTypeLabel(value?: string) {
	return value === 'stock' ? '股票' : value === 'etf' ? 'ETF' : value === 'index' ? '指數' : value?.toUpperCase() || '證券';
}

export function formatTaiwanPercent(value?: number | null, signed = false) {
	if (value == null) return '—';
	return `${signed && value > 0 ? '+' : ''}${value.toLocaleString('zh-TW', { maximumFractionDigits: 2 })}%`;
}

export function formatTaiwanRatio(value?: number | null) {
	return value == null ? '—' : formatTaiwanPercent(value * 100);
}

export function formatTaiwanTWD(value?: number | null) {
	return value == null ? '—' : `${(value / 100_000_000).toLocaleString('zh-TW', { maximumFractionDigits: 2 })} 億元`;
}

// Runs an async task as the latest "generation" of a scoped request (e.g. a market scope tab or
// a selected stock symbol changing). `ref` is a mutable counter (a useRef(0) works as-is) shared
// across calls: each call claims the next id, and a call's result is only applied if no newer
// call has started by the time it settles — so a slow response for a scope/selection the user
// has since navigated away from can never overwrite what is currently on screen. `onStart` runs
// synchronously before the task so callers can clear stale data before the new fetch begins.
export async function runScopedRequest<T>(
	ref: { current: number },
	task: () => Promise<T>,
	handlers: { onStart?: () => void; onSuccess: (value: T) => void; onError?: (reason: unknown) => void; onSettle?: () => void },
) {
	const requestID = ++ref.current;
	handlers.onStart?.();
	try {
		const value = await task();
		if (requestID === ref.current) handlers.onSuccess(value);
	} catch (reason) {
		if (requestID === ref.current) handlers.onError?.(reason);
	} finally {
		if (requestID === ref.current) handlers.onSettle?.();
	}
}

export function taiwanErrorMessage(reason: unknown, fallback: string) {
	const message = reason instanceof Error ? reason.message : '';
	if (!message) return fallback;
	if (/unavailable|provider/i.test(message)) return `${fallback}：資料來源目前無法取得`;
	if (/must resolve|canonical|symbol|required/i.test(message)) return `${fallback}：請從搜尋結果選擇一筆台灣證券`;
	return fallback;
}

// Backend TaiwanInterpretationDataQuality lists (available/indeterminate/unavailable/stale/partial
// component names) are documented as plain lists with no null-vs-empty distinction, but Go
// serializes a zero-length unset []string as JSON null rather than []. Normalize that null to []
// at the point of use so a legitimately empty category never crashes `.length` reads.
export function taiwanComponentList(value: string[] | null | undefined) {
	return value ?? [];
}

export function taiwanReasonLabel(value: string) {
	return value
		.replace('completed 5-session and 20-session returns are both required', '需要完整的 5 日與 20 日收盤報酬資料')
		.replace('M2B market state is unavailable', '市場情緒資料目前無法取得')
		.replace('M3 industry evidence is unavailable', '產業資料目前無法取得')
		.replace('industry relative breadth and relative capital are both required', '需要完整的產業相對廣度與相對成交方向資料')
		.replace('institutional section is unavailable', '法人資料目前無法取得')
		.replace('latest official institutional flow is unavailable', '缺少最新官方法人買賣超資料')
		.replace('margin section is unavailable', '融資融券資料目前無法取得')
		.replace('fundamentals section is unavailable', '基本面資料目前無法取得')
		.replace('official monthly revenue YoY is unavailable', '缺少官方月營收年增率資料')
		.replace('industry interpretation is not applicable to non-stock security types', '非普通股不適用產業解讀')
		.replace('ordinary-stock fundamentals are not applicable', '非普通股不適用普通股基本面解讀')
		.replaceAll(' is ', '為 ').replaceAll(' with ', '，資料狀態 ').replaceAll(' versus ', '，比較基準 ')
		.replaceAll('return_5d_percent', '5 日報酬').replaceAll('return_20d_percent', '20 日報酬')
		.replaceAll('M2B market state', '市場狀態').replaceAll('M2B confidence', '資料信心')
		.replaceAll('relative_breadth', '相對市場廣度').replaceAll('relative_capital', '相對成交方向');
}
