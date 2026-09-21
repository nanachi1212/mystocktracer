package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/nanachi1212/mystocktracer/backend/internal/hermes"
)

func (s *Server) settingsAgentGateway() (hermes.SettingsGateway, bool) {
	gateway, ok := s.agentRuntime.(hermes.SettingsGateway)
	return gateway, ok
}

func (s *Server) settingsAgentGet(w http.ResponseWriter, _ *http.Request) {
	gateway, ok := s.settingsAgentGateway()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "Hermes Skill/MCP 設定服務不可用")
		return
	}
	settings, err := gateway.AgentSettings()
	if err != nil {
		writeInternalError(w, "read_agent_settings", "無法讀取 Hermes Skill/MCP 設定", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": buildAgentSettingsView(settings)})
}

func (s *Server) settingsAgentUpdate(w http.ResponseWriter, r *http.Request) {
	gateway, ok := s.settingsAgentGateway()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "Hermes Skill/MCP 設定服務不可用")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var update agentSettingsUpdateRequest
	if err := decoder.Decode(&update); err != nil {
		writeError(w, http.StatusBadRequest, "Agent 設定格式無效")
		return
	}
	if err := ensureJSONEOF(decoder); err != nil {
		writeError(w, http.StatusBadRequest, "Agent 設定只能包含一個 JSON 物件")
		return
	}
	if err := validateAgentSettingsUpdate(update); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	current, err := gateway.AgentSettings()
	if err != nil {
		writeInternalError(w, "read_agent_settings_for_update", "無法讀取現有 Hermes 設定", err)
		return
	}
	if err := gateway.SyncAgentSettings(mergeAgentSettings(current, update)); err != nil {
		writeInternalError(w, "save_agent_settings", "無法儲存 Hermes Skill/MCP 設定", err)
		return
	}
	updated, err := gateway.AgentSettings()
	if err != nil {
		writeInternalError(w, "reload_agent_settings", "設定已儲存，但無法重新讀取", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": buildAgentSettingsView(updated)})
}
