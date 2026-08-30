package taiwan

import (
	"context"
	"encoding/json"
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
