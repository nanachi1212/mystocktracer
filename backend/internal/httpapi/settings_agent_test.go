package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nanachi1212/mystocktracer/backend/internal/agent"
)

func agentSettingsCall(server *Server, method, body string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(method, "/api/v1/settings/agent", strings.NewReader(body)))
	return response
}

func TestAgentSettingsFiltersHistoricalSkillAndKeepsOtherCapabilities(t *testing.T) {
	runtime := &fakeAgentRuntime{agentSettings: agent.Settings{Skills: []agent.SkillSetting{
		{Name: legacyMainlandSkillName, Enabled: true}, {Name: "taiwan-research", Enabled: true},
	}}}
	server := NewServer(Config{AgentRuntime: runtime})
	defer server.Close()
	response := agentSettingsCall(server, http.MethodGet, "")
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), legacyMainlandSkillName) ||
		!strings.Contains(response.Body.String(), "taiwan-research") {
		t.Fatalf("skill view changed: %d %s", response.Code, response.Body.String())
	}
}

func TestAgentSettingsPatchPreservesUnsubmittedSecrets(t *testing.T) {
	runtime := &fakeAgentRuntime{agentSettings: agent.Settings{
		ReasoningEffort: "medium",
		Skills:          []agent.SkillSetting{{Name: "taiwan-research", Enabled: true}},
		MCPServers: []agent.MCPServerSetting{{Name: "local-tool", Enabled: true, Transport: "stdio", Command: "npx",
			Env: map[string]string{"TOKEN": "synthetic-old-secret", "REMOVE": "synthetic-removal"}}},
	}}
	server := NewServer(Config{AgentRuntime: runtime})
	defer server.Close()
	partial := agentSettingsCall(server, http.MethodPut, `{"reasoning_effort":"xhigh"}`)
	if partial.Code != http.StatusOK || runtime.agentSettings.ReasoningEffort != "xhigh" ||
		len(runtime.agentSettings.Skills) != 1 || len(runtime.agentSettings.MCPServers) != 1 {
		t.Fatalf("partial update erased state: %d %+v", partial.Code, runtime.agentSettings)
	}
	body := `{"skills":[{"name":"taiwan-research","enabled":false}],"mcp_servers":[{"name":"renamed-tool","original_name":"local-tool","enabled":true,"transport":"stdio","command":"npx","args":["-y","tool"],"env":{},"clear_env":["REMOVE"],"headers":{},"clear_headers":[],"timeout":120,"connect_timeout":30,"supports_parallel_tool_calls":true}]}`
	response := agentSettingsCall(server, http.MethodPut, body)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "synthetic-old-secret") ||
		!strings.Contains(response.Body.String(), `"configured":true`) {
		t.Fatalf("unsafe agent response: %d %s", response.Code, response.Body.String())
	}
	serverState := runtime.agentSettings.MCPServers[0]
	if runtime.agentSettings.Skills[0].Enabled || serverState.Name != "renamed-tool" ||
		serverState.Env["TOKEN"] != "synthetic-old-secret" || serverState.Env["REMOVE"] != "" ||
		!serverState.SupportsParallelToolCall {
		t.Fatalf("secret-preserving merge changed: %+v", runtime.agentSettings)
	}
	if get := agentSettingsCall(server, http.MethodGet, ""); get.Code != http.StatusOK ||
		strings.Contains(get.Body.String(), "synthetic-old-secret") {
		t.Fatalf("GET exposed secret: %d %s", get.Code, get.Body.String())
	}
}

func TestAgentSettingsRejectsMalformedAndOversizedPatches(t *testing.T) {
	server := NewServer(Config{AgentRuntime: &fakeAgentRuntime{}})
	defer server.Close()
	for _, test := range []struct{ name, body string }{
		{"unsupported reasoning", `{"reasoning_effort":"turbo"}`},
		{"invalid server name", `{"mcp_servers":[{"name":"bad name","enabled":true,"transport":"stdio","command":"npx"}]}`},
		{"unsafe endpoint", `{"mcp_servers":[{"name":"remote","enabled":true,"transport":"http","url":"file:///secret"}]}`},
		{"command newline", `{"mcp_servers":[{"name":"local","enabled":true,"transport":"stdio","command":"bad\ncommand"}]}`},
		{"unknown field", `{"surprise":true}`},
		{"two objects", `{} {}`},
		{"too large", `{"reasoning_effort":"` + strings.Repeat("a", 256<<10) + `"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := agentSettingsCall(server, http.MethodPut, test.body)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("invalid patch accepted: %d", response.Code)
			}
		})
	}
	if response := agentSettingsCall(NewServer(Config{}), http.MethodGet, ""); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing service returned %d", response.Code)
	}
}
