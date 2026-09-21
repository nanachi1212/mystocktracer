package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"easy-stock/backend/internal/appsettings"
	"easy-stock/backend/internal/hermes"
	"easy-stock/backend/internal/providers/taiwan"
	"easy-stock/backend/internal/providers/toalpha"
	"easy-stock/backend/internal/runtimelog"
	"easy-stock/backend/internal/taiwanportfolio"
	"easy-stock/backend/internal/taiwanwatchlist"
)

type Server struct {
	mux                         *http.ServeMux
	token                       string
	taiwanDirectory             TaiwanDirectoryProvider
	taiwanMarket                TaiwanMarketProvider
	taiwanChip                  TaiwanChipProvider
	taiwanSnapshot              TaiwanSnapshotProvider
	taiwanBreadth               TaiwanBreadthProvider
	taiwanScreener              TaiwanScreenerProvider
	taiwanScreenerInstitutional TaiwanScreenerInstitutionalProvider
	taiwanScreenerMargin        TaiwanScreenerMarginProvider
	taiwanScreenerRevenue       TaiwanScreenerRevenueProvider
	taiwanScreenerValuation     TaiwanScreenerValuationProvider
	taiwanScreenerDividends     TaiwanScreenerDividendsProvider
	taiwanScreenerFinancials    TaiwanScreenerFinancialsProvider
	taiwanScreenerBalance       TaiwanScreenerBalanceProvider
	taiwanScreenerCashflow      TaiwanScreenerCashflowProvider
	taiwanEmotion               TaiwanEmotionProvider
	taiwanIndustryRadar         TaiwanIndustryRadarProvider
	taiwanIntelligence          TaiwanStockIntelligenceProvider
	taiwanCorporateEvents       TaiwanCorporateEventsProvider
	taiwanFundamentals          TaiwanFundamentalsProvider
	taiwanDirectories           *taiwanDirectoryCache
	watchlistStore              *taiwanwatchlist.Store
	taiwanPortfolioStore        *taiwanportfolio.Store
	settingsStore               *appsettings.Store
	hermesGateway               hermes.Gateway
	startupError                error
	logger                      *log.Logger
}

func NewServer(config any) *Server {
	cfg := normalizeConfig(config)
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}
	var startupErrors []error
	if cfg.TaiwanCorporateEvents == nil {
		cfg.TaiwanCorporateEvents = toalpha.NewClient(toalpha.Config{
			Enabled: cfg.ToAlphaMOPSEnabled, Endpoint: cfg.ToAlphaMOPSEndpoint,
			Timeout: cfg.ToAlphaMOPSTimeout, Retries: 1,
		})
	}
	if cfg.TaiwanDirectory == nil || cfg.TaiwanMarket == nil || cfg.TaiwanChip == nil || cfg.TaiwanFundamentals == nil || cfg.TaiwanSnapshot == nil || cfg.TaiwanBreadth == nil || cfg.TaiwanScreener == nil || cfg.TaiwanScreenerInstitutional == nil || cfg.TaiwanScreenerMargin == nil || cfg.TaiwanScreenerRevenue == nil || cfg.TaiwanScreenerValuation == nil || cfg.TaiwanScreenerDividends == nil || cfg.TaiwanScreenerFinancials == nil || cfg.TaiwanScreenerBalance == nil || cfg.TaiwanEmotion == nil || cfg.TaiwanIndustryRadar == nil || cfg.TaiwanIntelligence == nil {
		taiwanClient := taiwan.NewClient(taiwan.Config{
			CashflowCacheDir: cfg.TaiwanCashflowCacheDir,
			CorporateEvents:  cfg.TaiwanCorporateEvents,
		})
		if cfg.TaiwanDirectory == nil {
			cfg.TaiwanDirectory = taiwanClient
		}
		if cfg.TaiwanMarket == nil {
			cfg.TaiwanMarket = taiwanClient
		}
		if cfg.TaiwanChip == nil {
			cfg.TaiwanChip = taiwanClient
		}
		if cfg.TaiwanFundamentals == nil {
			cfg.TaiwanFundamentals = taiwanClient
		}
		if cfg.TaiwanSnapshot == nil {
			cfg.TaiwanSnapshot = taiwanClient
		}
		if cfg.TaiwanBreadth == nil {
			cfg.TaiwanBreadth = taiwanClient
		}
		if cfg.TaiwanScreener == nil {
			cfg.TaiwanScreener = taiwanClient
		}
		if cfg.TaiwanScreenerInstitutional == nil {
			cfg.TaiwanScreenerInstitutional = taiwanClient
		}
		if cfg.TaiwanScreenerMargin == nil {
			cfg.TaiwanScreenerMargin = taiwanClient
		}
		if cfg.TaiwanScreenerRevenue == nil {
			cfg.TaiwanScreenerRevenue = taiwanClient
		}
		if cfg.TaiwanScreenerValuation == nil {
			cfg.TaiwanScreenerValuation = taiwanClient
		}
		if cfg.TaiwanScreenerDividends == nil {
			cfg.TaiwanScreenerDividends = taiwanClient
		}
		if cfg.TaiwanScreenerFinancials == nil {
			cfg.TaiwanScreenerFinancials = taiwanClient
		}
		if cfg.TaiwanScreenerBalance == nil {
			cfg.TaiwanScreenerBalance = taiwanClient
		}
		if cfg.TaiwanScreenerCashflow == nil {
			cfg.TaiwanScreenerCashflow = taiwanClient
		}
		if cfg.TaiwanEmotion == nil {
			cfg.TaiwanEmotion = taiwanClient
		}
		if cfg.TaiwanIndustryRadar == nil {
			cfg.TaiwanIndustryRadar = taiwanClient
		}
		if cfg.TaiwanIntelligence == nil {
			cfg.TaiwanIntelligence = taiwanClient
		}
	}
	if cfg.WatchlistStore == nil {
		store, err := taiwanwatchlist.OpenStore(cfg.WatchlistDBPath)
		if err == nil {
			cfg.WatchlistStore = store
		} else if cfg.StrictPersistence {
			startupErrors = append(startupErrors, fmt.Errorf("open taiwan watchlist database: %w", err))
			cfg.WatchlistStore, _ = taiwanwatchlist.OpenStore(":memory:")
		} else {
			cfg.WatchlistStore, _ = taiwanwatchlist.OpenStore(":memory:")
		}
	}
	if cfg.TaiwanPortfolioStore == nil {
		store, err := taiwanportfolio.OpenStore(cfg.TaiwanPortfolioDBPath)
		if err == nil {
			cfg.TaiwanPortfolioStore = store
		} else if cfg.StrictPersistence {
			startupErrors = append(startupErrors, fmt.Errorf("open Taiwan portfolio database: %w", err))
			cfg.TaiwanPortfolioStore, _ = taiwanportfolio.OpenStore(":memory:")
		} else {
			cfg.TaiwanPortfolioStore, _ = taiwanportfolio.OpenStore(":memory:")
		}
	}
	if cfg.SettingsStore == nil {
		store, err := appsettings.Open(cfg.SettingsPath)
		if err == nil {
			cfg.SettingsStore = store
		} else if cfg.StrictPersistence {
			startupErrors = append(startupErrors, fmt.Errorf("open settings: %w", err))
			cfg.SettingsStore, _ = appsettings.Open("")
		} else {
			cfg.SettingsStore, _ = appsettings.Open("")
		}
	}
	if cfg.HermesGateway != nil && (!cfg.StrictPersistence || len(startupErrors) == 0) {
		values := cfg.SettingsStore.Snapshot()
		var migratedKey *string
		if strings.TrimSpace(values.LLM.APIKey) != "" {
			key := strings.TrimSpace(values.LLM.APIKey)
			migratedKey = &key
			if updated, err := cfg.SettingsStore.Update(func(next *appsettings.Values) error {
				next.LLM.APIKey = ""
				return nil
			}); err == nil {
				values = updated
			}
		}
		if profileGateway, ok := cfg.HermesGateway.(hermes.ProfileGateway); ok {
			if migratedKey == nil {
				if key, err := cfg.HermesGateway.ModelAPIKey(); err == nil && strings.TrimSpace(key) != "" {
					migratedKey = &key
				}
			}
			_ = profileGateway.SyncLLMProfile(values.LLM, values.ActiveLLMProfileID, migratedKey)
		} else {
			_ = cfg.HermesGateway.SyncLLM(values.LLM, migratedKey)
		}
	}
	s := &Server{
		mux:                         http.NewServeMux(),
		token:                       cfg.Token,
		taiwanDirectory:             cfg.TaiwanDirectory,
		taiwanMarket:                cfg.TaiwanMarket,
		taiwanChip:                  cfg.TaiwanChip,
		taiwanSnapshot:              cfg.TaiwanSnapshot,
		taiwanBreadth:               cfg.TaiwanBreadth,
		taiwanScreener:              cfg.TaiwanScreener,
		taiwanScreenerInstitutional: cfg.TaiwanScreenerInstitutional,
		taiwanScreenerMargin:        cfg.TaiwanScreenerMargin,
		taiwanScreenerRevenue:       cfg.TaiwanScreenerRevenue,
		taiwanScreenerValuation:     cfg.TaiwanScreenerValuation,
		taiwanScreenerDividends:     cfg.TaiwanScreenerDividends,
		taiwanScreenerFinancials:    cfg.TaiwanScreenerFinancials,
		taiwanScreenerBalance:       cfg.TaiwanScreenerBalance,
		taiwanScreenerCashflow:      cfg.TaiwanScreenerCashflow,
		taiwanEmotion:               cfg.TaiwanEmotion,
		taiwanIndustryRadar:         cfg.TaiwanIndustryRadar,
		taiwanIntelligence:          cfg.TaiwanIntelligence,
		taiwanCorporateEvents:       cfg.TaiwanCorporateEvents,
		taiwanFundamentals:          cfg.TaiwanFundamentals,
		taiwanDirectories:           newTaiwanDirectoryCache(12 * time.Hour),
		watchlistStore:              cfg.WatchlistStore,
		taiwanPortfolioStore:        cfg.TaiwanPortfolioStore,
		settingsStore:               cfg.SettingsStore,
		hermesGateway:               cfg.HermesGateway,
		startupError:                errors.Join(startupErrors...),
		logger:                      cfg.Logger,
	}
	s.routes()
	return s
}

func (s *Server) StartupError() error {
	if s == nil {
		return errors.New("server is nil")
	}
	return s.startupError
}

func (s *Server) Close() error {
	if s == nil {
		return nil
	}
	var closeErrors []error
	if s.watchlistStore != nil {
		closeErrors = append(closeErrors, s.watchlistStore.Close())
	}
	if s.taiwanPortfolioStore != nil {
		closeErrors = append(closeErrors, s.taiwanPortfolioStore.Close())
	}
	if closer, ok := s.taiwanScreenerCashflow.(io.Closer); ok {
		closeErrors = append(closeErrors, closer.Close())
	}
	return errors.Join(closeErrors...)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startedAt := time.Now()
	requestID := runtimeRequestID(r)
	loggedWriter := &requestLogWriter{ResponseWriter: w}
	loggedWriter.Header().Set("X-Request-ID", requestID)
	defer s.logRequest(r, loggedWriter, requestID, startedAt)

	s.withCORS(loggedWriter, r)
	if r.Method == http.MethodOptions {
		loggedWriter.WriteHeader(http.StatusNoContent)
		return
	}
	if !s.authorized(r) {
		writeError(loggedWriter, http.StatusUnauthorized, "unauthorized")
		return
	}
	s.mux.ServeHTTP(loggedWriter, r)
}

func (s *Server) logSchedulerLifecycle(_ context.Context, feature, task string, run func()) {
	if s.logger != nil {
		s.logger.Printf("level=info event=scheduler_start feature=%s task=%s", feature, task)
		defer s.logger.Printf("level=info event=scheduler_stop feature=%s task=%s", feature, task)
	}
	run()
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.health)
	s.mux.HandleFunc("GET /api/v1/tw/securities", s.taiwanDirectoryHandler)
	s.mux.HandleFunc("GET /api/v1/tw/quotes", s.taiwanQuotesHandler)
	s.mux.HandleFunc("GET /api/v1/tw/kline", s.taiwanKLineHandler)
	s.mux.HandleFunc("GET /api/v1/tw/indexes", s.taiwanIndexesHandler)
	s.mux.HandleFunc("GET /api/v1/tw/institutional", s.taiwanInstitutionalHandler)
	s.mux.HandleFunc("GET /api/v1/tw/margin", s.taiwanMarginHandler)
	s.mux.HandleFunc("GET /api/v1/tw/data-status", s.taiwanDataStatusHandler)
	s.mux.HandleFunc("GET /api/v1/tw/dashboard", s.taiwanDashboardHandler)
	s.mux.HandleFunc("GET /api/v1/tw/market-breadth", s.taiwanMarketBreadthHandler)
	s.mux.HandleFunc("GET /api/v1/tw/screener", s.taiwanScreenerHandler)
	s.mux.HandleFunc("GET /api/v1/tw/market-emotion", s.taiwanMarketEmotionHandler)
	s.mux.HandleFunc("GET /api/v1/tw/industry-radar", s.taiwanIndustryRadarHandler)
	s.mux.HandleFunc("GET /api/v1/tw/stocks/{symbol}/intelligence", s.taiwanStockIntelligenceHandler)
	s.mux.HandleFunc("GET /api/v1/tw/stocks/{symbol}/intelligence/core", s.taiwanStockIntelligenceCoreHandler)
	s.mux.HandleFunc("POST /api/v1/tw/stocks/{symbol}/research", s.taiwanStockResearchHandler)
	s.mux.HandleFunc("GET /api/v1/tw/stocks/{symbol}/research-history", s.taiwanResearchHistoryListHandler)
	s.mux.HandleFunc("GET /api/v1/tw/stocks/{symbol}/research-history/{runID}", s.taiwanResearchHistoryGetHandler)
	s.mux.HandleFunc("GET /api/v1/tw/stocks/{symbol}/research-history/{runID}/comparison", s.taiwanResearchHistoryComparisonHandler)
	s.mux.HandleFunc("GET /api/v1/tw/stocks/{symbol}/corporate-events", s.taiwanCorporateEventsHandler)
	s.mux.HandleFunc("POST /api/v1/tw/corporate-events/sync", s.taiwanCorporateEventSyncHandler)
	s.mux.HandleFunc("GET /api/v1/tw/alerts", s.taiwanAlertsListHandler)
	s.mux.HandleFunc("PUT /api/v1/tw/alerts/read-all", s.taiwanAlertsReadAllHandler)
	s.mux.HandleFunc("PUT /api/v1/tw/alerts/{id}/read", s.taiwanAlertReadHandler)
	s.mux.HandleFunc("GET /api/v1/tw/fundamentals", s.taiwanFundamentalsHandler)
	s.mux.HandleFunc("GET /api/v1/tw/watchlist", s.taiwanWatchlistListHandler)
	s.mux.HandleFunc("POST /api/v1/tw/watchlist", s.taiwanWatchlistAddHandler)
	s.mux.HandleFunc("DELETE /api/v1/tw/watchlist/{symbol}", s.taiwanWatchlistRemoveHandler)
	s.mux.HandleFunc("GET /api/v1/tw/portfolio", s.taiwanPortfolioListHandler)
	s.mux.HandleFunc("POST /api/v1/tw/portfolio", s.taiwanPortfolioAddHandler)
	s.mux.HandleFunc("PUT /api/v1/tw/portfolio/{symbol}", s.taiwanPortfolioUpdateHandler)
	s.mux.HandleFunc("DELETE /api/v1/tw/portfolio/{symbol}", s.taiwanPortfolioDeleteHandler)
	s.mux.HandleFunc("GET /api/v1/tw/portfolio/summary", s.taiwanPortfolioSummaryHandler)
	s.mux.HandleFunc("GET /api/v1/settings", s.settingsGet)
	s.mux.HandleFunc("PUT /api/v1/settings", s.settingsUpdate)
	s.mux.HandleFunc("GET /api/v1/settings/agent", s.settingsAgentGet)
	s.mux.HandleFunc("PUT /api/v1/settings/agent", s.settingsAgentUpdate)
	s.mux.HandleFunc("POST /api/v1/settings/llm/models", s.settingsLLMModels)
	s.mux.HandleFunc("POST /api/v1/settings/llm/test", s.settingsLLMTest)
	s.mux.HandleFunc("GET /api/v1/ai/ws", s.aiChatWebSocket)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"name": "easy-stock data foundation",
		"time": time.Now(),
	})
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err == io.EOF {
		return nil
	} else if err != nil {
		return err
	}
	return io.ErrUnexpectedEOF
}

func contextWithTimeout(r *http.Request, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), timeout)
}

func marketLimitQuery(r *http.Request, fallback int, maximum int) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 || value > maximum {
		return 0, fmt.Errorf("limit must be between 1 and %d", maximum)
	}
	return value, nil
}

func writeError(w http.ResponseWriter, status int, message string) {
	if status >= http.StatusInternalServerError {
		log.Printf("level=error event=http_error status=%d message=%q", status, runtimelog.Redact(message))
	}
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
