package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"easy-stock/backend/internal/foundation"
	"easy-stock/backend/internal/taiwanportfolio"
)

type portfolioDirectory struct{}

func (portfolioDirectory) Directory(context.Context) ([]foundation.SecurityIdentity, error) {
	now := time.Now()
	return []foundation.SecurityIdentity{
		{Canonical: "2330.TWSE", Code: "2330", Name: "台積電", Market: "TW", Exchange: "TWSE", Type: foundation.SecurityTypeStock, Currency: "TWD", Timezone: "Asia/Taipei", Industry: "24", RetrievedAt: now},
		{Canonical: "6488.TPEX", Code: "6488", Name: "環球晶", Market: "TW", Exchange: "TPEX", Type: foundation.SecurityTypeStock, Currency: "TWD", Timezone: "Asia/Taipei", Industry: "24", RetrievedAt: now},
		{Canonical: "0050.TWSE", Code: "0050", Name: "元大台灣50", Market: "TW", Exchange: "TWSE", Type: foundation.SecurityTypeETF, Currency: "TWD", Timezone: "Asia/Taipei", RetrievedAt: now},
	}, nil
}

type portfolioMarket struct {
	prices map[string]float64
	errors map[string]error
	stale  map[string]bool
}

func (p portfolioMarket) Quote(_ context.Context, security foundation.SecurityIdentity) (foundation.Quote, error) {
	if err := p.errors[security.Canonical]; err != nil {
		return foundation.Quote{}, err
	}
	return foundation.Quote{Symbol: security.Canonical, Name: security.Name, Price: p.prices[security.Canonical], Meta: foundation.SourceMeta{Source: "official", Stale: p.stale[security.Canonical]}}, nil
}
func (portfolioMarket) KLine(context.Context, foundation.SecurityIdentity, int) ([]foundation.KLine, error) {
	return nil, nil
}
func (portfolioMarket) Indexes(context.Context) ([]foundation.MarketIndexSeries, foundation.SourceMeta, error) {
	return nil, foundation.SourceMeta{}, nil
}

func newPortfolioServer(t *testing.T, market TaiwanMarketProvider) *Server {
	t.Helper()
	store, err := taiwanportfolio.OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(Config{TaiwanDirectory: portfolioDirectory{}, TaiwanMarket: market, TaiwanPortfolioStore: store})
	t.Cleanup(func() { _ = server.Close() })
	return server
}

func portfolioRequest(server *Server, method, path, body string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(method, path, strings.NewReader(body)))
	return response
}

func TestTaiwanPortfolioCRUDDuplicateAndETF(t *testing.T) {
	server := newPortfolioServer(t, portfolioMarket{prices: map[string]float64{"2330.TWSE": 1000, "0050.TWSE": 200}})
	created := portfolioRequest(server, http.MethodPost, "/api/v1/tw/portfolio", `{"symbol":"2330.TWSE","shares":100,"average_cost":900,"note":"核心"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	duplicate := portfolioRequest(server, http.MethodPost, "/api/v1/tw/portfolio", `{"symbol":"2330.twse","shares":120,"average_cost":910}`)
	if duplicate.Code != http.StatusOK {
		t.Fatalf("duplicate status=%d body=%s", duplicate.Code, duplicate.Body.String())
	}
	etf := portfolioRequest(server, http.MethodPost, "/api/v1/tw/portfolio", `{"symbol":"0050.TWSE","shares":10,"average_cost":180}`)
	if etf.Code != http.StatusCreated {
		t.Fatalf("ETF status=%d body=%s", etf.Code, etf.Body.String())
	}
	list := portfolioRequest(server, http.MethodGet, "/api/v1/tw/portfolio", "")
	if list.Code != http.StatusOK || strings.Count(list.Body.String(), `"canonical"`) != 2 || !strings.Contains(list.Body.String(), `"shares":120`) {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	updated := portfolioRequest(server, http.MethodPut, "/api/v1/tw/portfolio/2330.TWSE", `{"shares":150,"average_cost":920,"note":"更新"}`)
	if updated.Code != http.StatusOK || !strings.Contains(updated.Body.String(), `"shares":150`) {
		t.Fatalf("update status=%d body=%s", updated.Code, updated.Body.String())
	}
	deleted := portfolioRequest(server, http.MethodDelete, "/api/v1/tw/portfolio/2330.TWSE", "")
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d body=%s", deleted.Code, deleted.Body.String())
	}
	absent := portfolioRequest(server, http.MethodDelete, "/api/v1/tw/portfolio/2330.TWSE", "")
	if absent.Code != http.StatusNotFound {
		t.Fatalf("delete absent status=%d body=%s", absent.Code, absent.Body.String())
	}
}

func TestTaiwanPortfolioRejectsInvalidInputsWithoutWriting(t *testing.T) {
	server := newPortfolioServer(t, portfolioMarket{})
	requests := []string{
		`{"symbol":"9999.TWSE","shares":1,"average_cost":1}`,
		`{"symbol":"2330.TWSE","shares":0,"average_cost":1}`,
		`{"symbol":"2330.TWSE","shares":1,"average_cost":-1}`,
		`{"symbol":"2330.TWSE","shares":1,"average_cost":1,"note":"` + strings.Repeat("長", taiwanportfolio.MaxNoteLength+1) + `"}`,
	}
	for _, body := range requests {
		response := portfolioRequest(server, http.MethodPost, "/api/v1/tw/portfolio", body)
		if response.Code < 400 {
			t.Fatalf("invalid request accepted: status=%d body=%s", response.Code, response.Body.String())
		}
	}
	list := portfolioRequest(server, http.MethodGet, "/api/v1/tw/portfolio", "")
	if !strings.Contains(list.Body.String(), `"holdings":[]`) {
		t.Fatalf("invalid input wrote rows: %s", list.Body.String())
	}
}

func TestTaiwanPortfolioStorageErrorDoesNotLeakInternals(t *testing.T) {
	store, err := taiwanportfolio.OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	_ = store.Close()
	server := NewServer(Config{TaiwanDirectory: portfolioDirectory{}, TaiwanPortfolioStore: store})
	response := portfolioRequest(server, http.MethodPost, "/api/v1/tw/portfolio", `{"symbol":"2330.TWSE","shares":1,"average_cost":1}`)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if strings.Contains(strings.ToLower(response.Body.String()), "sql") || strings.Contains(response.Body.String(), "database is closed") {
		t.Fatalf("storage error leaked: %s", response.Body.String())
	}
}

func TestTaiwanPortfolioSummaryCalculationsAndConcentration(t *testing.T) {
	server := newPortfolioServer(t, portfolioMarket{prices: map[string]float64{"2330.TWSE": 1000, "6488.TPEX": 500}})
	portfolioRequest(server, http.MethodPost, "/api/v1/tw/portfolio", `{"symbol":"2330.TWSE","shares":100,"average_cost":900}`)
	portfolioRequest(server, http.MethodPost, "/api/v1/tw/portfolio", `{"symbol":"6488.TPEX","shares":100,"average_cost":400}`)
	response := portfolioRequest(server, http.MethodGet, "/api/v1/tw/portfolio/summary", "")
	if response.Code != http.StatusOK {
		t.Fatalf("summary status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Data taiwanPortfolioSummary `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	summary := payload.Data
	if summary.TotalMarketValue == nil || *summary.TotalMarketValue != 150000 || summary.TotalCost != 130000 || summary.TotalUnrealizedPL == nil || *summary.TotalUnrealizedPL != 20000 {
		t.Fatalf("totals: %+v", summary)
	}
	if summary.Holdings[0].Weight == nil || *summary.Holdings[0].Weight < 66.66 || summary.Concentration.Top3Percent == nil || *summary.Concentration.Top3Percent != 100 {
		t.Fatalf("weights: %+v", summary)
	}
	if len(summary.Concentration.Industries) != 1 || summary.Concentration.Industries[0].Weight != 100 {
		t.Fatalf("industry: %+v", summary.Concentration)
	}
}

func TestTaiwanPortfolioProviderOutageDoesNotCorruptHoldings(t *testing.T) {
	server := newPortfolioServer(t, portfolioMarket{prices: map[string]float64{"2330.TWSE": 1000}, errors: map[string]error{"6488.TPEX": errors.New("provider down")}})
	portfolioRequest(server, http.MethodPost, "/api/v1/tw/portfolio", `{"symbol":"2330.TWSE","shares":100,"average_cost":900}`)
	portfolioRequest(server, http.MethodPost, "/api/v1/tw/portfolio", `{"symbol":"6488.TPEX","shares":100,"average_cost":400}`)
	response := portfolioRequest(server, http.MethodGet, "/api/v1/tw/portfolio/summary", "")
	var payload struct {
		Data taiwanPortfolioSummary `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.Status != "partial" || payload.Data.TotalMarketValue != nil || payload.Data.TotalUnrealizedPL != nil || payload.Data.PricedHoldings != 1 {
		t.Fatalf("outage semantics: %+v", payload.Data)
	}
	if payload.Data.Holdings[1].PriceStatus != "unavailable" || payload.Data.Holdings[1].CurrentPrice != nil {
		t.Fatalf("unavailable holding: %+v", payload.Data.Holdings[1])
	}
	list := portfolioRequest(server, http.MethodGet, "/api/v1/tw/portfolio", "")
	if strings.Count(list.Body.String(), `"canonical"`) != 2 {
		t.Fatalf("provider outage corrupted holdings: %s", list.Body.String())
	}
}

func TestTaiwanPortfolioMarksStaleQuote(t *testing.T) {
	server := newPortfolioServer(t, portfolioMarket{prices: map[string]float64{"2330.TWSE": 1000}, stale: map[string]bool{"2330.TWSE": true}})
	portfolioRequest(server, http.MethodPost, "/api/v1/tw/portfolio", `{"symbol":"2330.TWSE","shares":100,"average_cost":900}`)
	response := portfolioRequest(server, http.MethodGet, "/api/v1/tw/portfolio/summary", "")
	if !strings.Contains(response.Body.String(), `"status":"stale"`) || !strings.Contains(response.Body.String(), `"price_status":"stale"`) {
		t.Fatalf("stale semantics: %s", response.Body.String())
	}
}

func TestTaiwanPortfolioHoldingCanUseExistingEventSync(t *testing.T) {
	store, err := taiwanportfolio.OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	provider := &fixedCorporateEventsProvider{feed: foundation.TaiwanCorporateEventFeed{
		Status: foundation.TaiwanCorporateEventsAvailable, Provider: foundation.TaiwanCorporateEventProviderToAlpha,
		Source: foundation.TaiwanCorporateEventSourceMOPS, Events: []foundation.TaiwanCorporateEvent{},
	}}
	server := NewServer(Config{TaiwanDirectory: portfolioDirectory{}, TaiwanPortfolioStore: store, TaiwanCorporateEvents: provider})
	defer server.Close()
	created := portfolioRequest(server, http.MethodPost, "/api/v1/tw/portfolio", `{"symbol":"2330.TWSE","shares":100,"average_cost":900}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	synced := portfolioRequest(server, http.MethodPost, "/api/v1/tw/corporate-events/sync", `{"symbols":["2330.TWSE"]}`)
	if synced.Code != http.StatusOK || len(provider.calls) != 1 || provider.calls[0] != "2330.TWSE" {
		t.Fatalf("sync status=%d calls=%v body=%s", synced.Code, provider.calls, synced.Body.String())
	}
}
