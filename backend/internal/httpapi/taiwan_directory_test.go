package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"easy-stock/backend/internal/foundation"
)

type fixedTaiwanDirectory struct{}

func (fixedTaiwanDirectory) Directory(context.Context) ([]foundation.SecurityIdentity, error) {
	now := time.Now()
	return []foundation.SecurityIdentity{
		{Canonical: "2330.TWSE", Code: "2330", Name: "台積電", FullName: "台灣積體電路製造股份有限公司", Market: "TW", Exchange: "TWSE", Type: foundation.SecurityTypeStock, Currency: "TWD", Timezone: "Asia/Taipei", Provider: "twse", SourceURL: "https://example.test/twse", RetrievedAt: now},
		{Canonical: "6488.TPEX", Code: "6488", Name: "環球晶", Market: "TW", Exchange: "TPEX", Type: foundation.SecurityTypeStock, Currency: "TWD", Timezone: "Asia/Taipei", Provider: "tpex", SourceURL: "https://example.test/tpex", RetrievedAt: now},
		{Canonical: "0050.TWSE", Code: "0050", Name: "元大台灣50", Market: "TW", Exchange: "TWSE", Type: foundation.SecurityTypeETF, Currency: "TWD", Timezone: "Asia/Taipei", Provider: "twse", SourceURL: "https://example.test/etf", RetrievedAt: now},
		{Canonical: "00631L.TWSE", Code: "00631L", Name: "元大台灣50正2", Market: "TW", Exchange: "TWSE", Type: foundation.SecurityTypeETF, Currency: "TWD", Timezone: "Asia/Taipei", Provider: "twse", SourceURL: "https://example.test/etf", RetrievedAt: now},
	}, nil
}

func TestTaiwanDirectorySearchesCodeAndName(t *testing.T) {
	server := NewServer(Config{TaiwanDirectory: fixedTaiwanDirectory{}})
	for _, query := range []string{"2330", "台積電", "6488", "環球晶", "0050", "元大台灣50"} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/tw/securities?query="+query, nil)
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("query %q status = %d, body=%s", query, response.Code, response.Body.String())
		}
		var payload struct {
			Data taiwanDirectoryData `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Data.Total != 1 || payload.Data.Securities[0].Currency != "TWD" {
			t.Fatalf("query %q data = %+v", query, payload.Data)
		}
	}
}
