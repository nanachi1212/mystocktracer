package httpapi

import (
	"strings"

	"github.com/nanachi1212/mystocktracer/backend/internal/agent"
	"github.com/nanachi1212/mystocktracer/backend/internal/appsettings"
)

func (s *Server) buildSettingsView(values appsettings.Values) settingsView {
	view := settingsView{ActiveLLMProfileID: values.ActiveLLMProfileID}
	if s.agentRuntime != nil {
		view.Agent = s.agentRuntime.Status()
	} else {
		view.Agent.Message = "AI 設定服務不可用"
	}
	view.Hermes = view.Agent // Saved clients still read the historical response key.
	if !values.UpdatedAt.IsZero() {
		stamp := values.UpdatedAt
		view.UpdatedAt = &stamp
	}

	provider := firstNonEmpty(values.LLM.Provider, "openai")
	view.LLM.Provider = provider
	view.LLM.BaseURL = values.LLM.BaseURL
	if values.LLM.Provider == "" && values.LLM.BaseURL == "" {
		view.LLM.BaseURL = "https://api.openai.com/v1"
	}
	view.LLM.Model = values.LLM.Model
	view.LLM.APIMode = normalizedAPIMode(values.LLM.APIMode, provider)
	view.LLM.ResponseTimeoutSeconds = appsettings.NormalizeLLMResponseTimeoutSeconds(values.LLM.ResponseTimeoutSeconds)
	view.LLM.APIKey = secretSettingStatus{Configured: view.Agent.APIKeyConfigured}
	view.TaiwanAlerts.CorporateEventsEnabled = values.TaiwanAlerts.CorporateEventsEnabled

	profileRuntime, _ := s.agentRuntime.(agent.ProfileConfigurator)
	view.LLMProfiles = make([]llmProfileView, 0, len(values.LLMProfiles))
	for _, saved := range values.LLMProfiles {
		view.LLMProfiles = append(view.LLMProfiles, llmProfileView{
			ID: saved.ID, Name: saved.Name, Provider: saved.Provider, BaseURL: saved.BaseURL,
			Model: saved.Model, APIMode: normalizedAPIMode(saved.APIMode, saved.Provider),
			APIKey: secretSettingStatus{Configured: profileKeyPresent(saved, values.ActiveLLMProfileID, view.Agent, profileRuntime)},
		})
	}
	return view
}

func profileKeyPresent(saved appsettings.LLMProfile, activeID string, status agent.Status, runtime agent.ProfileConfigurator) bool {
	if saved.ID == activeID {
		return status.APIKeyConfigured
	}
	if runtime != nil {
		if key, err := runtime.ModelAPIKeyForProfile(saved.ID); err == nil {
			return strings.TrimSpace(key) != ""
		}
	}
	return saved.APIKeyConfigured
}
