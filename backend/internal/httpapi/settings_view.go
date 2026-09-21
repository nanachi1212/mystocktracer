package httpapi

import (
	"strings"

	"github.com/nanachi1212/mystocktracer/backend/internal/appsettings"
	"github.com/nanachi1212/mystocktracer/backend/internal/hermes"
)

func (s *Server) buildSettingsView(values appsettings.Values) settingsView {
	var view settingsView
	if s.agentRuntime == nil {
		view.Hermes.Message = "Hermes 設定服務不可用"
	} else {
		view.Hermes = s.agentRuntime.Status()
	}
	if !values.UpdatedAt.IsZero() {
		updated := values.UpdatedAt
		view.UpdatedAt = &updated
	}
	view.LLM.Provider = firstNonEmpty(values.LLM.Provider, "openai")
	view.LLM.BaseURL = values.LLM.BaseURL
	if values.LLM.Provider == "" && view.LLM.BaseURL == "" {
		view.LLM.BaseURL = "https://api.openai.com/v1"
	}
	view.LLM.Model = values.LLM.Model
	view.LLM.APIMode = normalizedAPIMode(values.LLM.APIMode, view.LLM.Provider)
	view.LLM.ResponseTimeoutSeconds = appsettings.NormalizeLLMResponseTimeoutSeconds(values.LLM.ResponseTimeoutSeconds)
	view.LLM.APIKey.Configured = view.Hermes.APIKeyConfigured
	view.ActiveLLMProfileID = values.ActiveLLMProfileID
	view.TaiwanAlerts.CorporateEventsEnabled = values.TaiwanAlerts.CorporateEventsEnabled
	view.LLMProfiles = make([]llmProfileView, 0, len(values.LLMProfiles))

	profiles, _ := s.agentRuntime.(hermes.ProfileGateway)
	for _, profile := range values.LLMProfiles {
		configured := profile.APIKeyConfigured
		if profile.ID == values.ActiveLLMProfileID {
			configured = view.Hermes.APIKeyConfigured
		} else if profiles != nil {
			if key, err := profiles.ModelAPIKeyForProfile(profile.ID); err == nil {
				configured = strings.TrimSpace(key) != ""
			}
		}
		view.LLMProfiles = append(view.LLMProfiles, llmProfileView{
			ID: profile.ID, Name: profile.Name, Provider: profile.Provider,
			BaseURL: profile.BaseURL, Model: profile.Model,
			APIMode: normalizedAPIMode(profile.APIMode, profile.Provider),
			APIKey:  secretSettingStatus{Configured: configured},
		})
	}
	return view
}
