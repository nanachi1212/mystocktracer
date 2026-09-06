package httpapi

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"easy-stock/backend/internal/foundation"
)

// taiwanScreenerRow is the wire shape of one screened Taiwan security. M7A covers market-snapshot
// fields (price/volume/amount); M7D additively joins institutional and margin fields — all nullable,
// all backend-authoritative, none fabricated. No fundamentals, AI interpretation, or Watchlist state.
type taiwanScreenerRow struct {
	Canonical     string   `json:"canonical"`
	Code          string   `json:"code"`
	Name          string   `json:"name"`
	Exchange      string   `json:"exchange"`
	SecurityType  string   `json:"security_type"`
	TradeDate     string   `json:"trade_date"`
	Price         *float64 `json:"price"`
	Change        *float64 `json:"change"`
	ChangePercent *float64 `json:"change_percent"`
	Volume        *int64   `json:"volume"`
	Amount        *float64 `json:"amount"`
	// M7D — institutional (nil when the security has no institutional row for the target date, or
	// when the institutional domain was not requested at all; a genuine reported net of 0 is 0, never nil).
	ForeignNet       *int64 `json:"foreign_net"`
	TrustNet         *int64 `json:"trust_net"`
	DealerNet        *int64 `json:"dealer_net"`
	InstitutionalNet *int64 `json:"institutional_net"`
	// M7D — margin/short (values reused directly from the existing MarginTrading model; nil stays nil).
	MarginBalance    *int64   `json:"margin_balance"`
	MarginChange     *int64   `json:"margin_change"`
	ShortBalance     *int64   `json:"short_balance"`
	ShortChange      *int64   `json:"short_change"`
	ShortMarginRatio *float64 `json:"short_margin_ratio"`
	// M7E-A — revenue/valuation/dividends (all live-current, official-bulk-sourced; nil when that
	// domain was not requested at all, or when the security has no row in that domain's bulk payload).
	// monthly_revenue is in TWD (already converted from the source's thousand-TWD raw values by the
	// existing thousandTWD() parser — never rescaled here). revenue_yoy is the OFFICIAL YoY% carried
	// directly by the same bulk row, never FinMind, never locally computed.
	MonthlyRevenue *int64   `json:"monthly_revenue"`
	RevenueYoY     *float64 `json:"revenue_yoy"`
	// M7F — revenue_mom/cumulative_revenue_yoy come from the exact same official bulk row as
	// monthly_revenue/revenue_yoy above (never a second lookup, never FinMind). revenue_mom is the
	// official current-month-over-previous-month %; cumulative_revenue_yoy is the official
	// cumulative-revenue year-over-year %. Both are already percentage-scale, never rescaled here.
	RevenueMoM           *float64 `json:"revenue_mom"`
	CumulativeRevenueYoY *float64 `json:"cumulative_revenue_yoy"`
	PE                   *float64 `json:"pe"`
	PB                   *float64 `json:"pb"`
	DividendYield        *float64 `json:"dividend_yield"`
	CashDividend         *float64 `json:"cash_dividend"`
	StockDividend        *float64 `json:"stock_dividend"`
	TotalDividend        *float64 `json:"total_dividend"`
	// M7E-B — financial statement (income-statement only; live-current, official-bulk-sourced).
	// financial_period is the security's OWN actual reporting period (e.g. "2026-Q2"), always set
	// whenever a valid statement row was parsed for it — independent of whether the metric fields
	// below are populated. cumulative_eps/gross_margin/operating_margin are populated ONLY when
	// financial_period equals the domain's common target period (see taiwanScreenerResponse.
	// FinancialsPeriod): a cumulative Q1 value is never compared against a cumulative Q2 value.
	// gross_margin/operating_margin are additionally scoped to the "ci" (general industry) category
	// only — every other category (including insurance, which happens to carry compatible fields)
	// always leaves them nil, per the M7E-B.1 cross-industry accounting-semantics policy.
	FinancialPeriod *string  `json:"financial_period"`
	CumulativeEPS   *float64 `json:"cumulative_eps"`
	GrossMargin     *float64 `json:"gross_margin"`
	OperatingMargin *float64 `json:"operating_margin"`
	// M7E-C — net_margin (income-statement, ci-only, reuses the same financial_period gate as
	// gross_margin/operating_margin above — see ScreenerFinancials) and book_value_per_share
	// (balance-sheet, all categories, requires its own balance row's period to equal
	// financial_period AND financial_period to equal the domain target — see toTaiwanScreenerRow).
	// net_margin is deliberately scoped to "ci" only: the "fh" (financial holding) category's income
	// payload carries a small, unrelated `淨收益` line that the shared revenue-fallback parser can
	// otherwise mistake for total revenue, producing an economically meaningless net margin (see
	// M7E-C.0) — this field is never populated for non-ci rows, regardless of what the underlying
	// payload happens to contain.
	NetMargin         *float64 `json:"net_margin"`
	BookValuePerShare *float64 `json:"book_value_per_share"`
}

type taiwanScreenerResponse struct {
	Scope      string              `json:"scope"`
	AsOf       *string             `json:"as_of"`
	Freshness  string              `json:"freshness"`
	Total      int                 `json:"total"`
	Offset     int                 `json:"offset"`
	Limit      int                 `json:"limit"`
	Securities []taiwanScreenerRow `json:"securities"`
	// M7D — additive, present only when the corresponding domain was actually requested (a filter or
	// sort key engaged it). Sourced directly from the existing TaiwanFreshness model — trade-date
	// AsOf/status/days-behind only. Never a published_at/available_at timestamp: M7D.0 established
	// that the official payloads carry no such field, and 17:30 remains only a candidate-date
	// selector, never a publication guarantee.
	InstitutionalAsOf       *string `json:"institutional_as_of,omitempty"`
	InstitutionalStatus     string  `json:"institutional_status,omitempty"`
	InstitutionalDaysBehind *int    `json:"institutional_days_behind,omitempty"`
	MarginAsOf              *string `json:"margin_as_of,omitempty"`
	MarginStatus            string  `json:"margin_status,omitempty"`
	MarginDaysBehind        *int    `json:"margin_days_behind,omitempty"`
	// M7E-A — additive, present only when the corresponding domain was actually requested. Each
	// domain's AsOf/Status carries only that domain's own truthful cadence (see
	// foundation.TaiwanFundamentalsDomainFreshness / TaiwanValuationFreshness): revenue/dividends
	// never expose DaysBehind (a monthly period or dividend year has no truthful trading-day
	// days-behind equivalent), so those fields are always omitted (nil, omitempty). Never a
	// published_at/available_at claim — M7E.0 established none of these official payloads carry one.
	RevenueAsOf         *string `json:"revenue_as_of,omitempty"`
	RevenueStatus       string  `json:"revenue_status,omitempty"`
	RevenueDaysBehind   *int    `json:"revenue_days_behind,omitempty"`
	ValuationAsOf       *string `json:"valuation_as_of,omitempty"`
	ValuationStatus     string  `json:"valuation_status,omitempty"`
	ValuationDaysBehind *int    `json:"valuation_days_behind,omitempty"`
	DividendsAsOf       *string `json:"dividends_as_of,omitempty"`
	DividendsStatus     string  `json:"dividends_status,omitempty"`
	DividendsDaysBehind *int    `json:"dividends_days_behind,omitempty"`
	// M7E-B — additive, present only when the financials domain was actually requested.
	// FinancialsPeriod is the market-wide common TARGET period ("2026-Q2") used to gate which rows'
	// metric fields are populated (see taiwanScreenerRow.FinancialPeriod) — never a fabricated
	// quarter-end date. FinancialsStatus additionally supports "partial" (at least one of the 12
	// category/exchange requests failed but some succeeded), beyond the existing available/
	// unavailable vocabulary — no days-behind (quarterly filings have no truthful trading-day
	// cadence) and never a published_at/available_at claim.
	FinancialsPeriod *string `json:"financials_period,omitempty"`
	FinancialsStatus string  `json:"financials_status,omitempty"`
}

type taiwanScreenerQuery struct {
	scope                              string // "", "twse", "tpex", or "combined" — already validated/lowercased
	minPrice, maxPrice                 *float64
	minChangePercent, maxChangePercent *float64
	minVolume, maxVolume               *float64
	minAmount, maxAmount               *float64
	// M7D institutional filters.
	minForeignNet, maxForeignNet             *float64
	minTrustNet, maxTrustNet                 *float64
	minDealerNet, maxDealerNet               *float64
	minInstitutionalNet, maxInstitutionalNet *float64
	// M7D margin filters.
	minMarginBalance, maxMarginBalance       *float64
	minMarginChange, maxMarginChange         *float64
	minShortBalance, maxShortBalance         *float64
	minShortChange, maxShortChange           *float64
	minShortMarginRatio, maxShortMarginRatio *float64
	// M7E-A revenue/valuation/dividend filters.
	minMonthlyRevenue, maxMonthlyRevenue *float64
	minRevenueYoY, maxRevenueYoY         *float64
	// M7F revenue growth (official MoM% and cumulative YoY%, same bulk row as above).
	minRevenueMoM, maxRevenueMoM                     *float64
	minCumulativeRevenueYoY, maxCumulativeRevenueYoY *float64
	minPE, maxPE                                     *float64
	minPB, maxPB                                     *float64
	minDividendYield, maxDividendYield               *float64
	minCashDividend, maxCashDividend                 *float64
	minStockDividend, maxStockDividend               *float64
	minTotalDividend, maxTotalDividend               *float64
	// M7E-B financial statement filters (cumulative EPS + ci-only margins).
	minCumulativeEPS, maxCumulativeEPS     *float64
	minGrossMargin, maxGrossMargin         *float64
	minOperatingMargin, maxOperatingMargin *float64
	// M7E-C financial statement filters (ci-only net margin + all-category book value per share).
	minNetMargin, maxNetMargin                 *float64
	minBookValuePerShare, maxBookValuePerShare *float64
	sort                                       string
	order                                      string // "asc" | "desc"
}

var taiwanScreenerSortKeys = map[string]bool{
	"price": true, "change_percent": true, "volume": true, "amount": true,
	"foreign_net": true, "trust_net": true, "dealer_net": true, "institutional_net": true,
	"margin_balance": true, "margin_change": true, "short_balance": true, "short_change": true, "short_margin_ratio": true,
	"monthly_revenue": true, "revenue_yoy": true, "pe": true, "pb": true, "dividend_yield": true,
	"cash_dividend": true, "stock_dividend": true, "total_dividend": true,
	"cumulative_eps": true, "gross_margin": true, "operating_margin": true,
	"net_margin": true, "book_value_per_share": true,
	"revenue_mom": true, "cumulative_revenue_yoy": true,
}

// taiwanScreenerNeedsInstitutional/taiwanScreenerNeedsMargin decide whether this specific request
// actually engages that domain (an active min/max filter, or a sort key from that domain). This is
// the sole gate for fetching institutional/margin data — a vanilla M7A-style request (no advanced
// params) must never trigger either, preserving the legacy daily-only request cost exactly.
func taiwanScreenerNeedsInstitutional(q taiwanScreenerQuery) bool {
	switch q.sort {
	case "foreign_net", "trust_net", "dealer_net", "institutional_net":
		return true
	}
	return q.minForeignNet != nil || q.maxForeignNet != nil ||
		q.minTrustNet != nil || q.maxTrustNet != nil ||
		q.minDealerNet != nil || q.maxDealerNet != nil ||
		q.minInstitutionalNet != nil || q.maxInstitutionalNet != nil
}

func taiwanScreenerNeedsMargin(q taiwanScreenerQuery) bool {
	switch q.sort {
	case "margin_balance", "margin_change", "short_balance", "short_change", "short_margin_ratio":
		return true
	}
	return q.minMarginBalance != nil || q.maxMarginBalance != nil ||
		q.minMarginChange != nil || q.maxMarginChange != nil ||
		q.minShortBalance != nil || q.maxShortBalance != nil ||
		q.minShortChange != nil || q.maxShortChange != nil ||
		q.minShortMarginRatio != nil || q.maxShortMarginRatio != nil
}

// taiwanScreenerNeedsRevenue/taiwanScreenerNeedsValuation/taiwanScreenerNeedsDividends are the M7E-A
// analogues of the two functions above — the sole gate for fetching each fundamentals domain. A
// vanilla request (and one that only engages M7A/M7D fields) must never trigger any of these.
func taiwanScreenerNeedsRevenue(q taiwanScreenerQuery) bool {
	switch q.sort {
	case "monthly_revenue", "revenue_yoy", "revenue_mom", "cumulative_revenue_yoy":
		return true
	}
	return q.minMonthlyRevenue != nil || q.maxMonthlyRevenue != nil ||
		q.minRevenueYoY != nil || q.maxRevenueYoY != nil ||
		q.minRevenueMoM != nil || q.maxRevenueMoM != nil ||
		q.minCumulativeRevenueYoY != nil || q.maxCumulativeRevenueYoY != nil
}

func taiwanScreenerNeedsValuation(q taiwanScreenerQuery) bool {
	switch q.sort {
	case "pe", "pb", "dividend_yield":
		return true
	}
	return q.minPE != nil || q.maxPE != nil ||
		q.minPB != nil || q.maxPB != nil ||
		q.minDividendYield != nil || q.maxDividendYield != nil
}

func taiwanScreenerNeedsDividends(q taiwanScreenerQuery) bool {
	switch q.sort {
	case "cash_dividend", "stock_dividend", "total_dividend":
		return true
	}
	return q.minCashDividend != nil || q.maxCashDividend != nil ||
		q.minStockDividend != nil || q.maxStockDividend != nil ||
		q.minTotalDividend != nil || q.maxTotalDividend != nil
}

// taiwanScreenerNeedsFinancials is the M7E-B analogue — the sole gate for fetching the income-statement
// financials domain (12 bounded requests). A vanilla request, and one that only engages M7A/M7D/
// M7E-A fields, must never trigger it. M7E-C's net_margin reuses this exact same income-statement data
// (no new endpoint), so it is included here rather than in taiwanScreenerNeedsBalance below.
func taiwanScreenerNeedsFinancials(q taiwanScreenerQuery) bool {
	switch q.sort {
	case "cumulative_eps", "gross_margin", "operating_margin", "net_margin":
		return true
	}
	return q.minCumulativeEPS != nil || q.maxCumulativeEPS != nil ||
		q.minGrossMargin != nil || q.maxGrossMargin != nil ||
		q.minOperatingMargin != nil || q.maxOperatingMargin != nil ||
		q.minNetMargin != nil || q.maxNetMargin != nil
}

// taiwanScreenerNeedsBalance is the M7E-C analogue — the sole gate for fetching the balance-sheet
// domain (12 bounded requests, independent of and additional to the 12 income-statement requests
// above). A vanilla request, and one that only engages net_margin (or any earlier domain), must never
// trigger it — only an active book_value_per_share filter or sort key does.
func taiwanScreenerNeedsBalance(q taiwanScreenerQuery) bool {
	if q.sort == "book_value_per_share" {
		return true
	}
	return q.minBookValuePerShare != nil || q.maxBookValuePerShare != nil
}

// combineFinancialsStatus reports one truthful financials_status across the income-statement domain
// and (when actually requested) the M7E-C balance-sheet domain. When balance was never requested, this
// is byte-for-byte the existing M7E-B income-only semantics. When balance was requested: "unavailable"
// is reported whenever income itself is unavailable — book_value_per_share can never be verified
// against an unknown income period, so no usable financial data exists at all in that case (M7E-C.0);
// "available" requires both subdomains to have fully succeeded; every other combination (income healthy
// but balance degraded/unavailable, or income itself merely partial) reports "partial" — some usable
// financial data remains, so the whole domain is never marked unavailable just because the optional
// balance subdomain under-delivered.
func combineFinancialsStatus(income foundation.TaiwanFundamentalsDomainFreshness, balanceRequested bool, balance foundation.TaiwanFundamentalsDomainFreshness) string {
	incomeStatus := income.Status
	if incomeStatus == "" {
		incomeStatus = "unavailable"
	}
	if !balanceRequested {
		return incomeStatus
	}
	if incomeStatus == "unavailable" {
		return "unavailable"
	}
	balanceStatus := balance.Status
	if balanceStatus == "" {
		balanceStatus = "unavailable"
	}
	if incomeStatus == "available" && balanceStatus == "available" {
		return "available"
	}
	return "partial"
}

func (s *Server) taiwanScreenerHandler(w http.ResponseWriter, r *http.Request) {
	if s.taiwanScreener == nil {
		writeError(w, http.StatusServiceUnavailable, "Taiwan screener provider is unavailable")
		return
	}
	query, offset, limit, errMessage := parseTaiwanScreenerQuery(r)
	if errMessage != "" {
		writeError(w, http.StatusBadRequest, errMessage)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	// Reuses the existing bulk market-wide snapshot (one TWSE + one TPEx request total, the same
	// path MarketBreadth already uses) — never one request per security, regardless of how many
	// rows are ultimately filtered/paginated below.
	rows, freshness, err := s.taiwanScreener.ScreenerSnapshot(ctx, time.Now())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	// M7D lazy domain loading: institutional/margin are fetched ONLY when this request actually
	// engages them. A domain fetch failure (or an unconfigured provider) never fails the whole
	// request — it only leaves that domain's fields/freshness absent; base daily rows stay usable.
	var institutional map[string]foundation.InstitutionalFlow
	var institutionalFreshness foundation.TaiwanFreshness
	institutionalRequested := taiwanScreenerNeedsInstitutional(query)
	if institutionalRequested && s.taiwanScreenerInstitutional != nil {
		instRows, instFreshness, instErr := s.taiwanScreenerInstitutional.ScreenerInstitutional(ctx, time.Now())
		institutionalFreshness = instFreshness
		if instErr == nil {
			institutional = make(map[string]foundation.InstitutionalFlow, len(instRows))
			for _, row := range instRows {
				institutional[row.Canonical] = row
			}
		}
	}

	var margin map[string]foundation.MarginTrading
	var marginFreshness foundation.TaiwanFreshness
	marginRequested := taiwanScreenerNeedsMargin(query)
	if marginRequested && s.taiwanScreenerMargin != nil {
		marginRows, mFreshness, marginErr := s.taiwanScreenerMargin.ScreenerMargin(ctx, time.Now())
		marginFreshness = mFreshness
		if marginErr == nil {
			margin = make(map[string]foundation.MarginTrading, len(marginRows))
			for _, row := range marginRows {
				margin[row.Canonical] = row
			}
		}
	}

	// M7E-A lazy domain loading: revenue/valuation/dividends are fetched ONLY when this request
	// actually engages them, following the exact same pattern as institutional/margin above. A domain
	// fetch failure (or an unconfigured provider) never fails the whole request.
	var revenue map[string]foundation.MonthlyRevenue
	var revenueFreshness foundation.TaiwanFundamentalsDomainFreshness
	revenueRequested := taiwanScreenerNeedsRevenue(query)
	if revenueRequested && s.taiwanScreenerRevenue != nil {
		revRows, revFreshness, revErr := s.taiwanScreenerRevenue.ScreenerRevenue(ctx, time.Now())
		revenueFreshness = revFreshness
		if revErr == nil {
			revenue = make(map[string]foundation.MonthlyRevenue, len(revRows))
			for _, row := range revRows {
				revenue[row.Canonical] = row
			}
		}
	}

	var valuation map[string]foundation.ValuationSnapshot
	var valuationFreshness foundation.TaiwanValuationFreshness
	valuationRequested := taiwanScreenerNeedsValuation(query)
	if valuationRequested && s.taiwanScreenerValuation != nil {
		valRows, valFreshness, valErr := s.taiwanScreenerValuation.ScreenerValuation(ctx, time.Now())
		valuationFreshness = valFreshness
		if valErr == nil {
			valuation = make(map[string]foundation.ValuationSnapshot, len(valRows))
			for _, row := range valRows {
				valuation[row.Canonical] = row
			}
		}
	}

	var dividends map[string]foundation.DividendRecord
	var dividendsFreshness foundation.TaiwanFundamentalsDomainFreshness
	dividendsRequested := taiwanScreenerNeedsDividends(query)
	if dividendsRequested && s.taiwanScreenerDividends != nil {
		divRows, divFreshness, divErr := s.taiwanScreenerDividends.ScreenerDividends(ctx, time.Now())
		dividendsFreshness = divFreshness
		if divErr == nil {
			dividends = make(map[string]foundation.DividendRecord, len(divRows))
			for _, row := range divRows {
				dividends[row.Canonical] = row
			}
		}
	}

	// M7E-B lazy domain loading: financials (income statement only) is fetched ONLY when this request
	// actually engages it, following the exact same pattern as revenue/valuation/dividends above.
	// M7E-C's book_value_per_share additionally requires income data (it needs each row's own income
	// period to decide whether that security's BVPS is period-aligned — see the balance join below),
	// so a BVPS-only request also triggers this fetch even though it engages no income-only field.
	var financials map[string]foundation.FinancialStatementPeriod
	var financialsFreshness foundation.TaiwanFundamentalsDomainFreshness
	financialsRequested := taiwanScreenerNeedsFinancials(query)
	balanceRequested := taiwanScreenerNeedsBalance(query)
	if (financialsRequested || balanceRequested) && s.taiwanScreenerFinancials != nil {
		finRows, finFreshness, finErr := s.taiwanScreenerFinancials.ScreenerFinancials(ctx, time.Now())
		financialsFreshness = finFreshness
		if finErr == nil {
			financials = make(map[string]foundation.FinancialStatementPeriod, len(finRows))
			for _, row := range finRows {
				financials[row.Canonical] = row
			}
		}
	}

	// M7E-C lazy domain loading: the balance-sheet domain (book_value_per_share) is fetched ONLY when
	// this request actually engages it — a net_margin-only request must never trigger this.
	var balance map[string]foundation.FinancialStatementPeriod
	var balanceFreshness foundation.TaiwanFundamentalsDomainFreshness
	if balanceRequested && s.taiwanScreenerBalance != nil {
		balRows, balFreshness, balErr := s.taiwanScreenerBalance.ScreenerBalance(ctx, time.Now())
		balanceFreshness = balFreshness
		if balErr == nil {
			balance = make(map[string]foundation.FinancialStatementPeriod, len(balRows))
			for _, row := range balRows {
				balance[row.Canonical] = row
			}
		}
	}

	filtered := filterAndSortTaiwanScreener(rows, institutional, margin, revenue, valuation, dividends, financials, balance, financialsFreshness.AsOf, query)
	total := len(filtered)
	page := paginateTaiwanScreener(filtered, offset, limit)

	scopeLabel := "COMBINED"
	if query.scope != "" && query.scope != "combined" {
		scopeLabel = strings.ToUpper(query.scope)
	}
	response := taiwanScreenerResponse{
		Scope: scopeLabel, AsOf: freshness.DailyAsOf, Freshness: freshness.DailyStatus,
		Total: total, Offset: offset, Limit: limit, Securities: page,
	}
	if institutionalRequested {
		response.InstitutionalAsOf = institutionalFreshness.InstitutionalAsOf
		response.InstitutionalStatus = institutionalFreshness.InstitutionalStatus
		response.InstitutionalDaysBehind = institutionalFreshness.InstitutionalDaysBehind
		if response.InstitutionalStatus == "" {
			response.InstitutionalStatus = "unavailable"
		}
	}
	if marginRequested {
		response.MarginAsOf = marginFreshness.MarginAsOf
		response.MarginStatus = marginFreshness.MarginStatus
		response.MarginDaysBehind = marginFreshness.MarginDaysBehind
		if response.MarginStatus == "" {
			response.MarginStatus = "unavailable"
		}
	}
	if revenueRequested {
		response.RevenueAsOf = revenueFreshness.AsOf
		response.RevenueStatus = revenueFreshness.Status
		if response.RevenueStatus == "" {
			response.RevenueStatus = "unavailable"
		}
	}
	if valuationRequested {
		response.ValuationAsOf = valuationFreshness.AsOf
		response.ValuationStatus = valuationFreshness.Status
		response.ValuationDaysBehind = valuationFreshness.DaysBehind
		if response.ValuationStatus == "" {
			response.ValuationStatus = "unavailable"
		}
	}
	if dividendsRequested {
		response.DividendsAsOf = dividendsFreshness.AsOf
		response.DividendsStatus = dividendsFreshness.Status
		if response.DividendsStatus == "" {
			response.DividendsStatus = "unavailable"
		}
	}
	if financialsRequested || balanceRequested {
		response.FinancialsPeriod = financialsFreshness.AsOf
		response.FinancialsStatus = combineFinancialsStatus(financialsFreshness, balanceRequested, balanceFreshness)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": response})
}

// parseTaiwanScreenerQuery validates and parses every query parameter up front, returning a
// non-empty message (with the handler's 400 response) on the first invalid input.
func parseTaiwanScreenerQuery(r *http.Request) (taiwanScreenerQuery, int, int, string) {
	query := taiwanScreenerQuery{}

	query.scope = strings.ToLower(strings.TrimSpace(r.URL.Query().Get("scope")))
	if query.scope != "" && query.scope != "twse" && query.scope != "tpex" && query.scope != "combined" {
		return query, 0, 0, "scope must be twse, tpex, or combined"
	}

	query.sort = strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort")))
	if query.sort == "" {
		query.sort = "amount"
	}
	if !taiwanScreenerSortKeys[query.sort] {
		return query, 0, 0, "sort must be one of price, change_percent, volume, amount, foreign_net, trust_net, dealer_net, institutional_net, margin_balance, margin_change, short_balance, short_change, short_margin_ratio, monthly_revenue, revenue_yoy, pe, pb, dividend_yield, cash_dividend, stock_dividend, total_dividend, cumulative_eps, gross_margin, operating_margin, net_margin, book_value_per_share, revenue_mom, cumulative_revenue_yoy"
	}

	query.order = strings.ToLower(strings.TrimSpace(r.URL.Query().Get("order")))
	if query.order == "" {
		query.order = "desc"
	}
	if query.order != "asc" && query.order != "desc" {
		return query, 0, 0, "order must be asc or desc"
	}

	var err error
	if query.minPrice, err = parseOptionalScreenerFloat(r, "min_price"); err != nil {
		return query, 0, 0, err.Error()
	}
	if query.maxPrice, err = parseOptionalScreenerFloat(r, "max_price"); err != nil {
		return query, 0, 0, err.Error()
	}
	if rangeInvalid(query.minPrice, query.maxPrice) {
		return query, 0, 0, "min_price must not be greater than max_price"
	}

	if query.minChangePercent, err = parseOptionalScreenerFloat(r, "min_change_percent"); err != nil {
		return query, 0, 0, err.Error()
	}
	if query.maxChangePercent, err = parseOptionalScreenerFloat(r, "max_change_percent"); err != nil {
		return query, 0, 0, err.Error()
	}
	if rangeInvalid(query.minChangePercent, query.maxChangePercent) {
		return query, 0, 0, "min_change_percent must not be greater than max_change_percent"
	}

	if query.minVolume, err = parseOptionalScreenerFloat(r, "min_volume"); err != nil {
		return query, 0, 0, err.Error()
	}
	if query.maxVolume, err = parseOptionalScreenerFloat(r, "max_volume"); err != nil {
		return query, 0, 0, err.Error()
	}
	if rangeInvalid(query.minVolume, query.maxVolume) {
		return query, 0, 0, "min_volume must not be greater than max_volume"
	}

	if query.minAmount, err = parseOptionalScreenerFloat(r, "min_amount"); err != nil {
		return query, 0, 0, err.Error()
	}
	if query.maxAmount, err = parseOptionalScreenerFloat(r, "max_amount"); err != nil {
		return query, 0, 0, err.Error()
	}
	if rangeInvalid(query.minAmount, query.maxAmount) {
		return query, 0, 0, "min_amount must not be greater than max_amount"
	}

	// M7D institutional range params — reuse the exact same optional-float parsing and min<=max
	// validation already proven for the M7A fields above; no new validation logic invented.
	institutionalRanges := []struct {
		minKey, maxKey string
		min, max       **float64
		label          string
	}{
		{"min_foreign_net", "max_foreign_net", &query.minForeignNet, &query.maxForeignNet, "min_foreign_net must not be greater than max_foreign_net"},
		{"min_trust_net", "max_trust_net", &query.minTrustNet, &query.maxTrustNet, "min_trust_net must not be greater than max_trust_net"},
		{"min_dealer_net", "max_dealer_net", &query.minDealerNet, &query.maxDealerNet, "min_dealer_net must not be greater than max_dealer_net"},
		{"min_institutional_net", "max_institutional_net", &query.minInstitutionalNet, &query.maxInstitutionalNet, "min_institutional_net must not be greater than max_institutional_net"},
	}
	for _, item := range institutionalRanges {
		if *item.min, err = parseOptionalScreenerFloat(r, item.minKey); err != nil {
			return query, 0, 0, err.Error()
		}
		if *item.max, err = parseOptionalScreenerFloat(r, item.maxKey); err != nil {
			return query, 0, 0, err.Error()
		}
		if rangeInvalid(*item.min, *item.max) {
			return query, 0, 0, item.label
		}
	}

	// M7D margin range params — same pattern.
	marginRanges := []struct {
		minKey, maxKey string
		min, max       **float64
		label          string
	}{
		{"min_margin_balance", "max_margin_balance", &query.minMarginBalance, &query.maxMarginBalance, "min_margin_balance must not be greater than max_margin_balance"},
		{"min_margin_change", "max_margin_change", &query.minMarginChange, &query.maxMarginChange, "min_margin_change must not be greater than max_margin_change"},
		{"min_short_balance", "max_short_balance", &query.minShortBalance, &query.maxShortBalance, "min_short_balance must not be greater than max_short_balance"},
		{"min_short_change", "max_short_change", &query.minShortChange, &query.maxShortChange, "min_short_change must not be greater than max_short_change"},
		{"min_short_margin_ratio", "max_short_margin_ratio", &query.minShortMarginRatio, &query.maxShortMarginRatio, "min_short_margin_ratio must not be greater than max_short_margin_ratio"},
	}
	for _, item := range marginRanges {
		if *item.min, err = parseOptionalScreenerFloat(r, item.minKey); err != nil {
			return query, 0, 0, err.Error()
		}
		if *item.max, err = parseOptionalScreenerFloat(r, item.maxKey); err != nil {
			return query, 0, 0, err.Error()
		}
		if rangeInvalid(*item.min, *item.max) {
			return query, 0, 0, item.label
		}
	}

	// M7E-A revenue/valuation/dividend range params — same pattern as the M7D ranges above.
	fundamentalsRanges := []struct {
		minKey, maxKey string
		min, max       **float64
		label          string
	}{
		{"min_monthly_revenue", "max_monthly_revenue", &query.minMonthlyRevenue, &query.maxMonthlyRevenue, "min_monthly_revenue must not be greater than max_monthly_revenue"},
		{"min_revenue_yoy", "max_revenue_yoy", &query.minRevenueYoY, &query.maxRevenueYoY, "min_revenue_yoy must not be greater than max_revenue_yoy"},
		{"min_revenue_mom", "max_revenue_mom", &query.minRevenueMoM, &query.maxRevenueMoM, "min_revenue_mom must not be greater than max_revenue_mom"},
		{"min_cumulative_revenue_yoy", "max_cumulative_revenue_yoy", &query.minCumulativeRevenueYoY, &query.maxCumulativeRevenueYoY, "min_cumulative_revenue_yoy must not be greater than max_cumulative_revenue_yoy"},
		{"min_pe", "max_pe", &query.minPE, &query.maxPE, "min_pe must not be greater than max_pe"},
		{"min_pb", "max_pb", &query.minPB, &query.maxPB, "min_pb must not be greater than max_pb"},
		{"min_dividend_yield", "max_dividend_yield", &query.minDividendYield, &query.maxDividendYield, "min_dividend_yield must not be greater than max_dividend_yield"},
		{"min_cash_dividend", "max_cash_dividend", &query.minCashDividend, &query.maxCashDividend, "min_cash_dividend must not be greater than max_cash_dividend"},
		{"min_stock_dividend", "max_stock_dividend", &query.minStockDividend, &query.maxStockDividend, "min_stock_dividend must not be greater than max_stock_dividend"},
		{"min_total_dividend", "max_total_dividend", &query.minTotalDividend, &query.maxTotalDividend, "min_total_dividend must not be greater than max_total_dividend"},
	}
	for _, item := range fundamentalsRanges {
		if *item.min, err = parseOptionalScreenerFloat(r, item.minKey); err != nil {
			return query, 0, 0, err.Error()
		}
		if *item.max, err = parseOptionalScreenerFloat(r, item.maxKey); err != nil {
			return query, 0, 0, err.Error()
		}
		if rangeInvalid(*item.min, *item.max) {
			return query, 0, 0, item.label
		}
	}

	// M7E-B financial-statement range params — same pattern as the M7E-A ranges above.
	financialsRanges := []struct {
		minKey, maxKey string
		min, max       **float64
		label          string
	}{
		{"min_cumulative_eps", "max_cumulative_eps", &query.minCumulativeEPS, &query.maxCumulativeEPS, "min_cumulative_eps must not be greater than max_cumulative_eps"},
		{"min_gross_margin", "max_gross_margin", &query.minGrossMargin, &query.maxGrossMargin, "min_gross_margin must not be greater than max_gross_margin"},
		{"min_operating_margin", "max_operating_margin", &query.minOperatingMargin, &query.maxOperatingMargin, "min_operating_margin must not be greater than max_operating_margin"},
	}
	for _, item := range financialsRanges {
		if *item.min, err = parseOptionalScreenerFloat(r, item.minKey); err != nil {
			return query, 0, 0, err.Error()
		}
		if *item.max, err = parseOptionalScreenerFloat(r, item.maxKey); err != nil {
			return query, 0, 0, err.Error()
		}
		if rangeInvalid(*item.min, *item.max) {
			return query, 0, 0, item.label
		}
	}

	// M7E-C financial-statement range params (net margin + book value per share) — same pattern.
	m7ecRanges := []struct {
		minKey, maxKey string
		min, max       **float64
		label          string
	}{
		{"min_net_margin", "max_net_margin", &query.minNetMargin, &query.maxNetMargin, "min_net_margin must not be greater than max_net_margin"},
		{"min_book_value_per_share", "max_book_value_per_share", &query.minBookValuePerShare, &query.maxBookValuePerShare, "min_book_value_per_share must not be greater than max_book_value_per_share"},
	}
	for _, item := range m7ecRanges {
		if *item.min, err = parseOptionalScreenerFloat(r, item.minKey); err != nil {
			return query, 0, 0, err.Error()
		}
		if *item.max, err = parseOptionalScreenerFloat(r, item.maxKey); err != nil {
			return query, 0, 0, err.Error()
		}
		if rangeInvalid(*item.min, *item.max) {
			return query, 0, 0, item.label
		}
	}

	limit, err := marketLimitQuery(r, 50, 200)
	if err != nil {
		return query, 0, 0, err.Error()
	}
	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		value, convErr := strconv.Atoi(raw)
		if convErr != nil || value < 0 {
			return query, 0, 0, "offset must be zero or a positive integer"
		}
		offset = value
	}

	return query, offset, limit, ""
}

func parseOptionalScreenerFloat(r *http.Request, key string) (*float64, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return nil, fmt.Errorf("%s must be a number", key)
	}
	return &value, nil
}

func rangeInvalid(min, max *float64) bool {
	return min != nil && max != nil && *min > *max
}

// filterAndSortTaiwanScreener applies scope/range filters and deterministic sorting entirely in
// memory over the already-fetched bulk snapshot rows (plus the optionally-fetched institutional/
// margin maps, each keyed by exact canonical) — no additional provider calls are made here.
//
// Filter semantics: a row is excluded from a given range filter only if that filter was actually
// requested (min/max present) AND the row's value for that field is unavailable (nil) — a filter
// that was never requested never excludes a row for that field, and an unavailable value is never
// treated as 0.
func filterAndSortTaiwanScreener(rows []foundation.TaiwanDailySnapshot, institutional map[string]foundation.InstitutionalFlow, margin map[string]foundation.MarginTrading, revenue map[string]foundation.MonthlyRevenue, valuation map[string]foundation.ValuationSnapshot, dividends map[string]foundation.DividendRecord, financials map[string]foundation.FinancialStatementPeriod, balance map[string]foundation.FinancialStatementPeriod, financialsTargetPeriod *string, query taiwanScreenerQuery) []taiwanScreenerRow {
	filtered := make([]taiwanScreenerRow, 0, len(rows))
	for _, raw := range rows {
		if query.scope != "" && query.scope != "combined" && !strings.EqualFold(raw.Exchange, query.scope) {
			continue
		}
		row := toTaiwanScreenerRow(raw, institutional, margin, revenue, valuation, dividends, financials, balance, financialsTargetPeriod)
		if !passesRange(row.Price, query.minPrice, query.maxPrice) {
			continue
		}
		if !passesRange(row.ChangePercent, query.minChangePercent, query.maxChangePercent) {
			continue
		}
		if !passesIntRange(row.Volume, query.minVolume, query.maxVolume) {
			continue
		}
		if !passesRange(row.Amount, query.minAmount, query.maxAmount) {
			continue
		}
		if !passesIntRange(row.ForeignNet, query.minForeignNet, query.maxForeignNet) {
			continue
		}
		if !passesIntRange(row.TrustNet, query.minTrustNet, query.maxTrustNet) {
			continue
		}
		if !passesIntRange(row.DealerNet, query.minDealerNet, query.maxDealerNet) {
			continue
		}
		if !passesIntRange(row.InstitutionalNet, query.minInstitutionalNet, query.maxInstitutionalNet) {
			continue
		}
		if !passesIntRange(row.MarginBalance, query.minMarginBalance, query.maxMarginBalance) {
			continue
		}
		if !passesIntRange(row.MarginChange, query.minMarginChange, query.maxMarginChange) {
			continue
		}
		if !passesIntRange(row.ShortBalance, query.minShortBalance, query.maxShortBalance) {
			continue
		}
		if !passesIntRange(row.ShortChange, query.minShortChange, query.maxShortChange) {
			continue
		}
		if !passesRange(row.ShortMarginRatio, query.minShortMarginRatio, query.maxShortMarginRatio) {
			continue
		}
		if !passesIntRange(row.MonthlyRevenue, query.minMonthlyRevenue, query.maxMonthlyRevenue) {
			continue
		}
		if !passesRange(row.RevenueYoY, query.minRevenueYoY, query.maxRevenueYoY) {
			continue
		}
		if !passesRange(row.RevenueMoM, query.minRevenueMoM, query.maxRevenueMoM) {
			continue
		}
		if !passesRange(row.CumulativeRevenueYoY, query.minCumulativeRevenueYoY, query.maxCumulativeRevenueYoY) {
			continue
		}
		if !passesRange(row.PE, query.minPE, query.maxPE) {
			continue
		}
		if !passesRange(row.PB, query.minPB, query.maxPB) {
			continue
		}
		if !passesRange(row.DividendYield, query.minDividendYield, query.maxDividendYield) {
			continue
		}
		if !passesRange(row.CashDividend, query.minCashDividend, query.maxCashDividend) {
			continue
		}
		if !passesRange(row.StockDividend, query.minStockDividend, query.maxStockDividend) {
			continue
		}
		if !passesRange(row.TotalDividend, query.minTotalDividend, query.maxTotalDividend) {
			continue
		}
		if !passesRange(row.CumulativeEPS, query.minCumulativeEPS, query.maxCumulativeEPS) {
			continue
		}
		if !passesRange(row.GrossMargin, query.minGrossMargin, query.maxGrossMargin) {
			continue
		}
		if !passesRange(row.OperatingMargin, query.minOperatingMargin, query.maxOperatingMargin) {
			continue
		}
		if !passesRange(row.NetMargin, query.minNetMargin, query.maxNetMargin) {
			continue
		}
		if !passesRange(row.BookValuePerShare, query.minBookValuePerShare, query.maxBookValuePerShare) {
			continue
		}
		filtered = append(filtered, row)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		vi := taiwanScreenerSortValue(filtered[i], query.sort)
		vj := taiwanScreenerSortValue(filtered[j], query.sort)
		// Unavailable sort values are placed at the end consistently for both asc and desc —
		// a nil value is never treated as "less than everything" or "greater than everything"
		// depending on direction, it simply always sorts after any available value.
		if vi == nil && vj == nil {
			return filtered[i].Canonical < filtered[j].Canonical
		}
		if vi == nil {
			return false
		}
		if vj == nil {
			return true
		}
		if *vi == *vj {
			return filtered[i].Canonical < filtered[j].Canonical
		}
		if query.order == "desc" {
			return *vi > *vj
		}
		return *vi < *vj
	})
	return filtered
}

func toTaiwanScreenerRow(row foundation.TaiwanDailySnapshot, institutional map[string]foundation.InstitutionalFlow, margin map[string]foundation.MarginTrading, revenue map[string]foundation.MonthlyRevenue, valuation map[string]foundation.ValuationSnapshot, dividends map[string]foundation.DividendRecord, financials map[string]foundation.FinancialStatementPeriod, balance map[string]foundation.FinancialStatementPeriod, financialsTargetPeriod *string) taiwanScreenerRow {
	out := taiwanScreenerRow{
		Canonical: row.Canonical, Code: row.Code, Name: row.Name, Exchange: row.Exchange, SecurityType: string(row.Type),
		TradeDate: row.TradeDate, Price: row.Close, Change: row.Change,
		ChangePercent: taiwanScreenerChangePercent(row.Close, row.Change),
		Volume:        row.Volume, Amount: row.Amount,
	}
	// A nil map (domain never fetched) and a present-but-missing canonical (security absent from
	// that date's bulk payload) both correctly leave every field nil below — indexing a nil map is
	// safe in Go and the two-value form still reports ok=false, so this never needs special-casing.
	if flow, ok := institutional[row.Canonical]; ok {
		foreign, trust, dealer := flow.ForeignNet, flow.InvestmentTrustNet, flow.DealerNet
		total := foreign + trust + dealer
		out.ForeignNet, out.TrustNet, out.DealerNet, out.InstitutionalNet = &foreign, &trust, &dealer, &total
	}
	if trading, ok := margin[row.Canonical]; ok {
		// MarginTrading's own fields are already *int64/*float64 (nil-safe at the provider layer),
		// reused directly — never recomputed here.
		out.MarginBalance, out.MarginChange = trading.MarginBalance, trading.MarginChange
		out.ShortBalance, out.ShortChange, out.ShortMarginRatio = trading.ShortBalance, trading.ShortChange, trading.ShortMarginRatio
	}
	// M7E-A — same nil-map/absent-canonical safety as institutional/margin above: a nil map (domain
	// never fetched) and a present-but-missing canonical both correctly leave these fields nil.
	if rev, ok := revenue[row.Canonical]; ok {
		monthlyRevenue := rev.Revenue
		out.MonthlyRevenue, out.RevenueYoY = &monthlyRevenue, rev.OfficialYoY
		// M7F — same official bulk row as monthly_revenue/revenue_yoy above; never a second lookup.
		out.RevenueMoM, out.CumulativeRevenueYoY = rev.OfficialMoM, rev.CumulativeYoY
	}
	if val, ok := valuation[row.Canonical]; ok {
		out.PE, out.PB, out.DividendYield = val.PE, val.PB, val.DividendYield
	}
	if div, ok := dividends[row.Canonical]; ok {
		out.CashDividend, out.StockDividend, out.TotalDividend = div.CashDividend, div.StockDividend, div.TotalDividend
	}
	// M7E-B — financial_period is always set from the security's own parsed statement row (its true
	// reporting period); the three metric fields are copied verbatim from the provider result, which
	// has already nulled them out for any row not on the domain's common target period, and additionally
	// nulled gross/operating margin for any non-"ci" category (see ScreenerFinancials). This handler
	// never re-derives or re-gates those fields itself.
	if fin, ok := financials[row.Canonical]; ok {
		period := fmt.Sprintf("%d-Q%d", fin.FiscalYear, fin.FiscalQuarter)
		out.FinancialPeriod = &period
		out.CumulativeEPS, out.GrossMargin, out.OperatingMargin, out.NetMargin = fin.CumulativeEPS, fin.GrossMargin, fin.OperatingMargin, fin.NetMargin
		// M7E-C — book_value_per_share is exposed only when the security's own balance-sheet row
		// period exactly equals its own income-statement row period (financial_period), AND that
		// period equals the domain's common target period (financialsTargetPeriod, computed the
		// same "YYYY-QN" way by ScreenerFinancials) — never a balance-sheet period alone, and never
		// a mismatched/older balance period substituted in. financial_period itself is always the
		// income row's period; it is never overwritten by the balance row's period.
		if bal, ok := balance[row.Canonical]; ok {
			balancePeriod := fmt.Sprintf("%d-Q%d", bal.FiscalYear, bal.FiscalQuarter)
			if balancePeriod == period && financialsTargetPeriod != nil && period == *financialsTargetPeriod {
				out.BookValuePerShare = bal.BookValuePerShare
			}
		}
	}
	return out
}

// taiwanScreenerChangePercent derives change percent from the official exchange-reported price
// change: TWSE/TPEx daily quote feeds define change (漲跌價差) as close - previous_close, so
// previous_close = close - change and change_percent = change / previous_close * 100. This is
// confirmed by how `parseDailyRows` (snapshot.go) populates Change directly from each exchange's
// own reported price-change column — it is not something this backend derives itself elsewhere.
// A missing close/change, or a non-positive previous_close (e.g. a data anomaly), never fabricates
// a percentage — it returns nil (unavailable), never 0.
func taiwanScreenerChangePercent(closeValue, change *float64) *float64 {
	if closeValue == nil || change == nil {
		return nil
	}
	previousClose := *closeValue - *change
	if previousClose <= 0 {
		return nil
	}
	percent := *change / previousClose * 100
	return &percent
}

func taiwanScreenerSortValue(row taiwanScreenerRow, field string) *float64 {
	switch field {
	case "price":
		return row.Price
	case "change_percent":
		return row.ChangePercent
	case "volume":
		return int64ToFloatPointer(row.Volume)
	case "amount":
		return row.Amount
	case "foreign_net":
		return int64ToFloatPointer(row.ForeignNet)
	case "trust_net":
		return int64ToFloatPointer(row.TrustNet)
	case "dealer_net":
		return int64ToFloatPointer(row.DealerNet)
	case "institutional_net":
		return int64ToFloatPointer(row.InstitutionalNet)
	case "margin_balance":
		return int64ToFloatPointer(row.MarginBalance)
	case "margin_change":
		return int64ToFloatPointer(row.MarginChange)
	case "short_balance":
		return int64ToFloatPointer(row.ShortBalance)
	case "short_change":
		return int64ToFloatPointer(row.ShortChange)
	case "short_margin_ratio":
		return row.ShortMarginRatio
	case "monthly_revenue":
		return int64ToFloatPointer(row.MonthlyRevenue)
	case "revenue_yoy":
		return row.RevenueYoY
	case "revenue_mom":
		return row.RevenueMoM
	case "cumulative_revenue_yoy":
		return row.CumulativeRevenueYoY
	case "pe":
		return row.PE
	case "pb":
		return row.PB
	case "dividend_yield":
		return row.DividendYield
	case "cash_dividend":
		return row.CashDividend
	case "stock_dividend":
		return row.StockDividend
	case "total_dividend":
		return row.TotalDividend
	case "cumulative_eps":
		return row.CumulativeEPS
	case "gross_margin":
		return row.GrossMargin
	case "operating_margin":
		return row.OperatingMargin
	case "net_margin":
		return row.NetMargin
	case "book_value_per_share":
		return row.BookValuePerShare
	}
	return nil
}

func int64ToFloatPointer(value *int64) *float64 {
	if value == nil {
		return nil
	}
	converted := float64(*value)
	return &converted
}

func passesRange(value *float64, min, max *float64) bool {
	if min == nil && max == nil {
		return true
	}
	if value == nil {
		return false
	}
	if min != nil && *value < *min {
		return false
	}
	if max != nil && *value > *max {
		return false
	}
	return true
}

func passesIntRange(value *int64, min, max *float64) bool {
	if min == nil && max == nil {
		return true
	}
	if value == nil {
		return false
	}
	converted := float64(*value)
	return passesRange(&converted, min, max)
}

func paginateTaiwanScreener(rows []taiwanScreenerRow, offset, limit int) []taiwanScreenerRow {
	if offset >= len(rows) {
		return []taiwanScreenerRow{}
	}
	end := offset + limit
	if end > len(rows) {
		end = len(rows)
	}
	return append([]taiwanScreenerRow(nil), rows[offset:end]...)
}
