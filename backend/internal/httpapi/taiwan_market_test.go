package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"easy-stock/backend/internal/foundation"
	"easy-stock/backend/internal/hermes"
	"easy-stock/backend/internal/marketemotion"
	"easy-stock/backend/internal/sector"
	"easy-stock/backend/internal/stockanalysis"
)

type fakeTaiwanResearchGateway struct {
	*fakeHermesGateway
	content string
}

func (g *fakeTaiwanResearchGateway) PromptIsolated(context.Context, string, string) (hermes.PromptResult, error) {
	return hermes.PromptResult{Content: g.content}, nil
}

type fixedTaiwanMarket struct{}
type fixedTaiwanChip struct{}
type fixedTaiwanFundamentals struct{}
type fixedTaiwanSnapshot struct{}

func (fixedTaiwanMarket) Quote(_ context.Context, security foundation.SecurityIdentity) (foundation.Quote, error) {
	return foundation.Quote{Symbol: security.Canonical, Name: security.Name, Price: 100, Meta: foundation.SourceMeta{Source: "official", FetchedAt: time.Now(), Status: "official_close"}}, nil
}

type fixedTaiwanBreadth struct{}
type fixedTaiwanEmotion struct{}
type fixedTaiwanIndustryRadar struct{}
type fixedTaiwanIntelligence struct{}

func (fixedTaiwanBreadth) MarketBreadth(context.Context, time.Time) (foundation.TaiwanMarketBreadth, error) {
	return foundation.TaiwanMarketBreadth{
		TWSE:     foundation.TaiwanBreadthScope{Scope: "TWSE", Status: "current"},
		TPEX:     foundation.TaiwanBreadthScope{Scope: "TPEX", Status: "current"},
		Combined: foundation.TaiwanBreadthScope{Scope: "COMBINED", Status: "current"},
	}, nil
}

func (fixedTaiwanEmotion) MarketEmotion(context.Context, time.Time) (marketemotion.TaiwanMarketEmotion, error) {
	return marketemotion.TaiwanMarketEmotion{TWSE: marketemotion.TaiwanEmotionScope{Scope: "TWSE", ModelVersion: marketemotion.TaiwanEmotionModelVersion, Status: "current"}, TPEX: marketemotion.TaiwanEmotionScope{Scope: "TPEX", ModelVersion: marketemotion.TaiwanEmotionModelVersion, Status: "current"}, Combined: marketemotion.TaiwanEmotionScope{Scope: "COMBINED", ModelVersion: marketemotion.TaiwanEmotionModelVersion, Status: "current"}}, nil
}

func (fixedTaiwanIndustryRadar) IndustryRadar(context.Context, time.Time) (sector.TaiwanIndustryRadar, error) {
	return sector.TaiwanIndustryRadar{
		TWSE:     sector.TaiwanIndustryScope{Scope: "TWSE", ModelVersion: sector.TaiwanIndustryRadarVersion},
		TPEX:     sector.TaiwanIndustryScope{Scope: "TPEX", ModelVersion: sector.TaiwanIndustryRadarVersion},
		Combined: sector.TaiwanIndustryScope{Scope: "COMBINED", ModelVersion: sector.TaiwanIndustryRadarVersion},
	}, nil
}

func (fixedTaiwanIntelligence) StockIntelligence(_ context.Context, canonical string, _ time.Time) (stockanalysis.TaiwanStockIntelligence, error) {
	if canonical != "2330.TWSE" {
		return stockanalysis.TaiwanStockIntelligence{}, fmt.Errorf("unknown canonical Taiwan security %q", canonical)
	}
	interpretation := stockanalysis.CalculateTaiwanStockInterpretation(stockanalysis.TaiwanStockIntelligence{Symbol: canonical})
	return stockanalysis.TaiwanStockIntelligence{ModelVersion: stockanalysis.TaiwanStockIntelligenceVersion, Symbol: canonical, Interpretation: &interpretation}, nil
}

func TestTaiwanMarketBreadthScopesAndValidation(t *testing.T) {
	server := NewServer(Config{TaiwanBreadth: fixedTaiwanBreadth{}})
	for _, test := range []struct{ scope, want string }{{"twse", `"scope":"TWSE"`}, {"tpex", `"scope":"TPEX"`}, {"combined", `"scope":"COMBINED"`}} {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/tw/market-breadth?scope="+test.scope, nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), test.want) {
			t.Fatalf("scope=%s status=%d body=%s", test.scope, response.Code, response.Body.String())
		}
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/tw/market-breadth?scope=invalid", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid scope status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestTaiwanMarketEmotionScopesEvidenceAndValidation(t *testing.T) {
	server := NewServer(Config{TaiwanEmotion: fixedTaiwanEmotion{}})
	for _, test := range []struct{ scope, want string }{{"twse", `"scope":"TWSE"`}, {"tpex", `"scope":"TPEX"`}, {"combined", `"scope":"COMBINED"`}} {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/tw/market-emotion?scope="+test.scope, nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), test.want) || !strings.Contains(response.Body.String(), `"model_version":"taiwan_emotion_v1"`) {
			t.Fatalf("scope=%s status=%d body=%s", test.scope, response.Code, response.Body.String())
		}
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/tw/market-emotion?scope=invalid", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid scope status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestTaiwanIndustryRadarScopesAndValidation(t *testing.T) {
	server := NewServer(Config{TaiwanIndustryRadar: fixedTaiwanIndustryRadar{}})
	for _, test := range []struct{ scope, want string }{{"twse", `"scope":"TWSE"`}, {"tpex", `"scope":"TPEX"`}, {"combined", `"scope":"COMBINED"`}} {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/tw/industry-radar?scope="+test.scope, nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), test.want) || !strings.Contains(response.Body.String(), `"model_version":"taiwan_industry_radar_v1"`) {
			t.Fatalf("scope=%s status=%d body=%s", test.scope, response.Code, response.Body.String())
		}
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/tw/industry-radar?scope=invalid", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid scope status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestTaiwanStockIntelligenceRequiresCanonicalIdentity(t *testing.T) {
	server := NewServer(Config{TaiwanIntelligence: fixedTaiwanIntelligence{}})
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/tw/stocks/2330.TWSE/intelligence", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"model_version":"taiwan_stock_intelligence_v1"`) || !strings.Contains(response.Body.String(), `"model_version":"taiwan_stock_interpretation_v1"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/tw/stocks/2330/intelligence", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("raw code status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestTaiwanStockResearchIsOptInAndKeepsIntelligenceOnAIFailure(t *testing.T) {
	server := NewServer(Config{TaiwanIntelligence: fixedTaiwanIntelligence{}, HermesGateway: &fakeHermesGateway{}})
	r := httptest.NewRecorder()
	server.ServeHTTP(r, httptest.NewRequest(http.MethodPost, "/api/v1/tw/stocks/2330.TWSE/research", nil))
	if r.Code != http.StatusOK || !strings.Contains(r.Body.String(), `"status":"unavailable"`) || !strings.Contains(r.Body.String(), `"intelligence"`) {
		t.Fatalf("failure isolation=%d %s", r.Code, r.Body.String())
	}
}
func (fixedTaiwanMarket) KLine(_ context.Context, security foundation.SecurityIdentity, _ int) ([]foundation.KLine, error) {
	return []foundation.KLine{{Symbol: security.Canonical, Close: 100, Time: time.Now()}}, nil
}
func (fixedTaiwanMarket) Indexes(context.Context) ([]foundation.MarketIndexSeries, foundation.SourceMeta, error) {
	return []foundation.MarketIndexSeries{{Index: foundation.MarketIndexSnapshot{ID: "taiex"}}}, foundation.SourceMeta{Source: "official", FetchedAt: time.Now()}, nil
}
func (fixedTaiwanChip) Institutional(_ context.Context, security foundation.SecurityIdentity, _ int) (foundation.InstitutionalHistory, error) {
	return foundation.InstitutionalHistory{Security: security, Data: []foundation.InstitutionalFlow{{Canonical: security.Canonical}}}, nil
}
func (fixedTaiwanChip) Margin(_ context.Context, security foundation.SecurityIdentity, _ int) (foundation.MarginHistory, error) {
	return foundation.MarginHistory{Security: security, Data: []foundation.MarginTrading{{Canonical: security.Canonical}}}, nil
}
func (fixedTaiwanFundamentals) Fundamentals(_ context.Context, security foundation.SecurityIdentity, _ int) (foundation.TaiwanFundamentals, error) {
	return foundation.TaiwanFundamentals{Security: security, Capabilities: map[string]foundation.FundamentalCapability{"monthly_revenue": {Status: "official"}}}, nil
}
func (fixedTaiwanSnapshot) Freshness(time.Time) foundation.TaiwanFreshness {
	return foundation.TaiwanFreshness{DailyStatus: "current", Timezone: "Asia/Taipei", Cutoff: "17:30"}
}

func TestTaiwanMarketRoutesResolveCanonicalIdentity(t *testing.T) {
	server := NewServer(Config{TaiwanDirectory: fixedTaiwanDirectory{}, TaiwanMarket: fixedTaiwanMarket{}, TaiwanChip: fixedTaiwanChip{}, TaiwanFundamentals: fixedTaiwanFundamentals{}, TaiwanSnapshot: fixedTaiwanSnapshot{}, TaiwanBreadth: fixedTaiwanBreadth{}, TaiwanEmotion: fixedTaiwanEmotion{}, TaiwanIndustryRadar: fixedTaiwanIndustryRadar{}})
	for _, path := range []string{"/api/v1/tw/quotes?symbols=2330", "/api/v1/tw/kline?symbol=台積電&limit=5", "/api/v1/tw/indexes", "/api/v1/tw/institutional?symbol=2330&limit=20", "/api/v1/tw/margin?symbol=台積電&limit=20", "/api/v1/tw/fundamentals?symbol=2330&months=24", "/api/v1/tw/data-status", "/api/v1/tw/market-breadth", "/api/v1/tw/market-emotion", "/api/v1/tw/industry-radar"} {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("%s: status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}
