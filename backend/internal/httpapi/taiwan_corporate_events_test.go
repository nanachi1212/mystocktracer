package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nanachi1212/mystocktracer/backend/internal/foundation"
	"github.com/nanachi1212/mystocktracer/backend/internal/taiwanwatchlist"
)

type fixedCorporateEventsProvider struct {
	feed  foundation.TaiwanCorporateEventFeed
	calls []string
}

func (p *fixedCorporateEventsProvider) CorporateEvents(_ context.Context, canonical string, _, _ int) foundation.TaiwanCorporateEventFeed {
	p.calls = append(p.calls, canonical)
	feed := p.feed
	feed.Events = append([]foundation.TaiwanCorporateEvent(nil), feed.Events...)
	for index := range feed.Events {
		feed.Events[index].Symbol = canonical
	}
	return feed
}

func TestTaiwanCorporateEventsGetPreservesProviderState(t *testing.T) {
	provider := &fixedCorporateEventsProvider{feed: foundation.TaiwanCorporateEventFeed{
		Status: foundation.TaiwanCorporateEventsPartial, Provider: foundation.TaiwanCorporateEventProviderToAlpha,
		Source: foundation.TaiwanCorporateEventSourceMOPS, Partial: true, Reason: "full text is still being backfilled",
		Events: []foundation.TaiwanCorporateEvent{{ID: "mops:twse:2330:2026-09-16:1", Title: "test", Partial: true}},
	}}
	server := NewServer(Config{TaiwanCorporateEvents: provider, TaiwanDirectory: fixedTaiwanDirectory{}})
	defer server.Close()
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/tw/stocks/2330.TWSE/corporate-events", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Data foundation.TaiwanCorporateEventFeed `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.Status != foundation.TaiwanCorporateEventsPartial || !payload.Data.Partial || len(payload.Data.Events) != 1 || len(provider.calls) != 1 {
		t.Fatalf("payload=%+v calls=%v", payload.Data, provider.calls)
	}
}

func TestTaiwanCorporateEventSyncUsesExplicitSymbolsAndCreatesBaseline(t *testing.T) {
	store, err := taiwanwatchlist.OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	published := time.Now().UTC().Add(-time.Minute)
	provider := &fixedCorporateEventsProvider{feed: foundation.TaiwanCorporateEventFeed{
		Status: foundation.TaiwanCorporateEventsAvailable, Provider: foundation.TaiwanCorporateEventProviderToAlpha,
		Source: foundation.TaiwanCorporateEventSourceMOPS,
		Events: []foundation.TaiwanCorporateEvent{{ID: "mops:twse:2330:2026-09-16:1", Title: "test", PublishedAt: &published}},
	}}
	server := NewServer(Config{TaiwanCorporateEvents: provider, WatchlistStore: store, TaiwanDirectory: fixedTaiwanDirectory{}})
	defer server.Close()
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/tw/corporate-events/sync", strings.NewReader(`{"symbols":["2330.TWSE"]}`)))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Data struct {
			Items map[string]corporateEventSyncItem `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	item := payload.Data.Items["2330.TWSE"]
	if !item.Change.Baseline || len(item.Change.NewEvents) != 0 || item.Feed.Status != foundation.TaiwanCorporateEventsAvailable {
		t.Fatalf("first sync must be a non-notifying baseline: %+v", item)
	}
}

func TestTaiwanCorporateEventSyncRejectsExplicitInvalidSymbols(t *testing.T) {
	server := NewServer(Config{})
	defer server.Close()
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/tw/corporate-events/sync", strings.NewReader(`{"symbols":["2330.SSE"]}`)))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestTaiwanCorporateEventsReturnsUnsupportedWithoutCallingProvider(t *testing.T) {
	provider := &fixedCorporateEventsProvider{}
	server := NewServer(Config{TaiwanCorporateEvents: provider, TaiwanDirectory: fixedTaiwanDirectory{}})
	defer server.Close()
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/tw/stocks/0050.TWSE/corporate-events", nil))
	if response.Code != http.StatusOK || len(provider.calls) != 0 {
		t.Fatalf("status=%d calls=%v body=%s", response.Code, provider.calls, response.Body.String())
	}
	var payload struct {
		Data foundation.TaiwanCorporateEventFeed `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.Status != foundation.TaiwanCorporateEventsUnsupported {
		t.Fatalf("feed=%+v", payload.Data)
	}
}
