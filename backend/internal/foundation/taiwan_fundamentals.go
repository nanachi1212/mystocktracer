package foundation

import "time"

type FundamentalCapability struct {
	Status      string    `json:"status"`
	Provider    string    `json:"provider,omitempty"`
	Source      string    `json:"source,omitempty"`
	SourceURL   string    `json:"source_url,omitempty"`
	RetrievedAt time.Time `json:"retrieved_at"`
	Reason      string    `json:"reason,omitempty"`
}

type MonthlyRevenue struct {
	Canonical                     string            `json:"canonical"`
	Code                          string            `json:"code"`
	Name                          string            `json:"name"`
	Exchange                      string            `json:"exchange"`
	Year                          int               `json:"year"`
	Month                         int               `json:"month"`
	Period                        string            `json:"period"`
	Revenue                       int64             `json:"revenue"`
	PreviousMonthRevenue          *int64            `json:"previous_month_revenue"`
	PreviousYearRevenue           *int64            `json:"previous_year_revenue"`
	OfficialMoM                   *float64          `json:"official_mom_percent"`
	OfficialYoY                   *float64          `json:"official_yoy_percent"`
	ComputedMoM                   *float64          `json:"computed_mom_percent"`
	ComputedYoY                   *float64          `json:"computed_yoy_percent"`
	CumulativeRevenue             *int64            `json:"cumulative_revenue"`
	PreviousYearCumulativeRevenue *int64            `json:"previous_year_cumulative_revenue"`
	CumulativeYoY                 *float64          `json:"cumulative_yoy_percent"`
	Currency                      string            `json:"currency"`
	Unit                          string            `json:"unit"`
	RawUnit                       string            `json:"raw_unit"`
	RawValues                     map[string]string `json:"raw_values,omitempty"`
	Provider                      string            `json:"provider"`
	Source                        string            `json:"source"`
	SourceURL                     string            `json:"source_url"`
	RetrievedAt                   time.Time         `json:"retrieved_at"`
	PublishedAt                   *time.Time        `json:"published_at"`
	AvailableAt                   *time.Time        `json:"available_at"`
	Status                        string            `json:"status"`
	Discrepancies                 []string          `json:"discrepancies,omitempty"`
}

type FinancialStatementPeriod struct {
	Canonical          string   `json:"canonical"`
	Code               string   `json:"code"`
	Exchange           string   `json:"exchange"`
	FiscalYear         int      `json:"fiscal_year"`
	FiscalQuarter      int      `json:"fiscal_quarter"`
	PeriodStart        string   `json:"period_start"`
	PeriodEnd          string   `json:"period_end"`
	StatementType      string   `json:"statement_type"`
	AccountingCategory string   `json:"accounting_category"`
	Revision           string   `json:"revision,omitempty"`
	IsCumulative       bool     `json:"is_cumulative"`
	Revenue            *int64   `json:"revenue"`
	GrossProfit        *int64   `json:"gross_profit"`
	OperatingIncome    *int64   `json:"operating_income"`
	PretaxIncome       *int64   `json:"pretax_income"`
	NetIncome          *int64   `json:"net_income"`
	NetIncomeParent    *int64   `json:"net_income_attributable_to_parent"`
	CumulativeEPS      *float64 `json:"cumulative_eps"`
	TotalAssets        *int64   `json:"total_assets"`
	TotalLiabilities   *int64   `json:"total_liabilities"`
	Equity             *int64   `json:"equity"`
	EquityParent       *int64   `json:"equity_attributable_to_parent"`
	// M7G — current assets/liabilities feed CurrentRatio below; only populated by the balance-sheet
	// bulk reader (parseBalanceSheetRow), never by the income-statement reader.
	CurrentAssets      *int64   `json:"current_assets"`
	CurrentLiabilities *int64   `json:"current_liabilities"`
	BookValuePerShare  *float64 `json:"book_value_per_share"`
	GrossMargin        *float64 `json:"gross_margin_percent"`
	OperatingMargin    *float64 `json:"operating_margin_percent"`
	NetMargin          *float64 `json:"net_margin_percent"`
	// M7G — debt_ratio/debt_to_equity/current_ratio, derived in the provider layer from the raw
	// balance-sheet amounts above (ci-only policy + independent balance-target-period nulling are both
	// applied by ScreenerBalance, not here — see fundamentals.go).
	DebtRatio    *float64 `json:"debt_ratio_percent"`
	DebtToEquity *float64 `json:"debt_to_equity_percent"`
	CurrentRatio *float64 `json:"current_ratio_percent"`
	// M7H — operating_cash_flow/cash_flow_to_net_income, sourced from MOPS' official quarterly XBRL
	// bulk archive (see providers/taiwan/cashflow_xbrl.go), completely independent of the income-
	// statement/balance-sheet bulk payloads above. ProfitLoss (the ratio's denominator) is deliberately
	// NOT added here — it is provider-internal only (cashflowRow), never a public/shared field.
	OperatingCashFlow   *int64            `json:"operating_cash_flow"`
	CashFlowToNetIncome *float64          `json:"cash_flow_to_net_income"`
	Currency            string            `json:"currency"`
	Unit                string            `json:"unit"`
	RawUnit             string            `json:"raw_unit"`
	RawValues           map[string]string `json:"raw_values,omitempty"`
	Provider            string            `json:"provider"`
	Source              string            `json:"source"`
	SourceURL           string            `json:"source_url"`
	RetrievedAt         time.Time         `json:"retrieved_at"`
	PublishedAt         *time.Time        `json:"published_at"`
	AvailableAt         *time.Time        `json:"available_at"`
	Status              string            `json:"status"`
}

// LatestAvailableStatement applies the cross-project strict rule: an
// announcement becomes usable only after AvailableAt. Later revisions cannot
// rewrite results for an earlier query time.
func LatestAvailableStatement(history []FinancialStatementPeriod, queryAt time.Time) *FinancialStatementPeriod {
	var selected *FinancialStatementPeriod
	for i := range history {
		item := &history[i]
		if item.AvailableAt == nil || !queryAt.After(*item.AvailableAt) {
			continue
		}
		if selected == nil || selected.AvailableAt.Before(*item.AvailableAt) {
			copy := *item
			selected = &copy
		}
	}
	return selected
}

type ValuationSnapshot struct {
	Canonical     string     `json:"canonical"`
	Exchange      string     `json:"exchange"`
	DataDate      string     `json:"data_date"`
	PE            *float64   `json:"pe"`
	PB            *float64   `json:"pb"`
	DividendYield *float64   `json:"dividend_yield_percent"`
	Provider      string     `json:"provider"`
	Source        string     `json:"source"`
	SourceURL     string     `json:"source_url"`
	RetrievedAt   time.Time  `json:"retrieved_at"`
	PublishedAt   *time.Time `json:"published_at"`
	AvailableAt   *time.Time `json:"available_at"`
	Status        string     `json:"status"`
}

type DividendRecord struct {
	Canonical        string     `json:"canonical"`
	Year             int        `json:"year"`
	Period           string     `json:"period,omitempty"`
	CashDividend     *float64   `json:"cash_dividend"`
	StockDividend    *float64   `json:"stock_dividend"`
	TotalDividend    *float64   `json:"total_dividend"`
	DecisionDate     string     `json:"decision_date,omitempty"`
	ExDividendDate   string     `json:"ex_dividend_date,omitempty"`
	PaymentDate      string     `json:"payment_date,omitempty"`
	RawStatus        string     `json:"raw_status,omitempty"`
	NormalizedStatus string     `json:"normalized_status"`
	Provider         string     `json:"provider"`
	Source           string     `json:"source"`
	SourceURL        string     `json:"source_url"`
	RetrievedAt      time.Time  `json:"retrieved_at"`
	PublishedAt      *time.Time `json:"published_at"`
	AvailableAt      *time.Time `json:"available_at"`
	Status           string     `json:"status"`
}

// TaiwanFundamentalsDomainFreshness describes an irregular-cadence fundamentals domain (monthly
// revenue, annual dividends) using only the loaded dataset's own period identifier as AsOf — e.g.
// "2026-08" for a revenue period, or "2025" for a dividend year. This is never a calendar
// publication claim and never a trading-day days-behind count: translating a monthly period or a
// dividend year into daily cadence would be misleading, so Status is simply "available" (at least
// one row was successfully parsed) or "unavailable".
type TaiwanFundamentalsDomainFreshness struct {
	AsOf   *string `json:"as_of"`
	Status string  `json:"status"`
}

// TaiwanValuationFreshness describes the valuation domain (PE/PB/dividend yield), which — unlike
// revenue/dividends — is published on the same daily trading cadence as the market snapshot, so it
// truthfully supports a days-behind count against the latest completed trading day. AsOf is the
// dataset's own DataDate (a trade/as-of date), never a publication timestamp.
type TaiwanValuationFreshness struct {
	AsOf       *string `json:"as_of"`
	Status     string  `json:"status"`
	DaysBehind *int    `json:"days_behind,omitempty"`
}

type TaiwanFundamentals struct {
	Security     SecurityIdentity                 `json:"security"`
	Revenue      []MonthlyRevenue                 `json:"monthly_revenue"`
	Statement    *FinancialStatementPeriod        `json:"financial_statement"`
	Valuation    *ValuationSnapshot               `json:"valuation"`
	Dividends    []DividendRecord                 `json:"dividends"`
	Capabilities map[string]FundamentalCapability `json:"capabilities"`
	Meta         SourceMeta                       `json:"meta"`
}
