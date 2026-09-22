package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/nanachi1212/mystocktracer/backend/internal/agent"
)

type llmModelsRequest struct {
	Provider  string  `json:"provider"`
	BaseURL   string  `json:"base_url"`
	APIKey    *string `json:"api_key"`
	ProfileID string  `json:"profile_id"`
}
type llmModelsResult struct {
	Models    []agent.ModelOption `json:"models"`
	SourceURL string              `json:"source_url"`
}

func (s *Server) settingsLLMModels(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input llmModelsRequest
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid model list request")
		return
	}
	if err := ensureJSONEOF(decoder); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	saved := s.settingsStore.Snapshot().LLM
	provider := strings.ToLower(firstNonEmpty(strings.TrimSpace(input.Provider), strings.TrimSpace(saved.Provider), "openai"))
	baseURL := firstNonEmpty(strings.TrimSpace(input.BaseURL), strings.TrimSpace(saved.BaseURL))
	apiKey := ""
	if input.APIKey != nil {
		apiKey = strings.TrimSpace(*input.APIKey)
	} else if s.agentRuntime != nil {
		var err error
		if profiles, ok := s.agentRuntime.(agent.ProfileConfigurator); ok && strings.TrimSpace(input.ProfileID) != "" {
			apiKey, err = profiles.ModelAPIKeyForProfile(strings.TrimSpace(input.ProfileID))
		} else {
			apiKey, err = s.agentRuntime.ModelAPIKey()
		}
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "讀取已保存模型密鑰失敗")
			return
		}
	}
	if apiKey == "" && provider != "custom" {
		writeError(w, http.StatusPreconditionFailed, "請先輸入或保存模型 API Key")
		return
	}
	if _, err := agent.ModelsURL(provider, baseURL); err != nil {
		writeError(w, http.StatusBadRequest, "模型服務 URL 無效")
		return
	}
	result, err := agent.DiscoverModels(r.Context(), agent.ModelDiscoveryRequest{Provider: provider, BaseURL: baseURL, APIKey: apiKey})
	if err != nil {
		writeUpstreamError(w, "model_discovery", "模型服務未回傳可用的模型列表", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": llmModelsResult{Models: result.Models, SourceURL: result.SourceURL}})
}
