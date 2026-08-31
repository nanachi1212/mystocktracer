package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"easy-stock/backend/internal/foundation"
)

type fixedTaiwanMarket struct{}
type fixedTaiwanChip struct{}
type fixedTaiwanFundamentals struct{}
type fixedTaiwanSnapshot struct{}

func (fixedTaiwanMarket) Quote(_ context.Context, security foundation.SecurityIdentity) (foundation.Quote, error) {
	return foundation.Quote{Symbol: security.Canonical, Name: security.Name, Price: 100, Meta: foundation.SourceMeta{Source: "official", FetchedAt: time.Now(), Status: "official_close"}}, nil
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
	server := NewServer(Config{TaiwanDirectory: fixedTaiwanDirectory{}, TaiwanMarket: fixedTaiwanMarket{}, TaiwanChip: fixedTaiwanChip{}, TaiwanFundamentals: fixedTaiwanFundamentals{}, TaiwanSnapshot: fixedTaiwanSnapshot{}})
	for _, path := range []string{"/api/v1/tw/quotes?symbols=2330", "/api/v1/tw/kline?symbol=台積電&limit=5", "/api/v1/tw/indexes", "/api/v1/tw/institutional?symbol=2330&limit=20", "/api/v1/tw/margin?symbol=台積電&limit=20", "/api/v1/tw/fundamentals?symbol=2330&months=24", "/api/v1/tw/data-status"} {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("%s: status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}
