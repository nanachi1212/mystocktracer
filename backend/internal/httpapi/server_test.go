package httpapi

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func serveRequest(server *Server, method, path string, headers http.Header) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, nil)
	request.Header = headers
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	return response
}

func TestPrivateRoutesRequireProductToken(t *testing.T) {
	server := NewServer(Config{Token: "synthetic-token"})
	defer server.Close()
	for _, test := range []struct {
		name, path string
		headers    http.Header
		status     int
	}{
		{name: "public health", path: "/api/health", status: http.StatusOK},
		{name: "missing token", path: "/api/v1/settings", status: http.StatusUnauthorized},
		{name: "wrong token", path: "/api/v1/settings", headers: http.Header{"Authorization": {"Bearer wrong"}}, status: http.StatusUnauthorized},
		{name: "bearer token", path: "/api/v1/settings", headers: http.Header{"Authorization": {"Bearer synthetic-token"}}, status: http.StatusOK},
		{name: "legacy query compatibility", path: "/api/v1/settings?token=synthetic-token", status: http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := serveRequest(server, http.MethodGet, test.path, test.headers)
			if response.Code != test.status {
				t.Fatalf("%s returned %d: %s", test.path, response.Code, response.Body.String())
			}
		})
	}
}

func TestRequestLogPreservesCorrelationWithoutCredential(t *testing.T) {
	var output bytes.Buffer
	server := NewServer(Config{Token: "synthetic-token", Logger: log.New(&output, "", 0)})
	defer server.Close()
	response := serveRequest(server, http.MethodGet, "/api/v1/settings?token=synthetic-token",
		http.Header{"X-Request-Id": {"trace-synthetic"}})
	if response.Code != http.StatusOK || response.Header().Get("X-Request-ID") != "trace-synthetic" {
		t.Fatalf("correlation response: %d %q", response.Code, response.Header().Get("X-Request-ID"))
	}
	logText := output.String()
	for _, part := range []string{`feature="settings"`, `request_id="trace-synthetic"`, `path="/api/v1/settings"`} {
		if !strings.Contains(logText, part) {
			t.Fatalf("request log omitted %s: %s", part, logText)
		}
	}
	if strings.Contains(logText, "synthetic-token") || strings.Contains(logText, "?token=") {
		t.Fatal("request log included credential or query")
	}
	for _, path := range []string{"/api/v1/tw/dashboard", "/api/v1/tw/stocks/2330.TWSE/research-history"} {
		if requestFeature(path) != "taiwan" {
			t.Fatalf("Taiwan route lost its log category: %s", path)
		}
	}
}

func TestCORSAllowsLocalDeleteAndRejectsRemoteOrigin(t *testing.T) {
	server := NewServer(Config{Token: "synthetic-token"})
	defer server.Close()
	local := serveRequest(server, http.MethodOptions, "/api/v1/tw/watchlist/2330.TWSE", http.Header{
		"Origin": {"http://127.0.0.1:20073"}, "Access-Control-Request-Method": {http.MethodDelete},
		"Access-Control-Request-Headers": {"authorization"},
	})
	if local.Code != http.StatusNoContent || local.Header().Get("Access-Control-Allow-Origin") != "http://127.0.0.1:20073" ||
		!strings.Contains(local.Header().Get("Access-Control-Allow-Methods"), http.MethodDelete) ||
		!strings.Contains(strings.ToLower(local.Header().Get("Access-Control-Allow-Headers")), "authorization") {
		t.Fatalf("local preflight changed: %d %+v", local.Code, local.Header())
	}
	remote := serveRequest(server, http.MethodOptions, "/api/v1/settings", http.Header{"Origin": {"https://remote.example"}})
	if remote.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("remote origin was reflected")
	}
}

func TestRemovedMarketEndpointsRemainUnavailable(t *testing.T) {
	server := NewServer(Config{})
	defer server.Close()
	retired := []string{
		"/api/v1/sources", "/api/v1/quotes/realtime", "/api/v1/market/indexes", "/api/v1/themes/overview",
		"/api/v1/sector-map", "/api/v1/short-term/limit-up-ladder", "/api/v1/short-term/mastery",
		"/api/v1/stocks/ai-analysis", "/api/v1/stocks/directory", "/api/v1/stocks/hot-ranks",
		"/api/v1/portfolio-inspections", "/api/v1/reviews/posts", "/api/v1/strategy/inflections/evaluate",
		"/api/v1/ws/stream",
	}
	for _, path := range retired {
		if response := serveRequest(server, http.MethodGet, path, nil); response.Code != http.StatusNotFound {
			t.Errorf("retired route %s responded with %d", path, response.Code)
		}
	}
}
