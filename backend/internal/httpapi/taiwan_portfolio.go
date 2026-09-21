package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nanachi1212/mystocktracer/backend/internal/foundation"
	"github.com/nanachi1212/mystocktracer/backend/internal/taiwanportfolio"
)

type taiwanPortfolioWriteRequest struct {
	Symbol      string  `json:"symbol"`
	Shares      float64 `json:"shares"`
	AverageCost float64 `json:"average_cost"`
	Note        string  `json:"note"`
}

type taiwanPortfolioHoldingView struct {
	Canonical     string                 `json:"canonical"`
	Code          string                 `json:"code,omitempty"`
	Name          string                 `json:"name"`
	Exchange      string                 `json:"exchange,omitempty"`
	SecurityType  string                 `json:"security_type,omitempty"`
	Industry      string                 `json:"industry,omitempty"`
	Shares        float64                `json:"shares"`
	AverageCost   float64                `json:"average_cost"`
	Note          string                 `json:"note,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
	CurrentPrice  *float64               `json:"current_price"`
	ChangePercent *float64               `json:"change_percent"`
	PriceStatus   string                 `json:"price_status"`
	QuoteMeta     *foundation.SourceMeta `json:"quote_meta,omitempty"`
	MarketValue   *float64               `json:"market_value"`
	TotalCost     float64                `json:"total_cost"`
	UnrealizedPL  *float64               `json:"unrealized_pl"`
	UnrealizedPC  *float64               `json:"unrealized_pl_percent"`
	Weight        *float64               `json:"portfolio_weight_percent"`
}

type taiwanPortfolioIndustryConcentration struct {
	Industry    string  `json:"industry"`
	MarketValue float64 `json:"market_value"`
	Weight      float64 `json:"weight_percent"`
	Holdings    int     `json:"holdings_count"`
}

type taiwanPortfolioConcentration struct {
	HoldingsCount int                                    `json:"holdings_count"`
	Top3Percent   *float64                               `json:"top_3_percent"`
	Top5Percent   *float64                               `json:"top_5_percent"`
	Industries    []taiwanPortfolioIndustryConcentration `json:"industries"`
	Status        string                                 `json:"status"`
}

type taiwanPortfolioSummary struct {
	Holdings          []taiwanPortfolioHoldingView `json:"holdings"`
	HoldingsCount     int                          `json:"holdings_count"`
	PricedHoldings    int                          `json:"priced_holdings_count"`
	TotalMarketValue  *float64                     `json:"total_market_value"`
	AvailableValue    float64                      `json:"available_market_value"`
	TotalCost         float64                      `json:"total_cost"`
	TotalUnrealizedPL *float64                     `json:"total_unrealized_pl"`
	UnrealizedPC      *float64                     `json:"total_unrealized_pl_percent"`
	Currency          string                       `json:"currency"`
	Status            string                       `json:"status"`
	Concentration     taiwanPortfolioConcentration `json:"concentration"`
}

func (s *Server) taiwanPortfolioListHandler(w http.ResponseWriter, r *http.Request) {
	if s.taiwanPortfolioStore == nil {
		writeError(w, http.StatusServiceUnavailable, "Taiwan portfolio storage is unavailable")
		return
	}
	holdings, err := s.taiwanPortfolioStore.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load Taiwan portfolio")
		return
	}
	identities := s.portfolioIdentities(r.Context())
	views := make([]taiwanPortfolioHoldingView, 0, len(holdings))
	for _, holding := range holdings {
		views = append(views, portfolioHoldingView(holding, identities[holding.Canonical]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"holdings": views}})
}

func (s *Server) taiwanPortfolioAddHandler(w http.ResponseWriter, r *http.Request) {
	s.taiwanPortfolioSave(w, r, strings.TrimSpace(r.PathValue("symbol")), true)
}

func (s *Server) taiwanPortfolioUpdateHandler(w http.ResponseWriter, r *http.Request) {
	s.taiwanPortfolioSave(w, r, strings.TrimSpace(r.PathValue("symbol")), false)
}

func (s *Server) taiwanPortfolioSave(w http.ResponseWriter, r *http.Request, pathSymbol string, allowBodySymbol bool) {
	if s.taiwanPortfolioStore == nil {
		writeError(w, http.StatusServiceUnavailable, "Taiwan portfolio storage is unavailable")
		return
	}
	var request taiwanPortfolioWriteRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid Taiwan portfolio request")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid Taiwan portfolio request")
		return
	}
	rawSymbol := pathSymbol
	if allowBodySymbol {
		rawSymbol = request.Symbol
	} else if request.Symbol != "" && !strings.EqualFold(strings.TrimSpace(request.Symbol), pathSymbol) {
		writeError(w, http.StatusBadRequest, "request symbol must match the portfolio path")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	identity, statusCode, message := s.resolveTaiwanWatchlistSymbol(ctx, rawSymbol)
	if message != "" {
		writeError(w, statusCode, message)
		return
	}
	input := taiwanportfolio.Holding{
		Canonical: identity.Canonical, DisplayName: identity.Name, Shares: request.Shares,
		AverageCost: request.AverageCost, Note: request.Note,
	}
	if err := taiwanportfolio.ValidateHolding(input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	holding, existed, err := s.taiwanPortfolioStore.Upsert(ctx, input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save Taiwan portfolio holding")
		return
	}
	status := http.StatusCreated
	if existed {
		status = http.StatusOK
	}
	writeJSON(w, status, map[string]any{"data": map[string]any{"holding": portfolioHoldingView(holding, *identity)}})
}

func (s *Server) taiwanPortfolioDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if s.taiwanPortfolioStore == nil {
		writeError(w, http.StatusServiceUnavailable, "Taiwan portfolio storage is unavailable")
		return
	}
	symbol := strings.ToUpper(strings.TrimSpace(r.PathValue("symbol")))
	if !validTaiwanCanonical(symbol) {
		writeError(w, http.StatusBadRequest, "canonical Taiwan symbol must end in .TWSE or .TPEX")
		return
	}
	deleted, err := s.taiwanPortfolioStore.Delete(r.Context(), symbol)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete Taiwan portfolio holding")
		return
	}
	if !deleted {
		writeError(w, http.StatusNotFound, "Taiwan portfolio holding was not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) taiwanPortfolioSummaryHandler(w http.ResponseWriter, r *http.Request) {
	if s.taiwanPortfolioStore == nil {
		writeError(w, http.StatusServiceUnavailable, "Taiwan portfolio storage is unavailable")
		return
	}
	holdings, err := s.taiwanPortfolioStore.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load Taiwan portfolio")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	identities := s.portfolioIdentities(ctx)
	quotes := s.portfolioQuotes(ctx, holdings, identities)
	summary := calculateTaiwanPortfolioSummary(holdings, identities, quotes)
	writeJSON(w, http.StatusOK, map[string]any{"data": summary})
}

type portfolioQuoteResult struct {
	quote foundation.Quote
	err   error
}

func (s *Server) portfolioQuotes(ctx context.Context, holdings []taiwanportfolio.Holding, identities map[string]foundation.SecurityIdentity) map[string]portfolioQuoteResult {
	result := make(map[string]portfolioQuoteResult, len(holdings))
	if s.taiwanMarket == nil {
		return result
	}
	var mu sync.Mutex
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for _, holding := range holdings {
		identity, ok := identities[holding.Canonical]
		if !ok {
			continue
		}
		holding := holding
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			quote, err := s.taiwanMarket.Quote(ctx, identity)
			if err == nil && (quote.Price <= 0 || math.IsNaN(quote.Price) || math.IsInf(quote.Price, 0)) {
				err = errors.New("Taiwan quote price is unavailable")
			}
			mu.Lock()
			result[holding.Canonical] = portfolioQuoteResult{quote: quote, err: err}
			mu.Unlock()
		}()
	}
	wg.Wait()
	return result
}

func (s *Server) portfolioIdentities(ctx context.Context) map[string]foundation.SecurityIdentity {
	result := map[string]foundation.SecurityIdentity{}
	if s.taiwanDirectory == nil || s.taiwanDirectories == nil {
		return result
	}
	items, _, _, _, err := s.taiwanDirectories.load(ctx, s.taiwanDirectory)
	if err != nil {
		return result
	}
	for _, item := range items {
		result[item.Canonical] = item
	}
	return result
}

func portfolioHoldingView(holding taiwanportfolio.Holding, identity foundation.SecurityIdentity) taiwanPortfolioHoldingView {
	name := holding.DisplayName
	if identity.Canonical != "" {
		name = identity.Name
	}
	return taiwanPortfolioHoldingView{
		Canonical: holding.Canonical, Code: identity.Code, Name: name, Exchange: identity.Exchange,
		SecurityType: string(identity.Type), Industry: identity.Industry, Shares: holding.Shares,
		AverageCost: holding.AverageCost, Note: holding.Note, CreatedAt: holding.CreatedAt,
		UpdatedAt: holding.UpdatedAt, PriceStatus: "not_requested", TotalCost: holding.Shares * holding.AverageCost,
	}
}

func calculateTaiwanPortfolioSummary(holdings []taiwanportfolio.Holding, identities map[string]foundation.SecurityIdentity, quotes map[string]portfolioQuoteResult) taiwanPortfolioSummary {
	summary := taiwanPortfolioSummary{Holdings: make([]taiwanPortfolioHoldingView, 0, len(holdings)), HoldingsCount: len(holdings), Currency: "TWD", Status: "available"}
	type industryValue struct {
		value float64
		count int
	}
	industries := map[string]industryValue{}
	marketValues := make([]float64, 0, len(holdings))
	allPriced := true
	anyStale := false
	for _, holding := range holdings {
		identity := identities[holding.Canonical]
		view := portfolioHoldingView(holding, identity)
		summary.TotalCost += view.TotalCost
		quoteResult, found := quotes[holding.Canonical]
		if !found || quoteResult.err != nil {
			view.PriceStatus = "unavailable"
			allPriced = false
			summary.Holdings = append(summary.Holdings, view)
			continue
		}
		price := quoteResult.quote.Price
		marketValue := price * holding.Shares
		unrealized := marketValue - view.TotalCost
		if math.IsNaN(marketValue) || math.IsInf(marketValue, 0) || math.IsNaN(unrealized) || math.IsInf(unrealized, 0) {
			view.PriceStatus = "unavailable"
			allPriced = false
			summary.Holdings = append(summary.Holdings, view)
			continue
		}
		var unrealizedPercent *float64
		if view.TotalCost > 0 {
			value := unrealized / view.TotalCost * 100
			unrealizedPercent = &value
		}
		view.CurrentPrice, view.MarketValue, view.UnrealizedPL, view.UnrealizedPC = &price, &marketValue, &unrealized, unrealizedPercent
		changePercent := quoteResult.quote.ChangePercent
		if !math.IsNaN(changePercent) && !math.IsInf(changePercent, 0) {
			view.ChangePercent = &changePercent
		}
		view.QuoteMeta = &quoteResult.quote.Meta
		view.PriceStatus = "available"
		if quoteResult.quote.Meta.Stale || quoteResult.quote.Meta.Freshness == "stale" {
			view.PriceStatus = "stale"
			anyStale = true
		}
		summary.PricedHoldings++
		summary.AvailableValue += marketValue
		marketValues = append(marketValues, marketValue)
		industry := identity.Industry
		if industry == "" {
			industry = "未分類"
		}
		item := industries[industry]
		item.value += marketValue
		item.count++
		industries[industry] = item
		summary.Holdings = append(summary.Holdings, view)
	}
	if len(holdings) == 0 {
		summary.Concentration = taiwanPortfolioConcentration{Industries: []taiwanPortfolioIndustryConcentration{}, Status: "empty"}
		return summary
	}
	if !allPriced {
		summary.Status = "partial"
		summary.Concentration = taiwanPortfolioConcentration{HoldingsCount: len(holdings), Industries: []taiwanPortfolioIndustryConcentration{}, Status: "data_insufficient"}
		return summary
	}
	if anyStale {
		summary.Status = "stale"
	}
	totalMarket := summary.AvailableValue
	totalPL := totalMarket - summary.TotalCost
	summary.TotalMarketValue = &totalMarket
	summary.TotalUnrealizedPL = &totalPL
	if summary.TotalCost > 0 {
		percent := totalPL / summary.TotalCost * 100
		summary.UnrealizedPC = &percent
	}
	for index := range summary.Holdings {
		value := *summary.Holdings[index].MarketValue / totalMarket * 100
		summary.Holdings[index].Weight = &value
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(marketValues)))
	top := func(limit int) *float64 {
		value := 0.0
		for i := 0; i < len(marketValues) && i < limit; i++ {
			value += marketValues[i]
		}
		value = value / totalMarket * 100
		return &value
	}
	concentration := taiwanPortfolioConcentration{HoldingsCount: len(holdings), Top3Percent: top(3), Top5Percent: top(5), Industries: []taiwanPortfolioIndustryConcentration{}, Status: "available"}
	for industry, item := range industries {
		concentration.Industries = append(concentration.Industries, taiwanPortfolioIndustryConcentration{Industry: industry, MarketValue: item.value, Weight: item.value / totalMarket * 100, Holdings: item.count})
	}
	sort.Slice(concentration.Industries, func(i, j int) bool { return concentration.Industries[i].Weight > concentration.Industries[j].Weight })
	summary.Concentration = concentration
	return summary
}
