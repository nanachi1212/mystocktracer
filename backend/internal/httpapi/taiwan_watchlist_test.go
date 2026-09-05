package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"easy-stock/backend/internal/taiwanwatchlist"
)

func decodeTaiwanWatchlistSecurities(t *testing.T, response *httptest.ResponseRecorder) []taiwanWatchlistSecurity {
	t.Helper()
	var payload struct {
		Data struct {
			Securities []taiwanWatchlistSecurity `json:"securities"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v, body=%s", err, response.Body.String())
	}
	return payload.Data.Securities
}

func postTaiwanWatchlist(server *Server, symbol string) *httptest.ResponseRecorder {
	body := strings.NewReader(`{"symbol":"` + symbol + `"}`)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/tw/watchlist", body))
	return response
}

func TestTaiwanWatchlistGetIsEmptyByDefault(t *testing.T) {
	server := NewServer(Config{TaiwanDirectory: fixedTaiwanDirectory{}})
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/tw/watchlist", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	if securities := decodeTaiwanWatchlistSecurities(t, response); len(securities) != 0 {
		t.Fatalf("expected empty watchlist, got %+v", securities)
	}
}

func TestTaiwanWatchlistPostValidTWSESecurity(t *testing.T) {
	server := NewServer(Config{TaiwanDirectory: fixedTaiwanDirectory{}})
	response := postTaiwanWatchlist(server, "2330.TWSE")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Data struct {
			Security taiwanWatchlistSecurity `json:"security"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	security := payload.Data.Security
	if security.Canonical != "2330.TWSE" || security.Code != "2330" || security.Name != "台積電" || security.Exchange != "TWSE" || security.SecurityType != "stock" {
		t.Fatalf("unexpected saved security: %+v", security)
	}
	if security.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be set")
	}
}

func TestTaiwanWatchlistPostValidTPEXSecurity(t *testing.T) {
	server := NewServer(Config{TaiwanDirectory: fixedTaiwanDirectory{}})
	response := postTaiwanWatchlist(server, "6488.TPEX")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	securities := decodeTaiwanWatchlistSecurities(t, listTaiwanWatchlist(server))
	if len(securities) != 1 || securities[0].Canonical != "6488.TPEX" || securities[0].Exchange != "TPEX" {
		t.Fatalf("unexpected watchlist after TPEX add: %+v", securities)
	}
}

func listTaiwanWatchlist(server *Server) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/tw/watchlist", nil))
	return response
}

func TestTaiwanWatchlistGetReturnsPersistedIdentities(t *testing.T) {
	server := NewServer(Config{TaiwanDirectory: fixedTaiwanDirectory{}})
	postTaiwanWatchlist(server, "2330.TWSE")
	postTaiwanWatchlist(server, "6488.TPEX")
	securities := decodeTaiwanWatchlistSecurities(t, listTaiwanWatchlist(server))
	if len(securities) != 2 {
		t.Fatalf("expected 2 persisted securities, got %+v", securities)
	}
	if securities[0].Canonical != "2330.TWSE" || securities[1].Canonical != "6488.TPEX" {
		t.Fatalf("unexpected order/content: %+v", securities)
	}
}

func TestTaiwanWatchlistDoesNotTrustClientSuppliedIdentityFields(t *testing.T) {
	server := NewServer(Config{TaiwanDirectory: fixedTaiwanDirectory{}})
	body := strings.NewReader(`{"symbol":"2330.TWSE","name":"偽造名稱","exchange":"NYSE","security_type":"crypto"}`)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/tw/watchlist", body))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	securities := decodeTaiwanWatchlistSecurities(t, listTaiwanWatchlist(server))
	if len(securities) != 1 || securities[0].Name != "台積電" || securities[0].Exchange != "TWSE" || securities[0].SecurityType != "stock" {
		t.Fatalf("expected backend-authoritative identity, not client-supplied fields: %+v", securities)
	}
}

func TestTaiwanWatchlistDuplicatePostDoesNotDuplicate(t *testing.T) {
	server := NewServer(Config{TaiwanDirectory: fixedTaiwanDirectory{}})
	postTaiwanWatchlist(server, "2330.TWSE")
	postTaiwanWatchlist(server, "2330.twse")
	securities := decodeTaiwanWatchlistSecurities(t, listTaiwanWatchlist(server))
	if len(securities) != 1 {
		t.Fatalf("expected exactly one entry after duplicate POST, got %+v", securities)
	}
}

func TestTaiwanWatchlistDeleteRemovesEntry(t *testing.T) {
	server := NewServer(Config{TaiwanDirectory: fixedTaiwanDirectory{}})
	postTaiwanWatchlist(server, "2330.TWSE")
	postTaiwanWatchlist(server, "6488.TPEX")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/api/v1/tw/watchlist/2330.TWSE", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	securities := decodeTaiwanWatchlistSecurities(t, listTaiwanWatchlist(server))
	if len(securities) != 1 || securities[0].Canonical != "6488.TPEX" {
		t.Fatalf("expected only 6488.TPEX to remain, got %+v", securities)
	}
}

func TestTaiwanWatchlistDeleteAbsentSymbolIsSafe(t *testing.T) {
	server := NewServer(Config{TaiwanDirectory: fixedTaiwanDirectory{}})
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/api/v1/tw/watchlist/9999.TWSE", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("expected removing an absent symbol to be a safe no-op (200), got status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestTaiwanWatchlistPostMalformedSymbolRejected(t *testing.T) {
	server := NewServer(Config{TaiwanDirectory: fixedTaiwanDirectory{}})
	for _, symbol := range []string{"", "2330", "no-dot-here"} {
		response := postTaiwanWatchlist(server, symbol)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("symbol %q: expected 400 for malformed symbol, got status=%d body=%s", symbol, response.Code, response.Body.String())
		}
	}
}

func TestTaiwanWatchlistPostUnsupportedExchangeRejected(t *testing.T) {
	server := NewServer(Config{TaiwanDirectory: fixedTaiwanDirectory{}})
	response := postTaiwanWatchlist(server, "2330.SSE")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unsupported exchange, got status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestTaiwanWatchlistPostSecurityNotFoundRejected(t *testing.T) {
	server := NewServer(Config{TaiwanDirectory: fixedTaiwanDirectory{}})
	response := postTaiwanWatchlist(server, "9999.TWSE")
	if response.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for a well-formed but non-existent security, got status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestTaiwanWatchlistStorageErrorDoesNotExposeRawInternalError(t *testing.T) {
	store, err := taiwanwatchlist.OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	_ = store.Close() // force every subsequent store call to fail with a raw sql/driver error
	server := NewServer(Config{TaiwanDirectory: fixedTaiwanDirectory{}, WatchlistStore: store})
	response := postTaiwanWatchlist(server, "2330.TWSE")
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on storage failure, got status=%d body=%s", response.Code, response.Body.String())
	}
	if strings.Contains(strings.ToLower(response.Body.String()), "sql") || strings.Contains(response.Body.String(), "database is closed") {
		t.Fatalf("raw storage error leaked to client: %s", response.Body.String())
	}
}

func TestTaiwanWatchlistResolveHelperDistinguishesFailureKinds(t *testing.T) {
	server := NewServer(Config{TaiwanDirectory: fixedTaiwanDirectory{}})
	ctx := context.Background()
	if _, status, message := server.resolveTaiwanWatchlistSymbol(ctx, ""); status != http.StatusBadRequest || message == "" {
		t.Fatalf("empty symbol: status=%d message=%q", status, message)
	}
	if _, status, message := server.resolveTaiwanWatchlistSymbol(ctx, "abc"); status != http.StatusBadRequest || message == "" {
		t.Fatalf("no-dot symbol: status=%d message=%q", status, message)
	}
	if _, status, message := server.resolveTaiwanWatchlistSymbol(ctx, "2330.SSE"); status != http.StatusBadRequest || message == "" {
		t.Fatalf("unsupported exchange: status=%d message=%q", status, message)
	}
	if _, status, message := server.resolveTaiwanWatchlistSymbol(ctx, "9999.TWSE"); status != http.StatusNotFound || message == "" {
		t.Fatalf("not found: status=%d message=%q", status, message)
	}
	identity, status, message := server.resolveTaiwanWatchlistSymbol(ctx, "2330.TWSE")
	if message != "" || identity == nil || identity.Canonical != "2330.TWSE" {
		t.Fatalf("valid symbol: identity=%+v status=%d message=%q", identity, status, message)
	}
}
