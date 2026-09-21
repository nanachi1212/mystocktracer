package httpapi

import (
	"strings"

	"github.com/nanachi1212/mystocktracer/backend/internal/hermes"
)

const legacyMainlandSkillName = "a-stock-short-term-masters"

func buildAgentSettingsView(settings hermes.AgentSettings) agentSettingsView {
	view := agentSettingsView{ReasoningEffort: settings.ReasoningEffort, Skills: taiwanFirstSkills(settings.Skills), MCPServers: make([]mcpServerView, 0, len(settings.MCPServers))}
	for _, server := range settings.MCPServers {
		item := mcpServerView{Name: server.Name, Enabled: server.Enabled, Transport: server.Transport, Command: server.Command, Args: server.Args, URL: server.URL, Timeout: server.Timeout, ConnectTimeout: server.ConnectTimeout, SupportsParallelToolCall: server.SupportsParallelToolCall}
		item.Env = protectedMapView(server.Env)
		item.Headers = protectedMapView(server.Headers)
		view.MCPServers = append(view.MCPServers, item)
	}
	return view
}

func protectedMapView(values map[string]string) map[string]secretSettingStatus {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]secretSettingStatus, len(values))
	for key, value := range values {
		result[key] = secretStatus(value)
	}
	return result
}

func taiwanFirstSkills(skills []hermes.SkillInfo) []hermes.SkillInfo {
	result := make([]hermes.SkillInfo, 0, len(skills))
	for _, skill := range skills {
		if skill.Name != legacyMainlandSkillName {
			result = append(result, skill)
		}
	}
	return result
}

func mergeAgentSettings(current hermes.AgentSettings, update agentSettingsUpdateRequest) hermes.AgentSettings {
	if update.ReasoningEffort != nil {
		current.ReasoningEffort = strings.ToLower(strings.TrimSpace(*update.ReasoningEffort))
	}
	if update.Skills != nil {
		requested := make(map[string]bool, len(*update.Skills))
		for _, skill := range *update.Skills {
			requested[strings.TrimSpace(skill.Name)] = skill.Enabled
		}
		for index := range current.Skills {
			if enabled, found := requested[current.Skills[index].Name]; found {
				current.Skills[index].Enabled = enabled
			}
		}
	}
	if update.MCPServers == nil {
		return current
	}
	existing := make(map[string]hermes.MCPServerInfo, len(current.MCPServers))
	for _, server := range current.MCPServers {
		existing[server.Name] = server
	}
	current.MCPServers = make([]hermes.MCPServerInfo, 0, len(*update.MCPServers))
	for _, input := range *update.MCPServers {
		name := strings.TrimSpace(input.Name)
		lookup := strings.TrimSpace(input.OriginalName)
		if lookup == "" {
			lookup = name
		}
		server := existing[lookup]
		server.Name, server.Enabled, server.Transport = name, input.Enabled, strings.TrimSpace(input.Transport)
		server.Command, server.Args, server.URL = strings.TrimSpace(input.Command), append([]string(nil), input.Args...), strings.TrimSpace(input.URL)
		server.Timeout, server.ConnectTimeout, server.SupportsParallelToolCall = input.Timeout, input.ConnectTimeout, input.SupportsParallelToolCall
		server.Env = mergeProtectedMap(server.Env, input.Env, input.ClearEnv)
		server.Headers = mergeProtectedMap(server.Headers, input.Headers, input.ClearHeaders)
		current.MCPServers = append(current.MCPServers, server)
	}
	return current
}

func mergeProtectedMap(existing map[string]string, updates map[string]*string, clear []string) map[string]string {
	result := make(map[string]string, len(existing)+len(updates))
	for key, value := range existing {
		result[key] = value
	}
	for _, key := range clear {
		delete(result, strings.TrimSpace(key))
	}
	for key, value := range updates {
		if value != nil && strings.TrimSpace(*value) != "" {
			result[strings.TrimSpace(key)] = strings.TrimSpace(*value)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
