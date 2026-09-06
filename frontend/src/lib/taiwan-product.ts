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
// M7D adds institutional/margin fields — same nullable contract, reused directly from the backend's
// existing units (shares; short_margin_ratio already expressed as a percentage number by the
// backend, never re-scaled here).
export type TaiwanScreenerSecurity = {
	canonical: string; code: string; name: string; exchange: string; security_type: string; trade_date: string;
	price: number | null; change: number | null; change_percent: number | null; volume: number | null; amount: number | null;
	foreign_net: number | null; trust_net: number | null; dealer_net: number | null; institutional_net: number | null;
	margin_balance: number | null; margin_change: number | null; short_balance: number | null; short_change: number | null; short_margin_ratio: number | null;
	// M7E-A — revenue/valuation/dividends (live-current, official-bulk-sourced; nil when that domain
	// was not requested, or when the security has no row in that domain's backend payload). Units are
	// reused exactly as the backend already expresses them — never rescaled here: monthly_revenue is
	// TWD (the backend already converted it from the source's thousand-TWD raw value), revenue_yoy/
	// dividend_yield are the backend's own percentage-scale numbers.
	monthly_revenue: number | null; revenue_yoy: number | null;
	pe: number | null; pb: number | null; dividend_yield: number | null;
	cash_dividend: number | null; stock_dividend: number | null; total_dividend: number | null;
	// M7E-B — financial statement (income-statement only; live-current, official-bulk-sourced).
	// financial_period is THIS security's own actual reporting period (e.g. "2026-Q2") — it is set
	// whenever a valid statement row was parsed for it, independent of whether the metric fields below
	// are populated. cumulative_eps/gross_margin/operating_margin are populated ONLY when
	// financial_period matches the domain's common target period (see TaiwanScreenerResponse.
	// financials_period) — a security still on an older quarter keeps its own true financial_period but
	// has these three fields nulled by the backend, and the frontend must never substitute the domain
	// period into an older row's display. gross_margin/operating_margin are additionally backend-scoped
	// to the "ci" (general industry) category only; every other category is nil, never fabricated.
	financial_period: string | null;
	cumulative_eps: number | null; gross_margin: number | null; operating_margin: number | null;
	// M7E-C — net_margin (income-statement, backend-scoped to the "ci" category only — every other
	// category, e.g. a financial holding, is nil, never fabricated) and book_value_per_share
	// (balance-sheet, all categories where a real row exists). Both are gated by the exact same
	// financial_period/financials_period rule as cumulative_eps/gross_margin/operating_margin above —
	// the frontend never re-derives or second-guesses that gate, it only renders what the backend sent.
	net_margin: number | null; book_value_per_share: number | null;
};

export type TaiwanScreenerResponse = {
	scope: string; as_of: string | null; freshness: string; total: number; offset: number; limit: number;
	securities: TaiwanScreenerSecurity[];
	// M7D — additive, present only when the backend actually engaged that domain for this request
	// (an active filter or sort key). Absent (undefined) means the domain was not requested at all —
	// never confuse that with "unavailable". Sourced entirely from the backend's existing
	// TaiwanFreshness model; never a published_at/available_at timestamp.
	institutional_as_of?: string | null; institutional_status?: string; institutional_days_behind?: number | null;
	margin_as_of?: string | null; margin_status?: string; margin_days_behind?: number | null;
	// M7E-A — same additive-only-when-requested contract. revenue_as_of/dividends_as_of are the
	// backend's own period/year identifiers (e.g. "2026-08", "2025") — never a calendar date the
	// frontend invents. revenue_days_behind/dividends_days_behind are normally absent (the backend
	// never fabricates a trading-day cadence for monthly/annual data); valuation_days_behind may be
	// present since valuation shares the daily trading cadence.
	revenue_as_of?: string | null; revenue_status?: string; revenue_days_behind?: number | null;
	valuation_as_of?: string | null; valuation_status?: string; valuation_days_behind?: number | null;
	dividends_as_of?: string | null; dividends_status?: string; dividends_days_behind?: number | null;
	// M7E-B — additive, present only when the financials domain was actually requested.
	// financials_period is the market-wide common TARGET period ("2026-Q2") used to gate which rows'
	// metric fields are populated — a domain-level summary, never the exact period for every individual
	// row (use each row's own financial_period for that). financials_status supports available/
	// partial/unavailable — never a daily-cadence "最新"/days-behind claim, and never published_at/
	// available_at.
	financials_period?: string | null; financials_status?: string;
};

export type TaiwanScreenerSort =
	| 'price' | 'change_percent' | 'volume' | 'amount'
	| 'foreign_net' | 'trust_net' | 'dealer_net' | 'institutional_net'
	| 'margin_balance' | 'margin_change' | 'short_balance' | 'short_change' | 'short_margin_ratio'
	| 'monthly_revenue' | 'revenue_yoy' | 'pe' | 'pb' | 'dividend_yield' | 'cash_dividend' | 'stock_dividend' | 'total_dividend'
	| 'cumulative_eps' | 'gross_margin' | 'operating_margin' | 'net_margin' | 'book_value_per_share';
export type TaiwanScreenerOrder = 'asc' | 'desc';

export const taiwanScreenerSortOptions: { id: TaiwanScreenerSort; label: string }[] = [
	{ id: 'price', label: '股價' },
	{ id: 'change_percent', label: '漲跌幅' },
	{ id: 'volume', label: '成交量' },
	{ id: 'amount', label: '成交金額' },
	{ id: 'foreign_net', label: '外資買賣超' },
	{ id: 'trust_net', label: '投信買賣超' },
	{ id: 'dealer_net', label: '自營商買賣超' },
	{ id: 'institutional_net', label: '三大法人合計' },
	{ id: 'margin_balance', label: '融資餘額' },
	{ id: 'margin_change', label: '融資增減' },
	{ id: 'short_balance', label: '融券餘額' },
	{ id: 'short_change', label: '融券增減' },
	{ id: 'short_margin_ratio', label: '券資比' },
	{ id: 'monthly_revenue', label: '月營收' },
	{ id: 'revenue_yoy', label: '月營收年增率' },
	{ id: 'pe', label: '本益比（PE）' },
	{ id: 'pb', label: '股價淨值比（PB）' },
	{ id: 'dividend_yield', label: '殖利率' },
	{ id: 'cash_dividend', label: '現金股利' },
	{ id: 'stock_dividend', label: '股票股利' },
	{ id: 'total_dividend', label: '合計股利' },
	{ id: 'cumulative_eps', label: '累計 EPS' },
	{ id: 'gross_margin', label: '毛利率' },
	{ id: 'operating_margin', label: '營業利益率' },
	{ id: 'net_margin', label: '淨利率' },
	{ id: 'book_value_per_share', label: '每股參考淨值' },
];

const taiwanScreenerInstitutionalSortKeys = new Set<TaiwanScreenerSort>(['foreign_net', 'trust_net', 'dealer_net', 'institutional_net']);
const taiwanScreenerMarginSortKeys = new Set<TaiwanScreenerSort>(['margin_balance', 'margin_change', 'short_balance', 'short_change', 'short_margin_ratio']);
const taiwanScreenerRevenueSortKeys = new Set<TaiwanScreenerSort>(['monthly_revenue', 'revenue_yoy']);
const taiwanScreenerValuationSortKeys = new Set<TaiwanScreenerSort>(['pe', 'pb', 'dividend_yield']);
const taiwanScreenerDividendSortKeys = new Set<TaiwanScreenerSort>(['cash_dividend', 'stock_dividend', 'total_dividend']);
// M7E-C's net_margin/book_value_per_share join the same financials-criteria detection as the M7E-B
// metrics: the backend's financials_status now truthfully covers both the income-statement and (when
// engaged) balance-sheet subdomains as one combined status (see combineFinancialsStatus on the
// backend) — the frontend does not need a second "showBalance" concept, it reuses the single existing
// showFinancials/ScreenerFinancialsFreshness contract for all five metrics.
const taiwanScreenerFinancialsSortKeys = new Set<TaiwanScreenerSort>(['cumulative_eps', 'gross_margin', 'operating_margin', 'net_margin', 'book_value_per_share']);

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
	// M7D institutional.
	minForeignNet: string; maxForeignNet: string;
	minTrustNet: string; maxTrustNet: string;
	minDealerNet: string; maxDealerNet: string;
	minInstitutionalNet: string; maxInstitutionalNet: string;
	// M7D margin/short.
	minMarginBalance: string; maxMarginBalance: string;
	minMarginChange: string; maxMarginChange: string;
	minShortBalance: string; maxShortBalance: string;
	minShortChange: string; maxShortChange: string;
	minShortMarginRatio: string; maxShortMarginRatio: string;
	// M7E-A fundamentals: revenue, valuation, dividends.
	minMonthlyRevenue: string; maxMonthlyRevenue: string;
	minRevenueYoY: string; maxRevenueYoY: string;
	minPE: string; maxPE: string;
	minPB: string; maxPB: string;
	minDividendYield: string; maxDividendYield: string;
	minCashDividend: string; maxCashDividend: string;
	minStockDividend: string; maxStockDividend: string;
	minTotalDividend: string; maxTotalDividend: string;
	// M7E-B financial statement: cumulative EPS + ci-only margins.
	minCumulativeEPS: string; maxCumulativeEPS: string;
	minGrossMargin: string; maxGrossMargin: string;
	minOperatingMargin: string; maxOperatingMargin: string;
	// M7E-C financial statement: ci-only net margin + all-category book value per share.
	minNetMargin: string; maxNetMargin: string;
	minBookValuePerShare: string; maxBookValuePerShare: string;
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
		minForeignNet: '', maxForeignNet: '',
		minTrustNet: '', maxTrustNet: '',
		minDealerNet: '', maxDealerNet: '',
		minInstitutionalNet: '', maxInstitutionalNet: '',
		minMarginBalance: '', maxMarginBalance: '',
		minMarginChange: '', maxMarginChange: '',
		minShortBalance: '', maxShortBalance: '',
		minShortChange: '', maxShortChange: '',
		minShortMarginRatio: '', maxShortMarginRatio: '',
		minMonthlyRevenue: '', maxMonthlyRevenue: '',
		minRevenueYoY: '', maxRevenueYoY: '',
		minPE: '', maxPE: '',
		minPB: '', maxPB: '',
		minDividendYield: '', maxDividendYield: '',
		minCashDividend: '', maxCashDividend: '',
		minStockDividend: '', maxStockDividend: '',
		minTotalDividend: '', maxTotalDividend: '',
		minCumulativeEPS: '', maxCumulativeEPS: '',
		minGrossMargin: '', maxGrossMargin: '',
		minOperatingMargin: '', maxOperatingMargin: '',
		minNetMargin: '', maxNetMargin: '',
		minBookValuePerShare: '', maxBookValuePerShare: '',
		sort: 'amount', order: 'desc',
		limit: TAIWAN_SCREENER_PAGE_SIZE, offset: 0,
	};
}

// M7D — true when `filters` (pass the APPLIED filters, never draft) actually engages the
// institutional domain: an active min/max filter, or a sort key belonging to that domain. Used to
// decide result-presentation only — it never triggers a request itself.
export function taiwanScreenerHasInstitutionalCriteria(filters: TaiwanScreenerFilters): boolean {
	if (taiwanScreenerInstitutionalSortKeys.has(filters.sort)) return true;
	return [filters.minForeignNet, filters.maxForeignNet, filters.minTrustNet, filters.maxTrustNet, filters.minDealerNet, filters.maxDealerNet, filters.minInstitutionalNet, filters.maxInstitutionalNet]
		.some((raw) => taiwanScreenerRangeValue(raw) != null);
}

export function taiwanScreenerHasMarginCriteria(filters: TaiwanScreenerFilters): boolean {
	if (taiwanScreenerMarginSortKeys.has(filters.sort)) return true;
	return [filters.minMarginBalance, filters.maxMarginBalance, filters.minMarginChange, filters.maxMarginChange, filters.minShortBalance, filters.maxShortBalance, filters.minShortChange, filters.maxShortChange, filters.minShortMarginRatio, filters.maxShortMarginRatio]
		.some((raw) => taiwanScreenerRangeValue(raw) != null);
}

// M7E-A — same pattern as institutional/margin above: true if an active min/max filter for that
// domain is set OR the sort key belongs to that domain. Must be called with `applied`, never `draft`.
export function taiwanScreenerHasRevenueCriteria(filters: TaiwanScreenerFilters): boolean {
	if (taiwanScreenerRevenueSortKeys.has(filters.sort)) return true;
	return [filters.minMonthlyRevenue, filters.maxMonthlyRevenue, filters.minRevenueYoY, filters.maxRevenueYoY]
		.some((raw) => taiwanScreenerRangeValue(raw) != null);
}

export function taiwanScreenerHasValuationCriteria(filters: TaiwanScreenerFilters): boolean {
	if (taiwanScreenerValuationSortKeys.has(filters.sort)) return true;
	return [filters.minPE, filters.maxPE, filters.minPB, filters.maxPB, filters.minDividendYield, filters.maxDividendYield]
		.some((raw) => taiwanScreenerRangeValue(raw) != null);
}

export function taiwanScreenerHasDividendCriteria(filters: TaiwanScreenerFilters): boolean {
	if (taiwanScreenerDividendSortKeys.has(filters.sort)) return true;
	return [filters.minCashDividend, filters.maxCashDividend, filters.minStockDividend, filters.maxStockDividend, filters.minTotalDividend, filters.maxTotalDividend]
		.some((raw) => taiwanScreenerRangeValue(raw) != null);
}

// M7E-B — same pattern as the other domain-criteria helpers above: true if an active min/max
// financial-statement filter is set OR the sort key belongs to the financials domain. Must be called
// with `applied`, never `draft`.
export function taiwanScreenerHasFinancialsCriteria(filters: TaiwanScreenerFilters): boolean {
	if (taiwanScreenerFinancialsSortKeys.has(filters.sort)) return true;
	return [
		filters.minCumulativeEPS, filters.maxCumulativeEPS, filters.minGrossMargin, filters.maxGrossMargin, filters.minOperatingMargin, filters.maxOperatingMargin,
		filters.minNetMargin, filters.maxNetMargin, filters.minBookValuePerShare, filters.maxBookValuePerShare,
	].some((raw) => taiwanScreenerRangeValue(raw) != null);
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
	setRange('min_foreign_net', filters.minForeignNet);
	setRange('max_foreign_net', filters.maxForeignNet);
	setRange('min_trust_net', filters.minTrustNet);
	setRange('max_trust_net', filters.maxTrustNet);
	setRange('min_dealer_net', filters.minDealerNet);
	setRange('max_dealer_net', filters.maxDealerNet);
	setRange('min_institutional_net', filters.minInstitutionalNet);
	setRange('max_institutional_net', filters.maxInstitutionalNet);
	setRange('min_margin_balance', filters.minMarginBalance);
	setRange('max_margin_balance', filters.maxMarginBalance);
	setRange('min_margin_change', filters.minMarginChange);
	setRange('max_margin_change', filters.maxMarginChange);
	setRange('min_short_balance', filters.minShortBalance);
	setRange('max_short_balance', filters.maxShortBalance);
	setRange('min_short_change', filters.minShortChange);
	setRange('max_short_change', filters.maxShortChange);
	setRange('min_short_margin_ratio', filters.minShortMarginRatio);
	setRange('max_short_margin_ratio', filters.maxShortMarginRatio);
	setRange('min_monthly_revenue', filters.minMonthlyRevenue);
	setRange('max_monthly_revenue', filters.maxMonthlyRevenue);
	setRange('min_revenue_yoy', filters.minRevenueYoY);
	setRange('max_revenue_yoy', filters.maxRevenueYoY);
	setRange('min_pe', filters.minPE);
	setRange('max_pe', filters.maxPE);
	setRange('min_pb', filters.minPB);
	setRange('max_pb', filters.maxPB);
	setRange('min_dividend_yield', filters.minDividendYield);
	setRange('max_dividend_yield', filters.maxDividendYield);
	setRange('min_cash_dividend', filters.minCashDividend);
	setRange('max_cash_dividend', filters.maxCashDividend);
	setRange('min_stock_dividend', filters.minStockDividend);
	setRange('max_stock_dividend', filters.maxStockDividend);
	setRange('min_total_dividend', filters.minTotalDividend);
	setRange('max_total_dividend', filters.maxTotalDividend);
	setRange('min_cumulative_eps', filters.minCumulativeEPS);
	setRange('max_cumulative_eps', filters.maxCumulativeEPS);
	setRange('min_gross_margin', filters.minGrossMargin);
	setRange('max_gross_margin', filters.maxGrossMargin);
	setRange('min_operating_margin', filters.minOperatingMargin);
	setRange('max_operating_margin', filters.maxOperatingMargin);
	setRange('min_net_margin', filters.minNetMargin);
	setRange('max_net_margin', filters.maxNetMargin);
	setRange('min_book_value_per_share', filters.minBookValuePerShare);
	setRange('max_book_value_per_share', filters.maxBookValuePerShare);
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
		['minForeignNet', 'maxForeignNet', '最低外資買賣超不可高於最高外資買賣超'],
		['minTrustNet', 'maxTrustNet', '最低投信買賣超不可高於最高投信買賣超'],
		['minDealerNet', 'maxDealerNet', '最低自營商買賣超不可高於最高自營商買賣超'],
		['minInstitutionalNet', 'maxInstitutionalNet', '最低三大法人合計不可高於最高三大法人合計'],
		['minMarginBalance', 'maxMarginBalance', '最低融資餘額不可高於最高融資餘額'],
		['minMarginChange', 'maxMarginChange', '最低融資增減不可高於最高融資增減'],
		['minShortBalance', 'maxShortBalance', '最低融券餘額不可高於最高融券餘額'],
		['minShortChange', 'maxShortChange', '最低融券增減不可高於最高融券增減'],
		['minShortMarginRatio', 'maxShortMarginRatio', '最低券資比不可高於最高券資比'],
		['minMonthlyRevenue', 'maxMonthlyRevenue', '最低月營收不可高於最高月營收'],
		['minRevenueYoY', 'maxRevenueYoY', '最低月營收年增率不可高於最高月營收年增率'],
		['minPE', 'maxPE', '最低本益比不可高於最高本益比'],
		['minPB', 'maxPB', '最低股價淨值比不可高於最高股價淨值比'],
		['minDividendYield', 'maxDividendYield', '最低殖利率不可高於最高殖利率'],
		['minCashDividend', 'maxCashDividend', '最低現金股利不可高於最高現金股利'],
		['minStockDividend', 'maxStockDividend', '最低股票股利不可高於最高股票股利'],
		['minTotalDividend', 'maxTotalDividend', '最低合計股利不可高於最高合計股利'],
		['minCumulativeEPS', 'maxCumulativeEPS', '最低累計 EPS 不可高於最高累計 EPS'],
		['minGrossMargin', 'maxGrossMargin', '最低毛利率不可高於最高毛利率'],
		['minOperatingMargin', 'maxOperatingMargin', '最低營業利益率不可高於最高營業利益率'],
		['minNetMargin', 'maxNetMargin', '最低淨利率不可高於最高淨利率'],
		['minBookValuePerShare', 'maxBookValuePerShare', '最低每股參考淨值不可高於最高每股參考淨值'],
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

// M7E-B — dedicated status label for the financial-statement domain. Deliberately does NOT reuse
// taiwanStatusLabel() above: quarterly filings have no daily-cadence "最新" concept, and the shared
// map's 'partial' entry ('部分資料') is worded for the daily/trading-cadence domains — financials
// needs its own truthful 3-state vocabulary (available/partial/unavailable) that never implies
// "latest" or a trading-day freshness claim.
const financialsStatusLabels: Record<string, string> = { available: '可使用', partial: '部分可使用', unavailable: '無法取得' };
export function taiwanFinancialsStatusLabel(status?: string) {
	return status ? financialsStatusLabels[status] || status : '';
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

// M7D — institutional/margin net/change values are raw shares (never converted to lots) with a
// backend-authoritative sign: positive shows a leading +, negative already carries its own -, and
// a genuine 0 shows as plain 0 (never +0). Missing stays "—", never fabricated as 0.
export function formatTaiwanSignedShares(value?: number | null) {
	if (value == null) return '—';
	return `${value > 0 ? '+' : ''}${value.toLocaleString('zh-TW')}`;
}

// short_margin_ratio is already expressed by the backend as a percentage-scale number (e.g. 0.4
// means 0.4%) — this only appends the % sign, it never re-scales the value.
export function formatTaiwanRatioPercent(value?: number | null) {
	return value == null ? '—' : `${value.toLocaleString('zh-TW', { maximumFractionDigits: 2 })}%`;
}

// M7E-A — monthly_revenue is already TWD (the backend's thousandTWD() converted it from the
// source's thousand-TWD raw value) — this never divides by 1,000 or otherwise rescales, it only adds
// locale grouping for readability. No leading + sign for an absolute revenue amount.
export function formatTaiwanRevenueTWD(value?: number | null) {
	return value == null ? '—' : value.toLocaleString('zh-TW');
}

// M7E-A — PE/PB and per-share dividend values: plain decimal formatting, no sign, no unit. Missing
// stays "—"; a genuine 0 (e.g. no stock dividend this year) stays "0", never fabricated or hidden.
export function formatTaiwanPlainNumber(value?: number | null) {
	return value == null ? '—' : value.toLocaleString('zh-TW', { maximumFractionDigits: 2 });
}

// M7E-C — book_value_per_share is already a plain TWD-per-share decimal as reported by the backend's
// official 每股參考淨值 field — this never converts currency, never rescales, it only appends the
// unit label for display. Missing stays "—"; a genuine 0 or negative value is preserved untouched.
export function formatTaiwanBookValuePerShare(value?: number | null) {
	return value == null ? '—' : `${value.toLocaleString('zh-TW', { maximumFractionDigits: 2 })} 元／股`;
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
