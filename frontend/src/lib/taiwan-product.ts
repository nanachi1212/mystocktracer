export type TaiwanScope = 'twse' | 'tpex' | 'combined';

export const taiwanPrimaryNavigation = [
	['taiwan-overview', '台股總覽'], ['taiwan-breadth', '市場廣度'], ['taiwan-emotion', '市場情緒'],
	['taiwan-industry', '產業雷達'], ['taiwan-stock', '個股研究'], ['taiwan-research', 'AI 研究'],
] as const;

export const taiwanDefaultWorkspace = 'taiwan-overview';

export function resolveTaiwanWorkspace(hash: string) {
	const value = hash.replace(/^#/, '');
	return taiwanPrimaryNavigation.some(([id]) => id === value) ? value : taiwanDefaultWorkspace;
}

export const taiwanMarketPath = (kind: 'market-breadth' | 'market-emotion' | 'industry-radar', scope: TaiwanScope) => `/api/v1/tw/${kind}?scope=${scope}`;
export const taiwanIntelligencePath = (symbol: string) => `/api/v1/tw/stocks/${encodeURIComponent(symbol)}/intelligence`;
export const taiwanResearchPath = (symbol: string) => `/api/v1/tw/stocks/${encodeURIComponent(symbol)}/research`;

export const taiwanScopes: { id: TaiwanScope; label: string }[] = [
	{ id: 'combined', label: '台灣市場' },
	{ id: 'twse', label: '上市（TWSE）' },
	{ id: 'tpex', label: '上櫃（TPEx）' },
];

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
