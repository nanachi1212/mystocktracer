package taiwan

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOfficialDirectoryParsesTWSETPExAndETF(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/opendata/t187ap03_L":
			_, _ = w.Write([]byte(`[{"公司代號":"2330","公司簡稱":"台積電","公司名稱":"台灣積體電路製造股份有限公司","產業別":"24","上市日期":"19940905"}]`))
		case "/opendata/t187ap47_L":
			_, _ = w.Write([]byte(`[{"基金代號":"0050","基金簡稱":"元大台灣50","基金中文名稱":"元大台灣卓越50證券投資信託基金","基金類型":"國內成分證券指數股票型基金","上市日期":"0920630"}]`))
		case "/mopsfin_t187ap03_O":
			_, _ = w.Write([]byte(`[{"SecuritiesCompanyCode":"6488","CompanyAbbreviation":"環球晶","CompanyName":"環球晶圓股份有限公司","SecuritiesIndustryCode":"24","DateOfListing":"20150925"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, HTTPClient: server.Client()})
	items, err := client.Directory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("Directory() count = %d", len(items))
	}
	byCode := map[string]string{}
	for _, item := range items {
		byCode[item.Code] = item.Canonical + "/" + string(item.Type)
		if item.Market != "TW" || item.Currency != "TWD" || item.Timezone != "Asia/Taipei" || item.SourceURL == "" {
			t.Fatalf("incomplete identity: %+v", item)
		}
	}
	if byCode["2330"] != "2330.TWSE/stock" || byCode["6488"] != "6488.TPEX/stock" || byCode["0050"] != "0050.TWSE/etf" {
		t.Fatalf("identities = %v", byCode)
	}
}

func TestDirectoryRejectsPartialProviderSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/mopsfin_t187ap03_O" {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()
	client := NewClient(Config{TWSEBaseURL: server.URL, TPExBaseURL: server.URL, HTTPClient: server.Client()})
	if _, err := client.Directory(context.Background()); err == nil {
		t.Fatal("Directory() error = nil")
	}
}
