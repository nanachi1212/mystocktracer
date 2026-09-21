package toalpha

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nanachi1212/mystocktracer/backend/internal/foundation"
)

func TestCorporateEventsDisabledDoesNotCallNetwork(t *testing.T) {
	client := NewClient(Config{})
	got := client.CorporateEvents(t.Context(), "2330.TWSE", 30, 10)
	if got.Status != foundation.TaiwanCorporateEventsNotQueried || len(got.Events) != 0 {
		t.Fatalf("disabled feed = %+v", got)
	}
}

func TestCorporateEventsNormalizesProvenanceDedupesAndCaches(t *testing.T) {
	requests := 0
	payload := `{
		"stock":{"id":"2330","name":"台積電","market":"上市"},"count":2,
		"rows":[
			{"id":"2330","date":"2026-09-10","time":"13:51:47","seq":1,"subject":"營收公告","category":"財報營收","important":false,"fact_date":"2026-09-10","excerpt":"公告內容","full_text":true},
			{"id":"2330","date":"2026-09-10","time":"13:51:47","seq":1,"subject":"營收公告","category":"財報營收","important":false,"fact_date":"2026-09-10","excerpt":"公告內容","full_text":true}
		],"updated":"2026-09-16 15:20:06.466352+08","source":"公開資訊觀測站重大訊息","link":"https://mops.example/events"
	}`
	server := mockMCPServer(t, func(w http.ResponseWriter, _ *http.Request) {
		requests++
		writeToolResult(w, payload, false)
	})
	defer server.Close()
	now := time.Date(2026, 9, 16, 7, 0, 0, 0, time.UTC)
	client := NewClient(Config{Enabled: true, Endpoint: server.URL, HTTPClient: server.Client(), Now: func() time.Time { return now }})
	got := client.CorporateEvents(t.Context(), "2330.TWSE", 30, 10)
	if got.Status != foundation.TaiwanCorporateEventsAvailable || got.Provider != "toalpha" || len(got.Events) != 1 {
		t.Fatalf("normalized feed = %+v", got)
	}
	event := got.Events[0]
	if event.ID != "mops:上市:2330:2026-09-10:1" || event.Symbol != "2330.TWSE" || event.SourceURL != "https://mops.example/events" || event.PublishedAt == nil {
		t.Fatalf("normalized event = %+v", event)
	}
	if event.ClassificationSource != "third_party_enrichment" || event.Important == nil || *event.Important {
		t.Fatalf("classification provenance lost: %+v", event)
	}
	_ = client.CorporateEvents(t.Context(), "2330.TWSE", 30, 10)
	if requests != 1 {
		t.Fatalf("tool requests = %d, want one cached call", requests)
	}
}

func TestCorporateEventsPartialAndMissingFieldsStayPartial(t *testing.T) {
	payload := `{"stock":{"id":"2330","market":"上市"},"count":2,"rows":[{"id":"2330","date":"2026-09-10","time":"","seq":2,"subject":"","excerpt":"partial","full_text":false}],"source":"MOPS","link":"https://example.test"}`
	server := mockMCPServer(t, func(w http.ResponseWriter, _ *http.Request) { writeToolResult(w, payload, false) })
	defer server.Close()
	client := NewClient(Config{Enabled: true, Endpoint: server.URL, HTTPClient: server.Client()})
	got := client.CorporateEvents(t.Context(), "2330.TWSE", 30, 10)
	if got.Status != foundation.TaiwanCorporateEventsPartial || !got.Partial || len(got.Events) != 1 || !got.Events[0].Partial || got.Events[0].PublishedAt != nil {
		t.Fatalf("partial feed = %+v", got)
	}
}

func TestCorporateEventsCountWithoutRowsIsPartialNotNoEvents(t *testing.T) {
	payload := `{"stock":{"id":"2330"},"count":1,"rows":[],"source":"MOPS"}`
	server := mockMCPServer(t, func(w http.ResponseWriter, _ *http.Request) { writeToolResult(w, payload, false) })
	defer server.Close()
	got := NewClient(Config{Enabled: true, Endpoint: server.URL, HTTPClient: server.Client()}).CorporateEvents(t.Context(), "2330.TWSE", 30, 10)
	if got.Status != foundation.TaiwanCorporateEventsPartial || !got.Partial {
		t.Fatalf("count/rows mismatch feed = %+v", got)
	}
}

func TestStableEventIDNormalizesFallbackIdentity(t *testing.T) {
	composed := stableEventID("", "", "2026-09-10", "13:51:47", 0, "2330.TWSE", " MOPS ", "董事會通過 A&B")
	decomposed := stableEventID("", "", " 2026-09-10 ", " 13:51:47 ", 0, "2330.twse", "mops", " 董事會通過\r\nA&amp;B ")
	if composed != decomposed {
		t.Fatalf("normalized ids differ: %q != %q", composed, decomposed)
	}
	otherTime := stableEventID("", "", "2026-09-10", "13:51:48", 0, "2330.TWSE", "MOPS", "董事會通過 A&B")
	if composed == otherTime {
		t.Fatal("different published times collided")
	}
}

func TestCorporateEventsRejectsUnsafeSourceURL(t *testing.T) {
	payload := `{"stock":{"id":"2330","market":"上市"},"count":1,"rows":[{"id":"2330","date":"2026-09-10","time":"13:51:47","seq":1,"subject":"test","full_text":true}],"source":"MOPS","link":"javascript:alert(1)"}`
	server := mockMCPServer(t, func(w http.ResponseWriter, _ *http.Request) { writeToolResult(w, payload, false) })
	defer server.Close()
	got := NewClient(Config{Enabled: true, Endpoint: server.URL, HTTPClient: server.Client()}).CorporateEvents(t.Context(), "2330.TWSE", 30, 10)
	if got.Status != foundation.TaiwanCorporateEventsAvailable || got.SourceURL != "" || len(got.Events) != 1 || got.Events[0].SourceURL != "" {
		t.Fatalf("unsafe provider URL escaped normalization: %+v", got)
	}
}

func TestCorporateEventsReturnsExpiredCacheAsExplicitStaleOnOutage(t *testing.T) {
	now := time.Date(2026, 9, 16, 7, 0, 0, 0, time.UTC)
	failing := false
	payload := `{"stock":{"id":"2330","market":"上市"},"count":1,"rows":[{"id":"2330","date":"2026-09-16","time":"10:00:00","seq":1,"subject":"cached","full_text":true}],"source":"MOPS"}`
	server := mockMCPServer(t, func(w http.ResponseWriter, _ *http.Request) {
		if failing {
			http.Error(w, "temporary", http.StatusServiceUnavailable)
			return
		}
		writeToolResult(w, payload, false)
	})
	defer server.Close()
	client := NewClient(Config{Enabled: true, Endpoint: server.URL, HTTPClient: server.Client(), CacheTTL: time.Minute, Now: func() time.Time { return now }})
	if got := client.CorporateEvents(t.Context(), "2330.TWSE", 30, 10); got.Status != foundation.TaiwanCorporateEventsAvailable {
		t.Fatalf("initial feed=%+v", got)
	}
	now = now.Add(2 * time.Minute)
	failing = true
	got := client.CorporateEvents(t.Context(), "2330.TWSE", 30, 10)
	if got.Status != foundation.TaiwanCorporateEventsStale || !got.Stale || len(got.Events) != 1 || !got.Events[0].Stale {
		t.Fatalf("stale fallback=%+v", got)
	}
}

func TestCorporateEventsMalformedUnknownAndToolErrorsDegrade(t *testing.T) {
	tests := []struct {
		name string
		tool func(http.ResponseWriter, *http.Request)
	}{
		{"malformed payload", func(w http.ResponseWriter, _ *http.Request) { writeToolResult(w, `{`, false) }},
		{"unknown payload field", func(w http.ResponseWriter, _ *http.Request) {
			writeToolResult(w, `{"count":0,"rows":[],"unexpected":true}`, false)
		}},
		{"stock mismatch", func(w http.ResponseWriter, _ *http.Request) {
			writeToolResult(w, `{"stock":{"id":"2317"},"count":0,"rows":[]}`, false)
		}},
		{"unknown content", func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprint(w, `event: message\ndata: {"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"image"}],"isError":false}}\n\n`)
		}},
		{"tool error", func(w http.ResponseWriter, _ *http.Request) { writeToolResult(w, `Unknown tool result`, true) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := mockMCPServer(t, test.tool)
			defer server.Close()
			client := NewClient(Config{Enabled: true, Endpoint: server.URL, HTTPClient: server.Client()})
			got := client.CorporateEvents(t.Context(), "2330.TWSE", 30, 10)
			if got.Status != foundation.TaiwanCorporateEventsUnavailable || len(got.Events) != 0 {
				t.Fatalf("degraded feed = %+v", got)
			}
		})
	}
}

func TestCorporateEventsRetriesOnlyTransientHTTPFailures(t *testing.T) {
	initializes := 0
	payload := `{"stock":{"id":"2330","market":"上市"},"count":0,"rows":[],"source":"MOPS"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Method == "initialize" {
			initializes++
			if initializes == 1 {
				http.Error(w, "temporary", http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Mcp-Session-Id", "test-session")
			fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-03-26"}}`)
			return
		}
		if body.Method == "notifications/initialized" {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		writeToolResult(w, payload, false)
	}))
	defer server.Close()
	feed := NewClient(Config{Enabled: true, Endpoint: server.URL, HTTPClient: server.Client()}).CorporateEvents(t.Context(), "2330.TWSE", 30, 10)
	if feed.Status != foundation.TaiwanCorporateEventsNoEvents || initializes != 2 {
		t.Fatalf("feed=%+v initialize attempts=%d", feed, initializes)
	}
}

func TestCorporateEventsRejectsTrailingToolPayloadWithoutRetry(t *testing.T) {
	calls := 0
	server := mockMCPServer(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		writeToolResult(w, `{"count":0,"rows":[]} {}`, false)
	})
	defer server.Close()
	feed := NewClient(Config{Enabled: true, Endpoint: server.URL, HTTPClient: server.Client()}).CorporateEvents(t.Context(), "2330.TWSE", 30, 10)
	if feed.Status != foundation.TaiwanCorporateEventsUnavailable || calls != 1 {
		t.Fatalf("feed=%+v tool calls=%d", feed, calls)
	}
}

func TestCorporateEventsTimeoutAndConnectionFailureDegrade(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		server := mockMCPServer(t, func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(100 * time.Millisecond)
			writeToolResult(w, `{"count":0,"rows":[]}`, false)
		})
		defer server.Close()
		client := NewClient(Config{Enabled: true, Endpoint: server.URL, HTTPClient: server.Client(), Timeout: 20 * time.Millisecond})
		got := client.CorporateEvents(t.Context(), "2330.TWSE", 30, 10)
		if got.Status != foundation.TaiwanCorporateEventsUnavailable || !strings.Contains(got.Reason, "timed out") {
			t.Fatalf("timeout feed = %+v", got)
		}
	})
	t.Run("connection", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		endpoint, clientHTTP := server.URL, server.Client()
		server.Close()
		client := NewClient(Config{Enabled: true, Endpoint: endpoint, HTTPClient: clientHTTP})
		got := client.CorporateEvents(t.Context(), "2330.TWSE", 30, 10)
		if got.Status != foundation.TaiwanCorporateEventsUnavailable {
			t.Fatalf("connection feed = %+v", got)
		}
	})
}

func mockMCPServer(t *testing.T, toolHandler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		switch body.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "test-session")
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"protocolVersion\":\"2025-03-26\"}}\n\n")
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/call":
			w.Header().Set("Content-Type", "text/event-stream")
			toolHandler(w, r)
		default:
			http.Error(w, "unknown", http.StatusBadRequest)
		}
	}))
}

func writeToolResult(w http.ResponseWriter, text string, isError bool) {
	encoded := strconv.Quote(text)
	fmt.Fprintf(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":2,\"result\":{\"content\":[{\"type\":\"text\",\"text\":%s}],\"isError\":%t}}\n\n", encoded, isError)
}
