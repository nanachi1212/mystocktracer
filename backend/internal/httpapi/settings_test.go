package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nanachi1212/mystocktracer/backend/internal/agent"
	"github.com/nanachi1212/mystocktracer/backend/internal/agent/hermesadapter"
	"github.com/nanachi1212/mystocktracer/backend/internal/appsettings"
)

func settingsCall(server *Server, method, path, body string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(method, path, strings.NewReader(body)))
	return response
}

func TestSettingsSecretBoundaryAndStoredProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	store, err := appsettings.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	runtime := &fakeAgentRuntime{status: agent.Status{Available: true}}
	server := NewServer(Config{SettingsStore: store, AgentRuntime: runtime})
	defer server.Close()
	request := `{"llm":{"provider":"custom","base_url":"https://model.example/v1","model":"synthetic-model","api_mode":"chat_completions","api_key":"synthetic-private-key"}}`
	response := settingsCall(server, http.MethodPut, "/api/v1/settings", request)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "synthetic-private-key") ||
		!strings.Contains(response.Body.String(), `"api_key":{"configured":true}`) {
		t.Fatalf("unsafe update result: %d %s", response.Code, response.Body.String())
	}
	if runtime.lastKey == nil || *runtime.lastKey != "synthetic-private-key" {
		t.Fatal("secret was not sent to the agent runtime")
	}
	written, err := os.ReadFile(path)
	if err != nil || strings.Contains(string(written), "synthetic-private-key") {
		t.Fatal("general settings file stored a model secret")
	}
	reopened, err := appsettings.Open(path)
	if err != nil || reopened.Snapshot().LLM.Model != "synthetic-model" || reopened.Snapshot().LLM.APIKey != "" {
		t.Fatalf("saved public settings changed: %v", err)
	}
	get := settingsCall(server, http.MethodGet, "/api/v1/settings", "")
	if get.Code != http.StatusOK || strings.Contains(get.Body.String(), "synthetic-private-key") ||
		!strings.Contains(get.Body.String(), `"hermes":`) || !strings.Contains(get.Body.String(), `"agent":`) {
		t.Fatalf("GET settings contract changed: %d %s", get.Code, get.Body.String())
	}
}

func TestSettingsPatchValidationAndAlertPreference(t *testing.T) {
	store, _ := appsettings.Open("")
	server := NewServer(Config{SettingsStore: store, AgentRuntime: &fakeAgentRuntime{status: agent.Status{Available: true}}})
	defer server.Close()
	for _, test := range []struct{ name, body string }{
		{"unsafe URL", `{"llm":{"base_url":"file:///tmp/key"}}`},
		{"unknown field", `{"credentials":{"tushare_token":"synthetic"}}`},
		{"removed automation", `{"review_automation":{"profiles":[]}}`},
		{"removed secret", `{"clear_secrets":["xueqiu_cookie"]}`},
		{"two JSON objects", `{} {}`},
		{"large request", `{"llm":{"model":"` + strings.Repeat("x", 64<<10) + `"}}`},
		{"invalid timeout", `{"llm":{"response_timeout_seconds":29}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if response := settingsCall(server, http.MethodPut, "/api/v1/settings", test.body); response.Code != http.StatusBadRequest {
				t.Fatalf("invalid patch accepted: %d %s", response.Code, response.Body.String())
			}
		})
	}
	response := settingsCall(server, http.MethodPut, "/api/v1/settings", `{"taiwan_alerts":{"corporate_events_enabled":false}}`)
	if response.Code != http.StatusOK || store.Snapshot().TaiwanAlerts.CorporateEventsEnabled {
		t.Fatalf("alert preference was not saved: %d", response.Code)
	}
	var view struct {
		Data settingsView `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &view); err != nil || view.Data.TaiwanAlerts.CorporateEventsEnabled {
		t.Fatalf("alert preference was not returned: %v", err)
	}
	noRuntime := NewServer(Config{SettingsStore: store})
	defer noRuntime.Close()
	if result := settingsCall(noRuntime, http.MethodPut, "/api/v1/settings", `{"llm":{"api_key":"synthetic"}}`); result.Code != http.StatusServiceUnavailable {
		t.Fatalf("secret patch without runtime returned %d", result.Code)
	}
}

func TestSettingsCanClearSecretAndSelectProviders(t *testing.T) {
	store, _ := appsettings.Open("")
	runtime := &fakeAgentRuntime{status: agent.Status{Available: true, Configured: true, APIKeyConfigured: true}}
	server := NewServer(Config{SettingsStore: store, AgentRuntime: runtime})
	defer server.Close()
	for _, provider := range []struct{ id, url, model string }{
		{"moonshot", "https://api.moonshot.cn/v1", "moonshot-v1-8k"},
		{"minimax", "https://api.minimaxi.com/v1", "MiniMax-Text-01"},
		{"zhipu", "https://open.bigmodel.cn/api/paas/v4", "glm-4-plus"},
		{"siliconflow", "https://api.siliconflow.cn/v1", "vendor/model"},
	} {
		t.Run(provider.id, func(t *testing.T) {
			body := fmt.Sprintf(`{"llm":{"provider":%q,"base_url":%q,"model":%q,"api_mode":"chat_completions"}}`, provider.id, provider.url, provider.model)
			response := settingsCall(server, http.MethodPut, "/api/v1/settings", body)
			if response.Code != http.StatusOK || store.Snapshot().LLM.Provider != provider.id {
				t.Fatalf("provider update failed: %d", response.Code)
			}
		})
	}
	response := settingsCall(server, http.MethodPut, "/api/v1/settings", `{"clear_secrets":["llm_api_key"]}`)
	if response.Code != http.StatusOK || runtime.lastKey == nil || *runtime.lastKey != "" || store.Snapshot().LLM.APIKey != "" {
		t.Fatalf("secret clearing failed: %d", response.Code)
	}
}

func TestSettingsProfileSecretsAndTimeoutSurviveSelection(t *testing.T) {
	store, _ := appsettings.Open("")
	root := t.TempDir()
	python := filepath.Join(root, "python")
	if err := os.WriteFile(python, []byte("synthetic"), 0o700); err != nil {
		t.Fatal(err)
	}
	runtime := hermesadapter.New(hermesadapter.Config{Home: filepath.Join(root, "agent"), PythonPath: python})
	server := NewServer(Config{SettingsStore: store, AgentRuntime: runtime})
	defer server.Close()
	request := `{"llm":{"response_timeout_seconds":600},"llm_profiles":[
		{"id":"one","name":"模型一","provider":"custom","base_url":"https://one.example/v1","model":"model-one","api_mode":"chat_completions","api_key":"synthetic-one"},
		{"id":"two","name":"模型二","provider":"custom","base_url":"https://two.example/v1","model":"model-two","api_mode":"codex_responses","api_key":"synthetic-two"}],"active_llm_profile_id":"one"}`
	response := settingsCall(server, http.MethodPut, "/api/v1/settings", request)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "synthetic-one") ||
		strings.Contains(response.Body.String(), "synthetic-two") {
		t.Fatalf("profile update failed or leaked: %d %s", response.Code, response.Body.String())
	}
	selected := settingsCall(server, http.MethodPut, "/api/v1/settings", `{"active_llm_profile_id":"two"}`)
	if selected.Code != http.StatusOK || store.Snapshot().LLM.Model != "model-two" ||
		store.Snapshot().LLM.ResponseTimeoutSeconds != 600 {
		t.Fatalf("profile switch changed model or timeout: %d %+v", selected.Code, store.Snapshot().LLM)
	}
	if key, err := runtime.ModelAPIKey(); err != nil || key != "synthetic-two" {
		t.Fatalf("active profile secret was lost: %v", err)
	}
	if key, err := runtime.ModelAPIKeyForProfile("one"); err != nil || key != "synthetic-one" {
		t.Fatalf("inactive profile secret was lost: %v", err)
	}
}

func TestModelProbeSuccessAndProviderFailure(t *testing.T) {
	store, _ := appsettings.Open("")
	_, _ = store.Update(func(value *appsettings.Values) error {
		value.LLM = appsettings.LLM{Provider: "custom", BaseURL: "https://model.example/v1", Model: "synthetic-model"}
		return nil
	})
	runtime := &fakeAgentRuntime{status: agent.Status{Available: true, Configured: true, APIKeyConfigured: true},
		promptResult: agent.PromptResult{Content: llmProbeMarker}}
	server := NewServer(Config{SettingsStore: store, AgentRuntime: runtime})
	defer server.Close()
	response := settingsCall(server, http.MethodPost, "/api/v1/settings/llm/test", "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"ok":true`) ||
		!strings.Contains(response.Body.String(), llmProbeMarker) || !strings.Contains(response.Body.String(), `"runtime":"agent-runtime"`) {
		t.Fatalf("model probe changed: %d %s", response.Code, response.Body.String())
	}
	runtime.promptErr = errors.New("synthetic-provider-private-diagnostic")
	failed := settingsCall(server, http.MethodPost, "/api/v1/settings/llm/test", "")
	if failed.Code != http.StatusBadGateway || strings.Contains(failed.Body.String(), "synthetic-provider-private-diagnostic") {
		t.Fatalf("provider error leaked: %d %s", failed.Code, failed.Body.String())
	}
}

func TestModelTimeoutIncludesRuntimeAllowance(t *testing.T) {
	store, _ := appsettings.Open("")
	server := NewServer(Config{SettingsStore: store})
	defer server.Close()
	if got := server.modelResponseTimeout(); got != 315*time.Second {
		t.Fatalf("default model timeout: %s", got)
	}
	_, _ = store.Update(func(value *appsettings.Values) error { value.LLM.ResponseTimeoutSeconds = 600; return nil })
	if got := server.modelResponseTimeout(); got != 615*time.Second {
		t.Fatalf("configured model timeout: %s", got)
	}
}
