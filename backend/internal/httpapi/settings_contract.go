package httpapi

import (
	"strings"
	"time"

	"github.com/nanachi1212/mystocktracer/backend/internal/hermes"
)

type secretSettingStatus struct {
	Configured bool   `json:"configured"`
	Masked     string `json:"masked,omitempty"`
}

type settingsView struct {
	Hermes             hermes.Status    `json:"hermes"`
	ActiveLLMProfileID string           `json:"active_llm_profile_id"`
	LLMProfiles        []llmProfileView `json:"llm_profiles"`
	LLM                struct {
		Provider               string              `json:"provider"`
		BaseURL                string              `json:"base_url"`
		Model                  string              `json:"model"`
		APIMode                string              `json:"api_mode"`
		ResponseTimeoutSeconds int                 `json:"response_timeout_seconds"`
		APIKey                 secretSettingStatus `json:"api_key"`
	} `json:"llm"`
	TaiwanAlerts struct {
		CorporateEventsEnabled bool `json:"corporate_events_enabled"`
	} `json:"taiwan_alerts"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

type llmProfileView struct {
	ID       string              `json:"id"`
	Name     string              `json:"name"`
	Provider string              `json:"provider"`
	BaseURL  string              `json:"base_url"`
	Model    string              `json:"model"`
	APIMode  string              `json:"api_mode"`
	APIKey   secretSettingStatus `json:"api_key"`
}

type llmProfileUpdate struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Provider    string  `json:"provider"`
	BaseURL     string  `json:"base_url"`
	Model       string  `json:"model"`
	APIMode     string  `json:"api_mode"`
	APIKey      *string `json:"api_key"`
	ClearAPIKey bool    `json:"clear_api_key"`
}

type settingsUpdateRequest struct {
	LLMProfiles        *[]llmProfileUpdate `json:"llm_profiles"`
	ActiveLLMProfileID *string             `json:"active_llm_profile_id"`
	LLM                struct {
		Provider               *string `json:"provider"`
		BaseURL                *string `json:"base_url"`
		Model                  *string `json:"model"`
		APIMode                *string `json:"api_mode"`
		ResponseTimeoutSeconds *int    `json:"response_timeout_seconds"`
		APIKey                 *string `json:"api_key"`
	} `json:"llm"`
	ClearSecrets []string `json:"clear_secrets"`
	TaiwanAlerts struct {
		CorporateEventsEnabled *bool `json:"corporate_events_enabled"`
	} `json:"taiwan_alerts"`
}

func normalizedAPIMode(mode, provider string) string {
	mode = strings.TrimSpace(mode)
	if mode == "responses" {
		return "codex_responses"
	}
	if mode != "" {
		return mode
	}
	if strings.TrimSpace(provider) == "anthropic" {
		return "anthropic_messages"
	}
	return "chat_completions"
}

// Compatibility name retained for existing internal tests and callers.
func normalizeAPIMode(mode, provider string) string { return normalizedAPIMode(mode, provider) }

func secretStatus(secret string) secretSettingStatus {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return secretSettingStatus{}
	}
	runes := []rune(secret)
	masked := "••••"
	if len(runes) > 4 {
		masked = "••••••" + string(runes[len(runes)-4:])
	}
	return secretSettingStatus{Configured: true, Masked: masked}
}
