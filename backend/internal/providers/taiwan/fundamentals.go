package taiwan

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"easy-stock/backend/internal/foundation"
)

type fundSnapshot struct {
	rows      []map[string]string
	expiresAt time.Time
}

type finMindRevenueResponse struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
	Data   []struct {
		Date    string `json:"date"`
		StockID string `json:"stock_id"`
		Revenue int64  `json:"revenue"`
		Year    int    `json:"revenue_year"`
		Month   int    `json:"revenue_month"`
	} `json:"data"`
}

var statementCategories = []string{"ci", "basi", "bd", "fh", "ins", "mim"}

func (c *Client) Fundamentals(ctx context.Context, security foundation.SecurityIdentity, months int) (foundation.TaiwanFundamentals, error) {
	if months < 1 || months > 36 {
		return foundation.TaiwanFundamentals{}, fmt.Errorf("months must be between 1 and 36")
	}
	now := time.Now()
	result := foundation.TaiwanFundamentals{
		Security:     security,
		Capabilities: map[string]foundation.FundamentalCapability{},
		Meta:         foundation.SourceMeta{Source: "taiwan:fundamentals", FetchedAt: now, Status: "official", AvailableFields: []string{"record_level_provenance", "currency:TWD", "published_at:source_dependent", "available_at:source_dependent"}},
	}
	if security.Type == foundation.SecurityTypeETF {
		result.Capabilities["monthly_revenue"] = capability("unsupported", "", "", "", "ETF company revenue is not applicable")
		result.Capabilities["financial_statement"] = capability("unsupported", "", "", "", "ETF company financial statements are not applicable")
	} else {
		revenue, err := c.revenue(ctx, security, months)
		result.Revenue = revenue
		result.Capabilities["monthly_revenue"] = capabilityForRecords(revenue, err)
		statement, err := c.statement(ctx, security)
		if err == nil {
			result.Statement = &statement
			result.Capabilities["financial_statement"] = capability(statement.Status, statement.Provider, statement.Source, statement.SourceURL, "")
		} else {
			result.Capabilities["financial_statement"] = capability("data_insufficient", officialProvider(security), "", "", err.Error())
		}
	}
	valuation, err := c.valuation(ctx, security)
	if err == nil {
		result.Valuation = &valuation
		result.Capabilities["valuation"] = capability(valuation.Status, valuation.Provider, valuation.Source, valuation.SourceURL, "")
	} else {
		result.Capabilities["valuation"] = capability("data_insufficient", officialProvider(security), "", "", err.Error())
	}
	dividends, err := c.dividends(ctx, security)
	result.Dividends = dividends
	if err == nil {
		latest := dividends[0]
		result.Capabilities["dividends"] = capability(latest.Status, latest.Provider, latest.Source, latest.SourceURL, "")
	} else {
		result.Capabilities["dividends"] = capability("data_insufficient", officialProvider(security), "", "", err.Error())
	}
	for _, section := range result.Capabilities {
		if section.Status != "official" && section.Status != "third_party_fallback" {
			result.Meta.Status = "data_insufficient"
			break
		}
	}
	return result, nil
}

func capability(status, provider, source, sourceURL, reason string) foundation.FundamentalCapability {
	return foundation.FundamentalCapability{Status: status, Provider: provider, Source: source, SourceURL: sourceURL, RetrievedAt: time.Now(), Reason: reason}
}

func capabilityForRecords(records []foundation.MonthlyRevenue, err error) foundation.FundamentalCapability {
	if len(records) == 0 {
		reason := "monthly revenue unavailable"
		if err != nil {
			reason = err.Error()
		}
		return capability("data_insufficient", "", "", "", reason)
	}
	latest := records[len(records)-1]
	status, reason := latest.Status, ""
	if err != nil {
		status, reason = "data_insufficient", err.Error()
	}
	return capability(status, latest.Provider, latest.Source, latest.SourceURL, reason)
}

func officialProvider(s foundation.SecurityIdentity) string {
	if s.Exchange == "TPEX" {
		return "TPEx"
	}
	return "TWSE"
}

func (c *Client) revenue(ctx context.Context, s foundation.SecurityIdentity, months int) ([]foundation.MonthlyRevenue, error) {
	path, base := "/opendata/t187ap05_L", c.twseBaseURL
	if s.Exchange == "TPEX" {
		path, base = "/mopsfin_t187ap05_O", c.tpexBaseURL
	}
	officialURL := base + path
	rows, err := c.fundamentalsRows(ctx, officialURL, 24*time.Hour)
	if err != nil {
		return nil, err
	}
	row := findCompany(rows, s.Code)
	if row == nil {
		return nil, fmt.Errorf("official monthly revenue row not found")
	}
	latest, err := parseOfficialRevenue(s, officialURL, row)
	if err != nil {
		return nil, err
	}

	start := time.Now().AddDate(-3, 0, 0).Format("2006-01-01")
	finURL := c.finMindURL + "?dataset=TaiwanStockMonthRevenue&data_id=" + url.QueryEscape(s.Code) + "&start_date=" + start
	var history finMindRevenueResponse
	historyErr := c.getJSON(ctx, finURL, &history)
	if historyErr != nil || history.Status != 200 {
		return []foundation.MonthlyRevenue{latest}, fmt.Errorf("official latest available; FinMind history unavailable")
	}
	byPeriod := map[string]foundation.MonthlyRevenue{}
	for _, item := range history.Data {
		period := fmt.Sprintf("%04d-%02d", item.Year, item.Month)
		byPeriod[period] = foundation.MonthlyRevenue{Canonical: s.Canonical, Code: s.Code, Name: s.Name, Exchange: s.Exchange, Year: item.Year, Month: item.Month, Period: period, Revenue: item.Revenue, Currency: "TWD", Unit: "TWD", RawUnit: "TWD", RawValues: map[string]string{"revenue": strconv.FormatInt(item.Revenue, 10)}, Provider: "FinMind", Source: "FinMind TaiwanStockMonthRevenue", SourceURL: finURL, RetrievedAt: time.Now(), Status: "third_party_fallback"}
	}
	if fallback, exists := byPeriod[latest.Period]; exists && fallback.Revenue != latest.Revenue {
		latest.Discrepancies = append(latest.Discrepancies, fmt.Sprintf("FinMind revenue differs: %d", fallback.Revenue))
	}
	byPeriod[latest.Period] = latest // Official always wins for the duplicate month.
	periods := make([]string, 0, len(byPeriod))
	for period := range byPeriod {
		periods = append(periods, period)
	}
	sort.Strings(periods)
	if len(periods) > months {
		periods = periods[len(periods)-months:]
	}
	result := make([]foundation.MonthlyRevenue, 0, len(periods))
	for _, period := range periods {
		result = append(result, byPeriod[period])
	}
	deriveRevenueGrowth(result)
	return result, nil
}

func deriveRevenueGrowth(data []foundation.MonthlyRevenue) {
	for i := range data {
		if i > 0 {
			previous := data[i-1].Revenue
			data[i].PreviousMonthRevenue = &previous
			data[i].ComputedMoM = percentPointer(data[i].Revenue-previous, previous)
		}
		for j := i - 1; j >= 0; j-- {
			if data[j].Year == data[i].Year-1 && data[j].Month == data[i].Month {
				previous := data[j].Revenue
				data[i].PreviousYearRevenue = &previous
				data[i].ComputedYoY = percentPointer(data[i].Revenue-previous, previous)
				break
			}
		}
	}
}

func parseOfficialRevenue(s foundation.SecurityIdentity, sourceURL string, row map[string]string) (foundation.MonthlyRevenue, error) {
	year, month, err := rocMonth(row["資料年月"])
	if err != nil {
		return foundation.MonthlyRevenue{}, err
	}
	revenue, err := thousandTWD(row["營業收入-當月營收"])
	if err != nil {
		return foundation.MonthlyRevenue{}, fmt.Errorf("monthly revenue: %w", err)
	}
	previousMonth, err := optionalThousandTWD(row["營業收入-上月營收"])
	if err != nil {
		return foundation.MonthlyRevenue{}, fmt.Errorf("previous month revenue: %w", err)
	}
	previousYear, err := optionalThousandTWD(row["營業收入-去年當月營收"])
	if err != nil {
		return foundation.MonthlyRevenue{}, fmt.Errorf("previous year revenue: %w", err)
	}
	cumulative, err := optionalThousandTWD(row["累計營業收入-當月累計營收"])
	if err != nil {
		return foundation.MonthlyRevenue{}, fmt.Errorf("cumulative revenue: %w", err)
	}
	previousCumulative, err := optionalThousandTWD(row["累計營業收入-去年累計營收"])
	if err != nil {
		return foundation.MonthlyRevenue{}, fmt.Errorf("previous cumulative revenue: %w", err)
	}
	officialMoM, err := optionalFloat(row["營業收入-上月比較增減(%)"])
	if err != nil {
		return foundation.MonthlyRevenue{}, fmt.Errorf("official MoM: %w", err)
	}
	officialYoY, err := optionalFloat(row["營業收入-去年同月增減(%)"])
	if err != nil {
		return foundation.MonthlyRevenue{}, fmt.Errorf("official YoY: %w", err)
	}
	cumulativeYoY, err := optionalFloat(row["累計營業收入-前期比較增減(%)"])
	if err != nil {
		return foundation.MonthlyRevenue{}, fmt.Errorf("cumulative YoY: %w", err)
	}
	item := foundation.MonthlyRevenue{Canonical: s.Canonical, Code: s.Code, Name: s.Name, Exchange: s.Exchange, Year: year, Month: month, Period: fmt.Sprintf("%04d-%02d", year, month), Revenue: revenue, PreviousMonthRevenue: previousMonth, PreviousYearRevenue: previousYear, OfficialMoM: officialMoM, OfficialYoY: officialYoY, CumulativeRevenue: cumulative, PreviousYearCumulativeRevenue: previousCumulative, CumulativeYoY: cumulativeYoY, Currency: "TWD", Unit: "TWD", RawUnit: "thousand_TWD", RawValues: map[string]string{"revenue": row["營業收入-當月營收"]}, Provider: officialProvider(s), Source: strings.ToLower(s.Exchange) + ":monthly_revenue", SourceURL: sourceURL, RetrievedAt: time.Now(), Status: "official"}
	if previousMonth != nil {
		item.ComputedMoM = percentPointer(revenue-*previousMonth, *previousMonth)
	}
	if previousYear != nil {
		item.ComputedYoY = percentPointer(revenue-*previousYear, *previousYear)
	}
	if differs(item.OfficialMoM, item.ComputedMoM) {
		item.Discrepancies = append(item.Discrepancies, "MoM differs from official")
	}
	if differs(item.OfficialYoY, item.ComputedYoY) {
		item.Discrepancies = append(item.Discrepancies, "YoY differs from official")
	}
	return item, nil
}

func (c *Client) statement(ctx context.Context, s foundation.SecurityIdentity) (foundation.FinancialStatementPeriod, error) {
	prefix, balancePrefix, base := "/opendata/t187ap06_L_", "/opendata/t187ap07_L_", c.twseBaseURL
	if s.Exchange == "TPEX" {
		prefix, balancePrefix, base = "/mopsfin_t187ap06_O_", "/mopsfin_t187ap07_O_", c.tpexBaseURL
	}
	var income map[string]string
	category, incomeURL := "", ""
	for _, candidate := range statementCategories {
		u := base + prefix + candidate
		rows, err := c.fundamentalsRows(ctx, u, 7*24*time.Hour)
		if err != nil {
			continue
		}
		if row := findCompany(rows, s.Code); row != nil {
			if income != nil {
				return foundation.FinancialStatementPeriod{}, fmt.Errorf("financial category discrepancy: %s and %s", category, candidate)
			}
			income, category, incomeURL = row, candidate, u
		}
	}
	if income == nil {
		return foundation.FinancialStatementPeriod{}, fmt.Errorf("financial category unsupported")
	}
	balanceURL := base + balancePrefix + category
	balances, err := c.fundamentalsRows(ctx, balanceURL, 7*24*time.Hour)
	if err != nil {
		return foundation.FinancialStatementPeriod{}, err
	}
	balance := findCompany(balances, s.Code)
	if balance == nil {
		return foundation.FinancialStatementPeriod{}, fmt.Errorf("matching balance sheet unavailable")
	}
	year, err := rocYear(firstMap(income, "年度", "Year"))
	if err != nil {
		return foundation.FinancialStatementPeriod{}, err
	}
	quarter, err := strconv.Atoi(firstMap(income, "季別", "Season"))
	if err != nil || quarter < 1 || quarter > 4 {
		return foundation.FinancialStatementPeriod{}, fmt.Errorf("invalid fiscal quarter")
	}
	periodEnd := time.Date(year, time.Month(quarter*3)+1, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	item := foundation.FinancialStatementPeriod{Canonical: s.Canonical, Code: s.Code, Exchange: s.Exchange, FiscalYear: year, FiscalQuarter: quarter, PeriodStart: fmt.Sprintf("%04d-01-01", year), PeriodEnd: periodEnd, StatementType: "unknown", AccountingCategory: category, Revision: firstMap(income, "出表日期", "Date"), IsCumulative: true, Currency: "TWD", Unit: "TWD", RawUnit: "thousand_TWD", RawValues: map[string]string{}, Provider: officialProvider(s), Source: strings.ToLower(s.Exchange) + ":financial_statement", SourceURL: incomeURL + " | " + balanceURL, RetrievedAt: time.Now(), Status: "official"}
	for key, target := range map[string]**int64{
		"revenue": &item.Revenue, "gross_profit": &item.GrossProfit, "operating_income": &item.OperatingIncome, "pretax_income": &item.PretaxIncome, "net_income": &item.NetIncome, "net_income_parent": &item.NetIncomeParent,
		"total_assets": &item.TotalAssets, "total_liabilities": &item.TotalLiabilities, "equity": &item.Equity, "equity_parent": &item.EquityParent,
	} {
		var row map[string]string
		var keys []string
		switch key {
		case "revenue":
			row, keys = income, []string{"營業收入", "收益合計", "淨收益"}
		case "gross_profit":
			row, keys = income, []string{"營業毛利（毛損）淨額", "營業毛利（毛損）"}
		case "operating_income":
			row, keys = income, []string{"營業利益（損失）"}
		case "pretax_income":
			row, keys = income, []string{"稅前淨利（淨損）", "繼續營業單位稅前損益"}
		case "net_income":
			row, keys = income, []string{"本期淨利（淨損）", "本期稅後淨利（淨損）"}
		case "net_income_parent":
			row, keys = income, []string{"淨利（淨損）歸屬於母公司業主"}
		case "total_assets":
			row, keys = balance, []string{"資產總計"}
		case "total_liabilities":
			row, keys = balance, []string{"負債總計"}
		case "equity":
			row, keys = balance, []string{"權益總計"}
		case "equity_parent":
			row, keys = balance, []string{"歸屬於母公司業主之權益合計"}
		}
		raw := firstMap(row, keys...)
		value, parseErr := optionalThousandTWD(raw)
		if parseErr != nil {
			return foundation.FinancialStatementPeriod{}, fmt.Errorf("%s: %w", key, parseErr)
		}
		*target, item.RawValues[key] = value, raw
	}
	item.CumulativeEPS, err = optionalFloat(firstMap(income, "基本每股盈餘（元）"))
	if err != nil {
		return foundation.FinancialStatementPeriod{}, fmt.Errorf("cumulative EPS: %w", err)
	}
	item.BookValuePerShare, err = optionalFloat(firstMap(balance, "每股參考淨值"))
	if err != nil {
		return foundation.FinancialStatementPeriod{}, fmt.Errorf("book value per share: %w", err)
	}
	item.RawValues["cumulative_eps"] = firstMap(income, "基本每股盈餘（元）")
	item.GrossMargin, item.OperatingMargin, item.NetMargin = ratio(item.GrossProfit, item.Revenue), ratio(item.OperatingIncome, item.Revenue), ratio(item.NetIncomeParent, item.Revenue)
	return item, nil
}

func (c *Client) valuation(ctx context.Context, s foundation.SecurityIdentity) (foundation.ValuationSnapshot, error) {
	path, base := "/exchangeReport/BWIBBU_ALL", c.twseBaseURL
	if s.Exchange == "TPEX" {
		path, base = "/tpex_mainboard_peratio_analysis", c.tpexBaseURL
	}
	u := base + path
	rows, err := c.fundamentalsRows(ctx, u, 24*time.Hour)
	if err != nil {
		return foundation.ValuationSnapshot{}, err
	}
	row := findCompany(rows, s.Code)
	if row == nil {
		return foundation.ValuationSnapshot{}, fmt.Errorf("official valuation row unavailable")
	}
	return parseOfficialValuation(s, u, row)
}

// parseOfficialValuation parses one official BWIBBU_ALL/tpex_mainboard_peratio_analysis row into a
// ValuationSnapshot. Extracted out of valuation() so the M7E-A Screener bulk reader can parse every
// row of the same already-fetched bulk payload without duplicating this parsing logic.
func parseOfficialValuation(s foundation.SecurityIdentity, sourceURL string, row map[string]string) (foundation.ValuationSnapshot, error) {
	date, err := rocDate(firstMap(row, "Date", "日期"))
	if err != nil {
		return foundation.ValuationSnapshot{}, err
	}
	pe, err := optionalFloat(firstMap(row, "PEratio", "PriceEarningRatio"))
	if err != nil {
		return foundation.ValuationSnapshot{}, fmt.Errorf("PE: %w", err)
	}
	pb, err := optionalFloat(firstMap(row, "PBratio", "PriceBookRatio"))
	if err != nil {
		return foundation.ValuationSnapshot{}, fmt.Errorf("PB: %w", err)
	}
	yield, err := optionalFloat(firstMap(row, "DividendYield", "YieldRatio"))
	if err != nil {
		return foundation.ValuationSnapshot{}, fmt.Errorf("yield: %w", err)
	}
	return foundation.ValuationSnapshot{Canonical: s.Canonical, Exchange: s.Exchange, DataDate: date, PE: pe, PB: pb, DividendYield: yield, Provider: officialProvider(s), Source: strings.ToLower(s.Exchange) + ":valuation", SourceURL: sourceURL, RetrievedAt: time.Now(), Status: "official"}, nil
}

func (c *Client) dividends(ctx context.Context, s foundation.SecurityIdentity) ([]foundation.DividendRecord, error) {
	path, base := "/opendata/t187ap45_L", c.twseBaseURL
	if s.Exchange == "TPEX" {
		path, base = "/mopsfin_t187ap39_O", c.tpexBaseURL
	}
	u := base + path
	rows, err := c.fundamentalsRows(ctx, u, 7*24*time.Hour)
	if err != nil {
		return nil, err
	}
	items := []foundation.DividendRecord{}
	for _, row := range rows {
		if companyCode(row) != s.Code {
			continue
		}
		item, err := parseOfficialDividend(s, u, row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	sortDividendRecordsDesc(items)
	if len(items) > 8 {
		items = items[:8]
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("official dividend rows unavailable")
	}
	return items, nil
}

// parseOfficialDividend parses one official dividend-disclosure row into a DividendRecord.
// Extracted out of dividends() so the M7E-A Screener bulk reader can parse every company's rows
// from the same already-fetched bulk payload without duplicating this parsing logic.
func parseOfficialDividend(s foundation.SecurityIdentity, sourceURL string, row map[string]string) (foundation.DividendRecord, error) {
	year, err := rocYear(row["股利年度"])
	if err != nil {
		return foundation.DividendRecord{}, fmt.Errorf("dividend year: %w", err)
	}
	cash, err := sumNumbers(row, "股東配發-盈餘分配之現金股利(元/股)", "股東配發-法定盈餘公積發放之現金(元/股)", "股東配發-資本公積發放之現金(元/股)", "股東配發內容-盈餘分配之現金股利(元/股)", "股東配發內容-法定盈餘公積、資本公積發放之現金(元/股)")
	if err != nil {
		return foundation.DividendRecord{}, fmt.Errorf("cash dividend: %w", err)
	}
	stock, err := sumNumbers(row, "股東配發-盈餘轉增資配股(元/股)", "股東配發-法定盈餘公積轉增資配股(元/股)", "股東配發-資本公積轉增資配股(元/股)", "股東配發內容-盈餘轉增資配股(元/股)", "股東配發內容-法定盈餘公積、資本公積轉增資配股(元/股)")
	if err != nil {
		return foundation.DividendRecord{}, fmt.Errorf("stock dividend: %w", err)
	}
	var total *float64
	if cash != nil || stock != nil {
		v := value(cash) + value(stock)
		total = &v
	}
	decision := firstMap(row, "董事會（擬議）股利分派日", "董事會決議通過股利分派日")
	if decision != "" {
		var decisionErr error
		decision, decisionErr = rocDate(decision)
		if decisionErr != nil {
			return foundation.DividendRecord{}, fmt.Errorf("dividend decision date: %w", decisionErr)
		}
	}
	rawStatus := firstMap(row, "決議（擬議）進度")
	return foundation.DividendRecord{Canonical: s.Canonical, Year: year, Period: row["股利所屬年(季)度"], CashDividend: cash, StockDividend: stock, TotalDividend: total, DecisionDate: decision, RawStatus: rawStatus, NormalizedStatus: normalizeDividendStatus(rawStatus), Provider: officialProvider(s), Source: strings.ToLower(s.Exchange) + ":dividend", SourceURL: sourceURL, RetrievedAt: time.Now(), Status: "official"}, nil
}

// sortDividendRecordsDesc applies the repository's existing deterministic dividend ordering (latest
// year first, tie-broken by the later decision date) — extracted out of dividends() unchanged so the
// M7E-A Screener bulk reader picks the exact same "latest record" per company, never inventing a new
// publication-order rule.
func sortDividendRecordsDesc(items []foundation.DividendRecord) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Year == items[j].Year {
			return items[i].DecisionDate > items[j].DecisionDate
		}
		return items[i].Year > items[j].Year
	})
}

// ==================================================
// M7E-A — Screener bulk fundamentals readers (revenue / valuation / dividends)
// ==================================================
//
// Fundamentals() above is single-security only and reads exactly one row out of an already
// market-wide bulk payload via findCompany(). The three readers below reuse the exact same official
// endpoint URLs, the exact same fundamentalsRows bulk-fetch/TTL cache, and the exact same row parsers
// (parseOfficialRevenue/parseOfficialValuation/parseOfficialDividend) — they simply iterate every row
// of the bulk payload instead of stopping at one company, joining each row's canonical identity from
// Directory()+identityAllowlist() (never trusting a bare company code alone, and never inferring
// exchange from the code — matching the exact join pattern used by refreshDaily/refreshInstitutional/
// refreshMargin in snapshot.go). Each reader makes exactly 2 upstream requests total (one TWSE + one
// TPEx), regardless of how many securities the market has, and never calls FinMind.
//
// PIT note: none of the three readers assign or expose PublishedAt/AvailableAt (M7E.0 established no
// such timestamp exists in any of these official payloads). Freshness is expressed only as the
// loaded dataset's own period/date identifier (see revenueDomainFreshness/valuationDomainFreshness/
// dividendsDomainFreshness below) — never a fabricated publication claim.

// ScreenerRevenue returns, for every TWSE/TPEx stock security, the latest official monthly revenue
// row (including the official YoY% already carried by that same bulk row) — never FinMind, never
// per-security. ETF company revenue is not applicable, mirroring Fundamentals()'s existing
// "unsupported" capability for ETFs.
func (c *Client) ScreenerRevenue(ctx context.Context, now time.Time) ([]foundation.MonthlyRevenue, foundation.TaiwanFundamentalsDomainFreshness, error) {
	identities, err := c.Directory(ctx)
	if err != nil {
		return nil, foundation.TaiwanFundamentalsDomainFreshness{}, err
	}
	allow := identityAllowlist(identities)
	rows := make([]foundation.MonthlyRevenue, 0, len(identities))
	for _, exchange := range []string{"TWSE", "TPEX"} {
		path, base := "/opendata/t187ap05_L", c.twseBaseURL
		if exchange == "TPEX" {
			path, base = "/mopsfin_t187ap05_O", c.tpexBaseURL
		}
		u := base + path
		raw, fetchErr := c.fundamentalsRows(ctx, u, 24*time.Hour)
		if fetchErr != nil {
			continue
		}
		for _, row := range raw {
			identity, ok := allow[exchange][companyCode(row)]
			if !ok || identity.Type == foundation.SecurityTypeETF {
				continue
			}
			item, parseErr := parseOfficialRevenue(identity, u, row)
			if parseErr != nil {
				continue
			}
			rows = append(rows, item)
		}
	}
	return rows, revenueDomainFreshness(rows), nil
}

// ScreenerValuation returns, for every TWSE/TPEx security present in the official valuation bulk
// payloads, the source-provided PE/PB/dividend-yield snapshot — never recomputed, never per-security.
func (c *Client) ScreenerValuation(ctx context.Context, now time.Time) ([]foundation.ValuationSnapshot, foundation.TaiwanValuationFreshness, error) {
	identities, err := c.Directory(ctx)
	if err != nil {
		return nil, foundation.TaiwanValuationFreshness{}, err
	}
	allow := identityAllowlist(identities)
	rows := make([]foundation.ValuationSnapshot, 0, len(identities))
	for _, exchange := range []string{"TWSE", "TPEX"} {
		path, base := "/exchangeReport/BWIBBU_ALL", c.twseBaseURL
		if exchange == "TPEX" {
			path, base = "/tpex_mainboard_peratio_analysis", c.tpexBaseURL
		}
		u := base + path
		raw, fetchErr := c.fundamentalsRows(ctx, u, 24*time.Hour)
		if fetchErr != nil {
			continue
		}
		for _, row := range raw {
			identity, ok := allow[exchange][companyCode(row)]
			if !ok {
				continue
			}
			item, parseErr := parseOfficialValuation(identity, u, row)
			if parseErr != nil {
				continue
			}
			rows = append(rows, item)
		}
	}
	return rows, c.valuationDomainFreshness(now, rows), nil
}

// ScreenerDividends returns, for every TWSE/TPEx security with at least one official dividend
// disclosure row, only the single latest record per company (using the exact same deterministic
// ordering as dividends()) — never per-security, never a new publication-order rule.
func (c *Client) ScreenerDividends(ctx context.Context, now time.Time) ([]foundation.DividendRecord, foundation.TaiwanFundamentalsDomainFreshness, error) {
	identities, err := c.Directory(ctx)
	if err != nil {
		return nil, foundation.TaiwanFundamentalsDomainFreshness{}, err
	}
	allow := identityAllowlist(identities)
	byCanonical := map[string][]foundation.DividendRecord{}
	for _, exchange := range []string{"TWSE", "TPEX"} {
		path, base := "/opendata/t187ap45_L", c.twseBaseURL
		if exchange == "TPEX" {
			path, base = "/mopsfin_t187ap39_O", c.tpexBaseURL
		}
		u := base + path
		raw, fetchErr := c.fundamentalsRows(ctx, u, 7*24*time.Hour)
		if fetchErr != nil {
			continue
		}
		for _, row := range raw {
			identity, ok := allow[exchange][companyCode(row)]
			if !ok {
				continue
			}
			item, parseErr := parseOfficialDividend(identity, u, row)
			if parseErr != nil {
				continue
			}
			byCanonical[identity.Canonical] = append(byCanonical[identity.Canonical], item)
		}
	}
	rows := make([]foundation.DividendRecord, 0, len(byCanonical))
	for _, items := range byCanonical {
		sortDividendRecordsDesc(items)
		rows = append(rows, items[0])
	}
	return rows, dividendsDomainFreshness(rows), nil
}

// revenueDomainFreshness reports the latest loaded revenue PERIOD across all rows (e.g. "2026-08"),
// never a fabricated calendar publication date. Status is "available"/"unavailable" only — a monthly
// filing cadence has no truthful trading-day days-behind equivalent, so none is computed.
func revenueDomainFreshness(rows []foundation.MonthlyRevenue) foundation.TaiwanFundamentalsDomainFreshness {
	latest := ""
	for _, row := range rows {
		if row.Period > latest {
			latest = row.Period
		}
	}
	if latest == "" {
		return foundation.TaiwanFundamentalsDomainFreshness{Status: "unavailable"}
	}
	return foundation.TaiwanFundamentalsDomainFreshness{AsOf: &latest, Status: "available"}
}

// dividendsDomainFreshness reports the latest loaded dividend YEAR across all rows as a plain
// identifier (e.g. "2025") — never a DecisionDate reinterpreted as a publication timestamp, and never
// a "stale" claim based on elapsed days, since dividend disclosures are inherently irregular/annual.
func dividendsDomainFreshness(rows []foundation.DividendRecord) foundation.TaiwanFundamentalsDomainFreshness {
	latestYear := 0
	for _, row := range rows {
		if row.Year > latestYear {
			latestYear = row.Year
		}
	}
	if latestYear == 0 {
		return foundation.TaiwanFundamentalsDomainFreshness{Status: "unavailable"}
	}
	asOf := strconv.Itoa(latestYear)
	return foundation.TaiwanFundamentalsDomainFreshness{AsOf: &asOf, Status: "available"}
}

// valuationDomainFreshness reports the latest loaded valuation DataDate across all rows, compared
// against the same trading-calendar days-behind math already used for the daily/institutional/margin
// domains (snapshot.go's freshnessStatus/daysBehind) — valuation is published on the same daily
// cadence as the market snapshot, so this comparison is truthful, unlike revenue/dividends above.
func (c *Client) valuationDomainFreshness(now time.Time, rows []foundation.ValuationSnapshot) foundation.TaiwanValuationFreshness {
	latest := ""
	for _, row := range rows {
		if row.DataDate > latest {
			latest = row.DataDate
		}
	}
	if latest == "" {
		return foundation.TaiwanValuationFreshness{Status: "unavailable"}
	}
	target := c.calendar.LatestCompleted(now, marketCutoffHour, marketCutoffMinute)
	return foundation.TaiwanValuationFreshness{AsOf: &latest, Status: freshnessStatus(latest, target), DaysBehind: daysBehind(latest, target, c.calendar)}
}

func normalizeDividendStatus(raw string) string {
	switch {
	case strings.Contains(raw, "股東會") && strings.Contains(raw, "通過"):
		return "shareholder_approved"
	case strings.Contains(raw, "董事會") && (strings.Contains(raw, "決議") || strings.Contains(raw, "通過")):
		return "board_approved"
	case strings.Contains(raw, "擬議"):
		return "board_proposed"
	case strings.Contains(raw, "除息") || strings.Contains(raw, "除權"):
		return "ex_date_announced"
	case strings.Contains(raw, "發放") || strings.Contains(raw, "已付"):
		return "paid"
	default:
		return "unknown"
	}
}

func (c *Client) fundamentalsRows(ctx context.Context, u string, ttl time.Duration) ([]map[string]string, error) {
	c.fundMu.Lock()
	cached, ok := c.fundRows[u]
	c.fundMu.Unlock()
	if ok && time.Now().Before(cached.expiresAt) {
		return cached.rows, nil
	}
	var rows []map[string]string
	if err := c.getJSON(ctx, u, &rows); err != nil {
		return nil, err
	}
	c.fundMu.Lock()
	c.fundRows[u] = fundSnapshot{rows: rows, expiresAt: time.Now().Add(ttl)}
	c.fundMu.Unlock()
	return rows, nil
}

func findCompany(rows []map[string]string, code string) map[string]string {
	for _, row := range rows {
		if companyCode(row) == code {
			return row
		}
	}
	return nil
}
func companyCode(row map[string]string) string {
	return firstMap(row, "公司代號", "SecuritiesCompanyCode", "Code")
}
func firstMap(row map[string]string, keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(row[key]); v != "" {
			return v
		}
	}
	return ""
}
func cleanNumber(raw string) (string, bool) {
	v := strings.ReplaceAll(strings.TrimSpace(raw), ",", "")
	if v == "" || v == "-" || v == "--" || strings.EqualFold(v, "N/A") {
		return "", false
	}
	return v, true
}
func optionalFloat(raw string) (*float64, error) {
	v, ok := cleanNumber(raw)
	if !ok {
		return nil, nil
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return nil, fmt.Errorf("malformed number %q", raw)
	}
	return &n, nil
}
func thousandTWD(raw string) (int64, error) {
	v, ok := cleanNumber(raw)
	if !ok {
		return 0, fmt.Errorf("missing amount")
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("malformed amount %q", raw)
	}
	return int64(math.Round(n * 1000)), nil
}
func optionalThousandTWD(raw string) (*int64, error) {
	if _, ok := cleanNumber(raw); !ok {
		return nil, nil
	}
	v, err := thousandTWD(raw)
	if err != nil {
		return nil, err
	}
	return &v, nil
}
func percentPointer(delta, denominator int64) *float64 {
	if denominator <= 0 {
		return nil
	}
	v := float64(delta) / float64(denominator) * 100
	return &v
}
func ratio(numerator, denominator *int64) *float64 {
	if numerator == nil || denominator == nil || *denominator <= 0 {
		return nil
	}
	v := float64(*numerator) / float64(*denominator) * 100
	return &v
}
func differs(a, b *float64) bool { return a != nil && b != nil && math.Abs(*a-*b) > 0.0001 }
func rocYear(raw string) (int, error) {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("invalid year %q", raw)
	}
	if v < 1911 {
		v += 1911
	}
	return v, nil
}
func rocMonth(raw string) (int, int, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) != 5 && len(raw) != 6 {
		return 0, 0, fmt.Errorf("invalid ROC month %q", raw)
	}
	year, err := rocYear(raw[:len(raw)-2])
	if err != nil {
		return 0, 0, err
	}
	month, err := strconv.Atoi(raw[len(raw)-2:])
	if err != nil || month < 1 || month > 12 {
		return 0, 0, fmt.Errorf("invalid month %q", raw)
	}
	return year, month, nil
}
func rocDate(raw string) (string, error) {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "/", ""))
	if len(raw) != 7 && len(raw) != 8 {
		return "", fmt.Errorf("invalid ROC date %q", raw)
	}
	year, err := rocYear(raw[:len(raw)-4])
	if err != nil {
		return "", err
	}
	month, err := strconv.Atoi(raw[len(raw)-4 : len(raw)-2])
	if err != nil || month < 1 || month > 12 {
		return "", fmt.Errorf("invalid ROC date %q", raw)
	}
	day, err := strconv.Atoi(raw[len(raw)-2:])
	if err != nil || day < 1 || day > 31 {
		return "", fmt.Errorf("invalid ROC date %q", raw)
	}
	return fmt.Sprintf("%04d-%02d-%02d", year, month, day), nil
}
func sumNumbers(row map[string]string, keys ...string) (*float64, error) {
	total, found := 0.0, false
	for _, key := range keys {
		value, err := optionalFloat(row[key])
		if err != nil {
			return nil, err
		}
		if value != nil {
			total += *value
			found = true
		}
	}
	if !found {
		return nil, nil
	}
	return &total, nil
}
func value(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}
