package httpapi

import (
	"net/http"
	"time"
)

func (s *Server) registerRoutes() {
	routes := []struct {
		pattern string
		handler http.HandlerFunc
	}{
		{"GET /api/health", s.health},
		{"GET /api/v1/tw/securities", s.taiwanDirectoryHandler},
		{"GET /api/v1/tw/quotes", s.taiwanQuotesHandler},
		{"GET /api/v1/tw/kline", s.taiwanKLineHandler},
		{"GET /api/v1/tw/indexes", s.taiwanIndexesHandler},
		{"GET /api/v1/tw/institutional", s.taiwanInstitutionalHandler},
		{"GET /api/v1/tw/margin", s.taiwanMarginHandler},
		{"GET /api/v1/tw/data-status", s.taiwanDataStatusHandler},
		{"GET /api/v1/tw/dashboard", s.taiwanDashboardHandler},
		{"GET /api/v1/tw/market-breadth", s.taiwanMarketBreadthHandler},
		{"GET /api/v1/tw/screener", s.taiwanScreenerHandler},
		{"GET /api/v1/tw/market-emotion", s.taiwanMarketEmotionHandler},
		{"GET /api/v1/tw/industry-radar", s.taiwanIndustryRadarHandler},
		{"GET /api/v1/tw/stocks/{symbol}/intelligence", s.taiwanStockIntelligenceHandler},
		{"GET /api/v1/tw/stocks/{symbol}/intelligence/core", s.taiwanStockIntelligenceCoreHandler},
		{"POST /api/v1/tw/stocks/{symbol}/research", s.taiwanStockResearchHandler},
		{"GET /api/v1/tw/stocks/{symbol}/research-history", s.taiwanResearchHistoryListHandler},
		{"GET /api/v1/tw/stocks/{symbol}/research-history/{runID}", s.taiwanResearchHistoryGetHandler},
		{"GET /api/v1/tw/stocks/{symbol}/research-history/{runID}/comparison", s.taiwanResearchHistoryComparisonHandler},
		{"GET /api/v1/tw/stocks/{symbol}/corporate-events", s.taiwanCorporateEventsHandler},
		{"POST /api/v1/tw/corporate-events/sync", s.taiwanCorporateEventSyncHandler},
		{"GET /api/v1/tw/alerts", s.taiwanAlertsListHandler},
		{"PUT /api/v1/tw/alerts/read-all", s.taiwanAlertsReadAllHandler},
		{"PUT /api/v1/tw/alerts/{id}/read", s.taiwanAlertReadHandler},
		{"GET /api/v1/tw/fundamentals", s.taiwanFundamentalsHandler},
		{"GET /api/v1/tw/watchlist", s.taiwanWatchlistListHandler},
		{"POST /api/v1/tw/watchlist", s.taiwanWatchlistAddHandler},
		{"DELETE /api/v1/tw/watchlist/{symbol}", s.taiwanWatchlistRemoveHandler},
		{"GET /api/v1/tw/portfolio", s.taiwanPortfolioListHandler},
		{"POST /api/v1/tw/portfolio", s.taiwanPortfolioAddHandler},
		{"PUT /api/v1/tw/portfolio/{symbol}", s.taiwanPortfolioUpdateHandler},
		{"DELETE /api/v1/tw/portfolio/{symbol}", s.taiwanPortfolioDeleteHandler},
		{"GET /api/v1/tw/portfolio/summary", s.taiwanPortfolioSummaryHandler},
		{"GET /api/v1/settings", s.settingsGet},
		{"PUT /api/v1/settings", s.settingsUpdate},
		{"GET /api/v1/settings/agent", s.settingsAgentGet},
		{"PUT /api/v1/settings/agent", s.settingsAgentUpdate},
		{"POST /api/v1/settings/llm/models", s.settingsLLMModels},
		{"POST /api/v1/settings/llm/test", s.settingsLLMTest},
		{"GET /api/v1/ai/ws", s.aiChatWebSocket},
	}
	for _, route := range routes {
		s.router.HandleFunc(route.pattern, route.handler)
	}
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"name": "mystocktracer data foundation",
		"time": time.Now().UTC(),
	})
}
