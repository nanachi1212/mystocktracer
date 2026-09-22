package httpapi

import (
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/nanachi1212/mystocktracer/backend/internal/appsettings"
	"github.com/nanachi1212/mystocktracer/backend/internal/taiwanportfolio"
	"github.com/nanachi1212/mystocktracer/backend/internal/taiwanwatchlist"
)

type Server struct {
	router                      *http.ServeMux
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
	agentRuntime                AgentRuntime
	startupError                error
	logger                      *log.Logger
}

func NewServer(value any) *Server {
	config, startupError := prepareRuntimeDependencies(normalizeConfig(value))
	server := &Server{
		router:                      http.NewServeMux(),
		token:                       config.Token,
		taiwanDirectory:             config.TaiwanDirectory,
		taiwanMarket:                config.TaiwanMarket,
		taiwanChip:                  config.TaiwanChip,
		taiwanSnapshot:              config.TaiwanSnapshot,
		taiwanBreadth:               config.TaiwanBreadth,
		taiwanScreener:              config.TaiwanScreener,
		taiwanScreenerInstitutional: config.TaiwanScreenerInstitutional,
		taiwanScreenerMargin:        config.TaiwanScreenerMargin,
		taiwanScreenerRevenue:       config.TaiwanScreenerRevenue,
		taiwanScreenerValuation:     config.TaiwanScreenerValuation,
		taiwanScreenerDividends:     config.TaiwanScreenerDividends,
		taiwanScreenerFinancials:    config.TaiwanScreenerFinancials,
		taiwanScreenerBalance:       config.TaiwanScreenerBalance,
		taiwanScreenerCashflow:      config.TaiwanScreenerCashflow,
		taiwanEmotion:               config.TaiwanEmotion,
		taiwanIndustryRadar:         config.TaiwanIndustryRadar,
		taiwanIntelligence:          config.TaiwanIntelligence,
		taiwanCorporateEvents:       config.TaiwanCorporateEvents,
		taiwanFundamentals:          config.TaiwanFundamentals,
		taiwanDirectories:           newTaiwanDirectoryCache(12 * time.Hour),
		watchlistStore:              config.WatchlistStore,
		taiwanPortfolioStore:        config.TaiwanPortfolioStore,
		settingsStore:               config.SettingsStore,
		agentRuntime:                config.AgentRuntime,
		startupError:                startupError,
		logger:                      config.Logger,
	}
	server.registerRoutes()
	return server
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
