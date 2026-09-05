package taiwan

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"easy-stock/backend/internal/foundation"
)

func snapshotAllowlist() map[string]foundation.SecurityIdentity {
	return map[string]foundation.SecurityIdentity{
		"2330": {Canonical: "2330.TWSE", Code: "2330", Name: "台積電", Exchange: "TWSE", Type: foundation.SecurityTypeStock},
		"0050": {Canonical: "0050.TWSE", Code: "0050", Name: "ETF", Exchange: "TWSE", Type: foundation.SecurityTypeETF},
		"1101": {Canonical: "1101.TWSE", Code: "1101", Name: "台泥", Exchange: "TWSE", Type: foundation.SecurityTypeStock},
	}
}

func TestMarketWideDailyParsersAllowlistUnitsAndMissing(t *testing.T) {
	day := time.Date(2026, 8, 28, 0, 0, 0, 0, taipei())
	twse := monthlyResponse{Data: [][]string{
		{"2330", "台積電", "1,000", "10", "2,420,000", "2,400", "2,445", "2,390", "2,420", "<p>-</p>", "15"},
		{"0050", "ETF", "0", "0", "0", "--", "--", "--", "--"},
		{"1101", "台泥", "--", "--", "--", "--", "--", "--", "--", "", "--"},
		{"9999", "not allowed", "1", "1", "1", "1", "1", "1", "1"},
	}}
	rows := parseTWSEDailySnapshot(twse, day, snapshotAllowlist(), "https://official.test")
	if len(rows) != 3 || rows[0].Volume == nil || *rows[0].Volume != 1000 || rows[0].Amount == nil || *rows[0].Amount != 2420000 {
		t.Fatalf("unexpected rows: %#v", rows)
	}
	if rows[0].Change == nil || *rows[0].Change != -15 || rows[1].Change != nil {
		t.Fatalf("official signed change and missing change must be preserved: %#v", rows)
	}
	if rows[1].Close != nil || rows[1].Volume == nil || *rows[1].Volume != 0 || !rows[2].NoTrade {
		t.Fatalf("missing must stay nil and explicit zero must stay zero: %#v", rows[1])
	}
}

func TestOfficialSignedChangeParsing(t *testing.T) {
	day := time.Date(2026, 8, 28, 0, 0, 0, 0, taipei())
	allow := map[string]foundation.SecurityIdentity{}
	expected := map[string]*float64{"1001": floatPointer(1.5), "1002": floatPointer(-1.5), "1003": floatPointer(-1.5), "1004": floatPointer(0), "1005": floatPointer(1500), "1006": nil}
	for code := range expected {
		allow[code] = foundation.SecurityIdentity{Canonical: code + ".TWSE", Code: code, Exchange: "TWSE", Type: foundation.SecurityTypeStock}
	}
	payload := monthlyResponse{Data: [][]string{
		{"1001", "A", "1", "1", "1", "1", "1", "1", "1", "+", "+1.5"},
		{"1002", "B", "1", "1", "1", "1", "1", "1", "1", "-", "-1.5"},
		{"1003", "C", "1", "1", "1", "1", "1", "1", "1", "−", "1.5"},
		{"1004", "D", "1", "1", "1", "1", "1", "1", "1", "+", "0"},
		{"1005", "E", "1", "1", "1", "1", "1", "1", "1", "+", "1,500"},
		{"1006", "F", "1", "1", "1", "1", "1", "1", "1", "", "--"},
	}}
	rows := parseTWSEDailySnapshot(payload, day, allow, "https://official.test")
	if len(rows) != len(expected) {
		t.Fatalf("rows=%d want=%d", len(rows), len(expected))
	}
	for _, row := range rows {
		want := expected[row.Code]
		if want == nil && row.Change != nil || want != nil && (row.Change == nil || *row.Change != *want) {
			t.Fatalf("code=%s change=%v want=%v", row.Code, row.Change, want)
		}
	}
	if value, ok := optionalNumber("－1.5"); !ok || value == nil || *value != -1.5 {
		t.Fatalf("full-width minus parsed as %v ok=%t", value, ok)
	}
}

func floatPointer(value float64) *float64 { return &value }

func TestFreshnessUnavailableCurrentStaleAndIdempotent(t *testing.T) {
	zone := taipei()
	now := time.Date(2026, 8, 31, 18, 0, 0, 0, zone)
	client := NewClient(Config{Now: func() time.Time { return now }})
	if got := client.Freshness(now); got.DailyStatus != "unavailable" {
		t.Fatalf("freshness=%#v", got)
	}
	day := time.Date(2026, 8, 28, 0, 0, 0, 0, zone)
	row := foundation.TaiwanDailySnapshot{Canonical: "2330.TWSE"}
	client.storeDaily(day, []foundation.TaiwanDailySnapshot{row}, nil)
	client.storeDaily(day, []foundation.TaiwanDailySnapshot{row}, nil)
	dates, _, _ := client.AvailableMarketDates()
	if len(dates) != 1 || client.Freshness(now).DailyStatus != "stale" {
		t.Fatalf("dates=%v freshness=%#v", dates, client.Freshness(now))
	}
	client.storeDaily(time.Date(2026, 8, 31, 0, 0, 0, 0, zone), []foundation.TaiwanDailySnapshot{row}, nil)
	if client.Freshness(now).DailyStatus != "current" {
		t.Fatalf("freshness=%#v", client.Freshness(now))
	}
	if rows, err := client.DailyAsOf(day); err != nil || len(rows) != 1 {
		t.Fatalf("as-of rows=%v err=%v", rows, err)
	}
}

func TestDailyAsOfNeverFutureBackwardFills(t *testing.T) {
	client := NewClient(Config{})
	future := time.Date(2026, 8, 31, 0, 0, 0, 0, taipei())
	client.storeDaily(future, []foundation.TaiwanDailySnapshot{{Canonical: "2330.TWSE"}}, nil)
	query := time.Date(2026, 8, 28, 0, 0, 0, 0, taipei())
	if _, err := client.DailyAsOf(query); err == nil {
		t.Fatal("future snapshot must not backward-fill an earlier as-of query")
	}
}

func TestRefreshMarketWidePartialFailureAndOneRequestPerDatasetExchange(t *testing.T) {
	var dailyCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/opendata/t187ap03_L":
			_, _ = w.Write([]byte(`[{"公司代號":"2330","公司簡稱":"台積電"}]`))
		case "/opendata/t187ap47_L":
			_, _ = w.Write([]byte(`[]`))
		case "/mopsfin_t187ap03_O":
			_, _ = w.Write([]byte(`[{"SecuritiesCompanyCode":"6488","CompanyAbbreviation":"GlobalWafers"}]`))
		case "/rwd/zh/afterTrading/MI_INDEX":
			dailyCalls.Add(1)
			_, _ = w.Write([]byte(`{"data":[["2330","台積電","1000","1","2420000","2400","2445","2390","2420"]]}`))
		case "/www/zh-tw/afterTrading/dailyQuotes":
			dailyCalls.Add(1)
			_, _ = w.Write([]byte(`{"data":[["6488","GlobalWafers","100","1","99","101","98","100","2000","200000"]]}`))
		default:
			http.Error(w, "chip unavailable", http.StatusServiceUnavailable)
		}
	}))
	defer server.Close()
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, TWSEReportBaseURL: server.URL, TPExReportBaseURL: server.URL, HTTPClient: server.Client()})
	result := client.RefreshMarketWide(context.Background(), time.Date(2026, 8, 28, 0, 0, 0, 0, taipei()))
	if result.OverallStatus != "partial" || result.Daily.Status != "success" || result.Institutional.Status != "failed" || result.Margin.Status != "failed" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Daily.Rows != 2 || dailyCalls.Load() != 2 {
		t.Fatalf("daily=%#v calls=%d", result.Daily, dailyCalls.Load())
	}
}

func TestMarketBreadthUsesOneBulkDailyRequestPerExchange(t *testing.T) {
	var dailyCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/opendata/t187ap03_L":
			_, _ = w.Write([]byte(`[{"公司代號":"2330","公司簡稱":"台積電"}]`))
		case "/opendata/t187ap47_L":
			_, _ = w.Write([]byte(`[{"基金代號":"0050","基金簡稱":"元大台灣50"}]`))
		case "/mopsfin_t187ap03_O":
			_, _ = w.Write([]byte(`[{"SecuritiesCompanyCode":"6488","CompanyAbbreviation":"環球晶"}]`))
		case "/rwd/zh/afterTrading/MI_INDEX":
			dailyCalls.Add(1)
			_, _ = w.Write([]byte(`{"data":[["2330","台積電","1000","1","2420000","2400","2445","2390","2420","+","20"],["0050","ETF","100","1","10000","100","101","99","100","+","1"]]}`))
		case "/www/zh-tw/afterTrading/dailyQuotes":
			dailyCalls.Add(1)
			_, _ = w.Write([]byte(`{"data":[["6488","環球晶","100","-2","102","103","99","100","2000","200000"]]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	now := time.Date(2026, 8, 28, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, TWSEReportBaseURL: server.URL, TPExReportBaseURL: server.URL, HTTPClient: server.Client(), Now: func() time.Time { return now }})
	got, err := client.MarketBreadth(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if dailyCalls.Load() != 2 || got.TWSE.UniverseCount != 1 || got.TWSE.Advancers != 1 || got.TPEX.Decliners != 1 || got.Combined.UniverseCount != 2 {
		t.Fatalf("calls=%d breadth=%+v", dailyCalls.Load(), got)
	}
}

// TestScreenerSnapshotUsesOneBulkDailyRequestPerExchangeRegardlessOfRowCount proves the Screener's
// data source makes exactly one TWSE + one TPEx daily request in total, even with a directory and
// daily-quote fixture of several hundred securities — never one request per security (the same
// property MarketBreadth already guarantees, since ScreenerSnapshot reuses the identical path).
func TestScreenerSnapshotUsesOneBulkDailyRequestPerExchangeRegardlessOfRowCount(t *testing.T) {
	const rowsPerExchange = 300
	var directoryCalls, dailyCalls atomic.Int32

	var twseDirectory, twseDaily, tpexDirectory, tpexDaily strings.Builder
	twseDirectory.WriteString("[")
	tpexDirectory.WriteString("[")
	twseDaily.WriteString(`{"data":[`)
	tpexDaily.WriteString(`{"data":[`)
	for i := 0; i < rowsPerExchange; i++ {
		if i > 0 {
			twseDirectory.WriteString(",")
			tpexDirectory.WriteString(",")
			twseDaily.WriteString(",")
			tpexDaily.WriteString(",")
		}
		twseCode := fmt.Sprintf("1%03d", i)
		tpexCode := fmt.Sprintf("2%03d", i)
		fmt.Fprintf(&twseDirectory, `{"公司代號":"%s","公司簡稱":"twse-%d"}`, twseCode, i)
		fmt.Fprintf(&tpexDirectory, `{"SecuritiesCompanyCode":"%s","CompanyAbbreviation":"tpex-%d"}`, tpexCode, i)
		fmt.Fprintf(&twseDaily, `["%s","twse-%d","1000","1","100000","99","101","98","100","+","1"]`, twseCode, i)
		fmt.Fprintf(&tpexDaily, `["%s","tpex-%d","100","-2","102","103","99","100","2000","200000"]`, tpexCode, i)
	}
	twseDirectory.WriteString("]")
	tpexDirectory.WriteString("]")
	twseDaily.WriteString("]}")
	tpexDaily.WriteString("]}")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/opendata/t187ap03_L":
			directoryCalls.Add(1)
			_, _ = w.Write([]byte(twseDirectory.String()))
		case "/opendata/t187ap47_L":
			_, _ = w.Write([]byte(`[]`))
		case "/mopsfin_t187ap03_O":
			directoryCalls.Add(1)
			_, _ = w.Write([]byte(tpexDirectory.String()))
		case "/rwd/zh/afterTrading/MI_INDEX":
			dailyCalls.Add(1)
			_, _ = w.Write([]byte(twseDaily.String()))
		case "/www/zh-tw/afterTrading/dailyQuotes":
			dailyCalls.Add(1)
			_, _ = w.Write([]byte(tpexDaily.String()))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	now := time.Date(2026, 8, 28, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, TWSEReportBaseURL: server.URL, TPExReportBaseURL: server.URL, HTTPClient: server.Client(), Now: func() time.Time { return now }})
	rows, freshness, err := client.ScreenerSnapshot(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if dailyCalls.Load() != 2 {
		t.Fatalf("expected exactly 2 daily requests (one per exchange) regardless of %d rows per exchange, got %d", rowsPerExchange, dailyCalls.Load())
	}
	if len(rows) != rowsPerExchange*2 {
		t.Fatalf("expected %d total rows, got %d", rowsPerExchange*2, len(rows))
	}
	if freshness.DailyStatus != "current" {
		t.Fatalf("unexpected freshness: %+v", freshness)
	}
}

// TestScreenerInstitutionalUsesOneBulkRequestPerExchangeRegardlessOfRowCount (M7D) proves
// ScreenerInstitutional makes exactly one TWSE + one TPEx institutional request in total, even with
// a directory/institutional fixture of several hundred securities per exchange — never one request
// per security. Institutional data reuses the exact same bulk refresh path M7A already validated
// for daily snapshots (refreshInstitutional/instDays), so the same bound applies here.
func TestScreenerInstitutionalUsesOneBulkRequestPerExchangeRegardlessOfRowCount(t *testing.T) {
	const rowsPerExchange = 300
	var directoryCalls, instCalls atomic.Int32

	var twseDirectory, tpexDirectory, twseInst, tpexInst strings.Builder
	twseDirectory.WriteString("[")
	tpexDirectory.WriteString("[")
	twseInst.WriteString(`{"data":[`)
	tpexInst.WriteString(`{"data":[`)
	for i := 0; i < rowsPerExchange; i++ {
		if i > 0 {
			twseDirectory.WriteString(",")
			tpexDirectory.WriteString(",")
			twseInst.WriteString(",")
			tpexInst.WriteString(",")
		}
		twseCode := fmt.Sprintf("3%03d", i)
		tpexCode := fmt.Sprintf("4%03d", i)
		fmt.Fprintf(&twseDirectory, `{"公司代號":"%s","公司簡稱":"twse-%d"}`, twseCode, i)
		fmt.Fprintf(&tpexDirectory, `{"SecuritiesCompanyCode":"%s","CompanyAbbreviation":"tpex-%d"}`, tpexCode, i)
		// TWSE T86 row: parseTWSEInstitutional needs len>=19 (row[2:19] all numeric).
		fmt.Fprintf(&twseInst, `["%s","twse-%d","0","0","0","0","0","0","0","0","0","0","0","0","0","0","0","0","0"]`, twseCode, i)
		// TPEx insti row: parseTPExInstitutional needs len>=24 (row[2:24] all numeric).
		fmt.Fprintf(&tpexInst, `["%s","tpex-%d","0","0","0","0","0","0","0","0","0","0","0","0","0","0","0","0","0","0","0","0","0","0"]`, tpexCode, i)
	}
	twseDirectory.WriteString("]")
	tpexDirectory.WriteString("]")
	twseInst.WriteString("]}")
	tpexInst.WriteString("]}")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/opendata/t187ap03_L":
			directoryCalls.Add(1)
			_, _ = w.Write([]byte(twseDirectory.String()))
		case "/opendata/t187ap47_L":
			_, _ = w.Write([]byte(`[]`))
		case "/mopsfin_t187ap03_O":
			directoryCalls.Add(1)
			_, _ = w.Write([]byte(tpexDirectory.String()))
		case "/rwd/zh/fund/T86":
			instCalls.Add(1)
			_, _ = w.Write([]byte(twseInst.String()))
		case "/www/zh-tw/insti/dailyTrade":
			instCalls.Add(1)
			_, _ = w.Write([]byte(tpexInst.String()))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	now := time.Date(2026, 8, 28, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, TWSEReportBaseURL: server.URL, TPExReportBaseURL: server.URL, HTTPClient: server.Client(), Now: func() time.Time { return now }})
	rows, freshness, err := client.ScreenerInstitutional(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if instCalls.Load() != 2 {
		t.Fatalf("expected exactly 2 institutional requests (one per exchange) regardless of %d rows per exchange, got %d", rowsPerExchange, instCalls.Load())
	}
	if len(rows) != rowsPerExchange*2 {
		t.Fatalf("expected %d total institutional rows, got %d", rowsPerExchange*2, len(rows))
	}
	if freshness.InstitutionalStatus != "current" {
		t.Fatalf("unexpected freshness: %+v", freshness)
	}
	// Confirm exact canonical identity is preserved through the bulk parse (never code-only).
	byCanonical := map[string]bool{}
	for _, row := range rows {
		byCanonical[row.Canonical] = true
	}
	if !byCanonical["3000.TWSE"] || !byCanonical["4000.TPEX"] {
		t.Fatalf("expected exact TWSE/TPEX canonical identities present, got sample missing 3000.TWSE/4000.TPEX")
	}
}

// TestScreenerMarginUsesOneBulkRequestPerExchangeRegardlessOfRowCount (M7D) proves ScreenerMargin
// makes exactly one TWSE + one TPEx margin request in total, even with several hundred securities
// per exchange — never one request per security.
func TestScreenerMarginUsesOneBulkRequestPerExchangeRegardlessOfRowCount(t *testing.T) {
	const rowsPerExchange = 300
	var marginCalls atomic.Int32

	var twseDirectory, tpexDirectory, twseMargin, tpexMargin strings.Builder
	twseDirectory.WriteString("[")
	tpexDirectory.WriteString("[")
	twseMargin.WriteString(`{"data":[`)
	tpexMargin.WriteString(`{"data":[`)
	for i := 0; i < rowsPerExchange; i++ {
		if i > 0 {
			twseDirectory.WriteString(",")
			tpexDirectory.WriteString(",")
			twseMargin.WriteString(",")
			tpexMargin.WriteString(",")
		}
		twseCode := fmt.Sprintf("5%03d", i)
		tpexCode := fmt.Sprintf("6%03d", i)
		fmt.Fprintf(&twseDirectory, `{"公司代號":"%s","公司簡稱":"twse-%d"}`, twseCode, i)
		fmt.Fprintf(&tpexDirectory, `{"SecuritiesCompanyCode":"%s","CompanyAbbreviation":"tpex-%d"}`, tpexCode, i)
		// TWSE MI_MARGN row: parseMargin(..., 2,3,4,5,6,8,9,10,11,12,15) needs len>15 (>=16).
		fmt.Fprintf(&twseMargin, `["%s","twse-%d","0","0","0","0","0","0","0","0","0","0","0","0","0","0"]`, twseCode, i)
		// TPEx margin/balance row: parseMargin(..., 3,4,5,2,6,12,11,13,10,14,19) needs len>19 (>=20).
		fmt.Fprintf(&tpexMargin, `["%s","tpex-%d","0","0","0","0","0","0","0","0","0","0","0","0","0","0","0","0","0","0"]`, tpexCode, i)
	}
	twseDirectory.WriteString("]")
	tpexDirectory.WriteString("]")
	twseMargin.WriteString("]}")
	tpexMargin.WriteString("]}")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/opendata/t187ap03_L":
			_, _ = w.Write([]byte(twseDirectory.String()))
		case "/opendata/t187ap47_L":
			_, _ = w.Write([]byte(`[]`))
		case "/mopsfin_t187ap03_O":
			_, _ = w.Write([]byte(tpexDirectory.String()))
		case "/rwd/zh/marginTrading/MI_MARGN":
			marginCalls.Add(1)
			_, _ = w.Write([]byte(twseMargin.String()))
		case "/www/zh-tw/margin/balance":
			marginCalls.Add(1)
			_, _ = w.Write([]byte(tpexMargin.String()))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	now := time.Date(2026, 8, 28, 18, 0, 0, 0, taipei())
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, TWSEReportBaseURL: server.URL, TPExReportBaseURL: server.URL, HTTPClient: server.Client(), Now: func() time.Time { return now }})
	rows, freshness, err := client.ScreenerMargin(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if marginCalls.Load() != 2 {
		t.Fatalf("expected exactly 2 margin requests (one per exchange) regardless of %d rows per exchange, got %d", rowsPerExchange, marginCalls.Load())
	}
	if len(rows) != rowsPerExchange*2 {
		t.Fatalf("expected %d total margin rows, got %d", rowsPerExchange*2, len(rows))
	}
	if freshness.MarginStatus != "current" {
		t.Fatalf("unexpected freshness: %+v", freshness)
	}
	byCanonical := map[string]bool{}
	for _, row := range rows {
		byCanonical[row.Canonical] = true
	}
	if !byCanonical["5000.TWSE"] || !byCanonical["6000.TPEX"] {
		t.Fatalf("expected exact TWSE/TPEX canonical identities present, got sample missing 5000.TWSE/6000.TPEX")
	}
}
