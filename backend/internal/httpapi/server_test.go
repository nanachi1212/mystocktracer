package httpapi

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServerRequiresConfiguredTokenForPrivateRoutes(t *testing.T) {
	server := NewServer(Config{Token: "secret"})

	healthReq := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	healthRec := httptest.NewRecorder()
	server.ServeHTTP(healthRec, healthReq)
	if healthRec.Code != http.StatusOK {
		t.Fatalf("health status = %d, want %d", healthRec.Code, http.StatusOK)
	}

	unauthorizedReq := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	unauthorizedRec := httptest.NewRecorder()
	server.ServeHTTP(unauthorizedRec, unauthorizedReq)
	if unauthorizedRec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want %d", unauthorizedRec.Code, http.StatusUnauthorized)
	}

	headerReq := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	headerReq.Header.Set("Authorization", "Bearer secret")
	headerRec := httptest.NewRecorder()
	server.ServeHTTP(headerRec, headerReq)
	if headerRec.Code != http.StatusOK {
		t.Fatalf("header authorized status = %d, want %d", headerRec.Code, http.StatusOK)
	}

	queryReq := httptest.NewRequest(http.MethodGet, "/api/v1/settings?token=secret", nil)
	queryRec := httptest.NewRecorder()
	server.ServeHTTP(queryRec, queryReq)
	if queryRec.Code != http.StatusOK {
		t.Fatalf("query authorized status = %d, want %d", queryRec.Code, http.StatusOK)
	}
}

func TestServerLogsSanitizedFeatureRequestWithCorrelationID(t *testing.T) {
	var logs bytes.Buffer
	server := NewServer(Config{
		Token:  "secret-token",
		Logger: log.New(&logs, "", 0),
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/settings?token=secret-token", nil)
	request.Header.Set("X-Request-ID", "support-123")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || recorder.Header().Get("X-Request-ID") != "support-123" {
		t.Fatalf("unexpected response: status=%d request_id=%q", recorder.Code, recorder.Header().Get("X-Request-ID"))
	}
	content := logs.String()
	if !strings.Contains(content, `feature="settings"`) || !strings.Contains(content, `request_id="support-123"`) || !strings.Contains(content, `path="/api/v1/settings"`) {
		t.Fatalf("missing request diagnostics: %s", content)
	}
	if strings.Contains(content, "secret-token") || strings.Contains(content, "?token=") {
		t.Fatalf("request log leaked query credentials: %s", content)
	}
}

// Taiwan routes must carry their own feature label now that the legacy A-share prefixes are gone;
// otherwise every Taiwan request would log as the generic "http" bucket.
func TestServerLabelsTaiwanRequestsWithTaiwanFeature(t *testing.T) {
	if feature := requestFeature("/api/v1/tw/dashboard"); feature != "taiwan" {
		t.Fatalf("taiwan feature = %q, want %q", feature, "taiwan")
	}
	if feature := requestFeature("/api/v1/tw/stocks/2330.TWSE/research-history"); feature != "taiwan" {
		t.Fatalf("taiwan research-history feature = %q, want %q", feature, "taiwan")
	}
}

func TestServerCORSPreflightAllowsDelete(t *testing.T) {
	server := NewServer(Config{Token: "secret"})
	request := httptest.NewRequest(http.MethodOptions, "/api/v1/tw/watchlist/2330.TWSE", nil)
	request.Header.Set("Origin", "http://127.0.0.1:20073")
	request.Header.Set("Access-Control-Request-Method", http.MethodDelete)
	request.Header.Set("Access-Control-Request-Headers", "authorization")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if recorder.Header().Get("Access-Control-Allow-Origin") != "http://127.0.0.1:20073" {
		t.Fatalf("allow origin = %q", recorder.Header().Get("Access-Control-Allow-Origin"))
	}
	if !strings.Contains(recorder.Header().Get("Access-Control-Allow-Methods"), http.MethodDelete) {
		t.Fatalf("allow methods = %q", recorder.Header().Get("Access-Control-Allow-Methods"))
	}
	if !strings.Contains(strings.ToLower(recorder.Header().Get("Access-Control-Allow-Headers")), "authorization") {
		t.Fatalf("allow headers = %q", recorder.Header().Get("Access-Control-Allow-Headers"))
	}
}

func TestServerCORSDoesNotReflectUntrustedOrigin(t *testing.T) {
	server := NewServer(Config{})
	request := httptest.NewRequest(http.MethodOptions, "/api/v1/settings", nil)
	request.Header.Set("Origin", "https://attacker.example")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, request)

	if origin := recorder.Header().Get("Access-Control-Allow-Origin"); origin != "" {
		t.Fatalf("untrusted origin was reflected: %q", origin)
	}
}

// The legacy A-share surface was removed in Phase B1; these routes must stay gone so a future
// refactor cannot quietly re-register a China-market endpoint.
func TestServerNoLongerServesLegacyAShareRoutes(t *testing.T) {
	server := NewServer(Config{})
	for _, path := range []string{
		"/api/v1/sources",
		"/api/v1/quotes/realtime",
		"/api/v1/market/indexes",
		"/api/v1/themes/overview",
		"/api/v1/sector-map",
		"/api/v1/short-term/limit-up-ladder",
		"/api/v1/short-term/mastery",
		"/api/v1/stocks/ai-analysis",
		"/api/v1/stocks/directory",
		"/api/v1/stocks/hot-ranks",
		"/api/v1/portfolio-inspections",
		"/api/v1/reviews/posts",
		"/api/v1/strategy/inflections/evaluate",
		"/api/v1/ws/stream",
	} {
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want %d", path, recorder.Code, http.StatusNotFound)
		}
	}
}
