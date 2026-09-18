package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"easy-stock/backend/internal/foundation"
	"easy-stock/backend/internal/marketemotion"
	"easy-stock/backend/internal/sector"
	"easy-stock/backend/internal/stockanalysis"
	"easy-stock/backend/internal/taiwanportfolio"
	"easy-stock/backend/internal/taiwanwatchlist"
)

type dashboardMarketFake struct {
	quotes    map[string]foundation.Quote
	quoteErrs map[string]error
	indexes   []foundation.MarketIndexSeries
	indexErr  error
	indexMeta foundation.SourceMeta
	block     bool
}

func (f dashboardMarketFake) Quote(ctx context.Context, identity foundation.SecurityIdentity) (foundation.Quote, error) {
	if f.block {
		<-ctx.Done()
		return foundation.Quote{}, ctx.Err()
	}
	if err := f.quoteErrs[identity.Canonical]; err != nil {
		return foundation.Quote{}, err
	}
	return f.quotes[identity.Canonical], nil
}
func (f dashboardMarketFake) KLine(context.Context, foundation.SecurityIdentity, int) ([]foundation.KLine, error) {
	return nil, nil
}
func (f dashboardMarketFake) Indexes(ctx context.Context) ([]foundation.MarketIndexSeries, foundation.SourceMeta, error) {
	if f.block {
		<-ctx.Done()
		return nil, foundation.SourceMeta{}, ctx.Err()
	}
	if f.indexErr != nil {
		return nil, foundation.SourceMeta{}, f.indexErr
	}
	indexes := f.indexes
	if indexes == nil {
		indexes = []foundation.MarketIndexSeries{{Index: foundation.MarketIndexSnapshot{ID: "TAIEX", Name: "加權指數", Price: 25000, Change: 100, ChangePercent: .4}}}
	}
	return indexes, f.indexMeta, nil
}

type dashboardBreadthFake struct {
	status string
	err    error
}

func (f dashboardBreadthFake) MarketBreadth(context.Context, time.Time) (foundation.TaiwanMarketBreadth, error) {
	asOf := "2026-09-17"
	scope := foundation.TaiwanBreadthScope{Scope: "COMBINED", Status: f.status, Freshness: f.status, AsOf: &asOf, Advancers: 700, Decliners: 300, TotalAmountTWD: 300_000_000_000}
	ratio := .7
	scope.AdvanceRatio = &ratio
	return foundation.TaiwanMarketBreadth{Combined: scope}, f.err
}

type dashboardEmotionFake struct{ status string }

func (f dashboardEmotionFake) MarketEmotion(context.Context, time.Time) (marketemotion.TaiwanMarketEmotion, error) {
	status := f.status
	if status == "" {
		status = "current"
	}
	return marketemotion.TaiwanMarketEmotion{Combined: marketemotion.TaiwanEmotionScope{Status: status, State: "positive"}}, nil
}

type dashboardIndustryFake struct{ status string }

func (f dashboardIndustryFake) IndustryRadar(context.Context, time.Time) (sector.TaiwanIndustryRadar, error) {
	value := .2
	status := f.status
	if status == "" {
		status = "current"
	}
	return sector.TaiwanIndustryRadar{Combined: sector.TaiwanIndustryScope{Status: status, Industries: []sector.TaiwanIndustry{{IndustryID: "TWSE:24", IndustryName: "半導體業", RelativeBreadth: &value}}}}, nil
}

func newDashboardServer(t *testing.T, market TaiwanMarketProvider, breadth TaiwanBreadthProvider) (*Server, *taiwanwatchlist.Store, *taiwanportfolio.Store) {
	t.Helper()
	watchlist, err := taiwanwatchlist.OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	portfolio, err := taiwanportfolio.OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(Config{TaiwanDirectory: portfolioDirectory{}, TaiwanMarket: market, TaiwanBreadth: breadth, TaiwanEmotion: dashboardEmotionFake{}, TaiwanIndustryRadar: dashboardIndustryFake{}, WatchlistStore: watchlist, TaiwanPortfolioStore: portfolio})
	t.Cleanup(func() { _ = server.Close() })
	return server, watchlist, portfolio
}

func dashboardPayload(t *testing.T, server *Server) taiwanDashboardResponse {
	t.Helper()
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/tw/dashboard", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Data taiwanDashboardResponse `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	return payload.Data
}

func TestTaiwanDashboardEmptyUserStateAndMarketOnly(t *testing.T) {
	server, _, _ := newDashboardServer(t, dashboardMarketFake{}, dashboardBreadthFake{status: "current"})
	data := dashboardPayload(t, server)
	if data.Market.Status != "available" || data.Market.Advancers == nil || *data.Market.Advancers != 700 || len(data.Market.Industries) != 1 {
		t.Fatalf("market=%+v", data.Market)
	}
	if data.Portfolio.HoldingsCount != 0 || data.Watchlist.Count != 0 || data.Alerts.UnreadCount != 0 || len(data.Research.Items) != 0 {
		t.Fatalf("empty state=%+v", data)
	}
}

func TestTaiwanDashboardCombinedPortfolioWatchlistAndUnavailablePrice(t *testing.T) {
	market := dashboardMarketFake{quotes: map[string]foundation.Quote{
		"2330.TWSE": {Symbol: "2330.TWSE", Name: "台積電", Price: 1000, ChangePercent: 3.5, Meta: foundation.SourceMeta{TradeDate: "2026-09-17"}},
	}, quoteErrs: map[string]error{"6488.TPEX": errors.New("provider down")}}
	server, watchlist, portfolio := newDashboardServer(t, market, dashboardBreadthFake{status: "stale"})
	if _, err := watchlist.Add(t.Context(), taiwanwatchlist.Entry{Canonical: "2330.TWSE", Code: "2330", Name: "台積電", Exchange: "TWSE", SecurityType: "stock"}); err != nil {
		t.Fatal(err)
	}
	if _, err := watchlist.Add(t.Context(), taiwanwatchlist.Entry{Canonical: "6488.TPEX", Code: "6488", Name: "環球晶", Exchange: "TPEX", SecurityType: "stock"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := portfolio.Upsert(t.Context(), taiwanportfolio.Holding{Canonical: "2330.TWSE", DisplayName: "台積電", Shares: 10, AverageCost: 900}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := portfolio.Upsert(t.Context(), taiwanportfolio.Holding{Canonical: "6488.TPEX", DisplayName: "環球晶", Shares: 10, AverageCost: 400}); err != nil {
		t.Fatal(err)
	}
	data := dashboardPayload(t, server)
	if data.Portfolio.Status != "partial" || data.Portfolio.PricedCount != 1 || data.Portfolio.UnavailableCount != 1 || data.Portfolio.TotalMarketValue != nil {
		t.Fatalf("portfolio=%+v", data.Portfolio)
	}
	if data.Watchlist.Status != "partial" || data.Watchlist.AvailableCount != 1 || len(data.Watchlist.Movers) != 1 {
		t.Fatalf("watchlist=%+v", data.Watchlist)
	}
	if len(data.Attention) == 0 || data.Attention[0].ReasonCode != "stale_market_data" {
		t.Fatalf("attention=%+v", data.Attention)
	}
}

func TestTaiwanDashboardSectionFailureIsolatedAndPartial(t *testing.T) {
	server, _, _ := newDashboardServer(t, dashboardMarketFake{indexErr: errors.New("indexes down")}, dashboardBreadthFake{status: "current"})
	data := dashboardPayload(t, server)
	if data.Market.Status != "partial" || data.Portfolio.Status != "available" || data.Watchlist.Status != "available" {
		t.Fatalf("sections=%+v", data)
	}
}

func TestTaiwanDashboardUnavailableBreadthDoesNotFabricateZeroValues(t *testing.T) {
	server, _, _ := newDashboardServer(t, dashboardMarketFake{indexErr: errors.New("indexes down")}, dashboardBreadthFake{status: "unavailable"})
	data := dashboardPayload(t, server)
	if data.Market.Status != "partial" || data.Market.Advancers != nil || data.Market.Decliners != nil || data.Market.AdvanceRatio != nil || data.Market.TurnoverTWD != nil {
		t.Fatalf("market=%+v", data.Market)
	}
}

func TestTaiwanDashboardStaleBreadthWithMissingIndexIsPartial(t *testing.T) {
	server, _, _ := newDashboardServer(t, dashboardMarketFake{indexErr: errors.New("indexes down")}, dashboardBreadthFake{status: "stale"})
	data := dashboardPayload(t, server)
	if data.Market.Status != "partial" || data.Market.Reason != "market_section_partial_stale" || data.Market.Advancers == nil || *data.Market.Advancers != 700 || !hasDashboardReason(data.Attention, "stale_market_data") || !hasDashboardReason(data.Attention, "partial_data") {
		t.Fatalf("market=%+v", data.Market)
	}
}

func TestTaiwanDashboardUnavailableMarketUsesDistinctAttentionReason(t *testing.T) {
	items := buildDashboardAttention(taiwanDashboardResponse{Market: dashboardMarket{dashboardSection: dashboardSection{Status: "unavailable"}}})
	if len(items) != 1 || items[0].ReasonCode != "market_data_unavailable" {
		t.Fatalf("items=%+v", items)
	}
}

func TestTaiwanDashboardNilErrorUnavailableSubsourcesRemainUnavailable(t *testing.T) {
	server, _, _ := newDashboardServer(t, dashboardMarketFake{indexErr: errors.New("indexes down")}, dashboardBreadthFake{status: "unavailable"})
	server.taiwanEmotion = dashboardEmotionFake{status: "unavailable"}
	server.taiwanIndustryRadar = dashboardIndustryFake{status: "unavailable"}
	data := dashboardPayload(t, server)
	if data.Market.Status != "unavailable" || data.Market.Emotion != "" || len(data.Market.Industries) != 0 {
		t.Fatalf("market=%+v", data.Market)
	}
}

func TestTaiwanDashboardStaleIndexKeepsFreshnessAndAsOf(t *testing.T) {
	server, _, _ := newDashboardServer(t, dashboardMarketFake{indexMeta: foundation.SourceMeta{Stale: true, TradeDate: "2026-09-16"}}, dashboardBreadthFake{status: "current"})
	data := dashboardPayload(t, server)
	if data.Market.Status != "stale" || len(data.Market.Indexes) != 1 || data.Market.Indexes[0].Status != "stale" || data.Market.Indexes[0].AsOf == nil || *data.Market.Indexes[0].AsOf != "2026-09-16" {
		t.Fatalf("market=%+v", data.Market)
	}
}

func TestTaiwanDashboardOfficialCloseIndexRemainsAvailable(t *testing.T) {
	server, _, _ := newDashboardServer(t, dashboardMarketFake{indexMeta: foundation.SourceMeta{Status: "official_close", TradeDate: "2026-09-17"}}, dashboardBreadthFake{status: "current"})
	data := dashboardPayload(t, server)
	if data.Market.Status != "available" || len(data.Market.Indexes) != 1 || data.Market.Indexes[0].Status != "available" {
		t.Fatalf("market=%+v", data.Market)
	}
}

func TestTaiwanDashboardSkippedMalformedIndexMakesMarketPartial(t *testing.T) {
	indexes := []foundation.MarketIndexSeries{
		{Index: foundation.MarketIndexSnapshot{ID: "TAIEX", Price: 25000, Change: 100, ChangePercent: .4}},
		{Index: foundation.MarketIndexSnapshot{ID: "OTC", Price: math.NaN(), Change: 1, ChangePercent: .1}},
	}
	server, _, _ := newDashboardServer(t, dashboardMarketFake{indexes: indexes, indexMeta: foundation.SourceMeta{Status: "official_close"}}, dashboardBreadthFake{status: "current"})
	data := dashboardPayload(t, server)
	if data.Market.Status != "partial" || len(data.Market.Indexes) != 1 {
		t.Fatalf("market=%+v", data.Market)
	}
}

func TestTaiwanDashboardAlertsAndResearchActivity(t *testing.T) {
	server, watchlist, _ := newDashboardServer(t, dashboardMarketFake{}, dashboardBreadthFake{status: "current"})
	if _, err := watchlist.Add(t.Context(), taiwanwatchlist.Entry{Canonical: "2330.TWSE", Code: "2330", Name: "台積電", Exchange: "TWSE", SecurityType: "stock"}); err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 9, 17, 1, 0, 0, 0, time.UTC)
	baselineTime, newTime := t0.Add(-time.Hour), t0.Add(time.Minute)
	feed := func(id, title, sourceURL string, published *time.Time) foundation.TaiwanCorporateEventFeed {
		return foundation.TaiwanCorporateEventFeed{Status: foundation.TaiwanCorporateEventsAvailable, Provider: "provider", Source: "mops", Events: []foundation.TaiwanCorporateEvent{{ID: id, Symbol: "2330.TWSE", Title: title, PublishedAt: published, Provider: "provider", SourceURL: sourceURL}}}
	}
	if _, err := watchlist.ApplyCorporateEvents(t.Context(), "2330.TWSE", feed("baseline", "baseline", "https://example.com", &baselineTime), t0); err != nil {
		t.Fatal(err)
	}
	if _, err := watchlist.ApplyCorporateEvents(t.Context(), "2330.TWSE", feed("new", "董事會重大決議", "javascript:alert(1)", &newTime), t0.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	_, inserted, err := watchlist.SaveResearchHistory(t.Context(), taiwanwatchlist.ResearchHistoryRecord{
		RunID: "dashboard-research-0001", Canonical: "2330.TWSE", SecurityName: "台積電", CreatedAt: t0,
		ResearchVersion: "taiwan_ai_research_v2", PayloadVersion: "taiwan_ai_research_v2", Completeness: "complete",
		EvidenceSnapshot: json.RawMessage(`{"research_version":"taiwan_ai_research_v2"}`), ResearchResult: json.RawMessage(`{"model_version":"taiwan_ai_research_v2","sections":[]}`), Provenance: json.RawMessage(`{}`), Validity: json.RawMessage(`{}`),
	})
	if err != nil || !inserted {
		t.Fatalf("save research inserted=%v err=%v", inserted, err)
	}
	data := dashboardPayload(t, server)
	if data.Alerts.UnreadCount != 1 || len(data.Alerts.Items) != 1 || data.Alerts.Items[0].SourceURL != "" {
		t.Fatalf("alerts=%+v", data.Alerts)
	}
	if len(data.Research.Items) != 1 || data.Research.Items[0].Canonical != "2330.TWSE" {
		t.Fatalf("research=%+v", data.Research)
	}
	if !hasDashboardReason(data.Attention, "new_corporate_event") {
		t.Fatalf("attention=%+v", data.Attention)
	}
}

func TestTaiwanDashboardWatchlistSortingMalformedAndBounds(t *testing.T) {
	entries := make([]taiwanwatchlist.Entry, 0, 8)
	quotes := make(map[string]portfolioQuoteResult, 8)
	for i := 0; i < 8; i++ {
		canonical := string(rune('A'+i)) + ".TWSE"
		entries = append(entries, taiwanwatchlist.Entry{Canonical: canonical, Name: canonical})
		quotes[canonical] = portfolioQuoteResult{quote: foundation.Quote{Price: 100, ChangePercent: float64(i)}}
	}
	quotes["A.TWSE"] = portfolioQuoteResult{quote: foundation.Quote{Price: math.NaN(), ChangePercent: math.NaN()}, err: errors.New("malformed quote")}
	result := dashboardWatchlistFromQuotes(entries, quotes)
	if result.Count != 8 || result.UnavailableCount != 1 || len(result.Movers) != dashboardListLimit {
		t.Fatalf("result=%+v", result)
	}
	if result.Movers[0].Canonical != "H.TWSE" || result.Movers[4].Canonical != "D.TWSE" {
		t.Fatalf("sorting=%+v", result.Movers)
	}
}

func TestTaiwanDashboardAttentionSortingAndBounds(t *testing.T) {
	data := taiwanDashboardResponse{
		Market:    dashboardMarket{dashboardSection: dashboardSection{Status: "partial"}},
		Portfolio: dashboardPortfolio{dashboardSection: dashboardSection{Status: "partial"}, UnavailableCount: 2},
		Alerts:    dashboardAlerts{Items: []dashboardAlert{{Canonical: "2.TWSE", SecurityName: "B"}, {Canonical: "1.TWSE", SecurityName: "A"}, {Canonical: "3.TWSE", SecurityName: "C"}, {Canonical: "4.TWSE", SecurityName: "D"}, {Canonical: "5.TWSE", SecurityName: "E"}, {Canonical: "6.TWSE", SecurityName: "F"}, {Canonical: "7.TWSE", SecurityName: "G"}}},
	}
	items := buildDashboardAttention(data)
	if len(items) != 8 || items[0].ReasonCode != "partial_data" || items[1].ReasonCode != "price_unavailable" {
		t.Fatalf("items=%+v", items)
	}
	for i := 1; i < len(items); i++ {
		if items[i].Priority > items[i-1].Priority {
			t.Fatalf("not sorted=%+v", items)
		}
	}
}

func TestTaiwanDashboardResearchAttentionReasonIsDeterministic(t *testing.T) {
	comparison := stockanalysis.TaiwanResearchComparisonSummary{Changed: 1}
	data := taiwanDashboardResponse{Market: dashboardMarket{dashboardSection: dashboardSection{Status: "available"}}, Research: dashboardResearch{Items: []dashboardResearchItem{{Canonical: "2330.TWSE", SecurityName: "台積電", Comparison: &comparison}}}}
	first, second := buildDashboardAttention(data), buildDashboardAttention(data)
	if !reflect.DeepEqual(first, second) || len(first) != 1 || first[0].ReasonCode != "research_evidence_changed" || first[0].Target != "taiwan-stock" {
		t.Fatalf("first=%+v second=%+v", first, second)
	}
}

func TestTaiwanDashboardCancellationReturnsWithoutProviderHang(t *testing.T) {
	server, _, _ := newDashboardServer(t, dashboardMarketFake{block: true}, dashboardBreadthFake{status: "current"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() { _ = server.buildTaiwanDashboard(ctx, time.Now()); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("dashboard ignored cancellation")
	}
}

func TestSafeDashboardURLRejectsExecutableAndMalformedSchemes(t *testing.T) {
	for _, value := range []string{"javascript:alert(1)", "data:text/html,bad", "not a url"} {
		if safeDashboardURL(value) != "" {
			t.Fatalf("accepted %q", value)
		}
	}
	if got := safeDashboardURL("https://mops.twse.com.tw/event"); !strings.HasPrefix(got, "https://") {
		t.Fatalf("got=%q", got)
	}
	if got := safeDashboardURL("https://example.com/" + strings.Repeat("a", 2100)); got != "" {
		t.Fatalf("accepted oversized URL length=%d", len(got))
	}
}

func TestTaiwanDashboardMovementStringsAreBounded(t *testing.T) {
	long := strings.Repeat("長", 200)
	result := dashboardWatchlistFromQuotes([]taiwanwatchlist.Entry{{Canonical: long, Name: long}}, map[string]portfolioQuoteResult{long: {quote: foundation.Quote{Price: 1, ChangePercent: 1, Meta: foundation.SourceMeta{TradeDate: long}}}})
	if len(result.Movers) != 1 || len([]rune(result.Movers[0].Canonical)) != 64 || len([]rune(result.Movers[0].Name)) != 80 || result.Movers[0].AsOf == nil || len([]rune(*result.Movers[0].AsOf)) != 64 {
		t.Fatalf("movement=%+v", result.Movers)
	}
}

func hasDashboardReason(items []dashboardAttention, reason string) bool {
	for _, item := range items {
		if item.ReasonCode == reason {
			return true
		}
	}
	return false
}
