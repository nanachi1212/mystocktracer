package httpapi

import (
	"strings"

	"github.com/nanachi1212/mystocktracer/backend/internal/agent"
)

const legacyMainlandSkillName = "a-stock-short-term-masters"

func buildAgentSettingsView(state agent.Settings) agentSettingsView {
	view := agentSettingsView{
		ReasoningEffort: state.ReasoningEffort,
		Skills:          taiwanFirstSkills(state.Skills),
		MCPServers:      make([]mcpServerView, 0, len(state.MCPServers)),
	}
	for _, server := range state.MCPServers {
		view.MCPServers = append(view.MCPServers, mcpServerView{
			Name: server.Name, Enabled: server.Enabled, Transport: server.Transport,
			Command: server.Command, Args: server.Args, Env: protectedMapView(server.Env),
			URL: server.URL, Headers: protectedMapView(server.Headers),
			Timeout: server.Timeout, ConnectTimeout: server.ConnectTimeout,
			SupportsParallelToolCall: server.SupportsParallelToolCall,
		})
	}
	return view
}

func protectedMapView(entries map[string]string) map[string]secretSettingStatus {
	if len(entries) == 0 {
		return nil
	}
	masked := make(map[string]secretSettingStatus, len(entries))
	for name, secret := range entries {
		masked[name] = secretStatus(secret)
	}
	return masked
}

func taiwanFirstSkills(skills []agent.SkillSetting) []agent.SkillSetting {
	visible := make([]agent.SkillSetting, 0, len(skills))
	for _, skill := range skills {
		if skill.Name == legacyMainlandSkillName {
			continue
		}
		visible = append(visible, skill)
	}
	return visible
}

func mergeAgentSettings(current agent.Settings, patch agentSettingsUpdateRequest) agent.Settings {
	if patch.ReasoningEffort != nil {
		current.ReasoningEffort = strings.ToLower(strings.TrimSpace(*patch.ReasoningEffort))
	}
	if patch.Skills != nil {
		requested := make(map[string]bool, len(*patch.Skills))
		for _, change := range *patch.Skills {
			requested[strings.TrimSpace(change.Name)] = change.Enabled
		}
		for index := range current.Skills {
			if enabled, exists := requested[current.Skills[index].Name]; exists {
				current.Skills[index].Enabled = enabled
			}
		}
	}
	if patch.MCPServers != nil {
		current.MCPServers = replacementMCPServers(current.MCPServers, *patch.MCPServers)
	}
	return current
}

func replacementMCPServers(previous []agent.MCPServerSetting, changes []mcpServerUpdate) []agent.MCPServerSetting {
	byName := make(map[string]agent.MCPServerSetting, len(previous))
	for _, server := range previous {
		byName[server.Name] = server
	}
	result := make([]agent.MCPServerSetting, 0, len(changes))
	for _, change := range changes {
		name := strings.TrimSpace(change.Name)
		oldName := strings.TrimSpace(change.OriginalName)
		if oldName == "" {
			oldName = name
		}
		server := byName[oldName]
		server.Name = name
		server.Enabled = change.Enabled
		server.Transport = strings.TrimSpace(change.Transport)
		server.Command = strings.TrimSpace(change.Command)
		server.Args = append([]string(nil), change.Args...)
		server.URL = strings.TrimSpace(change.URL)
		server.Timeout = change.Timeout
		server.ConnectTimeout = change.ConnectTimeout
		server.SupportsParallelToolCall = change.SupportsParallelToolCall
		server.Env = applySecretMapChanges(server.Env, change.Env, change.ClearEnv)
		server.Headers = applySecretMapChanges(server.Headers, change.Headers, change.ClearHeaders)
		result = append(result, server)
	}
	return result
}

func applySecretMapChanges(previous map[string]string, changes map[string]*string, removals []string) map[string]string {
	updated := make(map[string]string, len(previous)+len(changes))
	for key, value := range previous {
		updated[key] = value
	}
	for _, key := range removals {
		delete(updated, strings.TrimSpace(key))
	}
	for name, supplied := range changes {
		if supplied == nil {
			continue
		}
		if value := strings.TrimSpace(*supplied); value != "" {
			updated[strings.TrimSpace(name)] = value
		}
	}
	if len(updated) == 0 {
		return nil
	}
	return updated
}
