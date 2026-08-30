package taiwan

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"easy-stock/backend/internal/foundation"
)

func TestLiveOfficialDirectorySmoke(t *testing.T) {
	if os.Getenv("EASY_STOCK_TW_LIVE_TEST") != "1" {
		t.Skip("set EASY_STOCK_TW_LIVE_TEST=1 to query TWSE and TPEx")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	items, err := NewClient(Config{}).Directory(ctx)
	if err != nil {
		t.Fatalf("official directory unavailable: %v", err)
	}
	expected := map[string]struct {
		name, canonical, exchange string
		securityType              foundation.SecurityType
	}{
		"2330": {"台積電", "2330.TWSE", "TWSE", foundation.SecurityTypeStock},
		"2317": {"鴻海", "2317.TWSE", "TWSE", foundation.SecurityTypeStock},
		"2454": {"聯發科", "2454.TWSE", "TWSE", foundation.SecurityTypeStock},
		"6488": {"環球晶", "6488.TPEX", "TPEX", foundation.SecurityTypeStock},
		"0050": {"元大台灣50", "0050.TWSE", "TWSE", foundation.SecurityTypeETF},
	}
	for code, want := range expected {
		byCode := liveMatches(items, code)
		byName := liveMatches(items, want.name)
		if len(byCode) != 1 {
			t.Fatalf("code %s returned %d identities: %+v", code, len(byCode), byCode)
		}
		if len(byName) != 1 {
			t.Fatalf("name %s returned %d identities: %+v", want.name, len(byName), byName)
		}
		got := byCode[0]
		if got != byName[0] {
			t.Fatalf("code/name mismatch for %s: code=%+v name=%+v", code, got, byName[0])
		}
		if got.Canonical != want.canonical || got.Exchange != want.exchange || got.Type != want.securityType || got.Name != want.name {
			t.Fatalf("identity mismatch for %s: got=%+v want=%+v", code, got, want)
		}
		if got.Market != "TW" || got.Currency != "TWD" || got.Timezone != "Asia/Taipei" || got.SourceURL == "" || got.RetrievedAt.IsZero() {
			t.Fatalf("metadata incomplete for %s: %+v", code, got)
		}
	}
}

func TestLiveOfficialMarketSmoke(t *testing.T) {
	if os.Getenv("EASY_STOCK_TW_LIVE_TEST") != "1" {
		t.Skip("set EASY_STOCK_TW_LIVE_TEST=1 to query TWSE and TPEx")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	client := NewClient(Config{})
	items, err := client.Directory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"2330", "2317", "2454", "6488", "0050"} {
		matches := liveMatches(items, code)
		if len(matches) != 1 {
			t.Fatalf("%s identity count=%d", code, len(matches))
		}
		security := matches[0]
		quote, err := client.Quote(ctx, security)
		if err != nil {
			t.Fatalf("%s quote: %v", code, err)
		}
		if quote.Symbol != security.Canonical || quote.Name != security.Name || quote.Price <= 0 || quote.Meta.SourceURL == "" || quote.Meta.IsRealtime {
			t.Fatalf("%s quote mismatch: %+v", code, quote)
		}
		lines, err := client.KLine(ctx, security, 5)
		if err != nil || len(lines) != 5 {
			t.Fatalf("%s kline rows=%d err=%v", code, len(lines), err)
		}
		if lines[len(lines)-1].Close != quote.Price {
			t.Fatalf("%s quote/kline mismatch: quote=%v kline=%v", code, quote.Price, lines[len(lines)-1].Close)
		}
		institutional, err := client.Institutional(ctx, security, 1)
		if err != nil || len(institutional.Data) != 1 || institutional.Data[0].Canonical != security.Canonical || institutional.Data[0].Unit != "shares" {
			t.Fatalf("%s institutional: %+v err=%v", code, institutional, err)
		}
		margin, err := client.Margin(ctx, security, 1)
		if err != nil || len(margin.Data) != 1 || margin.Data[0].Canonical != security.Canonical || margin.Data[0].Unit != "shares" {
			t.Fatalf("%s margin: %+v err=%v", code, margin, err)
		}
	}
	indexes, _, err := client.Indexes(ctx)
	if err != nil || len(indexes) != 2 || indexes[0].Index.ID != "taiex" || indexes[1].Index.ID != "tpex" {
		t.Fatalf("indexes=%+v err=%v", indexes, err)
	}
}

func TestLiveOfficialFundamentalsSmoke(t *testing.T) {
	if os.Getenv("EASY_STOCK_TW_LIVE_TEST") != "1" {
		t.Skip("set EASY_STOCK_TW_LIVE_TEST=1 to query official fundamentals and FinMind history")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	client := NewClient(Config{})
	items, err := client.Directory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"2330", "6488", "2881", "0050"} {
		matches := liveMatches(items, code)
		if len(matches) != 1 {
			t.Fatalf("%s identity count=%d", code, len(matches))
		}
		got, err := client.Fundamentals(ctx, matches[0], 24)
		if err != nil {
			t.Fatalf("%s fundamentals: %v", code, err)
		}
		if got.Security.Canonical != matches[0].Canonical {
			t.Fatalf("%s canonical mismatch: %+v", code, got.Security)
		}
		if code == "0050" {
			if got.Capabilities["monthly_revenue"].Status != "unsupported" || got.Capabilities["financial_statement"].Status != "unsupported" {
				t.Fatalf("0050 capabilities=%+v", got.Capabilities)
			}
			t.Logf("%s canonical=%s capabilities=%+v", code, got.Security.Canonical, got.Capabilities)
			continue
		}
		if len(got.Revenue) == 0 || got.Revenue[len(got.Revenue)-1].Provider != officialProvider(matches[0]) {
			t.Fatalf("%s revenue=%+v", code, got.Revenue)
		}
		if got.Statement == nil || got.Statement.CumulativeEPS == nil || got.Valuation == nil {
			t.Fatalf("%s incomplete fundamentals=%+v", code, got)
		}
		if code == "2330" && got.Capabilities["dividends"].Status != "official" {
			t.Fatalf("2330 dividend capability=%+v", got.Capabilities["dividends"])
		}
		if code == "6488" && got.Statement.AccountingCategory != "ci" {
			t.Fatalf("6488 category=%s", got.Statement.AccountingCategory)
		}
		if code == "2881" && got.Statement.AccountingCategory != "fh" {
			t.Fatalf("2881 category=%s", got.Statement.AccountingCategory)
		}
		t.Logf("%s canonical=%s revenue=%s category=%s valuation=%s dividends=%s", code, got.Security.Canonical, got.Revenue[len(got.Revenue)-1].Period, got.Statement.AccountingCategory, got.Capabilities["valuation"].Status, got.Capabilities["dividends"].Status)
	}
}

func liveMatches(items []foundation.SecurityIdentity, query string) []foundation.SecurityIdentity {
	query = strings.ToLower(strings.TrimSpace(query))
	matches := make([]foundation.SecurityIdentity, 0, 1)
	for _, item := range items {
		if strings.ToLower(strings.TrimSpace(item.Code)) == query || strings.ToLower(strings.TrimSpace(item.Name)) == query || strings.ToLower(strings.TrimSpace(item.FullName)) == query {
			matches = append(matches, item)
		}
	}
	return matches
}
