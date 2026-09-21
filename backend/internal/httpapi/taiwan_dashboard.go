package httpapi

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nanachi1212/mystocktracer/backend/internal/foundation"
	"github.com/nanachi1212/mystocktracer/backend/internal/stockanalysis"
	"github.com/nanachi1212/mystocktracer/backend/internal/taiwanportfolio"
	"github.com/nanachi1212/mystocktracer/backend/internal/taiwanwatchlist"
)

const (
	dashboardListLimit   = 5
	dashboardSymbolLimit = 100
)

type dashboardSection struct {
	Status string  `json:"status"`
	AsOf   *string `json:"as_of"`
	Reason string  `json:"reason,omitempty"`
}

type dashboardMarket struct {
	dashboardSection
	Indexes      []dashboardMarketIndex `json:"indexes"`
	Advancers    *int                   `json:"advancers"`
	Decliners    *int                   `json:"decliners"`
	AdvanceRatio *float64               `json:"advance_ratio"`
	TurnoverTWD  *float64               `json:"turnover_twd"`
	Emotion      string                 `json:"emotion,omitempty"`
	Industries   []dashboardIndustry    `json:"industries"`
}

type dashboardMarketIndex struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Price         float64 `json:"price"`
	Change        float64 `json:"change"`
	ChangePercent float64 `json:"change_percent"`
	Status        string  `json:"status"`
	AsOf          *string `json:"as_of"`
}

type dashboardIndustry struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	RelativeBreadth *float64 `json:"relative_breadth"`
	RelativeCapital *float64 `json:"relative_capital"`
}

type dashboardPortfolio struct {
	dashboardSection
	HoldingsCount     int                                    `json:"holdings_count"`
	PricedCount       int                                    `json:"priced_count"`
	UnavailableCount  int                                    `json:"unavailable_count"`
	TotalMarketValue  *float64                               `json:"total_market_value"`
	TotalUnrealizedPL *float64                               `json:"total_unrealized_pl"`
	LargestWeight     *float64                               `json:"largest_weight_percent"`
	Top3Percent       *float64                               `json:"top_3_percent"`
	Industries        []taiwanPortfolioIndustryConcentration `json:"industries"`
	Movers            []dashboardSecurityMovement            `json:"movers"`
}

type dashboardWatchlist struct {
	dashboardSection
	Count            int                         `json:"count"`
	AvailableCount   int                         `json:"available_count"`
	UnavailableCount int                         `json:"unavailable_count"`
	Movers           []dashboardSecurityMovement `json:"movers"`
}

type dashboardSecurityMovement struct {
	Canonical     string   `json:"canonical"`
	Name          string   `json:"name"`
	Price         *float64 `json:"price"`
	ChangePercent *float64 `json:"change_percent"`
	Status        string   `json:"status"`
	AsOf          *string  `json:"as_of"`
}

type dashboardAlerts struct {
	dashboardSection
	UnreadCount int              `json:"unread_count"`
	Items       []dashboardAlert `json:"items"`
}

type dashboardAlert struct {
	ID           int64      `json:"id"`
	Canonical    string     `json:"canonical"`
	SecurityName string     `json:"security_name"`
	Title        string     `json:"title"`
	PublishedAt  *time.Time `json:"published_at,omitempty"`
	Source       string     `json:"source"`
	SourceURL    string     `json:"source_url,omitempty"`
	Stale        bool       `json:"stale"`
	Partial      bool       `json:"partial"`
}

type dashboardResearch struct {
	dashboardSection
	Items []dashboardResearchItem `json:"items"`
}

type dashboardResearchItem struct {
	RunID        string                                         `json:"run_id"`
	Canonical    string                                         `json:"canonical"`
	SecurityName string                                         `json:"security_name"`
	CreatedAt    time.Time                                      `json:"created_at"`
	EvidenceAsOf string                                         `json:"evidence_as_of,omitempty"`
	Completeness string                                         `json:"completeness"`
	Stale        bool                                           `json:"stale"`
	Partial      bool                                           `json:"partial"`
	HasPrevious  bool                                           `json:"has_previous"`
	Comparison   *stockanalysis.TaiwanResearchComparisonSummary `json:"comparison,omitempty"`
}

type dashboardAttention struct {
	ReasonCode string `json:"reason_code"`
	Priority   int    `json:"priority"`
	Canonical  string `json:"canonical,omitempty"`
	Title      string `json:"title"`
	Target     string `json:"target"`
}

type taiwanDashboardResponse struct {
	GeneratedAt time.Time            `json:"generated_at"`
	Market      dashboardMarket      `json:"market"`
	Portfolio   dashboardPortfolio   `json:"portfolio"`
	Watchlist   dashboardWatchlist   `json:"watchlist"`
	Alerts      dashboardAlerts      `json:"alerts"`
	Research    dashboardResearch    `json:"research"`
	Attention   []dashboardAttention `json:"attention"`
}

func (s *Server) taiwanDashboardHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	data := s.buildTaiwanDashboard(ctx, time.Now())
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (s *Server) buildTaiwanDashboard(ctx context.Context, now time.Time) taiwanDashboardResponse {
	result := taiwanDashboardResponse{GeneratedAt: now.UTC(), Attention: []dashboardAttention{}}
	result.Market = dashboardMarket{dashboardSection: dashboardSection{Status: "not_queried"}, Indexes: []dashboardMarketIndex{}, Industries: []dashboardIndustry{}}
	result.Portfolio = dashboardPortfolio{dashboardSection: dashboardSection{Status: "not_queried"}, Industries: []taiwanPortfolioIndustryConcentration{}, Movers: []dashboardSecurityMovement{}}
	result.Watchlist = dashboardWatchlist{dashboardSection: dashboardSection{Status: "not_queried"}, Movers: []dashboardSecurityMovement{}}
	result.Alerts = dashboardAlerts{dashboardSection: dashboardSection{Status: "not_queried"}, Items: []dashboardAlert{}}
	result.Research = dashboardResearch{dashboardSection: dashboardSection{Status: "not_queried"}, Items: []dashboardResearchItem{}}

	var holdings []taiwanportfolio.Holding
	var entries []taiwanwatchlist.Entry
	var holdingsErr, entriesErr error
	var wg sync.WaitGroup
	run := func(fn func()) { wg.Add(1); go func() { defer wg.Done(); fn() }() }
	run(func() { result.Market = s.dashboardMarket(ctx, now) })
	run(func() {
		if s.taiwanPortfolioStore == nil {
			holdingsErr = errors.New("portfolio storage unavailable")
			return
		}
		holdings, holdingsErr = s.taiwanPortfolioStore.List(ctx)
	})
	run(func() {
		if s.watchlistStore == nil {
			entriesErr = errors.New("watchlist storage unavailable")
			return
		}
		entries, entriesErr = s.watchlistStore.List(ctx)
	})
	run(func() { result.Alerts = s.dashboardAlerts(ctx) })
	run(func() { result.Research = s.dashboardResearch(ctx) })
	wg.Wait()

	identities := s.portfolioIdentities(ctx)
	quoteHoldings := append([]taiwanportfolio.Holding(nil), holdings...)
	if len(quoteHoldings) > dashboardSymbolLimit {
		quoteHoldings = quoteHoldings[:dashboardSymbolLimit]
	}
	seen := make(map[string]bool, len(quoteHoldings))
	for _, holding := range quoteHoldings {
		seen[holding.Canonical] = true
	}
	for _, entry := range entries {
		if len(quoteHoldings) >= dashboardSymbolLimit {
			break
		}
		if !seen[entry.Canonical] {
			quoteHoldings = append(quoteHoldings, taiwanportfolio.Holding{Canonical: entry.Canonical, DisplayName: entry.Name})
			seen[entry.Canonical] = true
		}
		if _, ok := identities[entry.Canonical]; !ok {
			identities[entry.Canonical] = foundation.SecurityIdentity{Canonical: entry.Canonical, Code: entry.Code, Name: entry.Name, Exchange: entry.Exchange, Type: foundation.SecurityType(entry.SecurityType)}
		}
	}
	quotes := s.portfolioQuotes(ctx, quoteHoldings, identities)
	if holdingsErr != nil {
		result.Portfolio.dashboardSection = dashboardSection{Status: "unavailable", Reason: "portfolio_storage_unavailable"}
	} else {
		result.Portfolio = dashboardPortfolioFromSummary(calculateTaiwanPortfolioSummary(holdings, identities, quotes))
	}
	if entriesErr != nil {
		result.Watchlist.dashboardSection = dashboardSection{Status: "unavailable", Reason: "watchlist_storage_unavailable"}
	} else {
		result.Watchlist = dashboardWatchlistFromQuotes(entries, quotes)
	}

	result.Attention = buildDashboardAttention(result)
	return result
}

func (s *Server) dashboardMarket(ctx context.Context, now time.Time) dashboardMarket {
	result := dashboardMarket{dashboardSection: dashboardSection{Status: "unavailable", Reason: "market_data_unavailable"}, Indexes: []dashboardMarketIndex{}, Industries: []dashboardIndustry{}}
	var indexes []foundation.MarketIndexSeries
	var indexMeta foundation.SourceMeta
	var indexErr, breadthErr, emotionErr, industryErr error
	var breadth foundation.TaiwanBreadthScope
	var emotion, emotionStatus string
	var emotionAsOf *string
	var industryStatus string
	var industryAsOf *string
	var industries []dashboardIndustry
	var wg sync.WaitGroup
	if s.taiwanMarket != nil {
		wg.Add(1)
		go func() { defer wg.Done(); indexes, indexMeta, indexErr = s.taiwanMarket.Indexes(ctx) }()
	} else {
		indexErr = errors.New("market provider unavailable")
	}
	if s.taiwanBreadth != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value, err := s.taiwanBreadth.MarketBreadth(ctx, now)
			breadth, breadthErr = value.Combined, err
		}()
	} else {
		breadthErr = errors.New("breadth unavailable")
	}
	if s.taiwanEmotion != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value, err := s.taiwanEmotion.MarketEmotion(ctx, now)
			emotion, emotionStatus, emotionAsOf, emotionErr = value.Combined.State, normalizeDashboardStatus(value.Combined.Status), value.Combined.AsOf, err
		}()
	} else {
		emotionErr = errors.New("emotion unavailable")
	}
	if s.taiwanIndustryRadar != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value, err := s.taiwanIndustryRadar.IndustryRadar(ctx, now)
			industryErr = err
			if err == nil {
				industryStatus, industryAsOf = normalizeDashboardStatus(value.Combined.Status), value.Combined.AsOf
				for _, item := range value.Combined.Industries {
					if len(industries) >= dashboardListLimit {
						break
					}
					industries = append(industries, dashboardIndustry{ID: boundedDashboardText(item.IndustryID, 40), Name: boundedDashboardText(item.IndustryName, 80), RelativeBreadth: safeFloatPointer(item.RelativeBreadth), RelativeCapital: safeFloatPointer(item.RelativeCapital)})
				}
			}
		}()
	} else {
		industryErr = errors.New("industry unavailable")
	}
	wg.Wait()
	indexStale, indexPartial := false, false
	for _, item := range indexes {
		if len(result.Indexes) >= 3 {
			break
		}
		meta := item.Index.Meta
		if meta.Status == "" && meta.Freshness == "" && !meta.Stale {
			meta = item.Meta
		}
		if meta.Status == "" && meta.Freshness == "" && !meta.Stale {
			meta = indexMeta
		}
		status := "available"
		if item.Index.Status != "" {
			status = normalizeDashboardStatus(item.Index.Status)
		} else if meta.Status != "" {
			status = normalizeDashboardStatus(meta.Status)
		}
		if meta.Stale || meta.Freshness == "stale" {
			status = "stale"
		}
		if (status == "available" || status == "stale" || status == "partial") && finite(item.Index.Price) && finite(item.Index.Change) && finite(item.Index.ChangePercent) {
			index := dashboardMarketIndex{ID: boundedDashboardText(item.Index.ID, 40), Name: boundedDashboardText(item.Index.Name, 80), Price: item.Index.Price, Change: item.Index.Change, ChangePercent: item.Index.ChangePercent, Status: status}
			if !item.Index.TradeTime.IsZero() {
				value := item.Index.TradeTime.UTC().Format(time.RFC3339)
				index.AsOf = &value
			} else if meta.TradeDate != "" {
				value := boundedDashboardText(meta.TradeDate, 64)
				index.AsOf = &value
			}
			result.Indexes = append(result.Indexes, index)
			indexStale = indexStale || status == "stale"
			indexPartial = indexPartial || status == "partial"
		} else {
			indexPartial = true
		}
	}
	breadthUsable := false
	if breadthErr == nil {
		status := normalizeDashboardStatus(breadth.Status)
		if status == "available" || status == "stale" || status == "partial" {
			breadthUsable = true
			result.Status, result.AsOf, result.Reason = status, boundedDashboardStringPointer(breadth.AsOf, 64), ""
			if breadth.Advancers >= 0 && breadth.Decliners >= 0 {
				result.Advancers, result.Decliners = intPointer(breadth.Advancers), intPointer(breadth.Decliners)
			}
			result.AdvanceRatio = safeFloatPointer(breadth.AdvanceRatio)
			if finite(breadth.TotalAmountTWD) {
				result.TurnoverTWD = floatPointer(breadth.TotalAmountTWD)
			}
			if status == "partial" {
				result.Reason = "market_breadth_partial"
			}
		}
	}
	emotionUsable := emotionErr == nil && dashboardStatusUsable(emotionStatus)
	industryUsable := industryErr == nil && dashboardStatusUsable(industryStatus)
	if emotionUsable {
		result.Emotion = boundedDashboardText(emotion, 40)
	}
	if industryUsable {
		result.Industries = industries
	}
	if result.AsOf == nil && len(result.Indexes) > 0 {
		result.AsOf = result.Indexes[0].AsOf
	}
	if result.AsOf == nil && emotionUsable {
		result.AsOf = boundedDashboardStringPointer(emotionAsOf, 64)
	}
	if result.AsOf == nil && industryUsable {
		result.AsOf = boundedDashboardStringPointer(industryAsOf, 64)
	}
	indexUsable := indexErr == nil && len(result.Indexes) > 0
	anyUsable := breadthUsable || indexUsable || emotionUsable || industryUsable
	anyUnavailable := !breadthUsable || !indexUsable || !emotionUsable || !industryUsable
	anyPartial := normalizeDashboardStatus(breadth.Status) == "partial" || indexPartial || emotionStatus == "partial" || industryStatus == "partial"
	anyStale := normalizeDashboardStatus(breadth.Status) == "stale" || indexStale || emotionStatus == "stale" || industryStatus == "stale"
	switch {
	case !anyUsable:
		result.Status, result.Reason = "unavailable", "market_data_unavailable"
	case anyUnavailable || anyPartial:
		result.Status, result.Reason = "partial", "market_section_partial"
		if anyStale {
			result.Reason = "market_section_partial_stale"
		}
	case anyStale:
		result.Status, result.Reason = "stale", "market_section_stale"
	default:
		result.Status, result.Reason = "available", ""
	}
	return result
}

func dashboardPortfolioFromSummary(summary taiwanPortfolioSummary) dashboardPortfolio {
	result := dashboardPortfolio{dashboardSection: dashboardSection{Status: normalizeDashboardStatus(summary.Status)}, HoldingsCount: summary.HoldingsCount, PricedCount: summary.PricedHoldings, UnavailableCount: summary.HoldingsCount - summary.PricedHoldings, TotalMarketValue: summary.TotalMarketValue, TotalUnrealizedPL: summary.TotalUnrealizedPL, Top3Percent: summary.Concentration.Top3Percent, Industries: []taiwanPortfolioIndustryConcentration{}, Movers: []dashboardSecurityMovement{}}
	for _, industry := range summary.Concentration.Industries {
		if len(result.Industries) >= 3 {
			break
		}
		result.Industries = append(result.Industries, taiwanPortfolioIndustryConcentration{Industry: boundedDashboardText(industry.Industry, 80), MarketValue: industry.MarketValue, Weight: industry.Weight, Holdings: industry.Holdings})
	}
	for _, item := range summary.Holdings {
		if item.Weight != nil && (result.LargestWeight == nil || *item.Weight > *result.LargestWeight) {
			value := *item.Weight
			result.LargestWeight = &value
		}
		if item.ChangePercent != nil {
			result.Movers = append(result.Movers, movementFromHolding(item))
		}
		if result.AsOf == nil && item.QuoteMeta != nil && item.QuoteMeta.TradeDate != "" {
			value := boundedDashboardText(item.QuoteMeta.TradeDate, 64)
			result.AsOf = &value
		}
	}
	sortMovements(result.Movers)
	if len(result.Movers) > dashboardListLimit {
		result.Movers = result.Movers[:dashboardListLimit]
	}
	return result
}

func dashboardWatchlistFromQuotes(entries []taiwanwatchlist.Entry, quotes map[string]portfolioQuoteResult) dashboardWatchlist {
	result := dashboardWatchlist{dashboardSection: dashboardSection{Status: "available"}, Count: len(entries), Movers: []dashboardSecurityMovement{}}
	for _, entry := range entries {
		quote, ok := quotes[entry.Canonical]
		if !ok || quote.err != nil {
			result.UnavailableCount++
			continue
		}
		status := "available"
		if quote.quote.Meta.Stale || quote.quote.Meta.Freshness == "stale" {
			status = "stale"
			result.Status = "stale"
		}
		price := quote.quote.Price
		item := dashboardSecurityMovement{Canonical: boundedDashboardText(entry.Canonical, 64), Name: boundedDashboardText(entry.Name, 80), Price: floatPointer(price), ChangePercent: floatPointer(quote.quote.ChangePercent), Status: status}
		if quote.quote.Meta.TradeDate != "" {
			value := boundedDashboardText(quote.quote.Meta.TradeDate, 64)
			item.AsOf = &value
			if result.AsOf == nil {
				result.AsOf = &value
			}
		}
		if item.ChangePercent != nil {
			result.Movers = append(result.Movers, item)
		}
		result.AvailableCount++
	}
	if result.UnavailableCount > 0 {
		result.Status = "partial"
	}
	sortMovements(result.Movers)
	if len(result.Movers) > dashboardListLimit {
		result.Movers = result.Movers[:dashboardListLimit]
	}
	return result
}

func (s *Server) dashboardAlerts(ctx context.Context) dashboardAlerts {
	result := dashboardAlerts{dashboardSection: dashboardSection{Status: "unavailable", Reason: "alert_storage_unavailable"}, Items: []dashboardAlert{}}
	if s.watchlistStore == nil {
		return result
	}
	page, err := s.watchlistStore.ListAlerts(ctx, taiwanwatchlist.AlertFilterUnread, dashboardListLimit, 0)
	if err != nil {
		return result
	}
	result.Status = "available"
	result.Reason = ""
	result.UnreadCount = page.UnreadCount
	for _, item := range page.Alerts {
		result.Items = append(result.Items, dashboardAlert{ID: item.ID, Canonical: boundedDashboardText(item.Canonical, 64), SecurityName: boundedDashboardText(item.SecurityName, 80), Title: boundedDashboardText(item.Title, 240), PublishedAt: item.PublishedAt, Source: boundedDashboardText(item.Source, 80), SourceURL: safeDashboardURL(item.SourceURL), Stale: item.Stale, Partial: item.Partial})
	}
	if len(result.Items) > 0 {
		value := result.Items[0].PublishedAt
		if value != nil {
			text := value.UTC().Format(time.RFC3339)
			result.AsOf = &text
		}
	}
	return result
}

func (s *Server) dashboardResearch(ctx context.Context) dashboardResearch {
	result := dashboardResearch{dashboardSection: dashboardSection{Status: "unavailable", Reason: "research_storage_unavailable"}, Items: []dashboardResearchItem{}}
	if s.watchlistStore == nil {
		return result
	}
	runs, err := s.watchlistStore.ListRecentResearchHistory(ctx, dashboardListLimit)
	if err != nil {
		return result
	}
	result.Status = "available"
	result.Reason = ""
	for _, run := range runs {
		item := dashboardResearchItem{RunID: boundedDashboardText(run.RunID, 128), Canonical: boundedDashboardText(run.Canonical, 64), SecurityName: boundedDashboardText(run.SecurityName, 80), CreatedAt: run.CreatedAt, EvidenceAsOf: boundedDashboardText(run.EvidenceAsOf, 64), Completeness: boundedDashboardText(run.Completeness, 32), Stale: run.Stale, Partial: run.Partial, HasPrevious: run.HasPrevious}
		if run.HasPrevious {
			current, e1 := s.watchlistStore.GetResearchHistory(ctx, run.Canonical, run.RunID)
			previous, e2 := s.watchlistStore.PreviousResearchHistory(ctx, run.Canonical, run.RunID)
			if e1 == nil && e2 == nil {
				comparison := stockanalysis.CompareTaiwanResearchHistory(current.RunID, previous.RunID, current.ResearchVersion, previous.ResearchVersion, current.PayloadVersion, previous.PayloadVersion, current.EvidenceSnapshot, previous.EvidenceSnapshot, current.ResearchResult, previous.ResearchResult)
				if comparison.Comparable {
					summary := comparison.Summary
					item.Comparison = &summary
				}
			}
		}
		result.Items = append(result.Items, item)
	}
	if len(result.Items) > 0 {
		text := result.Items[0].CreatedAt.UTC().Format(time.RFC3339)
		result.AsOf = &text
	}
	return result
}

func buildDashboardAttention(data taiwanDashboardResponse) []dashboardAttention {
	items := []dashboardAttention{}
	add := func(reason string, priority int, canonical, title, target string) {
		items = append(items, dashboardAttention{ReasonCode: reason, Priority: priority, Canonical: canonical, Title: title, Target: target})
	}
	if data.Market.Status == "unavailable" {
		add("market_data_unavailable", 100, "", "市場資料目前無法取得", "taiwan-overview")
	} else if data.Market.Status == "stale" {
		add("stale_market_data", 95, "", "市場資料較舊", "taiwan-overview")
	} else if data.Market.Status == "partial" {
		add("partial_data", 90, "", "市場資料僅部分可用", "taiwan-overview")
		if strings.Contains(data.Market.Reason, "stale") {
			add("stale_market_data", 95, "", "部分市場資料較舊", "taiwan-overview")
		}
	}
	if data.Portfolio.UnavailableCount > 0 {
		add("price_unavailable", 85, "", "有持股缺少可用價格", "taiwan-portfolio")
	}
	for _, alert := range data.Alerts.Items {
		add("new_corporate_event", 80, alert.Canonical, alert.SecurityName+"有未讀公司事件", "taiwan-alerts")
	}
	for _, research := range data.Research.Items {
		if research.Stale || research.Partial {
			add("partial_data", 65, research.Canonical, research.SecurityName+"研究證據不完整", "taiwan-stock")
		}
		if research.Comparison != nil && (research.Comparison.Changed > 0 || research.Comparison.NewlyAvailable > 0 || research.Comparison.NoLongerAvailable > 0 || research.Comparison.CorporateEventsAdded > 0) {
			add("research_evidence_changed", 60, research.Canonical, research.SecurityName+"研究證據已有變化", "taiwan-stock")
		}
	}
	if data.Portfolio.LargestWeight != nil && *data.Portfolio.LargestWeight >= 50 {
		add("portfolio_concentration", 40, "", fmt.Sprintf("最大持股權重 %.2f%%（描述性集中度）", *data.Portfolio.LargestWeight), "taiwan-portfolio")
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Priority == items[j].Priority {
			if items[i].ReasonCode == items[j].ReasonCode {
				return items[i].Canonical < items[j].Canonical
			}
			return items[i].ReasonCode < items[j].ReasonCode
		}
		return items[i].Priority > items[j].Priority
	})
	if len(items) > 8 {
		items = items[:8]
	}
	return items
}

func normalizeDashboardStatus(status string) string {
	switch status {
	case "current", "available", "official_close":
		return "available"
	case "stale", "partial", "unavailable", "not_queried":
		return status
	case "empty":
		return "available"
	default:
		return "unavailable"
	}
}
func dashboardStatusUsable(status string) bool {
	return status == "available" || status == "stale" || status == "partial"
}
func intPointer(value int) *int { return &value }
func floatPointer(value float64) *float64 {
	if !finite(value) {
		return nil
	}
	return &value
}
func safeFloatPointer(value *float64) *float64 {
	if value == nil || !finite(*value) {
		return nil
	}
	copy := *value
	return &copy
}
func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
func movementFromHolding(item taiwanPortfolioHoldingView) dashboardSecurityMovement {
	result := dashboardSecurityMovement{Canonical: boundedDashboardText(item.Canonical, 64), Name: boundedDashboardText(item.Name, 80), Price: safeFloatPointer(item.CurrentPrice), ChangePercent: safeFloatPointer(item.ChangePercent), Status: boundedDashboardText(item.PriceStatus, 32)}
	if item.QuoteMeta != nil && item.QuoteMeta.TradeDate != "" {
		value := boundedDashboardText(item.QuoteMeta.TradeDate, 64)
		result.AsOf = &value
	}
	return result
}
func sortMovements(items []dashboardSecurityMovement) {
	sort.SliceStable(items, func(i, j int) bool {
		ai, aj := 0.0, 0.0
		if items[i].ChangePercent != nil {
			ai = math.Abs(*items[i].ChangePercent)
		}
		if items[j].ChangePercent != nil {
			aj = math.Abs(*items[j].ChangePercent)
		}
		if ai == aj {
			return items[i].Canonical < items[j].Canonical
		}
		return ai > aj
	})
}
func safeDashboardURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
		return ""
	}
	value := parsed.String()
	if len([]rune(value)) > 2048 {
		return ""
	}
	return value
}
func boundedDashboardText(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) > limit {
		runes = runes[:limit]
	}
	return string(runes)
}

func boundedDashboardStringPointer(value *string, limit int) *string {
	if value == nil {
		return nil
	}
	bounded := boundedDashboardText(*value, limit)
	return &bounded
}
