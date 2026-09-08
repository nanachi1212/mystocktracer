import type { Intelligence } from '../components/TaiwanStockResearchWorkspace';
import {
	formatTaiwanBookValuePerShare,
	formatTaiwanCashFlowTWD,
	formatTaiwanPercent,
	formatTaiwanPlainNumber,
	taiwanReasonLabel,
	taiwanSecurityTypeLabel,
	taiwanStatusLabel,
} from './taiwan-product';

export type SubscriptionAIProvider = 'chatgpt' | 'claude' | 'gemini';

export type SubscriptionAIAnalysisType = 'comprehensive' | 'fundamental' | 'technical_chips';

export const SUBSCRIPTION_AI_PROVIDERS: { id: SubscriptionAIProvider; name: string; url: string }[] = [
	{ id: 'chatgpt', name: 'ChatGPT', url: 'https://chatgpt.com/' },
	{ id: 'claude', name: 'Claude', url: 'https://claude.ai/' },
	{ id: 'gemini', name: 'Gemini', url: 'https://gemini.google.com/' },
];

export const SUBSCRIPTION_AI_ANALYSIS_TYPES: { id: SubscriptionAIAnalysisType; label: string; description: string }[] = [
	{ id: 'comprehensive', label: '綜合分析', description: '趨勢、基本面、籌碼、估值、優勢、風險與重要觀察條件' },
	{ id: 'fundamental', label: '基本面分析', description: '營收 / 獲利、財務品質、估值、產業與基本面風險' },
	{ id: 'technical_chips', label: '技術 / 籌碼分析', description: '價格趨勢、量價、法人、融資融券、市場環境與技術 / 籌碼風險' },
];

function formatPeriod(year?: number, quarter?: number): string {
	return year ? `${year}-Q${quarter}` : '資料未提供';
}

function valueOrUnavailable(val: string | number | null | undefined, formatter?: (v: any) => string): string {
	if (val === null || val === undefined) return '未提供 (unavailable)';
	if (formatter) return formatter(val);
	return String(val);
}

/**
 * Builds factual evidence text exclusively from the currently selected stock's Intelligence object.
 * Strictly guarantees isolation: no portfolio, no other watchlist items, no credentials/secrets.
 */
export function buildEvidenceSummary(intelligence: Intelligence): string {
	const {
		identity,
		quote,
		price_history_summary,
		fundamentals,
		institutional,
		margin,
		market_context,
		industry_context,
		interpretation,
	} = intelligence;

	const lines: string[] = [];
	lines.push('【證券識別資訊】');
	lines.push(`- 統一識別碼: ${identity.canonical_symbol}`);
	lines.push(`- 代碼: ${identity.code}`);
	lines.push(`- 名稱: ${identity.name}`);
	lines.push(`- 市場別: ${identity.exchange}`);
	lines.push(`- 證券類別: ${taiwanSecurityTypeLabel(identity.security_type)}`);
	lines.push(`- 幣別: ${identity.currency}`);
	lines.push(`- 產業: ${identity.industry_name || '未提供'}`);

	lines.push('');
	lines.push('【即時與收盤報價】');
	lines.push(`- 資料狀態: ${taiwanStatusLabel(quote.status)} (freshness: ${taiwanStatusLabel(quote.freshness)})`);
	lines.push(`- 資料日期: ${quote.as_of || quote.data?.meta?.trade_date || '未提供'}`);
	lines.push(`- 最新完成交易日: ${quote.target_latest_completed_trading_date || '未提供'}`);
	if (quote.data) {
		lines.push(`- 最新價格: ${quote.data.price.toLocaleString('zh-TW')} TWD`);
		lines.push(`- 漲跌幅: ${formatTaiwanPercent(quote.data.change_percent, true)}`);
		lines.push(`- 是否為盤中即時: ${quote.data.meta?.is_realtime ? '是' : '否 (收盤/歷史)'}`);
	} else {
		lines.push('- 報價數據: 未提供 (unavailable)');
	}

	lines.push('');
	lines.push('【近期價格與報酬表現】');
	lines.push(`- 資料狀態: ${taiwanStatusLabel(price_history_summary.status)} (freshness: ${taiwanStatusLabel(price_history_summary.freshness)})`);
	lines.push(`- 5 日收盤報酬率: ${valueOrUnavailable(price_history_summary.return_5d_percent, (v) => formatTaiwanPercent(v, true))}`);
	lines.push(`- 20 日收盤報酬率: ${valueOrUnavailable(price_history_summary.return_20d_percent, (v) => formatTaiwanPercent(v, true))}`);
	lines.push(`- 最近交易日: ${price_history_summary.latest_bar_date || '未提供'}`);

	lines.push('');
	lines.push('【市場環境】');
	lines.push(`- 市場狀態: ${taiwanStatusLabel(market_context.state)}`);
	lines.push(`- 資料信心: ${taiwanStatusLabel(market_context.confidence)}`);
	lines.push(`- 資料狀態: ${taiwanStatusLabel(market_context.status)} (freshness: ${taiwanStatusLabel(market_context.freshness)})`);
	lines.push(`- 上漲家數比率: ${valueOrUnavailable(market_context.advance_ratio, (v) => formatTaiwanPercent(v * 100))}`);
	lines.push(`- 上漲金額比率: ${valueOrUnavailable(market_context.advancing_amount_ratio, (v) => formatTaiwanPercent(v * 100))}`);

	lines.push('');
	lines.push('【產業環境】');
	lines.push(`- 資料狀態: ${taiwanStatusLabel(industry_context.status)} (freshness: ${taiwanStatusLabel(industry_context.freshness)})`);
	if (industry_context.industry) {
		lines.push(`- 產業名稱: ${industry_context.industry.industry_name}`);
		lines.push(`- 相對市場廣度: ${valueOrUnavailable(industry_context.industry.relative_breadth, (v) => formatTaiwanPercent(v * 100, true))}`);
		lines.push(`- 相對成交方向: ${valueOrUnavailable(industry_context.industry.relative_capital, (v) => formatTaiwanPercent(v * 100, true))}`);
	} else {
		lines.push(`- 產業備註: ${taiwanReasonLabel(industry_context.reason || '無產業細部資料/不適用')}`);
	}

	lines.push('');
	lines.push('【法人籌碼】');
	lines.push(`- 資料狀態: ${taiwanStatusLabel(institutional.status)} (freshness: ${taiwanStatusLabel(institutional.freshness)})`);
	lines.push(`- 資料日期: ${institutional.as_of || '未提供'}`);
	lines.push(`- 備註說明: ${taiwanReasonLabel(institutional.reason || '無')}`);

	lines.push('');
	lines.push('【融資融券】');
	lines.push(`- 資料狀態: ${taiwanStatusLabel(margin.status)} (freshness: ${taiwanStatusLabel(margin.freshness)})`);
	lines.push(`- 資料日期: ${margin.as_of || '未提供'}`);
	lines.push(`- 備註說明: ${taiwanReasonLabel(margin.reason || '無')}`);

	lines.push('');
	lines.push('【基本面與財務現況】');
	lines.push(`- 資料狀態: ${taiwanStatusLabel(fundamentals.status)} (freshness: ${taiwanStatusLabel(fundamentals.freshness)})`);
	lines.push(`- 資料日期/備註: ${fundamentals.as_of || taiwanReasonLabel(fundamentals.reason || '無')}`);

	if (identity.security_type === 'etf') {
		lines.push('- 證券屬性備註: 此為 ETF (指數股票型基金)，不適用一般公司財務比率與損益資產表。');
		lines.push('  [公司財務數據]');
		lines.push('  - 獲利能力 (EPS/毛利/營益/淨利): 不適用 (not_applicable)');
		lines.push('  - 資產負債表 (負債比/流動比/每股淨值): 不適用 (not_applicable)');
		lines.push('  - 現金流量表 (營業活動現金流): 不適用 (not_applicable)');

		const valuation = fundamentals.data?.valuation;
		if (valuation) {
			lines.push('  [估值與市場合規指標]');
			lines.push(`  - 資料日期: ${valuation.data_date || '未提供'}`);
			lines.push(`  - 本益比 (PE): ${valueOrUnavailable(valuation.pe, formatTaiwanPlainNumber)}`);
			lines.push(`  - 股價淨值比 (PB): ${valueOrUnavailable(valuation.pb, formatTaiwanPlainNumber)}`);
			lines.push(`  - 殖利率 (%): ${valueOrUnavailable(valuation.dividend_yield_percent, formatTaiwanPercent)}`);
		}
	} else {
		const valuation = fundamentals.data?.valuation;
		lines.push('  [估值指標]');
		if (valuation) {
			lines.push(`  - 資料日期: ${valuation.data_date || '未提供'}`);
			lines.push(`  - 本益比 (PE): ${valueOrUnavailable(valuation.pe, formatTaiwanPlainNumber)}`);
			lines.push(`  - 股價淨值比 (PB): ${valueOrUnavailable(valuation.pb, formatTaiwanPlainNumber)}`);
			lines.push(`  - 殖利率 (%): ${valueOrUnavailable(valuation.dividend_yield_percent, formatTaiwanPercent)}`);
		} else {
			lines.push('  - 估值資料: 未提供 (unavailable)');
		}

		const statement = fundamentals.data?.financial_statement;
		lines.push('  [獲利能力]');
		if (statement && statement.fiscal_year) {
			lines.push(`  - 財報期間: ${formatPeriod(statement.fiscal_year, statement.fiscal_quarter)}`);
			lines.push(`  - 累計 EPS: ${valueOrUnavailable(statement.cumulative_eps, formatTaiwanPlainNumber)}`);
			lines.push(`  - 毛利率 (%): ${valueOrUnavailable(statement.gross_margin_percent, formatTaiwanPercent)}`);
			lines.push(`  - 營業利益率 (%): ${valueOrUnavailable(statement.operating_margin_percent, formatTaiwanPercent)}`);
			lines.push(`  - 淨利率 (%): ${valueOrUnavailable(statement.net_margin_percent, formatTaiwanPercent)}`);
		} else {
			lines.push('  - 獲利能力資料: 未提供 (unavailable)');
		}

		lines.push('  [資產負債]');
		if (statement && statement.balance_fiscal_year) {
			lines.push(`  - 資產負債表期間: ${formatPeriod(statement.balance_fiscal_year, statement.balance_fiscal_quarter)}`);
			lines.push(`  - 每股參考淨值: ${valueOrUnavailable(statement.book_value_per_share, formatTaiwanBookValuePerShare)}`);
			lines.push(`  - 負債比率 (%): ${valueOrUnavailable(statement.debt_ratio_percent, formatTaiwanPercent)}`);
			lines.push(`  - 負債權益比 (%): ${valueOrUnavailable(statement.debt_to_equity_percent, formatTaiwanPercent)}`);
			lines.push(`  - 流動比率 (%): ${valueOrUnavailable(statement.current_ratio_percent, formatTaiwanPercent)}`);
		} else {
			lines.push('  - 資產負債資料: 未提供 (unavailable)');
		}

		lines.push('  [現金流量]');
		if (statement && statement.cashflow_fiscal_year) {
			lines.push(`  - 現金流量期間: ${formatPeriod(statement.cashflow_fiscal_year, statement.cashflow_fiscal_quarter)}`);
			lines.push(`  - 營業活動現金流量: ${valueOrUnavailable(statement.operating_cash_flow, formatTaiwanCashFlowTWD)}`);
			lines.push(`  - 營業現金流／淨利 (%): ${valueOrUnavailable(statement.cash_flow_to_net_income, formatTaiwanPercent)}`);
		} else {
			lines.push('  - 現金流量資料: 未提供 (unavailable)');
		}
	}

	if (interpretation) {
		lines.push('');
		lines.push('【確定性規則解讀摘要】');
		lines.push(`- 模型版本: ${interpretation.model_version}`);
		for (const [k, comp] of Object.entries(interpretation.components)) {
			lines.push(`- ${k}: 狀態=${taiwanStatusLabel(comp.state)} / ${taiwanStatusLabel(comp.status)}, 原因=${comp.reasons.map(taiwanReasonLabel).join('; ') || '無'}`);
		}
	}

	return lines.join('\n');
}

/**
 * Builds the complete prompt for the chosen analysis type and stock evidence.
 */
export function buildSubscriptionAIPrompt(intelligence: Intelligence, analysisType: SubscriptionAIAnalysisType): string {
	const evidenceText = buildEvidenceSummary(intelligence);

	let focusInstructions = '';
	if (analysisType === 'comprehensive') {
		focusInstructions = `【分析方式：綜合分析】
請針對以下構面進行全面客觀評估：
- 趨勢
- 基本面
- 籌碼
- 估值
- 優勢
- 風險
- 重要觀察條件`;
	} else if (analysisType === 'fundamental') {
		focusInstructions = `【分析方式：基本面分析】
請針對以下構面進行全面客觀評估：
- 營收 / 獲利
- 財務品質
- 估值
- 產業
- 基本面風險`;
	} else if (analysisType === 'technical_chips') {
		focusInstructions = `【分析方式：技術 / 籌碼分析】
請針對以下構面進行全面客觀評估：
- 價格趨勢
- 量價
- 法人
- 融資融券
- 市場環境
- 技術 / 籌碼風險`;
	}

	return `你是台灣股票研究助理。

請只根據以下提供的資料進行分析。

要求：
- 使用繁體中文
- 區分已知事實與推論
- 不捏造缺失資料
- null / unavailable / partial / stale 必須保留原意
- 清楚指出資料限制
- 不保證收益
- 不寫成確定的買進或賣出指令
- 不憑空預測目標價

${focusInstructions}

==================================================
【以下為該證券之官方與客觀市場研究資料】
==================================================
${evidenceText}
`;
}
