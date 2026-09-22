package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nanachi1212/mystocktracer/backend/internal/stockanalysis"
	"github.com/nanachi1212/mystocktracer/backend/internal/taiwanwatchlist"
)

func seedHTTPResearchHistory(t *testing.T, store *taiwanwatchlist.Store, runID string, at time.Time) {
	t.Helper()
	evidence := json.RawMessage(`{"research_version":"taiwan_ai_research_v2","price":{"status":"available"},"market":{"status":"available"},"industry":{"status":"available"},"institutional":{"status":"available"},"margin":{"status":"available"},"fundamentals":{"status":"available"},"corporate_events":{"status":"no_events","events":[]}}`)
	result := json.RawMessage(`{"model_version":"taiwan_ai_research_v2","symbol":"2330.TWSE","status":"available","headline":"研究摘要","summary":"結構化摘要","sections":{},"strengths":[],"risks":[],"conflicts":[],"data_limitations":[],"research_notes":[]}`)
	_, inserted, err := store.SaveResearchHistory(t.Context(), taiwanwatchlist.ResearchHistoryRecord{RunID: runID, Canonical: "2330.TWSE", SecurityName: "台積電", CreatedAt: at, ResearchVersion: stockanalysis.TaiwanAIResearchVersionV2, PayloadVersion: stockanalysis.TaiwanAIResearchVersionV2, EvidenceSnapshot: evidence, ResearchResult: result, Provenance: json.RawMessage(`{}`), Validity: json.RawMessage(`{"valid":true}`), Completeness: "complete"})
	if err != nil || !inserted {
		t.Fatalf("seed inserted=%v err=%v", inserted, err)
	}
}

func TestTaiwanResearchHistoryAPIsListGetCompareAndValidate(t *testing.T) {
	store, err := taiwanwatchlist.OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	t0 := time.Date(2026, 9, 17, 1, 0, 0, 0, time.UTC)
	seedHTTPResearchHistory(t, store, "research-http-0001", t0)
	seedHTTPResearchHistory(t, store, "research-http-0002", t0.Add(time.Minute))
	server := NewServer(Config{WatchlistStore: store})
	defer server.Close()

	for _, target := range []string{
		"/api/v1/tw/stocks/2330.TWSE/research-history?limit=1&offset=0",
		"/api/v1/tw/stocks/2330.TWSE/research-history/research-http-0002",
		"/api/v1/tw/stocks/2330.TWSE/research-history/research-http-0002/comparison",
	} {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("target=%s status=%d body=%s", target, response.Code, response.Body.String())
		}
	}
	for _, target := range []string{
		"/api/v1/tw/stocks/2330.TWSE/research-history?limit=0",
		"/api/v1/tw/stocks/2330.TWSE/research-history?limit=101",
		"/api/v1/tw/stocks/2330.TWSE/research-history?offset=-1",
		"/api/v1/tw/stocks/2330.TWSE/research-history/bad",
	} {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("target=%s status=%d body=%s", target, response.Code, response.Body.String())
		}
	}
}

func validHTTPResearchJSON() string {
	return `{"model_version":"taiwan_ai_research_v2","symbol":"2330.TWSE","headline":"證據摘要","summary":"各項證據狀態如下。","sections":{"price":{"text":"價格證據。","evidence_keys":["interpretation.components.price"]},"market":{"text":"市場證據。","evidence_keys":["interpretation.components.market"]},"industry":{"text":"產業證據。","evidence_keys":["interpretation.components.industry"]},"institutional":{"text":"法人證據。","evidence_keys":["interpretation.components.institutional"]},"margin":{"text":"融資融券證據。","evidence_keys":["interpretation.components.margin"]},"fundamentals":{"text":"基本面證據。","evidence_keys":["interpretation.components.fundamentals"]},"corporate_events":{"text":"事件未查詢。","evidence_keys":["corporate_events.status"]}},"strengths":[],"risks":[],"conflicts":[],"data_limitations":[],"research_notes":[]}`
}

func TestSuccessfulTaiwanResearchPersistsIdempotentlyAndFailureDoesNotPersist(t *testing.T) {
	store, err := taiwanwatchlist.OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := NewServer(Config{TaiwanIntelligence: fixedTaiwanIntelligence{}, AgentRuntime: &fakeTaiwanResearchGateway{fakeAgentRuntime: &fakeAgentRuntime{}, content: validHTTPResearchJSON()}, WatchlistStore: store})
	defer server.Close()
	for range 2 {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/tw/stocks/2330.TWSE/research", nil)
		request.Header.Set("X-Research-Run-ID", "research-request-0001")
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"history_run_id":"research-request-0001"`) {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
	}
	page, err := store.ListResearchHistory(t.Context(), "2330.TWSE", 10, 0)
	if err != nil || page.Total != 1 {
		t.Fatalf("page=%+v err=%v", page, err)
	}

	invalidRequest := httptest.NewRequest(http.MethodPost, "/api/v1/tw/stocks/2330.TWSE/research", nil)
	invalidRequest.Header.Set("X-Research-Run-ID", "invalid run id")
	invalidResponse := httptest.NewRecorder()
	server.ServeHTTP(invalidResponse, invalidRequest)
	if invalidResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalid run id status=%d body=%s", invalidResponse.Code, invalidResponse.Body.String())
	}
	page, err = store.ListResearchHistory(t.Context(), "2330.TWSE", 10, 0)
	if err != nil || page.Total != 1 {
		t.Fatalf("invalid run id changed history page=%+v err=%v", page, err)
	}

	failedStore, err := taiwanwatchlist.OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer failedStore.Close()
	failedServer := NewServer(Config{TaiwanIntelligence: fixedTaiwanIntelligence{}, AgentRuntime: &fakeAgentRuntime{}, WatchlistStore: failedStore})
	defer failedServer.Close()
	response := httptest.NewRecorder()
	failedServer.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/tw/stocks/2330.TWSE/research", nil))
	failedPage, err := failedStore.ListResearchHistory(t.Context(), "2330.TWSE", 10, 0)
	if response.Code != http.StatusOK || err != nil || failedPage.Total != 0 {
		t.Fatalf("status=%d page=%+v err=%v", response.Code, failedPage, err)
	}
}
