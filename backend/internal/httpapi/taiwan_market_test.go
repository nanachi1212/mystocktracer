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

func (fixedTaiwanMarket) Quote(_ context.Context, security foundation.SecurityIdentity) (foundation.Quote, error) {
	return foundation.Quote{Symbol: security.Canonical, Name: security.Name, Price: 100, Meta: foundation.SourceMeta{Source: "official", FetchedAt: time.Now(), Status: "official_close"}}, nil
}
func (fixedTaiwanMarket) KLine(_ context.Context, security foundation.SecurityIdentity, _ int) ([]foundation.KLine, error) {
	return []foundation.KLine{{Symbol: security.Canonical, Close: 100, Time: time.Now()}}, nil
}
func (fixedTaiwanMarket) Indexes(context.Context) ([]foundation.MarketIndexSeries, foundation.SourceMeta, error) {
	return []foundation.MarketIndexSeries{{Index: foundation.MarketIndexSnapshot{ID: "taiex"}}}, foundation.SourceMeta{Source: "official", FetchedAt: time.Now()}, nil
}

func TestTaiwanMarketRoutesResolveCanonicalIdentity(t *testing.T) {
	server := NewServer(Config{TaiwanDirectory: fixedTaiwanDirectory{}, TaiwanMarket: fixedTaiwanMarket{}})
	for _, path := range []string{"/api/v1/tw/quotes?symbols=2330", "/api/v1/tw/kline?symbol=台積電&limit=5", "/api/v1/tw/indexes"} {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("%s: status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}
