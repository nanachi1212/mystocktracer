// Package agent defines mystocktracer-owned AI capabilities. Runtime adapters
// implement these narrow interfaces; product consumers must not depend on an
// adapter's process protocol or configuration format.
package agent

import (
	"context"
	"io"

	"github.com/nanachi1212/mystocktracer/backend/internal/appsettings"
)

type Status struct {
	Available        bool   `json:"available"`
	Configured       bool   `json:"configured"`
	APIKeyConfigured bool   `json:"api_key_configured"`
	Version          string `json:"version,omitempty"`
	Message          string `json:"message,omitempty"`
}

type PromptResult struct{ Content, SessionID, StoredSessionID string }

type Process interface {
	Input() io.WriteCloser
	Output() io.ReadCloser
	Errors() io.ReadCloser
	Wait() error
	Stop() error
}
type StatusReporter interface{ Status() Status }
type Prompter interface {
	Prompt(context.Context, string) (PromptResult, error)
}
type IsolatedPrompter interface {
	PromptIsolated(context.Context, string, string) (PromptResult, error)
}
type ChatRuntime interface {
	Start(context.Context) (Process, error)
}
type SecretReader interface{ ModelAPIKey() (string, error) }
type ModelConfigurator interface {
	SyncLLM(appsettings.LLM, *string) error
}
type ProfileConfigurator interface {
	SyncLLMProfile(appsettings.LLM, string, *string) error
	StoreLLMProfileKey(string, *string) error
	ModelAPIKeyForProfile(string) (string, error)
}

type SkillSetting struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Enabled     bool   `json:"enabled"`
}
type MCPServerSetting struct {
	Name                     string            `json:"name"`
	Enabled                  bool              `json:"enabled"`
	Transport                string            `json:"transport"`
	Command                  string            `json:"command,omitempty"`
	Args                     []string          `json:"args,omitempty"`
	Env                      map[string]string `json:"env,omitempty"`
	URL                      string            `json:"url,omitempty"`
	Headers                  map[string]string `json:"headers,omitempty"`
	Timeout                  int               `json:"timeout,omitempty"`
	ConnectTimeout           int               `json:"connect_timeout,omitempty"`
	SupportsParallelToolCall bool              `json:"supports_parallel_tool_calls,omitempty"`
}
type Settings struct {
	ReasoningEffort string             `json:"reasoning_effort"`
	Skills          []SkillSetting     `json:"skills"`
	MCPServers      []MCPServerSetting `json:"mcp_servers"`
}
type CapabilityConfigurator interface {
	AgentSettings() (Settings, error)
	SyncAgentSettings(Settings) error
}

var ValidReasoningEfforts = []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"}

func IsValidReasoningEffort(value string) bool {
	for _, candidate := range ValidReasoningEfforts {
		if value == candidate {
			return true
		}
	}
	return false
}
