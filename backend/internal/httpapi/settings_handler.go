package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/nanachi1212/mystocktracer/backend/internal/agent"
	"github.com/nanachi1212/mystocktracer/backend/internal/appsettings"
)

func (s *Server) settingsGet(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"data": s.buildSettingsView(s.settingsStore.Snapshot())})
}

func (s *Server) settingsUpdate(w http.ResponseWriter, r *http.Request) {
	request, err := decodeSettingsUpdate(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	secrets := collectSecretUpdates(request)
	if secrets.hasUpdates() && s.agentRuntime == nil {
		writeError(w, http.StatusServiceUnavailable, "AI 設定服務不可用")
		return
	}
	values, err := s.settingsStore.Update(func(values *appsettings.Values) error {
		applySettingsUpdate(values, request, secrets.profile)
		return nil
	})
	if err != nil {
		writeInternalError(w, "save_settings", "無法儲存設定", err)
		return
	}
	if err := s.syncAgentSettings(values, secrets); err != nil {
		writeInternalError(w, "sync_agent_settings", "無法同步模型設定", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": s.buildSettingsView(values)})
}

func decodeSettingsUpdate(w http.ResponseWriter, r *http.Request) (settingsUpdateRequest, error) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request settingsUpdateRequest
	if err := decoder.Decode(&request); err != nil {
		return request, fmt.Errorf("無效的設定內容: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return request, fmt.Errorf("設定內容只能包含一個 JSON 物件")
	}
	return request, validateSettingsUpdate(request)
}

type settingsSecretUpdates struct {
	active  *string
	profile map[string]*string
}

func (u settingsSecretUpdates) hasUpdates() bool { return u.active != nil || len(u.profile) != 0 }

func collectSecretUpdates(request settingsUpdateRequest) settingsSecretUpdates {
	updates := settingsSecretUpdates{profile: map[string]*string{}}
	if request.LLMProfiles != nil {
		for _, profile := range *request.LLMProfiles {
			id := strings.TrimSpace(profile.ID)
			if profile.ClearAPIKey {
				empty := ""
				updates.profile[id] = &empty
			} else if profile.APIKey != nil && strings.TrimSpace(*profile.APIKey) != "" {
				value := strings.TrimSpace(*profile.APIKey)
				updates.profile[id] = &value
			}
		}
	}
	if request.LLM.APIKey != nil && strings.TrimSpace(*request.LLM.APIKey) != "" {
		value := strings.TrimSpace(*request.LLM.APIKey)
		updates.active = &value
	}
	for _, key := range request.ClearSecrets {
		if key == "llm_api_key" {
			empty := ""
			updates.active = &empty
		}
	}
	return updates
}

func applySettingsUpdate(values *appsettings.Values, request settingsUpdateRequest, keyUpdates map[string]*string) {
	if request.LLMProfiles != nil {
		knownSecretState := make(map[string]bool, len(values.LLMProfiles))
		for _, profile := range values.LLMProfiles {
			knownSecretState[profile.ID] = profile.APIKeyConfigured
		}
		profiles := make([]appsettings.LLMProfile, 0, len(*request.LLMProfiles))
		for _, input := range *request.LLMProfiles {
			id := strings.TrimSpace(input.ID)
			configured := knownSecretState[id]
			if update, ok := keyUpdates[id]; ok {
				configured = strings.TrimSpace(*update) != ""
			}
			profiles = append(profiles, appsettings.LLMProfile{
				ID: id, Name: strings.TrimSpace(input.Name), Provider: strings.TrimSpace(input.Provider),
				BaseURL: strings.TrimSpace(input.BaseURL), Model: strings.TrimSpace(input.Model),
				APIMode: normalizedAPIMode(input.APIMode, input.Provider), APIKeyConfigured: configured,
			})
		}
		values.LLMProfiles = profiles
	}
	if request.ActiveLLMProfileID != nil {
		values.ActiveLLMProfileID = strings.TrimSpace(*request.ActiveLLMProfileID)
	}
	if active, ok := findProfile(values.LLMProfiles, values.ActiveLLMProfileID); ok {
		values.LLM = appsettings.LLM{
			Provider: active.Provider, BaseURL: active.BaseURL, Model: active.Model,
			APIMode: active.APIMode, ResponseTimeoutSeconds: appsettings.NormalizeLLMResponseTimeoutSeconds(values.LLM.ResponseTimeoutSeconds),
		}
	}
	assignOptional(&values.LLM.Provider, request.LLM.Provider)
	assignOptional(&values.LLM.BaseURL, request.LLM.BaseURL)
	assignOptional(&values.LLM.Model, request.LLM.Model)
	assignOptional(&values.LLM.APIMode, request.LLM.APIMode)
	if request.LLM.ResponseTimeoutSeconds != nil {
		values.LLM.ResponseTimeoutSeconds = *request.LLM.ResponseTimeoutSeconds
	}
	values.LLM.ResponseTimeoutSeconds = appsettings.NormalizeLLMResponseTimeoutSeconds(values.LLM.ResponseTimeoutSeconds)
	values.LLM.APIKey = ""
	if request.LLMProfiles == nil && hasDirectLLMUpdate(request) {
		updateActiveProfile(values)
	}
	if request.TaiwanAlerts.CorporateEventsEnabled != nil {
		values.TaiwanAlerts.CorporateEventsEnabled = *request.TaiwanAlerts.CorporateEventsEnabled
	}
}

func (s *Server) syncAgentSettings(values appsettings.Values, updates settingsSecretUpdates) error {
	if s.agentRuntime == nil {
		return nil
	}
	if profiles, ok := s.agentRuntime.(agent.ProfileConfigurator); ok {
		for id, update := range updates.profile {
			if id != values.ActiveLLMProfileID {
				if err := profiles.StoreLLMProfileKey(id, update); err != nil {
					return err
				}
			}
		}
		active := updates.profile[values.ActiveLLMProfileID]
		if active == nil {
			active = updates.active
		}
		return profiles.SyncLLMProfile(values.LLM, values.ActiveLLMProfileID, active)
	}
	return s.agentRuntime.SyncLLM(values.LLM, updates.active)
}

func findProfile(profiles []appsettings.LLMProfile, id string) (appsettings.LLMProfile, bool) {
	for _, profile := range profiles {
		if profile.ID == id {
			return profile, true
		}
	}
	return appsettings.LLMProfile{}, false
}

func assignOptional(destination *string, source *string) {
	if source != nil {
		*destination = strings.TrimSpace(*source)
	}
}

func hasDirectLLMUpdate(request settingsUpdateRequest) bool {
	return request.LLM.Provider != nil || request.LLM.BaseURL != nil || request.LLM.Model != nil || request.LLM.APIMode != nil
}

func updateActiveProfile(values *appsettings.Values) {
	for index := range values.LLMProfiles {
		if values.LLMProfiles[index].ID == values.ActiveLLMProfileID {
			values.LLMProfiles[index].Provider = values.LLM.Provider
			values.LLMProfiles[index].BaseURL = values.LLM.BaseURL
			values.LLMProfiles[index].Model = values.LLM.Model
			values.LLMProfiles[index].APIMode = values.LLM.APIMode
			return
		}
	}
}
