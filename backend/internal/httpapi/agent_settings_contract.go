package httpapi

import "github.com/nanachi1212/mystocktracer/backend/internal/hermes"

type agentSettingsView struct {
	ReasoningEffort string             `json:"reasoning_effort"`
	Skills          []hermes.SkillInfo `json:"skills"`
	MCPServers      []mcpServerView    `json:"mcp_servers"`
}

type mcpServerView struct {
	Name                     string                         `json:"name"`
	Enabled                  bool                           `json:"enabled"`
	Transport                string                         `json:"transport"`
	Command                  string                         `json:"command,omitempty"`
	Args                     []string                       `json:"args,omitempty"`
	Env                      map[string]secretSettingStatus `json:"env,omitempty"`
	URL                      string                         `json:"url,omitempty"`
	Headers                  map[string]secretSettingStatus `json:"headers,omitempty"`
	Timeout                  int                            `json:"timeout,omitempty"`
	ConnectTimeout           int                            `json:"connect_timeout,omitempty"`
	SupportsParallelToolCall bool                           `json:"supports_parallel_tool_calls,omitempty"`
}

type agentSettingsUpdateRequest struct {
	ReasoningEffort *string            `json:"reasoning_effort"`
	Skills          *[]skillUpdate     `json:"skills"`
	MCPServers      *[]mcpServerUpdate `json:"mcp_servers"`
}

type skillUpdate struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type mcpServerUpdate struct {
	Name                     string             `json:"name"`
	OriginalName             string             `json:"original_name"`
	Enabled                  bool               `json:"enabled"`
	Transport                string             `json:"transport"`
	Command                  string             `json:"command"`
	Args                     []string           `json:"args"`
	Env                      map[string]*string `json:"env"`
	ClearEnv                 []string           `json:"clear_env"`
	URL                      string             `json:"url"`
	Headers                  map[string]*string `json:"headers"`
	ClearHeaders             []string           `json:"clear_headers"`
	Timeout                  int                `json:"timeout"`
	ConnectTimeout           int                `json:"connect_timeout"`
	SupportsParallelToolCall bool               `json:"supports_parallel_tool_calls"`
}
