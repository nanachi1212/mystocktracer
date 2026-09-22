package httpapi

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/nanachi1212/mystocktracer/backend/internal/agent"
)

var mcpNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
var envNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func validateAgentSettingsUpdate(update agentSettingsUpdateRequest) error {
	if update.ReasoningEffort != nil && !agent.IsValidReasoningEffort(strings.ToLower(strings.TrimSpace(*update.ReasoningEffort))) {
		return fmt.Errorf("無效的思考等級: %s", *update.ReasoningEffort)
	}
	if update.Skills != nil && len(*update.Skills) > 500 || update.MCPServers != nil && len(*update.MCPServers) > 100 {
		return fmt.Errorf("Skill 或 MCP Server 數量過多")
	}
	seenSkills := map[string]struct{}{}
	if update.Skills != nil {
		for _, skill := range *update.Skills {
			name := strings.TrimSpace(skill.Name)
			if name == "" || len(name) > 160 {
				return fmt.Errorf("Skill 名稱無效")
			}
			if _, duplicate := seenSkills[name]; duplicate {
				return fmt.Errorf("Skill 名稱重複")
			}
			seenSkills[name] = struct{}{}
		}
	}
	if update.MCPServers == nil {
		return nil
	}
	seenServers := map[string]struct{}{}
	for _, server := range *update.MCPServers {
		if err := validateMCPServer(server, seenServers); err != nil {
			return err
		}
	}
	return nil
}

func validateMCPServer(server mcpServerUpdate, seen map[string]struct{}) error {
	name := strings.TrimSpace(server.Name)
	if !mcpNamePattern.MatchString(name) {
		return fmt.Errorf("MCP Server 名稱必須為 1-64 位字母、數字、點、底線或短橫線")
	}
	if _, duplicate := seen[name]; duplicate {
		return fmt.Errorf("MCP Server 名稱不能重複")
	}
	seen[name] = struct{}{}
	if old := strings.TrimSpace(server.OriginalName); old != "" && !mcpNamePattern.MatchString(old) {
		return fmt.Errorf("MCP Server %s 的原名稱無效", name)
	}
	transport := strings.TrimSpace(server.Transport)
	if transport != "stdio" && transport != "http" && transport != "sse" {
		return fmt.Errorf("MCP Server %s 的傳輸方式無效", name)
	}
	if transport == "stdio" {
		if err := validateCommand(name, server.Command, server.Args); err != nil {
			return err
		}
	} else {
		parsed, err := url.Parse(strings.TrimSpace(server.URL))
		if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || len(server.URL) > 2048 {
			return fmt.Errorf("MCP Server %s 需要有效且不含使用者資訊的 HTTP/HTTPS URL", name)
		}
	}
	if server.Timeout < 0 || server.Timeout > 3600 || server.ConnectTimeout < 0 || server.ConnectTimeout > 600 {
		return fmt.Errorf("MCP Server %s 的逾時時間無效", name)
	}
	if err := validateProtectedMap(name, "環境變數", server.Env, server.ClearEnv, true); err != nil {
		return err
	}
	return validateProtectedMap(name, "請求標頭", server.Headers, server.ClearHeaders, false)
}

func validateCommand(name, command string, args []string) error {
	command = strings.TrimSpace(command)
	if command == "" || len(command) > 1024 || strings.ContainsAny(command, "\r\n") {
		return fmt.Errorf("MCP Server %s 需要有效的啟動命令", name)
	}
	if len(args) > 100 {
		return fmt.Errorf("MCP Server %s 的參數過多", name)
	}
	for _, arg := range args {
		if len(arg) > 4096 || strings.ContainsAny(arg, "\r\n") {
			return fmt.Errorf("MCP Server %s 包含無效參數", name)
		}
	}
	return nil
}

func validateProtectedMap(server, label string, values map[string]*string, clear []string, env bool) error {
	if len(values)+len(clear) > 100 {
		return fmt.Errorf("MCP Server %s 的%s過多", server, label)
	}
	seen := map[string]struct{}{}
	validateKey := func(raw string) error {
		key := strings.TrimSpace(raw)
		if key == "" || strings.ContainsAny(key, "\r\n:") || env && !envNamePattern.MatchString(key) {
			return fmt.Errorf("MCP Server %s 包含無效%s", server, label)
		}
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("MCP Server %s 的%s鍵重複", server, label)
		}
		seen[key] = struct{}{}
		return nil
	}
	for key, value := range values {
		if err := validateKey(key); err != nil {
			return err
		}
		if value != nil && (len(*value) > 32<<10 || strings.ContainsAny(*value, "\r\n")) {
			return fmt.Errorf("MCP Server %s 包含無效%s", server, label)
		}
	}
	for _, key := range clear {
		if err := validateKey(key); err != nil {
			return err
		}
	}
	return nil
}
