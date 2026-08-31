package taiwan

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, TWSEReportBaseURL: server.URL, HTTPClient: server.Client(), Now: fixedQuoteNow})
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
	client := NewClient(Config{TWSEBaseURL: server.URL, TWSEReportBaseURL: server.URL, HTTPClient: server.Client(), Now: fixedQuoteNow})
	quote, err := client.Quote(context.Background(), foundation.SecurityIdentity{Canonical: "2330.TWSE", Code: "2330", Name: "台積電", Exchange: "TWSE"})
	if err != nil || quote.Price != 2420 || quote.Meta.Status != "official_monthly_fallback" || quote.Meta.FallbackReason == "" || quote.Meta.Source != "twse:STOCK_DAY" || quote.Meta.TradeDate != "2026-08-28" || quote.Meta.Freshness != "stale" {
		t.Fatalf("unexpected fallback quote: %#v, %v", quote, err)
	}
}

func TestQuoteFreshnessUsesTradingCalendar(t *testing.T) {
	zone := taipei()
	calendar := foundation.TaiwanTradingCalendar{Holidays: map[string]bool{"2026-09-28": true}}
	for _, test := range []struct {
		name, tradeDate string
		now             time.Time
		want            string
	}{
		{"Friday on weekend", "2026-09-25", time.Date(2026, 9, 27, 20, 0, 0, 0, zone), "current"},
		{"holiday", "2026-09-25", time.Date(2026, 9, 28, 20, 0, 0, 0, zone), "current"},
		{"same target", "2026-08-31", time.Date(2026, 8, 31, 18, 0, 0, 0, zone), "current"},
		{"older", "2026-08-28", time.Date(2026, 8, 31, 18, 0, 0, 0, zone), "stale"},
		{"missing", "", time.Date(2026, 8, 31, 18, 0, 0, 0, zone), "unavailable"},
		{"future is not backward filled", "2026-09-01", time.Date(2026, 8, 31, 18, 0, 0, 0, zone), "unavailable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			target := calendar.LatestCompleted(test.now, marketCutoffHour, marketCutoffMinute)
			if got := quoteFreshness(test.tradeDate, target); got != test.want {
				t.Fatalf("freshness=%s want=%s target=%s", got, test.want, target.Format("2006-01-02"))
			}
		})
	}
}

func TestTWSEQuoteSelectsNewestValidOfficialTradeDate(t *testing.T) {
	tests := []struct {
		name, openAPIDate, openAPIClose, reportDate, reportClose string
		malformedReport                                          bool
		wantDate, wantClose, wantSource                          string
	}{
		{"newer report", "1150828", "2420", "1150831", "2405", false, "2026-08-31", "2405", "twse:www:STOCK_DAY_ALL"},
		{"same date keeps primary", "1150831", "2405", "1150831", "2405", false, "2026-08-31", "2405", "twse:STOCK_DAY_ALL"},
		{"malformed newer report", "1150828", "2420", "1150831", "bad", true, "2026-08-28", "2420", "twse:STOCK_DAY_ALL"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/openapi/") {
					_, _ = fmt.Fprintf(w, `[{"Date":%q,"Code":"2330","Name":"台積電","TradeVolume":"1000","TradeValue":"2420000","OpeningPrice":"2400","HighestPrice":"2445","LowestPrice":"2390","ClosingPrice":%q,"Change":"10"}]`, test.openAPIDate, test.openAPIClose)
					return
				}
				closeValue := test.reportClose
				if test.malformedReport {
					closeValue = "bad"
				}
				_, _ = fmt.Fprintf(w, "日期,證券代號,證券名稱,成交股數,成交金額,開盤價,最高價,最低價,收盤價,漲跌價差,成交筆數\n%q,%q,%q,%q,%q,%q,%q,%q,%q,%q,%q\n", test.reportDate, "2330", "台積電", "1000", "2420000", "2400", "2445", "2390", closeValue, "10", "100")
			}))
			defer server.Close()
			client := NewClient(Config{TWSEBaseURL: server.URL + "/openapi", TWSEReportBaseURL: server.URL + "/www", HTTPClient: server.Client(), Now: fixedQuoteNow})
			quote, err := client.Quote(context.Background(), foundation.SecurityIdentity{Canonical: "2330.TWSE", Code: "2330", Name: "台積電", Exchange: "TWSE"})
			if err != nil {
				t.Fatal(err)
			}
			if quote.Meta.TradeDate != test.wantDate || fmt.Sprint(quote.Price) != test.wantClose || quote.Meta.Source != test.wantSource {
				t.Fatalf("quote=%+v", quote)
			}
		})
	}
}

func TestStaleQuoteSuccessIsNotTransportFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/openapi/") {
			_, _ = w.Write([]byte(`[{"Date":"1150828","Code":"2330","Name":"台積電","TradeVolume":"1000","TradeValue":"2420000","OpeningPrice":"2400","HighestPrice":"2445","LowestPrice":"2390","ClosingPrice":"2420","Change":"10"}]`))
			return
		}
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	client := NewClient(Config{TWSEBaseURL: server.URL + "/openapi", TWSEReportBaseURL: server.URL + "/www", HTTPClient: server.Client(), Now: fixedQuoteNow})
	quote, err := client.Quote(context.Background(), foundation.SecurityIdentity{Canonical: "2330.TWSE", Code: "2330", Name: "台積電", Exchange: "TWSE"})
	if err != nil || quote.Meta.Freshness != "stale" || !quote.Meta.Stale || quote.Meta.FallbackReason != "" {
		t.Fatalf("quote=%+v err=%v", quote, err)
	}
}

func TestTPExQuoteFreshnessRegression(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"Date":"20260828","SecuritiesCompanyCode":"6488","CompanyName":"環球晶","Close":"912","Change":"-39","Open":"951","High":"953","Low":"875","TradingShares":"1000","TransactionAmount":"912000"}]`))
	}))
	defer server.Close()
	client := NewClient(Config{TPExBaseURL: server.URL, HTTPClient: server.Client(), Now: fixedQuoteNow})
	quote, err := client.Quote(context.Background(), foundation.SecurityIdentity{Canonical: "6488.TPEX", Code: "6488", Name: "環球晶", Exchange: "TPEX"})
	if err != nil || quote.Meta.TradeDate != "2026-08-28" || quote.Meta.Freshness != "stale" {
		t.Fatalf("quote=%+v err=%v", quote, err)
	}
}

func fixedQuoteNow() time.Time {
	return time.Date(2026, 8, 31, 18, 0, 0, 0, taipei())
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
