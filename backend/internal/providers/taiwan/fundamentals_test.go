package taiwan

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"easy-stock/backend/internal/foundation"
)

func TestFundamentalNumberSemanticsAndROCDates(t *testing.T) {
	zero, err := optionalFloat("0.0")
	if err != nil || zero == nil || *zero != 0 {
		t.Fatalf("explicit zero: %v %v", zero, err)
	}
	missing, err := optionalFloat("N/A")
	if err != nil || missing != nil {
		t.Fatalf("missing: %v %v", missing, err)
	}
	if _, err := optionalFloat("12x3"); err == nil {
		t.Fatal("malformed number must fail")
	}
	if amount, err := thousandTWD("1,234.5"); err != nil || amount != 1_234_500 {
		t.Fatalf("unit normalization=%d err=%v", amount, err)
	}
	if year, month, err := rocMonth("11507"); err != nil || year != 2026 || month != 7 {
		t.Fatalf("ROC month=%d-%d err=%v", year, month, err)
	}
	if date, err := rocDate("115/08/28"); err != nil || date != "2026-08-28" {
		t.Fatalf("ROC date=%q err=%v", date, err)
	}
}

func TestOfficialRevenuePreservesProvenanceZeroAndRejectsMalformed(t *testing.T) {
	s := foundation.SecurityIdentity{Canonical: "2330.TWSE", Code: "2330", Name: "台積電", Exchange: "TWSE"}
	row := map[string]string{"資料年月": "11507", "營業收入-當月營收": "0", "營業收入-上月營收": "100", "營業收入-去年當月營收": "200", "營業收入-上月比較增減(%)": "-100", "營業收入-去年同月增減(%)": "-100", "累計營業收入-當月累計營收": "0", "累計營業收入-去年累計營收": "300", "累計營業收入-前期比較增減(%)": "-100"}
	got, err := parseOfficialRevenue(s, "official", row)
	if err != nil || got.Revenue != 0 || got.Provider != "TWSE" || got.Status != "official" || got.RawUnit != "thousand_TWD" {
		t.Fatalf("revenue=%+v err=%v", got, err)
	}
	row["營業收入-當月營收"] = "abc"
	if _, err := parseOfficialRevenue(s, "official", row); err == nil {
		t.Fatal("malformed revenue must fail")
	}
}

func TestRevenueMergeOfficialWinsAndRecordsDiscrepancy(t *testing.T) {
	var officialCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/opendata/t187ap05_L") {
			officialCalls.Add(1)
			json.NewEncoder(w).Encode([]map[string]string{{"資料年月": "11507", "公司代號": "2330", "公司名稱": "台積電", "營業收入-當月營收": "200", "營業收入-上月營收": "100", "營業收入-去年當月營收": "100", "營業收入-上月比較增減(%)": "100", "營業收入-去年同月增減(%)": "100", "累計營業收入-當月累計營收": "200", "累計營業收入-去年累計營收": "100", "累計營業收入-前期比較增減(%)": "100"}})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"status": 200, "msg": "success", "data": []map[string]any{{"date": "2026-08-01", "stock_id": "2330", "revenue": 150000, "revenue_year": 2026, "revenue_month": 7}, {"date": "2026-07-01", "stock_id": "2330", "revenue": 100000, "revenue_year": 2026, "revenue_month": 6}}})
	}))
	defer server.Close()
	c := NewClient(Config{TWSEBaseURL: server.URL, FinMindURL: server.URL, HTTPClient: server.Client()})
	s := foundation.SecurityIdentity{Canonical: "2330.TWSE", Code: "2330", Name: "台積電", Exchange: "TWSE"}
	data, err := c.revenue(context.Background(), s, 24)
	if err != nil || len(data) != 2 {
		t.Fatalf("data=%+v err=%v", data, err)
	}
	latest := data[len(data)-1]
	if latest.Revenue != 200000 || latest.Provider != "TWSE" || len(latest.Discrepancies) != 1 {
		t.Fatalf("official precedence=%+v", latest)
	}
	if _, err := c.revenue(context.Background(), s, 24); err != nil || officialCalls.Load() != 1 {
		t.Fatalf("cache calls=%d err=%v", officialCalls.Load(), err)
	}
}

func TestFinancialCategoryResolverUsesFinancialHoldingSchema(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		rows := []map[string]string{}
		if strings.HasSuffix(r.URL.Path, "06_L_fh") {
			rows = []map[string]string{{"出表日期": "1150830", "年度": "115", "季別": "2", "公司代號": "2881", "淨收益": "5446990.00", "繼續營業單位稅前損益": "16468262.00", "本期稅後淨利（淨損）": "97780594.00", "淨利（淨損）歸屬於母公司業主": "97391130.00", "基本每股盈餘（元）": "6.67"}}
		}
		if strings.HasSuffix(r.URL.Path, "07_L_fh") {
			rows = []map[string]string{{"年度": "115", "季別": "2", "公司代號": "2881", "資產總計": "100.00", "負債總計": "60.00", "權益總計": "40.00", "歸屬於母公司業主之權益合計": "39.00", "每股參考淨值": "20.5"}}
		}
		json.NewEncoder(w).Encode(rows)
	}))
	defer server.Close()
	c := NewClient(Config{TWSEBaseURL: server.URL, HTTPClient: server.Client()})
	s := foundation.SecurityIdentity{Canonical: "2881.TWSE", Code: "2881", Exchange: "TWSE"}
	got, err := c.statement(context.Background(), s)
	if err != nil || got.AccountingCategory != "fh" || got.StatementType != "unknown" || got.CumulativeEPS == nil || *got.CumulativeEPS != 6.67 || got.TotalAssets == nil || *got.TotalAssets != 100000 {
		t.Fatalf("statement=%+v err=%v", got, err)
	}
	if got.PublishedAt != nil || got.AvailableAt != nil {
		t.Fatalf("unverified dates must remain nil: %+v", got)
	}
}

func TestDividendStatusNormalization(t *testing.T) {
	cases := map[string]string{"董事會決議": "board_approved", "股東會通過": "shareholder_approved", "擬議": "board_proposed", "": "unknown"}
	for raw, want := range cases {
		if got := normalizeDividendStatus(raw); got != want {
			t.Fatalf("%q=%q want %q", raw, got, want)
		}
	}
}

func TestETFFundamentalsSectionsDoNotFailBundle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]string{})
	}))
	defer server.Close()
	c := NewClient(Config{TWSEBaseURL: server.URL, HTTPClient: server.Client()})
	s := foundation.SecurityIdentity{Canonical: "0050.TWSE", Code: "0050", Exchange: "TWSE", Type: foundation.SecurityTypeETF}
	got, err := c.Fundamentals(context.Background(), s, 24)
	if err != nil {
		t.Fatal(err)
	}
	if got.Capabilities["monthly_revenue"].Status != "unsupported" || got.Capabilities["financial_statement"].Status != "unsupported" || got.Capabilities["valuation"].Status != "data_insufficient" {
		t.Fatalf("capabilities=%+v", got.Capabilities)
	}
}

// ==================================================
// M7E-A — Screener bulk fundamentals readers (revenue / valuation / dividends)
// ==================================================

func screenerFundamentalsDirectoryHandler(directoryCalls *atomic.Int32, twseDirectory, tpexDirectory string) func(w http.ResponseWriter, r *http.Request, path string) bool {
	return func(w http.ResponseWriter, r *http.Request, path string) bool {
		switch path {
		case "/opendata/t187ap03_L":
			directoryCalls.Add(1)
			_, _ = w.Write([]byte(twseDirectory))
			return true
		case "/opendata/t187ap47_L":
			_, _ = w.Write([]byte(`[]`))
			return true
		case "/mopsfin_t187ap03_O":
			directoryCalls.Add(1)
			_, _ = w.Write([]byte(tpexDirectory))
			return true
		}
		return false
	}
}

// TestScreenerRevenueUsesOneBulkRequestPerExchangeRegardlessOfRowCount (M7E-A) proves ScreenerRevenue
// makes exactly one TWSE + one TPEx official monthly-revenue request in total, even with several
// hundred securities per exchange — never one request per security, and never FinMind.
func TestScreenerRevenueUsesOneBulkRequestPerExchangeRegardlessOfRowCount(t *testing.T) {
	const rowsPerExchange = 300
	var directoryCalls, revenueCalls, finMindCalls atomic.Int32

	var twseDirectory, tpexDirectory, twseRevenue, tpexRevenue strings.Builder
	twseDirectory.WriteString("[")
	tpexDirectory.WriteString("[")
	twseRevenue.WriteString("[")
	tpexRevenue.WriteString("[")
	for i := 0; i < rowsPerExchange; i++ {
		if i > 0 {
			twseDirectory.WriteString(",")
			tpexDirectory.WriteString(",")
			twseRevenue.WriteString(",")
			tpexRevenue.WriteString(",")
		}
		twseCode := fmt.Sprintf("5%03d", i)
		tpexCode := fmt.Sprintf("6%03d", i)
		fmt.Fprintf(&twseDirectory, `{"公司代號":"%s","公司簡稱":"twse-%d"}`, twseCode, i)
		fmt.Fprintf(&tpexDirectory, `{"SecuritiesCompanyCode":"%s","CompanyAbbreviation":"tpex-%d"}`, tpexCode, i)
		fmt.Fprintf(&twseRevenue, `{"公司代號":"%s","資料年月":"11507","營業收入-當月營收":"%d","營業收入-去年同月增減(%%)":"12.5"}`, twseCode, 1000+i)
		fmt.Fprintf(&tpexRevenue, `{"公司代號":"%s","資料年月":"11507","營業收入-當月營收":"%d","營業收入-去年同月增減(%%)":"-3.2"}`, tpexCode, 2000+i)
	}
	twseDirectory.WriteString("]")
	tpexDirectory.WriteString("]")
	twseRevenue.WriteString("]")
	tpexRevenue.WriteString("]")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if screenerFundamentalsDirectoryHandler(&directoryCalls, twseDirectory.String(), tpexDirectory.String())(w, r, r.URL.Path) {
			return
		}
		switch r.URL.Path {
		case "/opendata/t187ap05_L":
			revenueCalls.Add(1)
			_, _ = w.Write([]byte(twseRevenue.String()))
		case "/mopsfin_t187ap05_O":
			revenueCalls.Add(1)
			_, _ = w.Write([]byte(tpexRevenue.String()))
		case "/api/v4/data":
			finMindCalls.Add(1)
			_, _ = w.Write([]byte(`{"status":200,"msg":"ok","data":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	now := time.Date(2026, 8, 28, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, FinMindURL: server.URL + "/api/v4/data", Now: func() time.Time { return now }})
	rows, freshness, err := client.ScreenerRevenue(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if revenueCalls.Load() != 2 {
		t.Fatalf("expected exactly 2 revenue requests (one per exchange) regardless of %d rows per exchange, got %d", rowsPerExchange, revenueCalls.Load())
	}
	if finMindCalls.Load() != 0 {
		t.Fatalf("ScreenerRevenue must never call FinMind, got %d calls", finMindCalls.Load())
	}
	if len(rows) != rowsPerExchange*2 {
		t.Fatalf("expected %d total revenue rows, got %d", rowsPerExchange*2, len(rows))
	}
	if freshness.Status != "available" || freshness.AsOf == nil || *freshness.AsOf != "2026-07" {
		t.Fatalf("unexpected revenue freshness: %+v", freshness)
	}
	byCanonical := map[string]foundation.MonthlyRevenue{}
	for _, row := range rows {
		byCanonical[row.Canonical] = row
	}
	twse, ok := byCanonical["5000.TWSE"]
	if !ok || twse.Revenue != 1000000 || twse.OfficialYoY == nil || *twse.OfficialYoY != 12.5 {
		t.Fatalf("expected exact canonical 5000.TWSE with revenue=1000000 (TWD, converted from thousand-TWD raw) and official YoY=12.5, got %+v", twse)
	}
	if twse.PublishedAt != nil || twse.AvailableAt != nil {
		t.Fatalf("revenue rows must never carry published_at/available_at: %+v", twse)
	}
	tpex, ok := byCanonical["6000.TPEX"]
	if !ok || tpex.Exchange != "TPEX" {
		t.Fatalf("expected exact canonical 6000.TPEX present, got %+v", byCanonical)
	}
}

// TestScreenerRevenueExcludesETFs confirms the bulk revenue reader mirrors Fundamentals()'s existing
// "unsupported" ETF behavior instead of fabricating a revenue row for a fund.
func TestScreenerRevenueExcludesETFs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/opendata/t187ap03_L":
			_, _ = w.Write([]byte(`[{"公司代號":"2330","公司簡稱":"台積電"}]`))
		case "/opendata/t187ap47_L":
			_, _ = w.Write([]byte(`[{"基金代號":"0050","基金名稱":"ETF"}]`))
		case "/mopsfin_t187ap03_O":
			_, _ = w.Write([]byte(`[]`))
		case "/opendata/t187ap05_L":
			_, _ = w.Write([]byte(`[{"公司代號":"2330","資料年月":"11507","營業收入-當月營收":"100"},{"公司代號":"0050","資料年月":"11507","營業收入-當月營收":"200"}]`))
		case "/mopsfin_t187ap05_O":
			_, _ = w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	now := time.Date(2026, 8, 28, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, Now: func() time.Time { return now }})
	rows, _, err := client.ScreenerRevenue(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Canonical == "0050.TWSE" {
			t.Fatalf("ETF must be excluded from the revenue Screener domain, got %+v", row)
		}
	}
	if len(rows) != 1 || rows[0].Canonical != "2330.TWSE" {
		t.Fatalf("expected only 2330.TWSE, got %+v", rows)
	}
}

// TestScreenerRevenueOneExchangeFailureReportsPartialStatus (M7F) proves that when exactly one of the
// two exchange revenue requests fails, the successful exchange's rows remain fully usable and the
// domain status truthfully reads "partial" — not "available" (M7F.0 found the domain previously had
// no partial state at all, silently reporting "available" whenever any rows existed regardless of
// whether both exchanges actually succeeded).
func TestScreenerRevenueOneExchangeFailureReportsPartialStatus(t *testing.T) {
	twseDirectory := `[{"公司代號":"2330","公司簡稱":"台積電"}]`
	tpexDirectory := `[{"SecuritiesCompanyCode":"6488","CompanyAbbreviation":"環球晶"}]`
	twseRevenue := `[{"公司代號":"2330","資料年月":"11507","營業收入-當月營收":"100"}]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/opendata/t187ap03_L":
			_, _ = w.Write([]byte(twseDirectory))
		case "/opendata/t187ap47_L":
			_, _ = w.Write([]byte(`[]`))
		case "/mopsfin_t187ap03_O":
			_, _ = w.Write([]byte(tpexDirectory))
		case "/opendata/t187ap05_L":
			_, _ = w.Write([]byte(twseRevenue))
		case "/mopsfin_t187ap05_O":
			http.Error(w, "boom", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	now := time.Date(2026, 8, 28, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, Now: func() time.Time { return now }})
	rows, freshness, err := client.ScreenerRevenue(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Canonical != "2330.TWSE" {
		t.Fatalf("TWSE's successful row must remain usable despite TPEx failure, got %+v", rows)
	}
	if freshness.Status != "partial" {
		t.Fatalf("expected partial status (1/2 exchanges succeeded), got %+v", freshness)
	}
}

// TestScreenerRevenueInverseExchangeFailureReportsPartialStatus (M7F) proves the symmetric case: TWSE
// fails, TPEx succeeds — TPEx rows remain usable, status is truthfully "partial".
func TestScreenerRevenueInverseExchangeFailureReportsPartialStatus(t *testing.T) {
	twseDirectory := `[{"公司代號":"2330","公司簡稱":"台積電"}]`
	tpexDirectory := `[{"SecuritiesCompanyCode":"6488","CompanyAbbreviation":"環球晶"}]`
	tpexRevenue := `[{"公司代號":"6488","資料年月":"11507","營業收入-當月營收":"100"}]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/opendata/t187ap03_L":
			_, _ = w.Write([]byte(twseDirectory))
		case "/opendata/t187ap47_L":
			_, _ = w.Write([]byte(`[]`))
		case "/mopsfin_t187ap03_O":
			_, _ = w.Write([]byte(tpexDirectory))
		case "/opendata/t187ap05_L":
			http.Error(w, "boom", http.StatusInternalServerError)
		case "/mopsfin_t187ap05_O":
			_, _ = w.Write([]byte(tpexRevenue))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	now := time.Date(2026, 8, 28, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, Now: func() time.Time { return now }})
	rows, freshness, err := client.ScreenerRevenue(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Canonical != "6488.TPEX" {
		t.Fatalf("TPEx's successful row must remain usable despite TWSE failure, got %+v", rows)
	}
	if freshness.Status != "partial" {
		t.Fatalf("expected partial status (1/2 exchanges succeeded), got %+v", freshness)
	}
}

// TestScreenerRevenueBothExchangesFailYieldsUnavailable (M7F) proves that when both exchange requests
// fail, the domain truthfully reports unavailable with zero rows — never a fabricated partial result.
func TestScreenerRevenueBothExchangesFailYieldsUnavailable(t *testing.T) {
	twseDirectory := `[{"公司代號":"2330","公司簡稱":"台積電"}]`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/opendata/t187ap03_L":
			_, _ = w.Write([]byte(twseDirectory))
		case "/opendata/t187ap47_L":
			_, _ = w.Write([]byte(`[]`))
		case "/mopsfin_t187ap03_O":
			_, _ = w.Write([]byte(`[]`))
		case "/opendata/t187ap05_L", "/mopsfin_t187ap05_O":
			http.Error(w, "boom", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	now := time.Date(2026, 8, 28, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, Now: func() time.Time { return now }})
	rows, freshness, err := client.ScreenerRevenue(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected zero rows when both exchanges fail, got %+v", rows)
	}
	if freshness.Status != "unavailable" {
		t.Fatalf("expected unavailable status, got %+v", freshness)
	}
}

// TestScreenerRevenueMalformedMoMOrCumulativeYoYIsolatesFieldNotWholeRow (M7F) proves that a malformed
// 營業收入-上月比較增減(%) or 累計營業收入-前期比較增減(%) value never fails the whole row — the row's
// other valid fields (including monthly_revenue and official YoY) remain usable, and only the
// malformed percentage field itself becomes nil. Before M7F, parseOfficialRevenue treated a malformed
// value for either field as a fatal row-level error (via optionalFloat's returned error propagating up
// and the caller's `continue`), silently dropping the entire security from the Screener domain.
func TestScreenerRevenueMalformedMoMOrCumulativeYoYIsolatesFieldNotWholeRow(t *testing.T) {
	twseDirectory := `[{"公司代號":"1001","公司簡稱":"A"},{"公司代號":"1002","公司簡稱":"B"}]`
	twseRevenue := `[
		{"公司代號":"1001","資料年月":"11507","營業收入-當月營收":"100","營業收入-去年同月增減(%)":"12.5","營業收入-上月比較增減(%)":"garbage","累計營業收入-前期比較增減(%)":"garbage"},
		{"公司代號":"1002","資料年月":"11507","營業收入-當月營收":"200","營業收入-去年同月增減(%)":"5.0","營業收入-上月比較增減(%)":"7.5","累計營業收入-前期比較增減(%)":"9.9"}
	]`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/opendata/t187ap03_L":
			_, _ = w.Write([]byte(twseDirectory))
		case "/opendata/t187ap47_L":
			_, _ = w.Write([]byte(`[]`))
		case "/mopsfin_t187ap03_O":
			_, _ = w.Write([]byte(`[]`))
		case "/opendata/t187ap05_L":
			_, _ = w.Write([]byte(twseRevenue))
		case "/mopsfin_t187ap05_O":
			_, _ = w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	now := time.Date(2026, 8, 28, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, Now: func() time.Time { return now }})
	rows, _, err := client.ScreenerRevenue(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	byCanonical := map[string]foundation.MonthlyRevenue{}
	for _, row := range rows {
		byCanonical[row.Canonical] = row
	}
	malformed, ok := byCanonical["1001.TWSE"]
	if !ok {
		t.Fatalf("row with malformed MoM/cumulative-YoY must still be present (row isolation), got %+v", byCanonical)
	}
	if malformed.Revenue != 100000 || malformed.OfficialYoY == nil || *malformed.OfficialYoY != 12.5 {
		t.Fatalf("row's other valid fields must remain usable despite malformed MoM/cumulative-YoY, got %+v", malformed)
	}
	if malformed.OfficialMoM != nil {
		t.Fatalf("malformed official MoM must become nil, not a fatal error, got %+v", malformed.OfficialMoM)
	}
	if malformed.CumulativeYoY != nil {
		t.Fatalf("malformed cumulative YoY must become nil, not a fatal error, got %+v", malformed.CumulativeYoY)
	}
	valid, ok := byCanonical["1002.TWSE"]
	if !ok || valid.OfficialMoM == nil || *valid.OfficialMoM != 7.5 || valid.CumulativeYoY == nil || *valid.CumulativeYoY != 9.9 {
		t.Fatalf("well-formed row must be unaffected, got %+v", valid)
	}
}

// TestScreenerValuationUsesOneBulkRequestPerExchangeRegardlessOfRowCount (M7E-A) proves
// ScreenerValuation makes exactly one TWSE + one TPEx official valuation request in total.
func TestScreenerValuationUsesOneBulkRequestPerExchangeRegardlessOfRowCount(t *testing.T) {
	const rowsPerExchange = 300
	var directoryCalls, valuationCalls atomic.Int32

	var twseDirectory, tpexDirectory, twseVal, tpexVal strings.Builder
	twseDirectory.WriteString("[")
	tpexDirectory.WriteString("[")
	twseVal.WriteString("[")
	tpexVal.WriteString("[")
	for i := 0; i < rowsPerExchange; i++ {
		if i > 0 {
			twseDirectory.WriteString(",")
			tpexDirectory.WriteString(",")
			twseVal.WriteString(",")
			tpexVal.WriteString(",")
		}
		twseCode := fmt.Sprintf("7%03d", i)
		tpexCode := fmt.Sprintf("8%03d", i)
		fmt.Fprintf(&twseDirectory, `{"公司代號":"%s","公司簡稱":"twse-%d"}`, twseCode, i)
		fmt.Fprintf(&tpexDirectory, `{"SecuritiesCompanyCode":"%s","CompanyAbbreviation":"tpex-%d"}`, tpexCode, i)
		fmt.Fprintf(&twseVal, `{"Code":"%s","Date":"1150828","PEratio":"15.5","PBratio":"2.1","DividendYield":"3.0"}`, twseCode)
		fmt.Fprintf(&tpexVal, `{"Code":"%s","Date":"1150827","PEratio":"20.0","PBratio":"1.5","DividendYield":"2.0"}`, tpexCode)
	}
	twseDirectory.WriteString("]")
	tpexDirectory.WriteString("]")
	twseVal.WriteString("]")
	tpexVal.WriteString("]")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if screenerFundamentalsDirectoryHandler(&directoryCalls, twseDirectory.String(), tpexDirectory.String())(w, r, r.URL.Path) {
			return
		}
		switch r.URL.Path {
		case "/exchangeReport/BWIBBU_ALL":
			valuationCalls.Add(1)
			_, _ = w.Write([]byte(twseVal.String()))
		case "/tpex_mainboard_peratio_analysis":
			valuationCalls.Add(1)
			_, _ = w.Write([]byte(tpexVal.String()))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	now := time.Date(2026, 8, 28, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, Now: func() time.Time { return now }})
	rows, freshness, err := client.ScreenerValuation(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if valuationCalls.Load() != 2 {
		t.Fatalf("expected exactly 2 valuation requests (one per exchange) regardless of %d rows per exchange, got %d", rowsPerExchange, valuationCalls.Load())
	}
	if len(rows) != rowsPerExchange*2 {
		t.Fatalf("expected %d total valuation rows, got %d", rowsPerExchange*2, len(rows))
	}
	if freshness.AsOf == nil || *freshness.AsOf != "2026-08-28" {
		t.Fatalf("expected valuation as_of=2026-08-28 (latest DataDate across rows), got %+v", freshness)
	}
	byCanonical := map[string]foundation.ValuationSnapshot{}
	for _, row := range rows {
		byCanonical[row.Canonical] = row
	}
	twse, ok := byCanonical["7000.TWSE"]
	if !ok || twse.PE == nil || *twse.PE != 15.5 || twse.PublishedAt != nil || twse.AvailableAt != nil {
		t.Fatalf("expected exact canonical 7000.TWSE, source-provided PE, no published_at/available_at: %+v", twse)
	}
	if _, ok := byCanonical["8000.TPEX"]; !ok {
		t.Fatalf("expected exact canonical 8000.TPEX present")
	}
}

// TestScreenerDividendsUsesOneBulkRequestPerExchangeRegardlessOfRowCount (M7E-A) proves
// ScreenerDividends makes exactly one TWSE + one TPEx official dividend request in total, and that
// the multi-year-per-company payload is reduced to exactly one (the latest) record per canonical.
func TestScreenerDividendsUsesOneBulkRequestPerExchangeRegardlessOfRowCount(t *testing.T) {
	const rowsPerExchange = 300
	var directoryCalls, dividendCalls atomic.Int32

	var twseDirectory, tpexDirectory, twseDiv, tpexDiv strings.Builder
	twseDirectory.WriteString("[")
	tpexDirectory.WriteString("[")
	twseDiv.WriteString("[")
	tpexDiv.WriteString("[")
	for i := 0; i < rowsPerExchange; i++ {
		if i > 0 {
			twseDirectory.WriteString(",")
			tpexDirectory.WriteString(",")
		} else {
			// keep separators aligned with the two dividend rows written per company below
		}
		twseCode := fmt.Sprintf("9%03d", i)
		tpexCode := fmt.Sprintf("A%03d", i)
		fmt.Fprintf(&twseDirectory, `{"公司代號":"%s","公司簡稱":"twse-%d"}`, twseCode, i)
		fmt.Fprintf(&tpexDirectory, `{"SecuritiesCompanyCode":"%s","CompanyAbbreviation":"tpex-%d"}`, tpexCode, i)
		if i > 0 {
			twseDiv.WriteString(",")
			tpexDiv.WriteString(",")
		}
		// two years per company: 113 (older) and 114 (latest) — only 114's cash dividend must win.
		fmt.Fprintf(&twseDiv, `{"公司代號":"%s","股利年度":"113","股東配發-盈餘分配之現金股利(元/股)":"1.0"},{"公司代號":"%s","股利年度":"114","股東配發-盈餘分配之現金股利(元/股)":"2.0"}`, twseCode, twseCode)
		fmt.Fprintf(&tpexDiv, `{"公司代號":"%s","股利年度":"113","股東配發-盈餘分配之現金股利(元/股)":"0.5"},{"公司代號":"%s","股利年度":"114","股東配發-盈餘分配之現金股利(元/股)":"1.5"}`, tpexCode, tpexCode)
	}
	twseDirectory.WriteString("]")
	tpexDirectory.WriteString("]")
	twseDiv.WriteString("]")
	tpexDiv.WriteString("]")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if screenerFundamentalsDirectoryHandler(&directoryCalls, twseDirectory.String(), tpexDirectory.String())(w, r, r.URL.Path) {
			return
		}
		switch r.URL.Path {
		case "/opendata/t187ap45_L":
			dividendCalls.Add(1)
			_, _ = w.Write([]byte(twseDiv.String()))
		case "/mopsfin_t187ap39_O":
			dividendCalls.Add(1)
			_, _ = w.Write([]byte(tpexDiv.String()))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	now := time.Date(2026, 8, 28, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, Now: func() time.Time { return now }})
	rows, freshness, err := client.ScreenerDividends(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if dividendCalls.Load() != 2 {
		t.Fatalf("expected exactly 2 dividend requests (one per exchange) regardless of %d rows per exchange, got %d", rowsPerExchange, dividendCalls.Load())
	}
	if len(rows) != rowsPerExchange*2 {
		t.Fatalf("expected exactly one (latest) record per canonical: %d companies, got %d rows", rowsPerExchange*2, len(rows))
	}
	if freshness.Status != "available" || freshness.AsOf == nil || *freshness.AsOf != "2025" {
		t.Fatalf("unexpected dividends freshness (expected latest year 114->2025): %+v", freshness)
	}
	byCanonical := map[string]foundation.DividendRecord{}
	for _, row := range rows {
		byCanonical[row.Canonical] = row
	}
	twse, ok := byCanonical["9000.TWSE"]
	if !ok || twse.Year != 2025 || twse.CashDividend == nil || *twse.CashDividend != 2.0 {
		t.Fatalf("expected 9000.TWSE latest record (year 2025, cash=2.0), got %+v", twse)
	}
	if twse.PublishedAt != nil || twse.AvailableAt != nil {
		t.Fatalf("dividend rows must never carry published_at/available_at: %+v", twse)
	}
}

// ==================================================
// M7E-B — Screener bulk financial-statement reader (cumulative EPS / gross margin / operating margin)
// ==================================================

func financialsCategoryPath(exchange, category string) string {
	if exchange == "TPEX" {
		return "/mopsfin_t187ap06_O_" + category
	}
	return "/opendata/t187ap06_L_" + category
}

// defaultFinancialsBodies seeds all 12 category/exchange income-statement endpoints with an empty
// (but successful) JSON array — tests override only the categories they care about.
func defaultFinancialsBodies() map[string]string {
	bodies := map[string]string{}
	for _, exchange := range []string{"TWSE", "TPEX"} {
		for _, category := range statementCategories {
			bodies[financialsCategoryPath(exchange, category)] = "[]"
		}
	}
	return bodies
}

// newFinancialsServer serves the Taiwan directory plus the 12 income-statement category endpoints
// from `bodies` (URL path -> JSON body; an empty-string body simulates a failed/500 category
// request rather than an empty array). It fails the test immediately if a balance-sheet or FinMind
// request is ever made — ScreenerFinancials must be income-statement only.
func newFinancialsServer(t *testing.T, twseDirectory, tpexDirectory string, bodies map[string]string, calls *atomic.Int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/opendata/t187ap03_L":
			_, _ = w.Write([]byte(twseDirectory))
			return
		case "/opendata/t187ap47_L":
			_, _ = w.Write([]byte(`[]`))
			return
		case "/mopsfin_t187ap03_O":
			_, _ = w.Write([]byte(tpexDirectory))
			return
		}
		if strings.Contains(r.URL.Path, "t187ap07_") {
			t.Errorf("ScreenerFinancials must never request a balance-sheet endpoint, got %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("dataset") != "" {
			t.Errorf("ScreenerFinancials must never call FinMind, got %s", r.URL.String())
			http.NotFound(w, r)
			return
		}
		body, ok := bodies[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if calls != nil {
			calls.Add(1)
		}
		if body == "" {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}

// TestScreenerFinancialsUsesTwelveBulkRequestsRegardlessOfRowCount proves ScreenerFinancials makes
// exactly 12 requests total (6 categories x 2 exchanges), even with 300+ securities per exchange, and
// that a second call within the existing 7-day URL cache adds zero new requests.
func TestScreenerFinancialsUsesTwelveBulkRequestsRegardlessOfRowCount(t *testing.T) {
	const rowsPerExchange = 300
	var calls atomic.Int32

	var twseDirectory, tpexDirectory, twseCI, tpexCI strings.Builder
	twseDirectory.WriteString("[")
	tpexDirectory.WriteString("[")
	twseCI.WriteString("[")
	tpexCI.WriteString("[")
	for i := 0; i < rowsPerExchange; i++ {
		if i > 0 {
			twseDirectory.WriteString(",")
			tpexDirectory.WriteString(",")
			twseCI.WriteString(",")
			tpexCI.WriteString(",")
		}
		twseCode := fmt.Sprintf("7%03d", i)
		tpexCode := fmt.Sprintf("8%03d", i)
		fmt.Fprintf(&twseDirectory, `{"公司代號":"%s","公司簡稱":"twse-%d"}`, twseCode, i)
		fmt.Fprintf(&tpexDirectory, `{"SecuritiesCompanyCode":"%s","CompanyAbbreviation":"tpex-%d"}`, tpexCode, i)
		fmt.Fprintf(&twseCI, `{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"%s","營業收入":"1000","營業毛利（毛損）淨額":"400","營業利益（損失）":"300","淨利（淨損）歸屬於母公司業主":"200","基本每股盈餘（元）":"1.50"}`, twseCode)
		fmt.Fprintf(&tpexCI, `{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"%s","營業收入":"1000","營業毛利（毛損）淨額":"400","營業利益（損失）":"300","淨利（淨損）歸屬於母公司業主":"200","基本每股盈餘（元）":"1.50"}`, tpexCode)
	}
	twseDirectory.WriteString("]")
	tpexDirectory.WriteString("]")
	twseCI.WriteString("]")
	tpexCI.WriteString("]")

	bodies := defaultFinancialsBodies()
	bodies[financialsCategoryPath("TWSE", "ci")] = twseCI.String()
	bodies[financialsCategoryPath("TPEX", "ci")] = tpexCI.String()

	server := newFinancialsServer(t, twseDirectory.String(), tpexDirectory.String(), bodies, &calls)
	defer server.Close()

	now := time.Date(2026, 8, 28, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, Now: func() time.Time { return now }})
	rows, freshness, err := client.ScreenerFinancials(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 12 {
		t.Fatalf("expected exactly 12 requests (6 categories x 2 exchanges) regardless of %d rows per exchange, got %d", rowsPerExchange, calls.Load())
	}
	if len(rows) != rowsPerExchange*2 {
		t.Fatalf("expected %d total rows, got %d", rowsPerExchange*2, len(rows))
	}
	if freshness.Status != "available" || freshness.AsOf == nil || *freshness.AsOf != "2026-Q2" {
		t.Fatalf("unexpected freshness: %+v", freshness)
	}
	for _, row := range rows {
		if row.CumulativeEPS == nil || *row.CumulativeEPS != 1.5 {
			t.Fatalf("expected cumulative_eps=1.5, got %+v", row)
		}
		if row.GrossMargin == nil || *row.GrossMargin != 40 || row.OperatingMargin == nil || *row.OperatingMargin != 30 {
			t.Fatalf("expected gross_margin=40 operating_margin=30 for ci category, got %+v", row)
		}
	}

	// Warm cache: a second call within the 7-day TTL must add zero new requests.
	callsBefore := calls.Load()
	rows2, _, err := client.ScreenerFinancials(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != callsBefore {
		t.Fatalf("warm cache call added %d new requests, want 0", calls.Load()-callsBefore)
	}
	if len(rows2) != len(rows) {
		t.Fatalf("warm cache row count mismatch: %d vs %d", len(rows2), len(rows))
	}
}

// TestScreenerFinancialsMixedPeriodGateOlderQuarterMetricsNulled (M7E-B.1) proves that a security
// still on an older fiscal quarter than the market-wide target keeps its own true financial_period
// (FiscalYear/FiscalQuarter) but has all three Screener metric fields nulled — a cumulative Q1 value
// must never be compared against a cumulative Q2 value.
func TestScreenerFinancialsMixedPeriodGateOlderQuarterMetricsNulled(t *testing.T) {
	var calls atomic.Int32
	twseDirectory := `[{"公司代號":"1001","公司簡稱":"A"},{"公司代號":"1002","公司簡稱":"B"}]`
	twseCI := `[{"出表日期":"1150905","年度":"115","季別":"1","公司代號":"1001","營業收入":"1000","營業毛利（毛損）淨額":"400","營業利益（損失）":"300","淨利（淨損）歸屬於母公司業主":"200","基本每股盈餘（元）":"1.00"},{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"1002","營業收入":"2000","營業毛利（毛損）淨額":"800","營業利益（損失）":"600","淨利（淨損）歸屬於母公司業主":"400","基本每股盈餘（元）":"2.00"}]`
	bodies := defaultFinancialsBodies()
	bodies[financialsCategoryPath("TWSE", "ci")] = twseCI
	server := newFinancialsServer(t, twseDirectory, `[]`, bodies, &calls)
	defer server.Close()
	now := time.Date(2026, 8, 28, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, Now: func() time.Time { return now }})
	rows, freshness, err := client.ScreenerFinancials(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if freshness.AsOf == nil || *freshness.AsOf != "2026-Q2" {
		t.Fatalf("expected target period 2026-Q2 (the max observed), got %+v", freshness)
	}
	byCode := map[string]foundation.FinancialStatementPeriod{}
	for _, row := range rows {
		byCode[row.Code] = row
	}
	q1 := byCode["1001"]
	if q1.FiscalYear != 2026 || q1.FiscalQuarter != 1 {
		t.Fatalf("1001 must keep its own true period 2026-Q1, got %+v", q1)
	}
	if q1.CumulativeEPS != nil || q1.GrossMargin != nil || q1.OperatingMargin != nil {
		t.Fatalf("older-period row (Q1, target is Q2) must have nulled metrics, got %+v", q1)
	}
	q2 := byCode["1002"]
	if q2.FiscalYear != 2026 || q2.FiscalQuarter != 2 {
		t.Fatalf("1002 period wrong: %+v", q2)
	}
	if q2.CumulativeEPS == nil || *q2.CumulativeEPS != 2.0 || q2.GrossMargin == nil || q2.OperatingMargin == nil {
		t.Fatalf("target-period row (Q2) must keep all metrics, got %+v", q2)
	}
}

// TestScreenerFinancialsYearTransitionUsesTupleComparisonNotLexical proves the target period is
// chosen by real (year, quarter) integer comparison, verified across a fiscal-year boundary.
func TestScreenerFinancialsYearTransitionUsesTupleComparisonNotLexical(t *testing.T) {
	var calls atomic.Int32
	twseDirectory := `[{"公司代號":"2001","公司簡稱":"A"},{"公司代號":"2002","公司簡稱":"B"}]`
	twseCI := `[{"出表日期":"1160305","年度":"115","季別":"4","公司代號":"2001","營業收入":"1000","營業毛利（毛損）淨額":"400","營業利益（損失）":"300","淨利（淨損）歸屬於母公司業主":"200","基本每股盈餘（元）":"1.00"},{"出表日期":"1160305","年度":"116","季別":"1","公司代號":"2002","營業收入":"2000","營業毛利（毛損）淨額":"800","營業利益（損失）":"600","淨利（淨損）歸屬於母公司業主":"400","基本每股盈餘（元）":"2.00"}]`
	bodies := defaultFinancialsBodies()
	bodies[financialsCategoryPath("TWSE", "ci")] = twseCI
	server := newFinancialsServer(t, twseDirectory, `[]`, bodies, &calls)
	defer server.Close()
	now := time.Date(2027, 3, 5, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, Now: func() time.Time { return now }})
	rows, freshness, err := client.ScreenerFinancials(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if freshness.AsOf == nil || *freshness.AsOf != "2027-Q1" {
		t.Fatalf("expected target 2027-Q1 across the fiscal-year boundary, got %+v", freshness)
	}
	byCode := map[string]foundation.FinancialStatementPeriod{}
	for _, row := range rows {
		byCode[row.Code] = row
	}
	if byCode["2001"].CumulativeEPS != nil {
		t.Fatalf("2026-Q4 row must be nulled once 2027-Q1 is the target, got %+v", byCode["2001"])
	}
	if byCode["2002"].CumulativeEPS == nil {
		t.Fatalf("2027-Q1 row (the target) must keep its EPS, got %+v", byCode["2002"])
	}
}

// TestScreenerFinancialsInvalidQuarterSkipsRowNotWholeDomain proves a single malformed row (invalid
// fiscal quarter) is skipped without failing the rest of the category/domain.
func TestScreenerFinancialsInvalidQuarterSkipsRowNotWholeDomain(t *testing.T) {
	var calls atomic.Int32
	twseDirectory := `[{"公司代號":"3001","公司簡稱":"A"},{"公司代號":"3002","公司簡稱":"B"}]`
	twseCI := `[{"出表日期":"1150905","年度":"115","季別":"9","公司代號":"3001","營業收入":"1000","營業毛利（毛損）淨額":"400","營業利益（損失）":"300","淨利（淨損）歸屬於母公司業主":"200","基本每股盈餘（元）":"1.00"},{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"3002","營業收入":"2000","營業毛利（毛損）淨額":"800","營業利益（損失）":"600","淨利（淨損）歸屬於母公司業主":"400","基本每股盈餘（元）":"2.00"}]`
	bodies := defaultFinancialsBodies()
	bodies[financialsCategoryPath("TWSE", "ci")] = twseCI
	server := newFinancialsServer(t, twseDirectory, `[]`, bodies, &calls)
	defer server.Close()
	now := time.Date(2026, 8, 28, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, Now: func() time.Time { return now }})
	rows, _, err := client.ScreenerFinancials(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Code != "3002" {
		t.Fatalf("expected only the valid row (invalid quarter=9 skipped), got %+v", rows)
	}
}

// TestScreenerFinancialsCumulativeEPSAllCategoriesMarginsCIOnly (M7E-B.1) proves cumulative_eps is
// available for every financial category, but gross_margin/operating_margin are nil for every
// category except "ci" -- explicitly including "ins" (insurance), whose payload happens to carry
// both 營業收入 and 營業利益（損失）, to prove the ci-only policy is actively enforced, not merely a
// side effect of missing source fields.
func TestScreenerFinancialsCumulativeEPSAllCategoriesMarginsCIOnly(t *testing.T) {
	var calls atomic.Int32
	directory := `[{"公司代號":"4001","公司簡稱":"CI"},{"公司代號":"4002","公司簡稱":"BASI"},{"公司代號":"4003","公司簡稱":"BD"},{"公司代號":"4004","公司簡稱":"FH"},{"公司代號":"4005","公司簡稱":"INS"},{"公司代號":"4006","公司簡稱":"MIM"}]`
	ci := `[{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"4001","營業收入":"1000","營業毛利（毛損）淨額":"400","營業利益（損失）":"300","淨利（淨損）歸屬於母公司業主":"200","基本每股盈餘（元）":"1.11"}]`
	basi := `[{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"4002","淨利（淨損）歸屬於母公司業主":"200","基本每股盈餘（元）":"2.22"}]`
	bd := `[{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"4003","淨利（淨損）歸屬於母公司業主":"200","基本每股盈餘（元）":"3.33"}]`
	fh := `[{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"4004","淨收益":"999","淨利（淨損）歸屬於母公司業主":"200","基本每股盈餘（元）":"4.44"}]`
	ins := `[{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"4005","營業收入":"5000","營業利益（損失）":"1000","淨利（淨損）歸屬於母公司業主":"200","基本每股盈餘（元）":"5.55"}]`
	mim := `[{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"4006","淨利（淨損）歸屬於母公司業主":"200","基本每股盈餘（元）":"6.66"}]`

	bodies := defaultFinancialsBodies()
	bodies[financialsCategoryPath("TWSE", "ci")] = ci
	bodies[financialsCategoryPath("TWSE", "basi")] = basi
	bodies[financialsCategoryPath("TWSE", "bd")] = bd
	bodies[financialsCategoryPath("TWSE", "fh")] = fh
	bodies[financialsCategoryPath("TWSE", "ins")] = ins
	bodies[financialsCategoryPath("TWSE", "mim")] = mim

	server := newFinancialsServer(t, directory, `[]`, bodies, &calls)
	defer server.Close()
	now := time.Date(2026, 8, 28, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, Now: func() time.Time { return now }})
	rows, _, err := client.ScreenerFinancials(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	byCode := map[string]foundation.FinancialStatementPeriod{}
	for _, row := range rows {
		byCode[row.Code] = row
	}
	if len(byCode) != 6 {
		t.Fatalf("expected all 6 categories represented, got %+v", byCode)
	}
	for code, wantEPS := range map[string]float64{"4001": 1.11, "4002": 2.22, "4003": 3.33, "4004": 4.44, "4005": 5.55, "4006": 6.66} {
		row := byCode[code]
		if row.CumulativeEPS == nil || *row.CumulativeEPS != wantEPS {
			t.Fatalf("%s: expected cumulative_eps=%v available for every category, got %+v", code, wantEPS, row)
		}
	}
	if byCode["4001"].GrossMargin == nil || byCode["4001"].OperatingMargin == nil {
		t.Fatalf("ci category must expose margins, got %+v", byCode["4001"])
	}
	for _, code := range []string{"4002", "4003", "4004", "4005", "4006"} {
		row := byCode[code]
		if row.GrossMargin != nil || row.OperatingMargin != nil {
			t.Fatalf("%s (non-ci) must have nil margins even when the payload contains compatible fields (ins case), got %+v", code, row)
		}
	}
}

// TestScreenerFinancialsMissingZeroNegativeSemantics audits missing/real-zero/negative behavior for
// cumulative_eps and both margins.
func TestScreenerFinancialsMissingZeroNegativeSemantics(t *testing.T) {
	var calls atomic.Int32
	directory := `[{"公司代號":"5001","公司簡稱":"A"},{"公司代號":"5002","公司簡稱":"B"},{"公司代號":"5003","公司簡稱":"C"},{"公司代號":"5004","公司簡稱":"D"}]`
	ci := `[
		{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"5001","營業收入":"","營業毛利（毛損）淨額":"","營業利益（損失）":"","淨利（淨損）歸屬於母公司業主":"","基本每股盈餘（元）":""},
		{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"5002","營業收入":"0","營業毛利（毛損）淨額":"0","營業利益（損失）":"0","淨利（淨損）歸屬於母公司業主":"0","基本每股盈餘（元）":"0"},
		{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"5003","營業收入":"1000","營業毛利（毛損）淨額":"-200","營業利益（損失）":"-300","淨利（淨損）歸屬於母公司業主":"-400","基本每股盈餘（元）":"-1.23"},
		{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"5004","營業收入":"0","營業毛利（毛損）淨額":"100","營業利益（損失）":"100","淨利（淨損）歸屬於母公司業主":"100","基本每股盈餘（元）":"1.00"}
	]`
	bodies := defaultFinancialsBodies()
	bodies[financialsCategoryPath("TWSE", "ci")] = ci
	server := newFinancialsServer(t, directory, `[]`, bodies, &calls)
	defer server.Close()
	now := time.Date(2026, 8, 28, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, Now: func() time.Time { return now }})
	rows, _, err := client.ScreenerFinancials(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	byCode := map[string]foundation.FinancialStatementPeriod{}
	for _, row := range rows {
		byCode[row.Code] = row
	}

	missing := byCode["5001"]
	if missing.CumulativeEPS != nil || missing.GrossMargin != nil || missing.OperatingMargin != nil || missing.Revenue != nil {
		t.Fatalf("all-blank row must be entirely nil (missing != zero), got %+v", missing)
	}
	zero := byCode["5002"]
	if zero.CumulativeEPS == nil || *zero.CumulativeEPS != 0 {
		t.Fatalf("real zero EPS must remain 0, not nil, got %+v", zero)
	}
	if zero.GrossMargin != nil || zero.OperatingMargin != nil {
		t.Fatalf("zero-denominator (revenue=0) margins must be nil, not a fabricated ratio, got %+v", zero)
	}
	negative := byCode["5003"]
	if negative.CumulativeEPS == nil || *negative.CumulativeEPS != -1.23 {
		t.Fatalf("negative EPS must be preserved, got %+v", negative)
	}
	if negative.GrossMargin == nil || *negative.GrossMargin != -20 || negative.OperatingMargin == nil || *negative.OperatingMargin != -30 {
		t.Fatalf("negative numerator with positive revenue must yield a valid negative margin, got %+v", negative)
	}
	zeroRevenuePositive := byCode["5004"]
	if zeroRevenuePositive.GrossMargin != nil || zeroRevenuePositive.OperatingMargin != nil {
		t.Fatalf("zero denominator must yield nil margin even with a positive numerator, got %+v", zeroRevenuePositive)
	}
}

// TestScreenerFinancialsExactCanonicalNeverCollapsesAcrossExchanges proves the same bare company
// code on TWSE and TPEx never merges into one row.
func TestScreenerFinancialsExactCanonicalNeverCollapsesAcrossExchanges(t *testing.T) {
	var calls atomic.Int32
	twseDirectory := `[{"公司代號":"6001","公司簡稱":"TWSE-A"}]`
	tpexDirectory := `[{"SecuritiesCompanyCode":"6001","CompanyAbbreviation":"TPEX-A"}]`
	twseCI := `[{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"6001","營業收入":"1000","營業毛利（毛損）淨額":"400","營業利益（損失）":"300","淨利（淨損）歸屬於母公司業主":"200","基本每股盈餘（元）":"1.00"}]`
	tpexCI := `[{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"6001","營業收入":"9000","營業毛利（毛損）淨額":"4000","營業利益（損失）":"3000","淨利（淨損）歸屬於母公司業主":"2000","基本每股盈餘（元）":"9.00"}]`
	bodies := defaultFinancialsBodies()
	bodies[financialsCategoryPath("TWSE", "ci")] = twseCI
	bodies[financialsCategoryPath("TPEX", "ci")] = tpexCI
	server := newFinancialsServer(t, twseDirectory, tpexDirectory, bodies, &calls)
	defer server.Close()
	now := time.Date(2026, 8, 28, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, Now: func() time.Time { return now }})
	rows, _, err := client.ScreenerFinancials(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	byCanonical := map[string]foundation.FinancialStatementPeriod{}
	for _, row := range rows {
		byCanonical[row.Canonical] = row
	}
	if len(byCanonical) != 2 {
		t.Fatalf("same code on two exchanges must remain two distinct rows, got %+v", byCanonical)
	}
	twse := byCanonical["6001.TWSE"]
	tpex := byCanonical["6001.TPEX"]
	if twse.CumulativeEPS == nil || *twse.CumulativeEPS != 1.0 {
		t.Fatalf("6001.TWSE wrong EPS: %+v", twse)
	}
	if tpex.CumulativeEPS == nil || *tpex.CumulativeEPS != 9.0 {
		t.Fatalf("6001.TPEX wrong EPS: %+v", tpex)
	}
}

// TestScreenerFinancialsEmptyPlaceholderRowIgnored proves a TPEx-style blank placeholder row (empty
// company code, empty everything) for a category with no real companies is silently ignored — never
// an empty canonical, never a usable row, never a parse failure for the rest of the domain.
func TestScreenerFinancialsEmptyPlaceholderRowIgnored(t *testing.T) {
	var calls atomic.Int32
	twseDirectory := `[{"公司代號":"7001","公司簡稱":"A"}]`
	tpexFH := `[{"Date":"1150905","Year":"","Season":"","公司代號":"","公司名稱":"","淨收益":"","基本每股盈餘（元）":""}]`
	twseCI := `[{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"7001","營業收入":"1000","營業毛利（毛損）淨額":"400","營業利益（損失）":"300","淨利（淨損）歸屬於母公司業主":"200","基本每股盈餘（元）":"1.00"}]`
	bodies := defaultFinancialsBodies()
	bodies[financialsCategoryPath("TWSE", "ci")] = twseCI
	bodies[financialsCategoryPath("TPEX", "fh")] = tpexFH
	server := newFinancialsServer(t, twseDirectory, `[]`, bodies, &calls)
	defer server.Close()
	now := time.Date(2026, 8, 28, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, Now: func() time.Time { return now }})
	rows, _, err := client.ScreenerFinancials(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Code != "7001" {
		t.Fatalf("blank placeholder row must be ignored: expected only 7001, got %+v", rows)
	}
	for _, row := range rows {
		if row.Canonical == "" || row.Code == "" {
			t.Fatalf("no row should ever have an empty canonical/code, got %+v", row)
		}
	}
}

// TestScreenerFinancialsPartialCategoryFailureRetainsSuccessfulRows proves one failed category
// endpoint never discards the other 11 successful ones, and the domain status truthfully reads
// "partial" rather than "available" or "unavailable".
func TestScreenerFinancialsPartialCategoryFailureRetainsSuccessfulRows(t *testing.T) {
	var calls atomic.Int32
	twseDirectory := `[{"公司代號":"8001","公司簡稱":"A"}]`
	ci := `[{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"8001","營業收入":"1000","營業毛利（毛損）淨額":"400","營業利益（損失）":"300","淨利（淨損）歸屬於母公司業主":"200","基本每股盈餘（元）":"1.00"}]`
	bodies := defaultFinancialsBodies()
	bodies[financialsCategoryPath("TWSE", "ci")] = ci
	bodies[financialsCategoryPath("TWSE", "fh")] = "" // simulate a failed category request (HTTP 500)
	server := newFinancialsServer(t, twseDirectory, `[]`, bodies, &calls)
	defer server.Close()
	now := time.Date(2026, 8, 28, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, Now: func() time.Time { return now }})
	rows, freshness, err := client.ScreenerFinancials(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Code != "8001" {
		t.Fatalf("successful category rows must survive a sibling category's failure, got %+v", rows)
	}
	if freshness.Status != "partial" {
		t.Fatalf("expected partial status (11/12 category requests succeeded), got %+v", freshness)
	}
}

// TestScreenerFinancialsAllEndpointsFailYieldsUnavailable proves that when every one of the 12
// category requests fails, the domain truthfully reports unavailable with zero rows — never a
// fabricated partial result.
func TestScreenerFinancialsAllEndpointsFailYieldsUnavailable(t *testing.T) {
	var calls atomic.Int32
	twseDirectory := `[{"公司代號":"9001","公司簡稱":"A"}]`
	bodies := defaultFinancialsBodies()
	for path := range bodies {
		bodies[path] = ""
	}
	server := newFinancialsServer(t, twseDirectory, `[]`, bodies, &calls)
	defer server.Close()
	now := time.Date(2026, 8, 28, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, Now: func() time.Time { return now }})
	rows, freshness, err := client.ScreenerFinancials(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected zero rows when every endpoint fails, got %+v", rows)
	}
	if freshness.Status != "unavailable" {
		t.Fatalf("expected unavailable status, got %+v", freshness)
	}
}

// ==================================================
// M7E-C — Screener bulk balance-sheet reader (book value per share) + net_margin ci-only policy
// ==================================================

func balanceCategoryPath(exchange, category string) string {
	if exchange == "TPEX" {
		return "/mopsfin_t187ap07_O_" + category
	}
	return "/opendata/t187ap07_L_" + category
}

// defaultBalanceBodies seeds all 12 category/exchange balance-sheet endpoints with an empty (but
// successful) JSON array — tests override only the categories they care about.
func defaultBalanceBodies() map[string]string {
	bodies := map[string]string{}
	for _, exchange := range []string{"TWSE", "TPEX"} {
		for _, category := range statementCategories {
			bodies[balanceCategoryPath(exchange, category)] = "[]"
		}
	}
	return bodies
}

// newBalanceServer serves the Taiwan directory plus the 12 balance-sheet category endpoints from
// `bodies`. It fails the test immediately if an income-statement or FinMind request is ever made —
// ScreenerBalance must be balance-sheet only.
func newBalanceServer(t *testing.T, twseDirectory, tpexDirectory string, bodies map[string]string, calls *atomic.Int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/opendata/t187ap03_L":
			_, _ = w.Write([]byte(twseDirectory))
			return
		case "/opendata/t187ap47_L":
			_, _ = w.Write([]byte(`[]`))
			return
		case "/mopsfin_t187ap03_O":
			_, _ = w.Write([]byte(tpexDirectory))
			return
		}
		if strings.Contains(r.URL.Path, "t187ap06_") {
			t.Errorf("ScreenerBalance must never request an income-statement endpoint, got %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("dataset") != "" {
			t.Errorf("ScreenerBalance must never call FinMind, got %s", r.URL.String())
			http.NotFound(w, r)
			return
		}
		body, ok := bodies[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if calls != nil {
			calls.Add(1)
		}
		if body == "" {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}

// TestScreenerBalanceUsesTwelveBulkRequestsRegardlessOfRowCount proves ScreenerBalance makes exactly 12
// requests total (6 categories x 2 exchanges), even with 300+ securities per exchange, and that a
// second call within the existing 7-day URL cache adds zero new requests.
func TestScreenerBalanceUsesTwelveBulkRequestsRegardlessOfRowCount(t *testing.T) {
	const rowsPerExchange = 300
	var calls atomic.Int32

	var twseDirectory, tpexDirectory, twseCI, tpexCI strings.Builder
	twseDirectory.WriteString("[")
	tpexDirectory.WriteString("[")
	twseCI.WriteString("[")
	tpexCI.WriteString("[")
	for i := 0; i < rowsPerExchange; i++ {
		if i > 0 {
			twseDirectory.WriteString(",")
			tpexDirectory.WriteString(",")
			twseCI.WriteString(",")
			tpexCI.WriteString(",")
		}
		twseCode := fmt.Sprintf("7%03d", i)
		tpexCode := fmt.Sprintf("8%03d", i)
		fmt.Fprintf(&twseDirectory, `{"公司代號":"%s","公司簡稱":"twse-%d"}`, twseCode, i)
		fmt.Fprintf(&tpexDirectory, `{"SecuritiesCompanyCode":"%s","CompanyAbbreviation":"tpex-%d"}`, tpexCode, i)
		fmt.Fprintf(&twseCI, `{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"%s","每股參考淨值":"30.86"}`, twseCode)
		fmt.Fprintf(&tpexCI, `{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"%s","每股參考淨值":"19.52"}`, tpexCode)
	}
	twseDirectory.WriteString("]")
	tpexDirectory.WriteString("]")
	twseCI.WriteString("]")
	tpexCI.WriteString("]")

	bodies := defaultBalanceBodies()
	bodies[balanceCategoryPath("TWSE", "ci")] = twseCI.String()
	bodies[balanceCategoryPath("TPEX", "ci")] = tpexCI.String()

	server := newBalanceServer(t, twseDirectory.String(), tpexDirectory.String(), bodies, &calls)
	defer server.Close()

	now := time.Date(2026, 8, 28, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, Now: func() time.Time { return now }})
	rows, freshness, err := client.ScreenerBalance(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 12 {
		t.Fatalf("expected exactly 12 requests (6 categories x 2 exchanges) regardless of %d rows per exchange, got %d", rowsPerExchange, calls.Load())
	}
	if len(rows) != rowsPerExchange*2 {
		t.Fatalf("expected %d total rows, got %d", rowsPerExchange*2, len(rows))
	}
	if freshness.Status != "available" || freshness.AsOf == nil || *freshness.AsOf != "2026-Q2" {
		t.Fatalf("unexpected freshness: %+v", freshness)
	}

	// Warm cache: a second call within the 7-day TTL must add zero new requests.
	callsBefore := calls.Load()
	rows2, _, err := client.ScreenerBalance(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != callsBefore {
		t.Fatalf("warm cache call added %d new requests, want 0", calls.Load()-callsBefore)
	}
	if len(rows2) != len(rows) {
		t.Fatalf("warm cache row count mismatch: %d vs %d", len(rows2), len(rows))
	}
}

// TestScreenerBalanceUsesOfficialFieldForAllSixCategories proves book_value_per_share is read directly
// from the official 每股參考淨值 field for every one of the six accounting categories, never derived.
func TestScreenerBalanceUsesOfficialFieldForAllSixCategories(t *testing.T) {
	var calls atomic.Int32
	directory := `[{"公司代號":"4001","公司簡稱":"CI"},{"公司代號":"4002","公司簡稱":"BASI"},{"公司代號":"4003","公司簡稱":"BD"},{"公司代號":"4004","公司簡稱":"FH"},{"公司代號":"4005","公司簡稱":"INS"},{"公司代號":"4006","公司簡稱":"MIM"}]`
	bodies := defaultBalanceBodies()
	bvpsByCode := map[string]string{"4001": "30.86", "4002": "19.52", "4003": "29.99", "4004": "17.37", "4005": "43.10", "4006": "29.83"}
	for code, category := range map[string]string{"4001": "ci", "4002": "basi", "4003": "bd", "4004": "fh", "4005": "ins", "4006": "mim"} {
		bodies[balanceCategoryPath("TWSE", category)] = fmt.Sprintf(`[{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"%s","每股參考淨值":"%s"}]`, code, bvpsByCode[code])
	}
	server := newBalanceServer(t, directory, `[]`, bodies, &calls)
	defer server.Close()
	now := time.Date(2026, 8, 28, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, Now: func() time.Time { return now }})
	rows, _, err := client.ScreenerBalance(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	byCode := map[string]foundation.FinancialStatementPeriod{}
	for _, row := range rows {
		byCode[row.Code] = row
	}
	if len(byCode) != 6 {
		t.Fatalf("expected all 6 categories represented, got %+v", byCode)
	}
	for code, want := range bvpsByCode {
		row := byCode[code]
		wantFloat := 0.0
		fmt.Sscanf(want, "%f", &wantFloat)
		if row.BookValuePerShare == nil || *row.BookValuePerShare != wantFloat {
			t.Fatalf("%s: expected book_value_per_share=%v (official 每股參考淨值, direct not derived), got %+v", code, wantFloat, row)
		}
	}
}

// TestScreenerBalanceEmptyPlaceholderRowIgnored proves a TPEx-style blank placeholder row for a
// category with no real companies is silently ignored, mirroring ScreenerFinancials.
func TestScreenerBalanceEmptyPlaceholderRowIgnored(t *testing.T) {
	var calls atomic.Int32
	twseDirectory := `[{"公司代號":"7001","公司簡稱":"A"}]`
	tpexFH := `[{"Date":"1150905","Year":"","Season":"","公司代號":"","公司名稱":"","每股參考淨值":""}]`
	twseCI := `[{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"7001","每股參考淨值":"30.86"}]`
	bodies := defaultBalanceBodies()
	bodies[balanceCategoryPath("TWSE", "ci")] = twseCI
	bodies[balanceCategoryPath("TPEX", "fh")] = tpexFH
	server := newBalanceServer(t, twseDirectory, `[]`, bodies, &calls)
	defer server.Close()
	now := time.Date(2026, 8, 28, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, Now: func() time.Time { return now }})
	rows, _, err := client.ScreenerBalance(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Code != "7001" {
		t.Fatalf("blank placeholder row must be ignored: expected only 7001, got %+v", rows)
	}
}

// TestScreenerBalanceMalformedRowSkipsRowNotWholeDomain proves a single malformed row (invalid fiscal
// quarter) is skipped without failing the rest of the category/domain.
func TestScreenerBalanceMalformedRowSkipsRowNotWholeDomain(t *testing.T) {
	var calls atomic.Int32
	twseDirectory := `[{"公司代號":"3001","公司簡稱":"A"},{"公司代號":"3002","公司簡稱":"B"}]`
	twseCI := `[{"出表日期":"1150905","年度":"115","季別":"9","公司代號":"3001","每股參考淨值":"10.00"},{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"3002","每股參考淨值":"20.00"}]`
	bodies := defaultBalanceBodies()
	bodies[balanceCategoryPath("TWSE", "ci")] = twseCI
	server := newBalanceServer(t, twseDirectory, `[]`, bodies, &calls)
	defer server.Close()
	now := time.Date(2026, 8, 28, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, Now: func() time.Time { return now }})
	rows, _, err := client.ScreenerBalance(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Code != "3002" {
		t.Fatalf("expected only the valid row (invalid quarter=9 skipped), got %+v", rows)
	}
}

// TestScreenerBalancePartialCategoryFailureRetainsSuccessfulRows proves one failed category endpoint
// never discards the other 11 successful ones, and the domain status truthfully reads "partial".
func TestScreenerBalancePartialCategoryFailureRetainsSuccessfulRows(t *testing.T) {
	var calls atomic.Int32
	twseDirectory := `[{"公司代號":"8001","公司簡稱":"A"}]`
	ci := `[{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"8001","每股參考淨值":"30.86"}]`
	bodies := defaultBalanceBodies()
	bodies[balanceCategoryPath("TWSE", "ci")] = ci
	bodies[balanceCategoryPath("TWSE", "fh")] = "" // simulate a failed category request (HTTP 500)
	server := newBalanceServer(t, twseDirectory, `[]`, bodies, &calls)
	defer server.Close()
	now := time.Date(2026, 8, 28, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, Now: func() time.Time { return now }})
	rows, freshness, err := client.ScreenerBalance(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Code != "8001" {
		t.Fatalf("successful category rows must survive a sibling category's failure, got %+v", rows)
	}
	if freshness.Status != "partial" {
		t.Fatalf("expected partial status (11/12 category requests succeeded), got %+v", freshness)
	}
}

// TestScreenerBalanceAllEndpointsFailYieldsUnavailable proves that when every one of the 12 category
// requests fails, the domain truthfully reports unavailable with zero rows.
func TestScreenerBalanceAllEndpointsFailYieldsUnavailable(t *testing.T) {
	var calls atomic.Int32
	twseDirectory := `[{"公司代號":"9001","公司簡稱":"A"}]`
	bodies := defaultBalanceBodies()
	for path := range bodies {
		bodies[path] = ""
	}
	server := newBalanceServer(t, twseDirectory, `[]`, bodies, &calls)
	defer server.Close()
	now := time.Date(2026, 8, 28, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, Now: func() time.Time { return now }})
	rows, freshness, err := client.ScreenerBalance(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected zero rows when every endpoint fails, got %+v", rows)
	}
	if freshness.Status != "unavailable" {
		t.Fatalf("expected unavailable status, got %+v", freshness)
	}
}

// TestScreenerBalanceExactCanonicalNeverCollapsesAcrossExchanges proves the same bare company code on
// TWSE and TPEx never merges into one balance-sheet row.
func TestScreenerBalanceExactCanonicalNeverCollapsesAcrossExchanges(t *testing.T) {
	var calls atomic.Int32
	twseDirectory := `[{"公司代號":"6001","公司簡稱":"TWSE-A"}]`
	tpexDirectory := `[{"SecuritiesCompanyCode":"6001","CompanyAbbreviation":"TPEX-A"}]`
	twseCI := `[{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"6001","每股參考淨值":"30.86"}]`
	tpexCI := `[{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"6001","每股參考淨值":"19.52"}]`
	bodies := defaultBalanceBodies()
	bodies[balanceCategoryPath("TWSE", "ci")] = twseCI
	bodies[balanceCategoryPath("TPEX", "ci")] = tpexCI
	server := newBalanceServer(t, twseDirectory, tpexDirectory, bodies, &calls)
	defer server.Close()
	now := time.Date(2026, 8, 28, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, Now: func() time.Time { return now }})
	rows, _, err := client.ScreenerBalance(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	byCanonical := map[string]foundation.FinancialStatementPeriod{}
	for _, row := range rows {
		byCanonical[row.Canonical] = row
	}
	if len(byCanonical) != 2 {
		t.Fatalf("same code on two exchanges must remain two distinct rows, got %+v", byCanonical)
	}
	twse := byCanonical["6001.TWSE"]
	tpex := byCanonical["6001.TPEX"]
	if twse.BookValuePerShare == nil || *twse.BookValuePerShare != 30.86 {
		t.Fatalf("6001.TWSE wrong BVPS: %+v", twse)
	}
	if tpex.BookValuePerShare == nil || *tpex.BookValuePerShare != 19.52 {
		t.Fatalf("6001.TPEX wrong BVPS: %+v", tpex)
	}
}

// TestScreenerFinancialsNetMarginCIOnlyIncludingFHHazard (M7E-C) proves net_margin is populated only
// for the "ci" category, and explicitly reproduces the fh hazard identified by M7E-C.0: a "fh" row
// whose NetIncomeParent is materially larger than its unrelated `淨收益` line must never produce a
// misleading (e.g. >1000%) net margin — net_margin must be nil for fh regardless.
func TestScreenerFinancialsNetMarginCIOnlyIncludingFHHazard(t *testing.T) {
	var calls atomic.Int32
	directory := `[{"公司代號":"4001","公司簡稱":"CI"},{"公司代號":"4002","公司簡稱":"BASI"},{"公司代號":"4003","公司簡稱":"BD"},{"公司代號":"4004","公司簡稱":"FH"},{"公司代號":"4005","公司簡稱":"INS"},{"公司代號":"4006","公司簡稱":"MIM"}]`
	ci := `[{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"4001","營業收入":"1000","營業毛利（毛損）淨額":"400","營業利益（損失）":"300","淨利（淨損）歸屬於母公司業主":"200","基本每股盈餘（元）":"1.11"}]`
	basi := `[{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"4002","淨利（淨損）歸屬於母公司業主":"200","基本每股盈餘（元）":"2.22"}]`
	bd := `[{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"4003","淨利（淨損）歸屬於母公司業主":"200","基本每股盈餘（元）":"3.33"}]`
	// fh hazard fixture: NetIncomeParent (17,363,019) is ~11,000x larger than the unrelated `淨收益`
	// line (1,521,964) — exactly reproducing the live 華南金 payload shape from M7E-C.0. If net_margin
	// were computed as NetIncomeParent/淨收益 here, it would be a nonsensical >1000%.
	fh := `[{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"4004","淨收益":"1521964","淨利（淨損）歸屬於母公司業主":"17363019","基本每股盈餘（元）":"4.44"}]`
	ins := `[{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"4005","營業收入":"5000","營業利益（損失）":"1000","淨利（淨損）歸屬於母公司業主":"200","基本每股盈餘（元）":"5.55"}]`
	mim := `[{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"4006","淨利（淨損）歸屬於母公司業主":"200","基本每股盈餘（元）":"6.66"}]`

	bodies := defaultFinancialsBodies()
	bodies[financialsCategoryPath("TWSE", "ci")] = ci
	bodies[financialsCategoryPath("TWSE", "basi")] = basi
	bodies[financialsCategoryPath("TWSE", "bd")] = bd
	bodies[financialsCategoryPath("TWSE", "fh")] = fh
	bodies[financialsCategoryPath("TWSE", "ins")] = ins
	bodies[financialsCategoryPath("TWSE", "mim")] = mim

	server := newFinancialsServer(t, directory, `[]`, bodies, &calls)
	defer server.Close()
	now := time.Date(2026, 8, 28, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, Now: func() time.Time { return now }})
	rows, _, err := client.ScreenerFinancials(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	byCode := map[string]foundation.FinancialStatementPeriod{}
	for _, row := range rows {
		byCode[row.Code] = row
	}
	ci4001 := byCode["4001"]
	wantNetMargin := 200.0 / 1000.0 * 100 // NetIncomeParent / Revenue * 100 = 20%
	if ci4001.NetMargin == nil || *ci4001.NetMargin != wantNetMargin {
		t.Fatalf("ci: expected net_margin=%v, got %+v", wantNetMargin, ci4001)
	}
	for _, code := range []string{"4002", "4003", "4004", "4005", "4006"} {
		row := byCode[code]
		if row.NetMargin != nil {
			t.Fatalf("%s (non-ci) must have nil net_margin, even fh whose fallback would otherwise compute a materially misleading value, got %+v", code, row)
		}
	}
}

// ==================================================
// M7G — balance-sheet ratios (debt_ratio / debt_to_equity / current_ratio, ci-only, Model B independent
// balance target period)
// ==================================================

func balanceRowFixture(category string, extra map[string]string) map[string]string {
	row := map[string]string{"出表日期": "1150905", "年度": "115", "季別": "2", "公司代號": "5001"}
	for k, v := range extra {
		row[k] = v
	}
	_ = category
	return row
}

func testIdentity() foundation.SecurityIdentity {
	return foundation.SecurityIdentity{Canonical: "5001.TWSE", Code: "5001", Exchange: "TWSE"}
}

// TestParseBalanceSheetRowCiComputesAllThreeRatios proves ci-category rows compute debt_ratio/
// debt_to_equity/current_ratio directly from the official 資產總計/負債總計/權益總計/流動資產/流動負債
// fields.
func TestParseBalanceSheetRowCiComputesAllThreeRatios(t *testing.T) {
	row := balanceRowFixture("ci", map[string]string{
		"資產總計": "1000", "負債總計": "300", "權益總計": "700", "流動資產": "500", "流動負債": "200", "每股參考淨值": "28.5",
	})
	item, err := parseBalanceSheetRow(testIdentity(), "ci", "http://example/ci", row)
	if err != nil {
		t.Fatal(err)
	}
	wantDebtRatio, wantDebtToEquity, wantCurrentRatio := 30.0, 300.0/700.0*100, 250.0
	if item.DebtRatio == nil || *item.DebtRatio != wantDebtRatio {
		t.Fatalf("debt_ratio: want %v, got %v", wantDebtRatio, item.DebtRatio)
	}
	if item.DebtToEquity == nil || *item.DebtToEquity != wantDebtToEquity {
		t.Fatalf("debt_to_equity: want %v, got %v", wantDebtToEquity, item.DebtToEquity)
	}
	if item.CurrentRatio == nil || *item.CurrentRatio != wantCurrentRatio {
		t.Fatalf("current_ratio: want %v, got %v", wantCurrentRatio, item.CurrentRatio)
	}
	if item.BookValuePerShare == nil || *item.BookValuePerShare != 28.5 {
		t.Fatalf("book_value_per_share regression: got %v", item.BookValuePerShare)
	}
}

// TestParseBalanceSheetRowAlternateKeyVariant proves the 資產總額/負債總額/權益總額 fallback keys
// (used by basi/fh/mim) parse correctly for the raw amounts, even though basi has no current-
// asset/liability fields at all and is non-ci (so ratios stay nil per category policy below).
func TestParseBalanceSheetRowAlternateKeyVariant(t *testing.T) {
	row := balanceRowFixture("basi", map[string]string{
		"資產總額": "1000", "負債總額": "300", "權益總額": "700", "每股參考淨值": "19.52",
	})
	item, err := parseBalanceSheetRow(testIdentity(), "basi", "http://example/basi", row)
	if err != nil {
		t.Fatal(err)
	}
	if item.TotalAssets == nil || *item.TotalAssets != 1000000 || item.TotalLiabilities == nil || *item.TotalLiabilities != 300000 || item.Equity == nil || *item.Equity != 700000 {
		t.Fatalf("expected 資產總額/負債總額/權益總額 fallback keys parsed (thousand-TWD scaled), got %+v", item)
	}
	if item.CurrentAssets != nil || item.CurrentLiabilities != nil {
		t.Fatalf("basi has no 流動資產/流動負債 fields at all: expected nil, got %+v", item)
	}
}

// TestParseBalanceSheetRowNonCiNeverComputesRatiosEvenWhenFieldsPresent proves the ci-only policy is
// enforced regardless of whether the underlying payload happens to carry compatible-looking fields —
// here "bd" technically has both key variants of every field, but must still never expose ratios.
func TestParseBalanceSheetRowNonCiNeverComputesRatiosEvenWhenFieldsPresent(t *testing.T) {
	row := balanceRowFixture("bd", map[string]string{
		"資產總計": "1000", "負債總計": "300", "權益總計": "700", "流動資產": "500", "流動負債": "200",
	})
	item, err := parseBalanceSheetRow(testIdentity(), "bd", "http://example/bd", row)
	if err != nil {
		t.Fatal(err)
	}
	if item.DebtRatio != nil || item.DebtToEquity != nil || item.CurrentRatio != nil {
		t.Fatalf("bd (non-ci) must never expose any of the three ratios even when all source fields are present, got %+v", item)
	}
	if item.TotalAssets == nil || item.CurrentLiabilities == nil {
		t.Fatalf("raw amounts must still parse for non-ci categories (only the derived ratios are policy-gated), got %+v", item)
	}
}

// TestParseBalanceSheetRowZeroAndNegativeDenominatorSafety proves zero/negative denominators null the
// corresponding ratio, never NaN/Infinity/a fabricated zero.
func TestParseBalanceSheetRowZeroAndNegativeDenominatorSafety(t *testing.T) {
	cases := []struct {
		name  string
		extra map[string]string
		check func(t *testing.T, item foundation.FinancialStatementPeriod)
	}{
		{"zero total assets", map[string]string{"資產總計": "0", "負債總計": "300", "權益總計": "700", "流動資產": "500", "流動負債": "200"}, func(t *testing.T, item foundation.FinancialStatementPeriod) {
			if item.DebtRatio != nil {
				t.Fatalf("zero total assets: expected debt_ratio nil, got %v", *item.DebtRatio)
			}
		}},
		{"zero equity", map[string]string{"資產總計": "1000", "負債總計": "300", "權益總計": "0", "流動資產": "500", "流動負債": "200"}, func(t *testing.T, item foundation.FinancialStatementPeriod) {
			if item.DebtToEquity != nil {
				t.Fatalf("zero equity: expected debt_to_equity nil, got %v", *item.DebtToEquity)
			}
		}},
		{"negative equity", map[string]string{"資產總計": "1000", "負債總計": "1300", "權益總計": "-300", "流動資產": "500", "流動負債": "200"}, func(t *testing.T, item foundation.FinancialStatementPeriod) {
			if item.DebtToEquity != nil {
				t.Fatalf("negative equity: expected debt_to_equity nil (never a fabricated/negative leverage ratio), got %v", *item.DebtToEquity)
			}
		}},
		{"zero current liabilities", map[string]string{"資產總計": "1000", "負債總計": "300", "權益總計": "700", "流動資產": "500", "流動負債": "0"}, func(t *testing.T, item foundation.FinancialStatementPeriod) {
			if item.CurrentRatio != nil {
				t.Fatalf("zero current liabilities: expected current_ratio nil, got %v", *item.CurrentRatio)
			}
		}},
	}
	for _, c := range cases {
		row := balanceRowFixture("ci", c.extra)
		item, err := parseBalanceSheetRow(testIdentity(), "ci", "http://example/ci", row)
		if err != nil {
			t.Fatalf("%s: unexpected error %v", c.name, err)
		}
		c.check(t, item)
	}
}

// TestParseBalanceSheetRowMalformedFieldIsolation proves a malformed M7G ratio input only nulls the
// ratio(s) that depend on it — it never drops the whole row, and never fails the row with an error
// (unlike book_value_per_share's unchanged row-drop-on-malformed behavior below).
func TestParseBalanceSheetRowMalformedFieldIsolation(t *testing.T) {
	row := balanceRowFixture("ci", map[string]string{
		"資產總計": "not-a-number", "負債總計": "300", "權益總計": "700", "流動資產": "500", "流動負債": "200", "每股參考淨值": "28.5",
	})
	item, err := parseBalanceSheetRow(testIdentity(), "ci", "http://example/ci", row)
	if err != nil {
		t.Fatalf("malformed 資產總計 must not fail the row, got error: %v", err)
	}
	if item.TotalAssets != nil {
		t.Fatalf("expected total_assets nil for a malformed value, got %v", *item.TotalAssets)
	}
	if item.DebtRatio != nil {
		t.Fatalf("expected debt_ratio nil (depends on the malformed total_assets), got %v", *item.DebtRatio)
	}
	// The other two ratios (whose own inputs are valid) and BVPS must remain preserved.
	if item.DebtToEquity == nil {
		t.Fatalf("debt_to_equity must remain available (its own inputs are valid), got nil")
	}
	if item.CurrentRatio == nil {
		t.Fatalf("current_ratio must remain available (its own inputs are valid), got nil")
	}
	if item.BookValuePerShare == nil || *item.BookValuePerShare != 28.5 {
		t.Fatalf("book_value_per_share must be preserved despite the malformed M7G field, got %v", item.BookValuePerShare)
	}
}

// TestParseBalanceSheetRowMalformedBVPSStillDropsRowUnchanged proves BVPS's pre-existing row-drop-on-
// malformed behavior is left completely unchanged (backward compatibility, M7G.0 §38) even though the
// new M7G fields around it now use field-level isolation.
func TestParseBalanceSheetRowMalformedBVPSStillDropsRowUnchanged(t *testing.T) {
	row := balanceRowFixture("ci", map[string]string{
		"資產總計": "1000", "負債總計": "300", "權益總計": "700", "每股參考淨值": "not-a-number",
	})
	_, err := parseBalanceSheetRow(testIdentity(), "ci", "http://example/ci", row)
	if err == nil {
		t.Fatal("malformed 每股參考淨值 must still fail the whole row, unchanged from pre-M7G behavior")
	}
}

// TestScreenerBalanceModelBIndependentTargetPeriodNullsOffTargetRows proves the three M7G ratios are
// gated by ScreenerBalance's own independent balance-target-period (the max FiscalYear/FiscalQuarter
// among balance rows themselves) — an older row's ratios are nulled, but its FiscalYear/FiscalQuarter
// and BookValuePerShare are left completely untouched (so httpapi can still report its true
// balance_period, and BVPS's own separate Model A gate is unaffected).
func TestScreenerBalanceModelBIndependentTargetPeriodNullsOffTargetRows(t *testing.T) {
	var calls atomic.Int32
	twseDirectory := `[{"公司代號":"5001","公司簡稱":"A"},{"公司代號":"5002","公司簡稱":"B"}]`
	ci := `[` +
		`{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"5001","資產總計":"1000","負債總計":"300","權益總計":"700","流動資產":"500","流動負債":"200","每股參考淨值":"28.5"},` +
		`{"出表日期":"1150605","年度":"115","季別":"1","公司代號":"5002","資產總計":"2000","負債總計":"600","權益總計":"1400","流動資產":"900","流動負債":"300","每股參考淨值":"45.2"}` +
		`]`
	bodies := defaultBalanceBodies()
	bodies[balanceCategoryPath("TWSE", "ci")] = ci
	server := newBalanceServer(t, twseDirectory, `[]`, bodies, &calls)
	defer server.Close()
	now := time.Date(2026, 8, 28, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, Now: func() time.Time { return now }})
	rows, freshness, err := client.ScreenerBalance(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if freshness.AsOf == nil || *freshness.AsOf != "2026-Q2" {
		t.Fatalf("expected independent balance target period 2026-Q2 (max of Q2/Q1), got %+v", freshness)
	}
	byCode := map[string]foundation.FinancialStatementPeriod{}
	for _, row := range rows {
		byCode[row.Code] = row
	}
	onTarget := byCode["5001"]
	if onTarget.DebtRatio == nil || onTarget.DebtToEquity == nil || onTarget.CurrentRatio == nil {
		t.Fatalf("5001 (on target 2026-Q2): expected all three ratios exposed, got %+v", onTarget)
	}
	offTarget := byCode["5002"]
	if offTarget.DebtRatio != nil || offTarget.DebtToEquity != nil || offTarget.CurrentRatio != nil {
		t.Fatalf("5002 (2026-Q1, off the independent balance target): expected all three ratios nil, got %+v", offTarget)
	}
	if offTarget.FiscalYear != 2026 || offTarget.FiscalQuarter != 1 {
		t.Fatalf("5002's own FiscalYear/FiscalQuarter must remain untouched (so its true balance_period can still be reported), got %+v", offTarget)
	}
	if offTarget.BookValuePerShare == nil || *offTarget.BookValuePerShare != 45.2 {
		t.Fatalf("5002's book_value_per_share must remain untouched by the M7G target-period nulling (BVPS uses its own separate Model A gate), got %v", offTarget.BookValuePerShare)
	}
}

func TestFundamentalsCacheDoesNotHoldLockDuringFetch(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow" {
			close(started)
			<-release
		}
		json.NewEncoder(w).Encode([]map[string]string{})
	}))
	defer server.Close()
	c := NewClient(Config{HTTPClient: server.Client()})
	done := make(chan struct{})
	go func() { c.fundamentalsRows(context.Background(), server.URL+"/slow", time.Hour); close(done) }()
	<-started
	fastDone := make(chan struct{})
	go func() { c.fundamentalsRows(context.Background(), server.URL+"/fast", time.Hour); close(fastDone) }()
	select {
	case <-fastDone:
	case <-time.After(time.Second):
		t.Fatal("unrelated cache key blocked by slow network fetch")
	}
	close(release)
	<-done
}

// ==================================================
// M8A -- single-security statement() reuse of parseBalanceSheetRow + M7H cashflowData()
// ==================================================

// statementTestServer serves TWSE/TPEx directory endpoints (needed by cashflowData's eligible-universe
// computation), income/balance bulk endpoints for the "ci"/"fh" categories used below (empty for the
// other categories, so statement()'s category loop never reports a discrepancy), and the MOPS
// discovery/download endpoints for the cash-flow archive. counts (if non-nil) are incremented per
// endpoint hit, so tests can assert warm-cache reuse.
func statementTestServer(t *testing.T, twseCodes, tpexCodes []string, incomeRows, balanceRows map[string][]map[string]string, zipBytes []byte, discoveryCalls, downloadCalls *int64) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/opendata/t187ap03_L", func(w http.ResponseWriter, r *http.Request) {
		rows := make([]map[string]string, 0, len(twseCodes))
		for _, code := range twseCodes {
			rows = append(rows, map[string]string{"公司代號": code, "公司簡稱": code, "公司名稱": code, "產業別": "一般業", "上市日期": "1994/09/05"})
		}
		json.NewEncoder(w).Encode(rows)
	})
	mux.HandleFunc("/opendata/t187ap47_L", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `[]`) })
	mux.HandleFunc("/mopsfin_t187ap03_O", func(w http.ResponseWriter, r *http.Request) {
		rows := make([]map[string]string, 0, len(tpexCodes))
		for _, code := range tpexCodes {
			rows = append(rows, map[string]string{"SecuritiesCompanyCode": code, "CompanyAbbreviation": code, "CompanyName": code, "SecuritiesIndustryCode": "一般業", "DateOfListing": "2019/07/10"})
		}
		json.NewEncoder(w).Encode(rows)
	})
	for _, category := range statementCategories {
		category := category
		mux.HandleFunc("/opendata/t187ap06_L_"+category, func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(incomeRows["TWSE:"+category])
		})
		mux.HandleFunc("/opendata/t187ap07_L_"+category, func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(balanceRows["TWSE:"+category])
		})
		mux.HandleFunc("/mopsfin_t187ap06_O_"+category, func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(incomeRows["TPEX:"+category])
		})
		mux.HandleFunc("/mopsfin_t187ap07_O_"+category, func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(balanceRows["TPEX:"+category])
		})
	}
	mux.HandleFunc("/mops/web/t203sb02", func(w http.ResponseWriter, r *http.Request) {
		if discoveryCalls != nil {
			atomic.AddInt64(discoveryCalls, 1)
		}
		fmt.Fprint(w, `<a onclick="...fileName=tifrs-2026Q2.zip...">x</a>`)
	})
	mux.HandleFunc("/server-java/FileDownLoad", func(w http.ResponseWriter, r *http.Request) {
		if downloadCalls != nil {
			atomic.AddInt64(downloadCalls, 1)
		}
		if zipBytes == nil {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<html>maintenance</html>")
			return
		}
		w.Header().Set("Content-Type", "application/x-zip-compressed")
		w.Write(zipBytes)
	})
	return httptest.NewServer(mux)
}

func newStatementTestClient(server *httptest.Server) *Client {
	return NewClient(Config{
		TWSEBaseURL: server.URL, TPExBaseURL: server.URL, CashflowBaseURL: server.URL, HTTPClient: server.Client(),
		SyncCashflow: true,
	})
}

func tsmcIncomeRow() map[string]string {
	return map[string]string{"出表日期": "1150905", "年度": "115", "季別": "2", "公司代號": "2330", "營業收入": "1,000,000", "本期稅前淨利（淨損）": "300,000", "本期稅後淨利（淨損）": "250,000", "淨利（淨損）歸屬於母公司業主": "248,000", "基本每股盈餘（元）": "9.55"}
}
func tsmcBalanceRow() map[string]string {
	return map[string]string{"出表日期": "1150905", "年度": "115", "季別": "1", "公司代號": "2330", "資產總計": "8000000", "負債總計": "2500000", "權益總計": "5500000", "流動資產": "3000000", "流動負債": "1200000", "每股參考淨值": "28.5"}
}
func gwsIncomeRow() map[string]string {
	return map[string]string{"出表日期": "1150905", "年度": "115", "季別": "2", "公司代號": "6488", "營業收入": "50,000", "本期稅前淨利（淨損）": "9,000", "本期稅後淨利（淨損）": "7,300", "淨利（淨損）歸屬於母公司業主": "7,300", "基本每股盈餘（元）": "3.5"}
}
func gwsBalanceRow() map[string]string {
	return map[string]string{"出表日期": "1150905", "年度": "115", "季別": "1", "公司代號": "6488", "資產總計": "100000", "負債總計": "40000", "權益總計": "60000", "流動資產": "50000", "流動負債": "20000", "每股參考淨值": "45.2"}
}
func tsmcCashflowZIP(t *testing.T, ocfText, ocfSign, plText, plSign string) []byte {
	doc := buildIXBRLDocument(ixbrlFixture{code: "2330", year: 2026, quarter: 2, ocfText: ocfText, ocfScale: "3", ocfSign: ocfSign, plText: plText, plScale: "3", plSign: plSign})
	return buildZipBytes(t, map[string]string{"tifrs-fr1-m1-ci-cr-2330-2026Q2.html": doc})
}

func ratioValue(numerator, denominator int64) float64 {
	return float64(numerator) / float64(denominator) * 100
}

func TestStatement_SingleStockBalanceIntegration_TWSE(t *testing.T) {
	server := statementTestServer(t, []string{"2330"}, nil,
		map[string][]map[string]string{"TWSE:ci": {tsmcIncomeRow()}},
		map[string][]map[string]string{"TWSE:ci": {tsmcBalanceRow()}},
		nil, nil, nil)
	defer server.Close()
	c := newStatementTestClient(server)
	got, err := c.statement(context.Background(), foundation.SecurityIdentity{Canonical: "2330.TWSE", Code: "2330", Exchange: "TWSE"})
	if err != nil {
		t.Fatalf("statement: %v", err)
	}
	if got.DebtRatio == nil || *got.DebtRatio != ratioValue(2500000, 8000000) {
		t.Fatalf("debt_ratio = %v, want %v", got.DebtRatio, ratioValue(2500000, 8000000))
	}
	if got.DebtToEquity == nil || *got.DebtToEquity != ratioValue(2500000, 5500000) {
		t.Fatalf("debt_to_equity = %v", got.DebtToEquity)
	}
	if got.CurrentRatio == nil || *got.CurrentRatio != ratioValue(3000000, 1200000) {
		t.Fatalf("current_ratio = %v", got.CurrentRatio)
	}
	if got.BookValuePerShare == nil || *got.BookValuePerShare != 28.5 {
		t.Fatalf("book_value_per_share = %v", got.BookValuePerShare)
	}
	// M8A period independence: income row is 115-Q2, balance row is 115-Q1 -- never collapsed together.
	if got.FiscalYear != 2026 || got.FiscalQuarter != 2 {
		t.Fatalf("income period = %d-Q%d, want 2026-Q2", got.FiscalYear, got.FiscalQuarter)
	}
	if got.BalanceFiscalYear != 2026 || got.BalanceFiscalQuarter != 1 {
		t.Fatalf("balance period = %d-Q%d, want 2026-Q1", got.BalanceFiscalYear, got.BalanceFiscalQuarter)
	}
}

func TestStatement_SingleStockCashflowIntegration_TPEXCanonicalMapping(t *testing.T) {
	doc := buildIXBRLDocument(ixbrlFixture{code: "6488", year: 2026, quarter: 2, ocfText: "12,744,715", ocfScale: "3", plText: "7,311,661", plScale: "3"})
	zipBytes := buildZipBytes(t, map[string]string{"tifrs-fr1-m1-ci-cr-6488-2026Q2.html": doc})
	server := statementTestServer(t, nil, []string{"6488"},
		map[string][]map[string]string{"TPEX:ci": {gwsIncomeRow()}},
		map[string][]map[string]string{"TPEX:ci": {gwsBalanceRow()}},
		zipBytes, nil, nil)
	defer server.Close()
	c := newStatementTestClient(server)
	got, err := c.statement(context.Background(), foundation.SecurityIdentity{Canonical: "6488.TPEX", Code: "6488", Exchange: "TPEX"})
	if err != nil {
		t.Fatalf("statement: %v", err)
	}
	if got.OperatingCashFlow == nil || *got.OperatingCashFlow != 12744715 {
		t.Fatalf("operating_cash_flow = %v, want 12744715 (correct TPEX canonical join, not TWSE)", got.OperatingCashFlow)
	}
	if got.CashFlowToNetIncome == nil {
		t.Fatal("cash_flow_to_net_income should not be nil")
	}
	if got.CashflowFiscalYear != 2026 || got.CashflowFiscalQuarter != 2 {
		t.Fatalf("cashflow period = %d-Q%d, want 2026-Q2", got.CashflowFiscalYear, got.CashflowFiscalQuarter)
	}
}

func TestStatement_CashflowWarmReuse(t *testing.T) {
	zipBytes := tsmcCashflowZIP(t, "1,122,637,757", "", "758,226,085", "")
	var discoveries, downloads int64
	server := statementTestServer(t, []string{"2330"}, nil,
		map[string][]map[string]string{"TWSE:ci": {tsmcIncomeRow()}},
		map[string][]map[string]string{"TWSE:ci": {tsmcBalanceRow()}},
		zipBytes, &discoveries, &downloads)
	defer server.Close()
	c := newStatementTestClient(server)
	s := foundation.SecurityIdentity{Canonical: "2330.TWSE", Code: "2330", Exchange: "TWSE"}
	if _, err := c.statement(context.Background(), s); err != nil {
		t.Fatalf("first statement: %v", err)
	}
	if discoveries != 1 || downloads != 1 {
		t.Fatalf("expected 1 discovery + 1 download on cold call, got discoveries=%d downloads=%d", discoveries, downloads)
	}
	if _, err := c.statement(context.Background(), s); err != nil {
		t.Fatalf("second statement: %v", err)
	}
	if discoveries != 1 || downloads != 1 {
		t.Fatalf("expected warm reuse (0 additional HTTP), got discoveries=%d downloads=%d", discoveries, downloads)
	}
}

func TestStatement_CashflowMissingIssuer(t *testing.T) {
	// 2882.TWSE is in the directory but has no entry in the cash-flow archive -- 4 of 5 eligible TWSE
	// companies ARE covered (80%, meeting the completeness threshold), so the archive is genuinely
	// accepted; 2882 itself is simply the one absent issuer, not a stub/incomplete archive.
	entries := map[string]string{}
	for _, code := range []string{"2330", "3001", "3002", "3003"} {
		entries[fmt.Sprintf("tifrs-fr1-m1-ci-cr-%s-2026Q2.html", code)] = buildIXBRLDocument(ixbrlFixture{code: code, year: 2026, quarter: 2, ocfText: "1,000", ocfScale: "3", plText: "500", plScale: "3"})
	}
	zipBytes := buildZipBytes(t, entries)
	server := statementTestServer(t, []string{"2330", "2882", "3001", "3002", "3003"}, nil,
		map[string][]map[string]string{"TWSE:fh": {{"出表日期": "1150905", "年度": "115", "季別": "2", "公司代號": "2882", "淨收益": "1,000,000", "本期稅後淨利（淨損）": "500,000", "淨利（淨損）歸屬於母公司業主": "498,000", "基本每股盈餘（元）": "3.2"}}},
		map[string][]map[string]string{"TWSE:fh": {{"出表日期": "1150905", "年度": "115", "季別": "1", "公司代號": "2882", "資產總額": "9000000", "負債總額": "8000000", "權益總額": "1000000", "每股參考淨值": "12.1"}}},
		zipBytes, nil, nil)
	defer server.Close()
	c := newStatementTestClient(server)
	got, err := c.statement(context.Background(), foundation.SecurityIdentity{Canonical: "2882.TWSE", Code: "2882", Exchange: "TWSE"})
	if err != nil {
		t.Fatalf("statement: %v (missing issuer in cashflow archive must not fail the whole statement)", err)
	}
	if got.OperatingCashFlow != nil || got.CashFlowToNetIncome != nil {
		t.Fatalf("expected nil cashflow fields for an issuer absent from the archive, got %+v / %+v", got.OperatingCashFlow, got.CashFlowToNetIncome)
	}
	if got.CashflowFiscalYear != 0 || got.CashflowFiscalQuarter != 0 {
		t.Fatalf("cashflow period must stay unpopulated (0) when absent, got %d-Q%d", got.CashflowFiscalYear, got.CashflowFiscalQuarter)
	}
	// fh category never computes the three ratios (ci-only policy, unchanged).
	if got.DebtRatio != nil || got.DebtToEquity != nil || got.CurrentRatio != nil {
		t.Fatalf("fh category must never expose the three ratios, got %+v/%+v/%+v", got.DebtRatio, got.DebtToEquity, got.CurrentRatio)
	}
}

func TestStatement_CashflowUnavailableNeverFailsStatement(t *testing.T) {
	server := statementTestServer(t, []string{"2330"}, nil,
		map[string][]map[string]string{"TWSE:ci": {tsmcIncomeRow()}},
		map[string][]map[string]string{"TWSE:ci": {tsmcBalanceRow()}},
		nil, nil, nil) // nil zipBytes -> every candidate download is a fake-HTML failure.
	defer server.Close()
	c := newStatementTestClient(server)
	got, err := c.statement(context.Background(), foundation.SecurityIdentity{Canonical: "2330.TWSE", Code: "2330", Exchange: "TWSE"})
	if err != nil {
		t.Fatalf("statement must still succeed with income+balance when cashflow is unavailable: %v", err)
	}
	if got.OperatingCashFlow != nil || got.CashFlowToNetIncome != nil {
		t.Fatalf("expected nil cashflow fields when the archive is unavailable, got %+v/%+v", got.OperatingCashFlow, got.CashFlowToNetIncome)
	}
	// Income and balance must remain fully intact -- an optional domain outage never poisons them.
	if got.DebtRatio == nil || got.BookValuePerShare == nil || got.CumulativeEPS == nil {
		t.Fatalf("income/balance fields must remain populated despite cashflow outage: %+v", got)
	}
}

// M8C.1 — the archive-wide "partial" status (some OTHER issuer's document failed to parse) must
// survive into statement()'s CashflowStatus even though 2330's OWN row parsed successfully. A
// present/valid row for the security under test must never silently upgrade an authoritatively
// partial archive to "available".
func TestStatement_CashflowPartialStatusSurvivesEvenWhenIssuerRowPresent(t *testing.T) {
	good2330 := buildIXBRLDocument(ixbrlFixture{code: "2330", year: 2026, quarter: 2, ocfText: "111,000", ocfScale: "3", plText: "50,000", plScale: "3"})
	good9999 := buildIXBRLDocument(ixbrlFixture{code: "9999", year: 2026, quarter: 2, ocfText: "333,000", ocfScale: "3", plText: "70,000", plScale: "3"})
	bad6488 := buildIXBRLDocument(ixbrlFixture{code: "6488", year: 2026, quarter: 2, malformedXML: true})
	zipBytes := buildZipBytes(t, map[string]string{
		"tifrs-fr1-m1-ci-cr-2330-2026Q2.html": good2330,
		"tifrs-fr1-m1-ci-cr-9999-2026Q2.html": good9999,
		"tifrs-fr1-m1-ci-cr-6488-2026Q2.html": bad6488,
	})
	server := statementTestServer(t, []string{"2330", "9999"}, []string{"6488"},
		map[string][]map[string]string{"TWSE:ci": {tsmcIncomeRow()}},
		map[string][]map[string]string{"TWSE:ci": {tsmcBalanceRow()}},
		zipBytes, nil, nil)
	defer server.Close()
	c := newStatementTestClient(server)
	got, err := c.statement(context.Background(), foundation.SecurityIdentity{Canonical: "2330.TWSE", Code: "2330", Exchange: "TWSE"})
	if err != nil {
		t.Fatalf("statement: %v", err)
	}
	if got.OperatingCashFlow == nil || *got.OperatingCashFlow != 111000 {
		t.Fatalf("2330's own row must still parse and populate normally: %+v", got.OperatingCashFlow)
	}
	if got.CashflowStatus != "partial" {
		t.Fatalf("cashflow_status must reflect the archive-wide partial signal, got %q", got.CashflowStatus)
	}
}

func TestStatement_NegativeOCFAndRatioPolicy(t *testing.T) {
	cases := []struct {
		name             string
		ocfText, ocfSign string
		plText, plSign   string
		wantOCF          int64
		wantRatioNil     bool
		wantRatioNeg     bool
	}{
		{"negative OCF, positive ProfitLoss", "150,005", "-", "60,000", "", -150005, false, true},
		{"zero ProfitLoss yields nil ratio", "1,000", "", "0", "", 1000, true, false},
		{"negative ProfitLoss still computes a valid (negative) ratio", "41,699", "", "4,420", "-", 41699, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := buildIXBRLDocument(ixbrlFixture{code: "2330", year: 2026, quarter: 2, ocfText: tc.ocfText, ocfScale: "3", ocfSign: tc.ocfSign, plText: tc.plText, plScale: "3", plSign: tc.plSign})
			zipBytes := buildZipBytes(t, map[string]string{"tifrs-fr1-m1-ci-cr-2330-2026Q2.html": doc})
			server := statementTestServer(t, []string{"2330"}, nil,
				map[string][]map[string]string{"TWSE:ci": {tsmcIncomeRow()}},
				map[string][]map[string]string{"TWSE:ci": {tsmcBalanceRow()}},
				zipBytes, nil, nil)
			defer server.Close()
			c := newStatementTestClient(server)
			got, err := c.statement(context.Background(), foundation.SecurityIdentity{Canonical: "2330.TWSE", Code: "2330", Exchange: "TWSE"})
			if err != nil {
				t.Fatalf("statement: %v", err)
			}
			if got.OperatingCashFlow == nil || *got.OperatingCashFlow != tc.wantOCF {
				t.Fatalf("operating_cash_flow = %v, want %d", got.OperatingCashFlow, tc.wantOCF)
			}
			if tc.wantRatioNil {
				if got.CashFlowToNetIncome != nil {
					t.Fatalf("expected nil ratio (zero ProfitLoss), got %v", *got.CashFlowToNetIncome)
				}
				return
			}
			if got.CashFlowToNetIncome == nil {
				t.Fatal("expected a non-nil ratio")
			}
			if tc.wantRatioNeg && *got.CashFlowToNetIncome >= 0 {
				t.Fatalf("expected a negative ratio, got %v", *got.CashFlowToNetIncome)
			}
		})
	}
}
