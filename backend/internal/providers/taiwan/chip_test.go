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

func TestInstitutionalParsingAndDerivedCalculations(t *testing.T) {
	security := foundation.SecurityIdentity{Canonical: "2330.TWSE", Code: "2330", Name: "台積電", Exchange: "TWSE"}
	row := []string{"2330", "台積電", "10,027,572", "6,995,917", "3,031,655", "0", "0", "0", "109,915", "1,874,910", "-1,764,995", "-23,367", "70,050", "15,400", "54,650", "113,681", "191,698", "-78,017", "1,243,293"}
	flow, err := parseTWSEInstitutional(security, time.Date(2026, 8, 28, 0, 0, 0, 0, taipei()), "official", row)
	if err != nil {
		t.Fatal(err)
	}
	if flow.ForeignNet != 3_031_655 || flow.InvestmentTrustNet != -1_764_995 || flow.DealerNet != -23_367 || flow.DealerOfficialNet != -23_367 {
		t.Fatalf("unexpected flow: %+v", flow)
	}
	if len(flow.Discrepancies) != 0 {
		t.Fatalf("unexpected discrepancy: %v", flow.Discrepancies)
	}
	summary := institutionalSummary([]foundation.InstitutionalFlow{{ForeignNet: -1, InvestmentTrustNet: 2, DealerNet: 3}, flow})
	if summary.ForeignNet5D != 3_031_654 || summary.InvestmentTrustNet5D != -1_764_993 || summary.DealerNet5D != -23_364 || summary.ForeignConsecutiveBuyDays != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestMarginNormalizesLotsToSharesAndCalculatesChanges(t *testing.T) {
	security := foundation.SecurityIdentity{Canonical: "6488.TPEX", Code: "6488", Name: "環球晶", Exchange: "TPEX"}
	row := []string{"6488", "環球晶", "11,604", "1,132", "432", "0", "12,304", "216", "10.29", "119,528", "113", "2", "14", "0", "101", "1", "0.08", "119,528", "19", "11 A"}
	margin, err := parseMargin(security, time.Now(), "official", row, 3, 4, 5, 2, 6, 12, 11, 13, 10, 14, 19)
	if err != nil {
		t.Fatal(err)
	}
	if *margin.MarginBalance != 12_304_000 || *margin.MarginChange != 700_000 || *margin.ShortBalance != 101_000 || *margin.ShortChange != -12_000 {
		t.Fatalf("unexpected margin: %+v", margin)
	}
	if margin.ShortMarginRatio == nil || *margin.ShortMarginRatio < 0.82 || *margin.ShortMarginRatio > 0.83 {
		t.Fatalf("unexpected ratio: %v", margin.ShortMarginRatio)
	}
}

func TestInstitutionalBoundedConcurrencyAndOrdering(t *testing.T) {
	var inFlight atomic.Int32
	var maxInFlight atomic.Int32
	var totalRequests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			oldMax := maxInFlight.Load()
			if current <= oldMax || maxInFlight.CompareAndSwap(oldMax, current) {
				break
			}
		}
		totalRequests.Add(1)
		time.Sleep(20 * time.Millisecond)

		switch {
		case strings.HasPrefix(r.URL.Path, "/exchangeReport/STOCK_DAY"):
			// Return 10 dates for KLine
			var klines [][]string
			for day := 1; day <= 10; day++ {
				klines = append(klines, []string{
					fmt.Sprintf("115/08/%02d", day),
					"1,000", "2,420,000", "2,400", "2,445", "2,390", "2,420", "10", "100",
				})
			}
			json.NewEncoder(w).Encode(map[string]any{"stat": "OK", "data": klines})
		case strings.HasPrefix(r.URL.Path, "/rwd/zh/fund/T86"):
			dateStr := r.URL.Query().Get("date")
			// TWSE institutional row format: code, name, fb, fs, fn... (19 elements)
			row := []string{"2330", "台積電", "100", "50", "50", "0", "0", "0", "10", "5", "5", "0", "0", "0", "0", "0", "0", "0", "0"}
			json.NewEncoder(w).Encode(monthlyResponse{Data: [][]string{row}})
			_ = dateStr
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(Config{
		TWSEReportBaseURL: server.URL,
		TWSEBaseURL:       server.URL,
		HTTPClient:        server.Client(),
	})
	security := foundation.SecurityIdentity{Canonical: "2330.TWSE", Code: "2330", Name: "台積電", Exchange: "TWSE"}

	history, err := client.Institutional(context.Background(), security, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(history.Data) != 10 {
		t.Fatalf("expected 10 items, got %d", len(history.Data))
	}
	// Verify strict chronological date ordering
	for i := 1; i < len(history.Data); i++ {
		if history.Data[i].TradeDate <= history.Data[i-1].TradeDate {
			t.Fatalf("dates out of order: [%d]=%s, [%d]=%s", i-1, history.Data[i-1].TradeDate, i, history.Data[i].TradeDate)
		}
	}
	if max := maxInFlight.Load(); max > 4 {
		t.Fatalf("max concurrency exceeded bound of 4: got %d", max)
	}
}

func TestRefreshDailyCacheHitAndSingleflight(t *testing.T) {
	var dailyRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dailyRequests.Add(1)
		time.Sleep(10 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rwd/zh/afterTrading/MI_INDEX":
			_, _ = w.Write([]byte(`{"data":[["2330","台積電","1000","10","2420000","2400","2445","2390","2420","+","10"]]}`))
		case "/www/zh-tw/afterTrading/dailyQuotes":
			_, _ = w.Write([]byte(`{"data":[["6488","環球晶","1000","10","900000","900","910","890","900","+","5"]]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(Config{
		TWSEReportBaseURL: server.URL,
		TPExReportBaseURL: server.URL,
		HTTPClient:        server.Client(),
	})
	target := time.Date(2026, 8, 28, 0, 0, 0, 0, taipei())
	allow := map[string]map[string]foundation.SecurityIdentity{
		"TWSE": {"2330": {Canonical: "2330.TWSE", Code: "2330", Exchange: "TWSE"}},
		"TPEX": {"6488": {Canonical: "6488.TPEX", Code: "6488", Exchange: "TPEX"}},
	}

	// First call: makes 2 HTTP requests (TWSE + TPEx)
	sec1 := client.refreshDaily(context.Background(), target, allow)
	if sec1.Status != "success" || sec1.Rows != 2 {
		t.Fatalf("first call unexpected: %+v", sec1)
	}
	if dailyRequests.Load() != 2 {
		t.Fatalf("expected 2 network calls, got %d", dailyRequests.Load())
	}

	// Second call: cache hit! Should NOT make additional HTTP requests
	sec2 := client.refreshDaily(context.Background(), target, allow)
	if sec2.Status != "success" || sec2.Rows != 2 {
		t.Fatalf("second call unexpected: %+v", sec2)
	}
	if dailyRequests.Load() != 2 {
		t.Fatalf("expected still 2 network calls on cache hit, got %d", dailyRequests.Load())
	}
}

func TestMarginBoundedConcurrencyAndOrdering(t *testing.T) {
	var inFlight atomic.Int32
	var maxInFlight atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			oldMax := maxInFlight.Load()
			if current <= oldMax || maxInFlight.CompareAndSwap(oldMax, current) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)

		switch {
		case strings.HasPrefix(r.URL.Path, "/exchangeReport/STOCK_DAY"):
			var klines [][]string
			for day := 1; day <= 10; day++ {
				klines = append(klines, []string{
					fmt.Sprintf("115/08/%02d", day),
					"1,000", "2,420,000", "2,400", "2,445", "2,390", "2,420", "10", "100",
				})
			}
			json.NewEncoder(w).Encode(map[string]any{"stat": "OK", "data": klines})
		case strings.HasPrefix(r.URL.Path, "/rwd/zh/marginTrading/MI_MARGN"):
			// TWSE margin row format: code, name, and 14 numeric columns
			row := []string{"2330", "台積電", "10", "1", "1", "0", "10", "0", "5", "0", "0", "0", "5", "0", "0", "Note"}
			json.NewEncoder(w).Encode(monthlyResponse{Data: [][]string{row}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(Config{
		TWSEReportBaseURL: server.URL,
		TWSEBaseURL:       server.URL,
		HTTPClient:        server.Client(),
	})
	security := foundation.SecurityIdentity{Canonical: "2330.TWSE", Code: "2330", Name: "台積電", Exchange: "TWSE"}

	history, err := client.Margin(context.Background(), security, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(history.Data) != 10 {
		t.Fatalf("expected 10 items, got %d", len(history.Data))
	}
	for i := 1; i < len(history.Data); i++ {
		if history.Data[i].TradeDate <= history.Data[i-1].TradeDate {
			t.Fatalf("dates out of order: [%d]=%s, [%d]=%s", i-1, history.Data[i-1].TradeDate, i, history.Data[i].TradeDate)
		}
	}
	if max := maxInFlight.Load(); max > 4 {
		t.Fatalf("max concurrency exceeded bound of 4: got %d", max)
	}
}
