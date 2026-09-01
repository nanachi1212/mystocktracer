package taiwan

import (
	"context"
	"fmt"
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
		latest := lines[len(lines)-1]
		if quote.Meta.TradeDate == "" || latest.Meta.TradeDate == "" {
			t.Fatalf("%s quote/kline trade date unavailable: quote=%+v kline=%+v", code, quote.Meta, latest.Meta)
		}
		if quote.TradeTime.Format("2006-01-02") != quote.Meta.TradeDate || latest.Time.Format("2006-01-02") != latest.Meta.TradeDate {
			t.Fatalf("%s quote/kline time metadata mismatch: quote=%+v kline=%+v", code, quote, latest)
		}
		if quote.Meta.TradeDate == latest.Meta.TradeDate && latest.Close != quote.Price {
			t.Fatalf("%s same-date quote/kline mismatch on %s: quote=%v kline=%v", code, quote.Meta.TradeDate, quote.Price, latest.Close)
		}
		if quote.Meta.TradeDate != latest.Meta.TradeDate {
			t.Logf("%s official publication lag: quote date=%s close=%v source=%s; kline date=%s close=%v source=%s", code, quote.Meta.TradeDate, quote.Price, quote.Meta.Source, latest.Meta.TradeDate, latest.Close, latest.Meta.Source)
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

func TestLiveTPExMonthlyKLineUnits(t *testing.T) {
	if os.Getenv("EASY_STOCK_TW_LIVE_TEST") != "1" {
		t.Skip("set EASY_STOCK_TW_LIVE_TEST=1 to query TPEx")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	security := foundation.SecurityIdentity{Canonical: "6488.TPEX", Code: "6488", Name: "環球晶", Exchange: "TPEX"}
	lines, err := NewClient(Config{}).KLine(ctx, security, 1)
	if err != nil || len(lines) != 1 {
		t.Fatalf("6488 monthly K-line: rows=%d err=%v", len(lines), err)
	}
	line := lines[0]
	if line.Symbol != security.Canonical || line.Time.IsZero() || line.Open <= 0 || line.High <= 0 || line.Low <= 0 || line.Close <= 0 {
		t.Fatalf("6488 monthly K-line identity/OHLC: %+v", line)
	}
	if line.Volume <= 0 || line.Amount <= 0 || int64(line.Volume)%1000 != 0 || int64(line.Amount)%1000 != 0 {
		t.Fatalf("6488 monthly K-line units are not normalized shares/TWD: %+v", line)
	}
	t.Logf("6488 date=%s OHLC=%v/%v/%v/%v volume_shares=%.0f amount_TWD=%.0f source=%s", line.Time.Format("2006-01-02"), line.Open, line.High, line.Low, line.Close, line.Volume, line.Amount, line.Meta.Source)
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

func TestLiveOfficialSecurityRuleProfiles(t *testing.T) {
	if os.Getenv("EASY_STOCK_TW_LIVE_TEST") != "1" {
		t.Skip("set EASY_STOCK_TW_LIVE_TEST=1 to query TWSE and TPEx")
	}
	items, err := NewClient(Config{}).Directory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	byCode := map[string]foundation.SecurityIdentity{}
	for _, item := range items {
		byCode[item.Code] = item
	}
	expected := map[string]string{"2330": "official", "6488": "official", "0050": "official", "00646": "official", "00631L": "data_insufficient", "00632R": "data_insufficient"}
	for code, status := range expected {
		item, ok := byCode[code]
		if !ok || item.RuleProfile == nil {
			t.Fatalf("%s official identity/profile unavailable", code)
		}
		if item.RuleProfile.Status != status {
			t.Fatalf("%s profile=%+v", code, item.RuleProfile)
		}
		t.Logf("%s canonical=%s type=%s metadata=%+v profile=%+v", code, item.Canonical, item.Type, item.TaiwanMetadata, item.RuleProfile)
	}
}

func TestLiveOfficialM11SixSymbolFreshness(t *testing.T) {
	if os.Getenv("EASY_STOCK_TW_LIVE_TEST") != "1" {
		t.Skip("set EASY_STOCK_TW_LIVE_TEST=1 to query TWSE and TPEx")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	client := NewClient(Config{})
	target := client.calendar.LatestCompleted(time.Now(), marketCutoffHour, marketCutoffMinute)
	result := client.RefreshMarketWide(ctx, target)
	items, err := client.Directory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byCode := make(map[string]foundation.SecurityIdentity, len(items))
	for _, item := range items {
		byCode[item.Code] = item
	}
	asOf := func(value *string) string {
		if value == nil {
			return "unavailable"
		}
		return *value
	}
	for _, code := range []string{"2330", "6488", "2881", "0050", "00631L", "00632R"} {
		identity, ok := byCode[code]
		if !ok {
			t.Fatalf("%s official identity unavailable", code)
		}
		t.Logf("%s identity=%s exchange=%s instrument=%s daily_as_of=%s daily=%s institutional_as_of=%s institutional=%s margin_as_of=%s margin=%s target=%s", code, identity.Canonical, identity.Exchange, identity.Type, asOf(result.Freshness.DailyAsOf), result.Freshness.DailyStatus, asOf(result.Freshness.InstitutionalAsOf), result.Freshness.InstitutionalStatus, asOf(result.Freshness.MarginAsOf), result.Freshness.MarginStatus, result.Freshness.TargetLatestTradingDate)
	}
	if result.Freshness.TargetLatestTradingDate != target.Format("2006-01-02") {
		t.Fatalf("target trading date=%s want=%s", result.Freshness.TargetLatestTradingDate, target.Format("2006-01-02"))
	}
	if result.OverallStatus == "success" && (result.Freshness.DailyStatus != "current" || result.Freshness.InstitutionalStatus != "current" || result.Freshness.MarginStatus != "current") {
		t.Fatalf("successful refresh reported non-current freshness: %+v", result)
	}
}

func TestLiveOfficialM11QuoteKLineEvidence(t *testing.T) {
	if os.Getenv("EASY_STOCK_TW_LIVE_TEST") != "1" {
		t.Skip("set EASY_STOCK_TW_LIVE_TEST=1 to query TWSE")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := NewClient(Config{})
	items, err := client.Directory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	matches := liveMatches(items, "2330")
	if len(matches) != 1 {
		t.Fatalf("2330 identity count=%d", len(matches))
	}
	quote, err := client.Quote(ctx, matches[0])
	if err != nil {
		t.Fatal(err)
	}
	lines, err := client.KLine(ctx, matches[0], 1)
	if err != nil || len(lines) != 1 {
		t.Fatalf("2330 kline rows=%d err=%v", len(lines), err)
	}
	line := lines[0]
	t.Logf("QUOTE source=%s url=%s fetched_at=%s trade_date=%s trade_time=%s close=%v open=%v high=%v low=%v", quote.Meta.Source, quote.Meta.SourceURL, quote.Meta.FetchedAt.Format(time.RFC3339Nano), quote.Meta.TradeDate, quote.TradeTime.Format(time.RFC3339), quote.Price, quote.Open, quote.High, quote.Low)
	t.Logf("KLINE source=%s url=%s fetched_at=%s trade_date=%s time=%s close=%v open=%v high=%v low=%v volume=%.0f amount=%.0f", line.Meta.Source, line.Meta.SourceURL, line.Meta.FetchedAt.Format(time.RFC3339Nano), line.Meta.TradeDate, line.Time.Format(time.RFC3339), line.Close, line.Open, line.High, line.Low, line.Volume, line.Amount)
}

func TestLiveOfficialM12QuoteFreshness(t *testing.T) {
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
	target := client.calendar.LatestCompleted(client.now(), marketCutoffHour, marketCutoffMinute).Format("2006-01-02")
	for _, code := range []string{"2330", "2881", "0050", "6488", "00631L", "00632R"} {
		matches := liveMatches(items, code)
		if len(matches) != 1 {
			t.Fatalf("%s identity count=%d", code, len(matches))
		}
		quote, err := client.Quote(ctx, matches[0])
		if err != nil {
			t.Logf("%s canonical=%s source=unavailable target_latest_trading_date=%s freshness=unavailable error=%v", code, matches[0].Canonical, target, err)
			continue
		}
		t.Logf("%s canonical=%s source=%s url=%s fetched_at=%s trade_date=%s target_latest_trading_date=%s price=%v freshness=%s fallback=%t", code, matches[0].Canonical, quote.Meta.Source, quote.Meta.SourceURL, quote.Meta.FetchedAt.Format(time.RFC3339Nano), quote.Meta.TradeDate, target, quote.Price, quote.Meta.Freshness, quote.Meta.FallbackReason != "")
		if quote.Meta.Freshness == "" {
			t.Fatalf("%s quote freshness unavailable: %+v", code, quote)
		}
	}
}

func TestLiveOfficialM2AMarketBreadth(t *testing.T) {
	if os.Getenv("EASY_STOCK_TW_LIVE_TEST") != "1" {
		t.Skip("set EASY_STOCK_TW_LIVE_TEST=1 to query official Taiwan market breadth")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	client := NewClient(Config{})
	result, err := client.MarketBreadth(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range []foundation.TaiwanBreadthScope{result.TWSE, result.TPEX, result.Combined} {
		if scope.Advancers+scope.Decliners+scope.Unchanged+scope.NoTrade+scope.Unknown != scope.UniverseCount {
			t.Fatalf("%s sanity equation failed: %+v", scope.Scope, scope)
		}
		bucketAmount := scope.AdvancingAmountTWD + scope.DecliningAmountTWD + scope.UnchangedAmountTWD + scope.NoTradeAmountTWD + scope.UnknownAmountTWD
		if bucketAmount > scope.TotalAmountTWD+0.01 {
			t.Fatalf("%s amount buckets exceed total: %+v", scope.Scope, scope)
		}
		t.Logf("scope=%s as_of=%s target=%s freshness=%s status=%s universe=%s universe_count=%d traded_count=%d advancers=%d decliners=%d unchanged=%d no_trade=%d unknown=%d advance_decline_diff=%d advance_ratio=%s total_amount_twd=%.0f included=%v missing=%v", scope.Scope, liveString(scope.AsOf), scope.TargetLatestTradingDate, scope.Freshness, scope.Status, scope.Universe, scope.UniverseCount, scope.TradedCount, scope.Advancers, scope.Decliners, scope.Unchanged, scope.NoTrade, scope.Unknown, scope.AdvanceDeclineDiff, liveRatio(scope.AdvanceRatio), scope.TotalAmountTWD, scope.IncludedExchanges, scope.MissingExchanges)
	}
	items, err := client.Directory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"2330", "2881", "6488", "0050", "00631L", "00632R"} {
		matches := liveMatches(items, code)
		if len(matches) != 1 {
			t.Fatalf("%s identity count=%d", code, len(matches))
		}
		included := matches[0].Type == foundation.SecurityTypeStock
		t.Logf("code=%s canonical=%s exchange=%s type=%s included=%t reason=%s", code, matches[0].Canonical, matches[0].Exchange, matches[0].Type, included, map[bool]string{true: "SecurityTypeStock", false: "stock_breadth excludes non-stock"}[included])
	}
}

func liveString(value *string) string {
	if value == nil {
		return "unavailable"
	}
	return *value
}

func liveRatio(value *float64) string {
	if value == nil {
		return "unavailable"
	}
	return fmt.Sprintf("%.6f", *value)
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
