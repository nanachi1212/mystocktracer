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
			rows = []map[string]string{{"公司代號": "2881", "資產總計": "100.00", "負債總計": "60.00", "權益總計": "40.00", "歸屬於母公司業主之權益合計": "39.00", "每股參考淨值": "20.5"}}
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
