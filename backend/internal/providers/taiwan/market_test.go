package taiwan

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"easy-stock/backend/internal/foundation"
)

func TestOfficialTaiwanQuoteKLineAndIndexes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/exchangeReport/STOCK_DAY_ALL":
			_, _ = w.Write([]byte(`[{"Date":"1150828","Code":"2330","Name":"台積電","TradeVolume":"1000","TradeValue":"2420000","OpeningPrice":"2400","HighestPrice":"2445","LowestPrice":"2390","ClosingPrice":"2420","Change":"10"}]`))
		case "/exchangeReport/STOCK_DAY":
			_, _ = w.Write([]byte(`{"stat":"OK","data":[["115/08/28","1,000","2,420,000","2,400","2,445","2,390","2,420","10","100"]]}`))
		case "/indicesReport/MI_5MINS_HIST":
			_, _ = w.Write([]byte(`[{"Date":"1150828","OpeningIndex":"24000","HighestIndex":"24200","LowestIndex":"23900","ClosingIndex":"24100"}]`))
		case "/tpex_index":
			_, _ = w.Write([]byte(`[{"Date":"20260828","Open":"300","High":"305","Low":"298","Close":"304","Change":"4"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, HTTPClient: server.Client()})
	security := foundation.SecurityIdentity{Canonical: "2330.TWSE", Code: "2330", Name: "台積電", Exchange: "TWSE"}
	quote, err := client.Quote(context.Background(), security)
	if err != nil || quote.Price != 2420 || quote.PreviousClose != 2410 || quote.Meta.IsRealtime {
		t.Fatalf("unexpected quote: %#v, %v", quote, err)
	}
	lines, err := client.KLine(context.Background(), security, 1)
	if err != nil || len(lines) != 1 || lines[0].Symbol != "2330.TWSE" {
		t.Fatalf("unexpected kline: %#v, %v", lines, err)
	}
	indexes, _, err := client.Indexes(context.Background())
	if err != nil || len(indexes) != 2 || indexes[1].Index.Market != "TPEX" {
		t.Fatalf("unexpected indexes: %#v, %v", indexes, err)
	}
}

func TestQuoteFallsBackToOfficialMonthlyReport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/exchangeReport/STOCK_DAY" {
			_, _ = w.Write([]byte(`{"stat":"OK","data":[["115/08/28","1,000","2,420,000","2,400","2,445","2,390","2,420","10","100"]]}`))
			return
		}
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	client := NewClient(Config{TWSEBaseURL: server.URL, HTTPClient: server.Client()})
	quote, err := client.Quote(context.Background(), foundation.SecurityIdentity{Canonical: "2330.TWSE", Code: "2330", Name: "台積電", Exchange: "TWSE"})
	if err != nil || quote.Price != 2420 || quote.Meta.Status != "official_monthly_fallback" || quote.Meta.FallbackReason == "" {
		t.Fatalf("unexpected fallback quote: %#v, %v", quote, err)
	}
}

func TestMonthlyVolumeAmountUnits(t *testing.T) {
	volume, amount, err := monthlyVolumeAmount("123", "456", "TPEX")
	if err != nil || volume != 123000 || amount != 456000 {
		t.Fatalf("TPEx units: volume=%v amount=%v err=%v", volume, amount, err)
	}
	volume, amount, err = monthlyVolumeAmount("123", "456", "TWSE")
	if err != nil || volume != 123 || amount != 456 {
		t.Fatalf("TWSE units changed: volume=%v amount=%v err=%v", volume, amount, err)
	}
	volume, amount, err = monthlyVolumeAmount("0", "0.0", "TPEX")
	if err != nil || volume != 0 || amount != 0 {
		t.Fatalf("explicit zero: volume=%v amount=%v err=%v", volume, amount, err)
	}
	for _, test := range []struct{ volume, amount string }{
		{"--", "456"},
		{"123", ""},
		{"12x3", "456"},
		{"123", "abc"},
	} {
		if _, _, err := monthlyVolumeAmount(test.volume, test.amount, "TPEX"); err == nil {
			t.Fatalf("invalid values must fail: volume=%q amount=%q", test.volume, test.amount)
		}
	}
}
