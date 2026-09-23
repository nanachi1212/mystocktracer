package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nanachi1212/mystocktracer/backend/internal/agent"
	"github.com/nanachi1212/mystocktracer/backend/internal/appsettings"
)

func TestModelDiscoveryKeepsCredentialsOutOfResults(t *testing.T) {
	cases := []struct {
		name, provider, requestKey, savedKey string
		wantHeader, otherHeader              string
	}{
		{name: "saved compatible key", provider: "custom", savedKey: "synthetic-saved-key",
			wantHeader: "Authorization", otherHeader: "x-api-key"},
		{name: "new Anthropic key", provider: "anthropic", requestKey: "synthetic-request-key",
			wantHeader: "x-api-key", otherHeader: "Authorization"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var path, credential, unwanted, version string
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path, credential = r.URL.Path, r.Header.Get(test.wantHeader)
				unwanted, version = r.Header.Get(test.otherHeader), r.Header.Get("anthropic-version")
				_, _ = w.Write([]byte(`{"data":[{"id":"zeta"},{"id":"alpha"},{"id":"alpha"},{"id":""}]}`))
			}))
			defer provider.Close()
			store, err := appsettings.Open("")
			if err != nil {
				t.Fatal(err)
			}
			runtime := &fakeAgentRuntime{status: agent.Status{Available: true, APIKeyConfigured: test.savedKey != ""}, modelAPIKey: test.savedKey}
			server := NewServer(Config{SettingsStore: store, AgentRuntime: runtime})
			baseURL := provider.URL
			if test.provider == "custom" {
				baseURL += "/v1"
			}
			requestBody := fmt.Sprintf(`{"provider":%q,"base_url":%q}`, test.provider, baseURL)
			if test.requestKey != "" {
				requestBody = fmt.Sprintf(`{"provider":%q,"base_url":%q,"api_key":%q}`, test.provider, baseURL, test.requestKey)
			}
			response := httptest.NewRecorder()
			server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/settings/llm/models", strings.NewReader(requestBody)))
			if response.Code != http.StatusOK || path != "/v1/models" || unwanted != "" {
				t.Fatalf("discovery failed: status=%d path=%q payload=%s", response.Code, path, response.Body.String())
			}
			key := test.requestKey
			if key == "" {
				key = test.savedKey
			}
			if test.provider == "custom" {
				key = "Bearer " + key
			} else if version == "" {
				t.Fatal("Anthropic version header missing")
			}
			if credential != key || strings.Contains(response.Body.String(), key) ||
				strings.Count(response.Body.String(), `"id":"alpha"`) != 1 ||
				strings.Index(response.Body.String(), "alpha") > strings.Index(response.Body.String(), "zeta") {
				t.Fatalf("credential or normalized result mismatch: %s", response.Body.String())
			}
		})
	}
}

func TestModelDiscoveryRejectsUnsafeInputsAndProviderFailures(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"debug":"synthetic-provider-secret"}`))
	}))
	defer provider.Close()
	store, _ := appsettings.Open("")
	server := NewServer(Config{SettingsStore: store})
	for _, test := range []struct {
		name, body string
		status     int
	}{
		{name: "non-http URL", body: `{"provider":"custom","base_url":"file:///private"}`, status: http.StatusBadRequest},
		{name: "missing hosted key", body: `{"provider":"openai","base_url":"https://api.openai.com/v1","api_key":""}`, status: http.StatusPreconditionFailed},
		{name: "provider rejection", body: fmt.Sprintf(`{"provider":"custom","base_url":%q,"api_key":"synthetic-key"}`, provider.URL), status: http.StatusBadGateway},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/settings/llm/models", strings.NewReader(test.body)))
			if response.Code != test.status || strings.Contains(response.Body.String(), "synthetic-provider-secret") ||
				strings.Contains(response.Body.String(), "synthetic-key") {
				t.Fatalf("unsafe discovery response %d: %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestOfficialProviderModelRoutes(t *testing.T) {
	for _, row := range []struct{ provider, base string }{
		{"moonshot", "https://api.moonshot.cn/v1"},
		{"minimax", "https://api.minimaxi.com/v1"},
		{"zhipu", "https://open.bigmodel.cn/api/paas/v4"},
		{"siliconflow", "https://api.siliconflow.cn/v1"},
	} {
		t.Run(row.provider, func(t *testing.T) {
			url, err := agent.ModelsURL(row.provider, row.base)
			if !agent.SupportedModelProvider(row.provider) || err != nil || url != row.base+"/models" {
				t.Fatalf("model route for %s: %q, %v", row.provider, url, err)
			}
		})
	}
}
