package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/nanachi1212/mystocktracer/backend/internal/agent"
)

func (s *Server) agentSettingsService(w http.ResponseWriter) agent.CapabilityConfigurator {
	service, available := s.agentRuntime.(agent.CapabilityConfigurator)
	if !available {
		writeError(w, http.StatusServiceUnavailable, "AI Skill/MCP 設定服務不可用")
		return nil
	}
	return service
}

func (s *Server) settingsAgentGet(w http.ResponseWriter, _ *http.Request) {
	service := s.agentSettingsService(w)
	if service == nil {
		return
	}
	state, err := service.AgentSettings()
	if err != nil {
		writeInternalError(w, "read_agent_settings", "無法讀取 AI Skill/MCP 設定", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": buildAgentSettingsView(state)})
}

func decodeAgentPatch(w http.ResponseWriter, r *http.Request) (agentSettingsUpdateRequest, string, error) {
	var patch agentSettingsUpdateRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&patch); err != nil {
		return patch, "Agent 設定格式無效", err
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return patch, "Agent 設定只能包含一個 JSON 物件", err
	}
	if err := validateAgentSettingsUpdate(patch); err != nil {
		return patch, err.Error(), err
	}
	return patch, "", nil
}

func (s *Server) settingsAgentUpdate(w http.ResponseWriter, r *http.Request) {
	service := s.agentSettingsService(w)
	if service == nil {
		return
	}
	patch, message, err := decodeAgentPatch(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, message)
		return
	}
	before, err := service.AgentSettings()
	if err != nil {
		writeInternalError(w, "read_agent_settings_for_update", "無法讀取現有 AI 設定", err)
		return
	}
	if err := service.SyncAgentSettings(mergeAgentSettings(before, patch)); err != nil {
		writeInternalError(w, "save_agent_settings", "無法儲存 AI Skill/MCP 設定", err)
		return
	}
	after, err := service.AgentSettings()
	if err != nil {
		writeInternalError(w, "reload_agent_settings", "設定已儲存，但無法重新讀取", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": buildAgentSettingsView(after)})
}
