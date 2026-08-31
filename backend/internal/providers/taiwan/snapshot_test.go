package taiwan

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"easy-stock/backend/internal/foundation"
)

func snapshotAllowlist() map[string]foundation.SecurityIdentity {
	return map[string]foundation.SecurityIdentity{
		"2330": {Canonical: "2330.TWSE", Code: "2330", Name: "台積電", Exchange: "TWSE", Type: foundation.SecurityTypeStock},
		"0050": {Canonical: "0050.TWSE", Code: "0050", Name: "ETF", Exchange: "TWSE", Type: foundation.SecurityTypeETF},
	}
}

func TestMarketWideDailyParsersAllowlistUnitsAndMissing(t *testing.T) {
	day := time.Date(2026, 8, 28, 0, 0, 0, 0, taipei())
	twse := monthlyResponse{Data: [][]string{
		{"2330", "台積電", "1,000", "10", "2,420,000", "2,400", "2,445", "2,390", "2,420"},
		{"0050", "ETF", "0", "0", "0", "--", "--", "--", "--"},
		{"9999", "not allowed", "1", "1", "1", "1", "1", "1", "1"},
	}}
	rows := parseTWSEDailySnapshot(twse, day, snapshotAllowlist(), "https://official.test")
	if len(rows) != 2 || rows[0].Volume == nil || *rows[0].Volume != 1000 || rows[0].Amount == nil || *rows[0].Amount != 2420000 {
		t.Fatalf("unexpected rows: %#v", rows)
	}
	if rows[1].Close != nil || rows[1].Volume == nil || *rows[1].Volume != 0 {
		t.Fatalf("missing must stay nil and explicit zero must stay zero: %#v", rows[1])
	}
}

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
