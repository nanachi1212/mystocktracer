package httpapi

import (
	"context"
	"log"
	"net/http"
	"time"

	"easy-stock/backend/internal/appsettings"
	"easy-stock/backend/internal/foundation"
	"easy-stock/backend/internal/hermes"
	"easy-stock/backend/internal/marketemotion"
	"easy-stock/backend/internal/methodology"
	"easy-stock/backend/internal/portfolioinspection"
	"easy-stock/backend/internal/review"
	"easy-stock/backend/internal/sector"
	"easy-stock/backend/internal/stockanalysis"
	"easy-stock/backend/internal/strategy/inflection"
	"easy-stock/backend/internal/taiwanwatchlist"
)

type RealtimeProvider interface {
	Realtime(ctx context.Context, symbols []string) ([]foundation.Quote, error)
}

type KLineProvider interface {
	KLine(ctx context.Context, symbol string, period string, limit int) ([]foundation.KLine, error)
}

type NewsProvider interface {
	LatestNews(ctx context.Context, limit int) ([]foundation.NewsItem, error)
}

type SectorMapProvider interface {
	Build(ctx context.Context, themeID string) (foundation.SectorMap, error)
}

type SnapshotSectorMapProvider interface {
	BuildSnapshot(ctx context.Context, themeID string, snapshotID string) (foundation.SectorMap, error)
}

type ThemeOverviewProvider interface {
	Overviews(ctx context.Context) ([]foundation.ThemeOverview, foundation.SourceMeta, error)
}

type ThemeRadarFallback interface {
	SectorMapProvider
	ThemeOverviewProvider
}

type LimitUpProvider interface {
	RecentLimitUps(ctx context.Context, lookbackDays int) ([]foundation.LimitUpEvent, error)
}

type StockThemeAttributionProvider interface {
	StockThemes(ctx context.Context, symbol string, lookbackDays int) ([]foundation.StockThemeAttribution, error)
}

type MarketPoolProvider interface {
	BrokenLimitUpPool(ctx context.Context, date time.Time) ([]foundation.MarketLimitEvent, error)
	LimitDownPool(ctx context.Context, date time.Time) ([]foundation.MarketLimitEvent, error)
}

type StockConceptProvider interface {
	StockCatalog(ctx context.Context) ([]foundation.StockCatalogEntry, error)
}

type StockBusinessProfileProvider interface {
	StockBusinessProfile(ctx context.Context, symbol string) (foundation.StockBusinessProfile, error)
	StockFundamentals(ctx context.Context, symbol string) (foundation.StockFundamentals, error)
}

type StockDirectoryProvider interface {
	StockCatalog(ctx context.Context) ([]foundation.StockCatalogEntry, error)
}

type TaiwanDirectoryProvider interface {
	Directory(ctx context.Context) ([]foundation.SecurityIdentity, error)
}

type TaiwanMarketProvider interface {
	Quote(ctx context.Context, security foundation.SecurityIdentity) (foundation.Quote, error)
	KLine(ctx context.Context, security foundation.SecurityIdentity, limit int) ([]foundation.KLine, error)
	Indexes(ctx context.Context) ([]foundation.MarketIndexSeries, foundation.SourceMeta, error)
}

type TaiwanChipProvider interface {
	Institutional(ctx context.Context, security foundation.SecurityIdentity, limit int) (foundation.InstitutionalHistory, error)
	Margin(ctx context.Context, security foundation.SecurityIdentity, limit int) (foundation.MarginHistory, error)
}

type TaiwanSnapshotProvider interface {
	Freshness(now time.Time) foundation.TaiwanFreshness
}

type TaiwanBreadthProvider interface {
	MarketBreadth(ctx context.Context, now time.Time) (foundation.TaiwanMarketBreadth, error)
}

type TaiwanScreenerProvider interface {
	ScreenerSnapshot(ctx context.Context, now time.Time) ([]foundation.TaiwanDailySnapshot, foundation.TaiwanFreshness, error)
}

// TaiwanScreenerInstitutionalProvider / TaiwanScreenerMarginProvider are deliberately separate,
// narrow interfaces (not folded into TaiwanScreenerProvider) so existing Screener test fakes that
// only cover M7A (price/volume/amount) never need to implement institutional/margin methods they
// don't use. M7D calls these ONLY when a Screener request actually needs that domain (see
// taiwanScreenerNeedsInstitutional/taiwanScreenerNeedsMargin) — never unconditionally.
type TaiwanScreenerInstitutionalProvider interface {
	ScreenerInstitutional(ctx context.Context, now time.Time) ([]foundation.InstitutionalFlow, foundation.TaiwanFreshness, error)
}

type TaiwanScreenerMarginProvider interface {
	ScreenerMargin(ctx context.Context, now time.Time) ([]foundation.MarginTrading, foundation.TaiwanFreshness, error)
}

// TaiwanScreenerRevenueProvider / TaiwanScreenerValuationProvider / TaiwanScreenerDividendsProvider
// are the M7E-A analogues of the M7D institutional/margin interfaces above: separate, narrow
// capabilities so existing M7A/M7D Screener test fakes never need to implement fundamentals methods
// they don't use. Called ONLY when a Screener request actually needs that domain (an active filter
// or a sort key from that domain) — never unconditionally.
type TaiwanScreenerRevenueProvider interface {
	ScreenerRevenue(ctx context.Context, now time.Time) ([]foundation.MonthlyRevenue, foundation.TaiwanFundamentalsDomainFreshness, error)
}

type TaiwanScreenerValuationProvider interface {
	ScreenerValuation(ctx context.Context, now time.Time) ([]foundation.ValuationSnapshot, foundation.TaiwanValuationFreshness, error)
}

type TaiwanScreenerDividendsProvider interface {
	ScreenerDividends(ctx context.Context, now time.Time) ([]foundation.DividendRecord, foundation.TaiwanFundamentalsDomainFreshness, error)
}

// TaiwanScreenerFinancialsProvider is the M7E-B analogue of the M7E-A providers above — a separate,
// narrow capability so existing M7A/M7D/M7E-A Screener test fakes never need to implement a financials
// method they don't use. Called ONLY when a Screener request actually needs the financials domain (an
// active cumulative_eps/gross_margin/operating_margin filter or sort key) — never unconditionally.
type TaiwanScreenerFinancialsProvider interface {
	ScreenerFinancials(ctx context.Context, now time.Time) ([]foundation.FinancialStatementPeriod, foundation.TaiwanFundamentalsDomainFreshness, error)
}

// TaiwanScreenerBalanceProvider is the M7E-C analogue — a separate, narrow capability so existing
// M7A/M7D/M7E-A/M7E-B Screener test fakes never need to implement a balance-sheet method they don't
// use. Called ONLY when a Screener request actually needs the balance-sheet domain (an active
// book_value_per_share filter or sort key) — never unconditionally, and never merely because the
// income-statement financials domain was requested.
type TaiwanScreenerBalanceProvider interface {
	ScreenerBalance(ctx context.Context, now time.Time) ([]foundation.FinancialStatementPeriod, foundation.TaiwanFundamentalsDomainFreshness, error)
}

// TaiwanScreenerCashflowProvider is the M7H analogue — a separate, narrow capability so existing
// Screener test fakes never need to implement a cash-flow method they don't use. Called ONLY when a
// Screener request actually needs the cash-flow domain (an active operating_cash_flow/
// cash_flow_to_net_income filter or sort key) — never unconditionally, and never merely because the
// income-statement or balance-sheet domains were requested (a genuinely independent official source).
type TaiwanScreenerCashflowProvider interface {
	ScreenerCashflow(ctx context.Context, now time.Time) ([]foundation.FinancialStatementPeriod, foundation.TaiwanFundamentalsDomainFreshness, error)
}

type TaiwanEmotionProvider interface {
	MarketEmotion(ctx context.Context, now time.Time) (marketemotion.TaiwanMarketEmotion, error)
}

type TaiwanIndustryRadarProvider interface {
	IndustryRadar(ctx context.Context, now time.Time) (sector.TaiwanIndustryRadar, error)
}

type TaiwanStockIntelligenceProvider interface {
	StockIntelligence(ctx context.Context, canonical string, now time.Time) (stockanalysis.TaiwanStockIntelligence, error)
}

type TaiwanFundamentalsProvider interface {
	Fundamentals(ctx context.Context, security foundation.SecurityIdentity, months int) (foundation.TaiwanFundamentals, error)
}

type HotStockProvider interface {
	HotStockRanks(ctx context.Context, limit int) []foundation.HotStockRankList
}

type MarketOverviewProvider interface {
	MarketIndexes(ctx context.Context, scope string) ([]foundation.MarketIndexSnapshot, foundation.SourceMeta, error)
	MarketIndexSeries(ctx context.Context, id string, period string, limit int) (foundation.MarketIndexSeries, error)
	IndustryMomentum(ctx context.Context, limit int) ([]foundation.MarketIndustryMomentum, foundation.SourceMeta, error)
	MarketFundFlows(ctx context.Context, dimension string, sortKey string, limit int) ([]foundation.MarketFundFlow, foundation.SourceMeta, error)
	MarketMarginSeries(ctx context.Context, limit int) ([]foundation.MarketMarginPoint, foundation.SourceMeta, error)
	MarketBillboard(ctx context.Context, tradeDate string, limit int) ([]foundation.MarketBillboardItem, foundation.SourceMeta, error)
	MarketBillboardDetail(ctx context.Context, symbol string, tradeDate string, reason string) (foundation.MarketBillboardDetail, foundation.SourceMeta, error)
	MarketAnnouncements(ctx context.Context, query string, symbol string, category string, limit int) ([]foundation.MarketResearchItem, foundation.SourceMeta, error)
	MarketReports(ctx context.Context, kind string, query string, symbol string, industry string, limit int) ([]foundation.MarketResearchItem, foundation.SourceMeta, error)
}

type InflectionEvaluator interface {
	Evaluate(request inflection.EvaluationRequest) (inflection.Evaluation, error)
}

type ReviewImporter interface {
	ImportURL(ctx context.Context, rawURL string) (review.Post, error)
}

type Config struct {
	Token                       string
	Realtime                    RealtimeProvider
	KLinePrimary                KLineProvider
	KLineFallback               KLineProvider
	News                        NewsProvider
	SectorMap                   SectorMapProvider
	ThemeOverview               ThemeOverviewProvider
	ThemeRadarFallback          ThemeRadarFallback
	LimitUp                     LimitUpProvider
	MarketPools                 MarketPoolProvider
	StockConcept                StockConceptProvider
	StockBusiness               StockBusinessProfileProvider
	StockDirectory              StockDirectoryProvider
	TaiwanDirectory             TaiwanDirectoryProvider
	TaiwanMarket                TaiwanMarketProvider
	TaiwanChip                  TaiwanChipProvider
	TaiwanSnapshot              TaiwanSnapshotProvider
	TaiwanBreadth               TaiwanBreadthProvider
	TaiwanScreener              TaiwanScreenerProvider
	TaiwanScreenerInstitutional TaiwanScreenerInstitutionalProvider
	TaiwanScreenerMargin        TaiwanScreenerMarginProvider
	TaiwanScreenerRevenue       TaiwanScreenerRevenueProvider
	TaiwanScreenerValuation     TaiwanScreenerValuationProvider
	TaiwanScreenerDividends     TaiwanScreenerDividendsProvider
	TaiwanScreenerFinancials    TaiwanScreenerFinancialsProvider
	TaiwanScreenerBalance       TaiwanScreenerBalanceProvider
	TaiwanScreenerCashflow      TaiwanScreenerCashflowProvider
	TaiwanEmotion               TaiwanEmotionProvider
	TaiwanIndustryRadar         TaiwanIndustryRadarProvider
	TaiwanIntelligence          TaiwanStockIntelligenceProvider
	TaiwanFundamentals          TaiwanFundamentalsProvider
	HotStocks                   HotStockProvider
	MarketOverview              MarketOverviewProvider
	Inflection                  InflectionEvaluator
	ReviewDBPath                string
	PortfolioDBPath             string
	MarketEmotionDBPath         string
	ThemeRadarDBPath            string
	WatchlistDBPath             string
	DuanxianxiaBaseURL          string
	WeChatAPIURL                string
	ReviewHTTP                  *http.Client
	ReviewStore                 *review.Store
	PortfolioStore              *portfolioinspection.Store
	MarketEmotionStore          *marketemotion.Store
	WatchlistStore              *taiwanwatchlist.Store
	ReviewImporter              ReviewImporter
	SettingsPath                string
	SettingsStore               *appsettings.Store
	ReviewAutomation            *review.Automation
	RemoteDailyReviewURL        string
	RemoteDailySync             *review.RemoteDailySync
	HermesGateway               hermes.Gateway
	MasteryLibrary              *methodology.Library
	Logger                      *log.Logger
	StrictPersistence           bool
}

func normalizeConfig(value any) Config {
	switch cfg := value.(type) {
	case Config:
		return cfg
	case *Config:
		if cfg != nil {
			return *cfg
		}
	}
	return Config{}
}
