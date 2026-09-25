package appsettings

import "strings"

const (
	retiredAnthropicHaikuModel = "claude-3-5-haiku-latest"
	currentAnthropicHaikuModel = "claude-haiku-4-5"
)

// upgradeRetiredModel replaces the retired Anthropic default that earlier versions saved.
func upgradeRetiredModel(model string) string {
	if strings.TrimSpace(model) == retiredAnthropicHaikuModel {
		return currentAnthropicHaikuModel
	}
	return model
}

func compatibleValues(values Values) Values {
	values.LLM.Model = upgradeRetiredModel(values.LLM.Model)
	for i := range values.LLMProfiles {
		values.LLMProfiles[i].Model = upgradeRetiredModel(values.LLMProfiles[i].Model)
	}
	if len(values.LLMProfiles) == 0 {
		values.LLMProfiles = []LLMProfile{profileFromLLM(values.LLM)}
	}
	if values.ActiveLLMProfileID == "" {
		values.ActiveLLMProfileID = matchingProfileID(values.LLMProfiles, values.LLM)
	}
	if active, ok := profileByID(values.LLMProfiles, values.ActiveLLMProfileID); ok {
		secret := values.LLM.APIKey
		timeout := NormalizeLLMResponseTimeoutSeconds(values.LLM.ResponseTimeoutSeconds)
		values.LLM = LLM{
			Provider: active.Provider, BaseURL: active.BaseURL, Model: active.Model,
			APIMode: active.APIMode, APIKey: secret, ResponseTimeoutSeconds: timeout,
		}
	}
	return values
}

func profileFromLLM(value LLM) LLMProfile {
	name := strings.TrimSpace(value.Model)
	if name == "" {
		name = strings.TrimSpace(value.Provider)
	}
	if name == "" {
		name = "預設模型"
	}
	return LLMProfile{
		ID: "llm-default", Name: name, Provider: value.Provider,
		BaseURL: value.BaseURL, Model: value.Model, APIMode: value.APIMode,
	}
}

func matchingProfileID(profiles []LLMProfile, value LLM) string {
	for _, profile := range profiles {
		if profile.Provider == value.Provider && profile.BaseURL == value.BaseURL &&
			profile.Model == value.Model && profile.APIMode == value.APIMode {
			return profile.ID
		}
	}
	if len(profiles) == 0 {
		return ""
	}
	return profiles[len(profiles)-1].ID
}

func profileByID(profiles []LLMProfile, id string) (LLMProfile, bool) {
	for _, profile := range profiles {
		if profile.ID == id {
			return profile, true
		}
	}
	return LLMProfile{}, false
}
