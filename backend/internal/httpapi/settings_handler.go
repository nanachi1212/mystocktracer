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
	patch, err := readSettingsPatch(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	keys := settingsKeysFrom(patch)
	if keys.changed() && s.agentRuntime == nil {
		writeError(w, http.StatusServiceUnavailable, "AI 設定服務不可用")
		return
	}

	values, err := s.settingsStore.Update(func(values *appsettings.Values) error {
		mergeSettingsPatch(values, patch, keys.profile)
		return nil
	})
	if err != nil {
		writeInternalError(w, "save_settings", "無法儲存設定", err)
		return
	}
	if err := s.saveAgentConfiguration(values, keys); err != nil {
		writeInternalError(w, "sync_agent_settings", "無法同步模型設定", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": s.buildSettingsView(values)})
}

func readSettingsPatch(w http.ResponseWriter, r *http.Request) (settingsUpdateRequest, error) {
	var patch settingsUpdateRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&patch); err != nil {
		return patch, fmt.Errorf("無效的設定內容: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return patch, fmt.Errorf("設定內容只能包含一個 JSON 物件")
	}
	return patch, validateSettingsUpdate(patch)
}

type settingsKeyPatch struct {
	active  *string
	profile map[string]*string
}

func (patch settingsKeyPatch) changed() bool { return patch.active != nil || len(patch.profile) > 0 }

func settingsKeysFrom(request settingsUpdateRequest) settingsKeyPatch {
	keys := settingsKeyPatch{profile: make(map[string]*string)}
	if request.LLMProfiles != nil {
		for _, profile := range *request.LLMProfiles {
			name := strings.TrimSpace(profile.ID)
			switch {
			case profile.ClearAPIKey:
				empty := ""
				keys.profile[name] = &empty
			case profile.APIKey != nil && strings.TrimSpace(*profile.APIKey) != "":
				key := strings.TrimSpace(*profile.APIKey)
				keys.profile[name] = &key
			}
		}
	}
	if request.LLM.APIKey != nil && strings.TrimSpace(*request.LLM.APIKey) != "" {
		key := strings.TrimSpace(*request.LLM.APIKey)
		keys.active = &key
	}
	for _, field := range request.ClearSecrets {
		if field == "llm_api_key" {
			empty := ""
			keys.active = &empty
		}
	}
	return keys
}

func mergeSettingsPatch(values *appsettings.Values, patch settingsUpdateRequest, keys map[string]*string) {
	if patch.LLMProfiles != nil {
		configured := make(map[string]bool, len(values.LLMProfiles))
		for _, profile := range values.LLMProfiles {
			configured[profile.ID] = profile.APIKeyConfigured
		}
		replacement := make([]appsettings.LLMProfile, 0, len(*patch.LLMProfiles))
		for _, input := range *patch.LLMProfiles {
			name := strings.TrimSpace(input.ID)
			hasKey := configured[name]
			if key, supplied := keys[name]; supplied {
				hasKey = strings.TrimSpace(*key) != ""
			}
			replacement = append(replacement, appsettings.LLMProfile{
				ID: name, Name: strings.TrimSpace(input.Name), Provider: strings.TrimSpace(input.Provider),
				BaseURL: strings.TrimSpace(input.BaseURL), Model: strings.TrimSpace(input.Model),
				APIMode: normalizedAPIMode(input.APIMode, input.Provider), APIKeyConfigured: hasKey,
			})
		}
		values.LLMProfiles = replacement
	}
	if patch.ActiveLLMProfileID != nil {
		values.ActiveLLMProfileID = strings.TrimSpace(*patch.ActiveLLMProfileID)
	}
	for _, profile := range values.LLMProfiles {
		if profile.ID != values.ActiveLLMProfileID {
			continue
		}
		values.LLM.Provider = profile.Provider
		values.LLM.BaseURL = profile.BaseURL
		values.LLM.Model = profile.Model
		values.LLM.APIMode = profile.APIMode
		break
	}
	copyOptionalString(&values.LLM.Provider, patch.LLM.Provider)
	copyOptionalString(&values.LLM.BaseURL, patch.LLM.BaseURL)
	copyOptionalString(&values.LLM.Model, patch.LLM.Model)
	copyOptionalString(&values.LLM.APIMode, patch.LLM.APIMode)
	if patch.LLM.ResponseTimeoutSeconds != nil {
		values.LLM.ResponseTimeoutSeconds = *patch.LLM.ResponseTimeoutSeconds
	}
	values.LLM.ResponseTimeoutSeconds = appsettings.NormalizeLLMResponseTimeoutSeconds(values.LLM.ResponseTimeoutSeconds)
	values.LLM.APIKey = ""
	if patch.LLMProfiles == nil && directModelFieldsChanged(patch) {
		for index := range values.LLMProfiles {
			if values.LLMProfiles[index].ID == values.ActiveLLMProfileID {
				values.LLMProfiles[index].Provider = values.LLM.Provider
				values.LLMProfiles[index].BaseURL = values.LLM.BaseURL
				values.LLMProfiles[index].Model = values.LLM.Model
				values.LLMProfiles[index].APIMode = values.LLM.APIMode
				break
			}
		}
	}
	if patch.TaiwanAlerts.CorporateEventsEnabled != nil {
		values.TaiwanAlerts.CorporateEventsEnabled = *patch.TaiwanAlerts.CorporateEventsEnabled
	}
}

func copyOptionalString(target, from *string) {
	if from != nil {
		*target = strings.TrimSpace(*from)
	}
}

func directModelFieldsChanged(patch settingsUpdateRequest) bool {
	return patch.LLM.Provider != nil || patch.LLM.BaseURL != nil || patch.LLM.Model != nil || patch.LLM.APIMode != nil
}

func (s *Server) saveAgentConfiguration(values appsettings.Values, keys settingsKeyPatch) error {
	if s.agentRuntime == nil {
		return nil
	}
	profiles, supportsProfiles := s.agentRuntime.(agent.ProfileConfigurator)
	if !supportsProfiles {
		return s.agentRuntime.SyncLLM(values.LLM, keys.active)
	}
	for name, key := range keys.profile {
		if name != values.ActiveLLMProfileID {
			if err := profiles.StoreLLMProfileKey(name, key); err != nil {
				return err
			}
		}
	}
	active := keys.profile[values.ActiveLLMProfileID]
	if active == nil {
		active = keys.active
	}
	return profiles.SyncLLMProfile(values.LLM, values.ActiveLLMProfileID, active)
}
