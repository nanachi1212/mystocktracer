package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"easy-stock/backend/internal/foundation"
	"easy-stock/backend/internal/taiwanwatchlist"
)

func seedHTTPAlert(t *testing.T, store *taiwanwatchlist.Store) taiwanwatchlist.Alert {
	t.Helper()
	if _, err := store.Add(t.Context(), taiwanwatchlist.Entry{Canonical: "2330.TWSE", Code: "2330", Name: "台積電", Exchange: "TWSE", SecurityType: "stock"}); err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 9, 17, 5, 0, 0, 0, time.UTC)
	baselineTime := t0.Add(-time.Minute)
	baseline := foundation.TaiwanCorporateEventFeed{Status: foundation.TaiwanCorporateEventsAvailable, Provider: "provider-a", Events: []foundation.TaiwanCorporateEvent{{ID: "baseline", PublishedAt: &baselineTime}}}
	if _, err := store.ApplyCorporateEventsWithPreference(t.Context(), "2330.TWSE", baseline, t0, true); err != nil {
		t.Fatal(err)
	}
	newTime := t0.Add(time.Minute)
	feed := foundation.TaiwanCorporateEventFeed{Status: foundation.TaiwanCorporateEventsAvailable, Provider: "provider-a", Events: []foundation.TaiwanCorporateEvent{{ID: "new", Title: "重大公告", Category: "重大訊息", PublishedAt: &newTime, Provider: "provider-a", Source: "official-feed", SourceURL: "https://example.com/new", Status: foundation.TaiwanCorporateEventsAvailable}}}
	if _, err := store.ApplyCorporateEventsWithPreference(t.Context(), "2330.TWSE", feed, t0.Add(2*time.Minute), true); err != nil {
		t.Fatal(err)
	}
	page, err := store.ListAlerts(t.Context(), taiwanwatchlist.AlertFilterAll, 10, 0)
	if err != nil || len(page.Alerts) != 1 {
		t.Fatalf("seeded alerts = %+v, %v", page, err)
	}
	return page.Alerts[0]
}

func TestTaiwanAlertsListReadAndReadAll(t *testing.T) {
	store, err := taiwanwatchlist.OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	alert := seedHTTPAlert(t, store)
	server := NewServer(Config{WatchlistStore: store})
	defer server.Close()

	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/tw/alerts?status=unread&limit=1&offset=0", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Data struct {
			Alerts                 []taiwanwatchlist.Alert `json:"alerts"`
			UnreadCount            int                     `json:"unread_count"`
			Total                  int                     `json:"total"`
			CorporateEventsEnabled bool                    `json:"corporate_events_enabled"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data.Alerts) != 1 || payload.Data.UnreadCount != 1 || payload.Data.Total != 1 || !payload.Data.CorporateEventsEnabled {
		t.Fatalf("list payload=%+v", payload.Data)
	}

	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/api/v1/tw/alerts/"+strconv.FormatInt(alert.ID, 10)+"/read", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("mark read status=%d body=%s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/api/v1/tw/alerts/read-all", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("mark all status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestTaiwanAlertsRejectMalformedPaginationFilterAndID(t *testing.T) {
	store, err := taiwanwatchlist.OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(Config{WatchlistStore: store})
	defer server.Close()
	for _, target := range []string{
		"/api/v1/tw/alerts?limit=0", "/api/v1/tw/alerts?limit=101", "/api/v1/tw/alerts?offset=-1", "/api/v1/tw/alerts?status=bogus",
	} {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("target=%s status=%d body=%s", target, response.Code, response.Body.String())
		}
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/api/v1/tw/alerts/not-a-number/read", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid id status=%d body=%s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/api/v1/tw/alerts/999/read", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("missing id status=%d body=%s", response.Code, response.Body.String())
	}
}
