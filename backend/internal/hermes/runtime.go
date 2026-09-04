package hermes

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"easy-stock/backend/internal/appsettings"
	"gopkg.in/yaml.v3"
)

const (
	providerSlug          = "easy-stock"
	modelAPIKeyEnvName    = "MODEL_API_KEY"
	staleTimeoutEnvName   = "HERMES_API_CALL_STALE_TIMEOUT"
	modelProfileKeyPrefix = "MODEL_API_KEY_PROFILE_"
	hermesErrorTailSize   = 32 << 10
)

var diagnosticSecretPattern = regexp.MustCompile(`(?i)(authorization\s*[:=]\s*(?:bearer\s+)?|(?:api[_ -]?key|token)\s*[:=]\s*)[^\s,;]+`)

// systemPrompt configures the general-purpose Hermes agent (sidebar chat and the settings
// connection probe). Taiwan stock research does NOT use this prompt — GenerateTaiwanResearch
// calls Hermes through PromptIsolated with its own dedicated, evidence-only system prompt
// (see stockanalysis.taiwanResearchSystemPrompt) that never inherits skills or this identity.
const systemPrompt = `你是 easy-stock 的 AI 投研助手。easy-stock 目前以台灣股票（TWSE、TPEx）研究為主要產品方向；個股研究另有獨立、僅限官方證據的專用流程，不使用本系統提示詞。像 Codex 一樣協作：先理解目標，再基於可追蹤的資料與原文證據給出清晰、可執行、可驗證的回答；主動區分事實、推斷、市場預期與待驗證條件，不編造即時資料，不承諾收益。除非使用者明確詢問中國 A 股市場，否則不得假設使用者所在市場、貨幣或交易規則為 A 股。本機另安裝歷史遺留的 a-stock-short-term-masters 技能，僅在使用者主動詢問 A 股游資、心法、情緒週期、龍頭戰法、首板、打板、倉位或預期差等歷史經驗資料時使用，並說明其屬於歷史經驗與二次整理材料，不代表本產品目前市場定位。預設使用繁體中文，除非使用者要求其他語言。`

type Status struct {
	Available        bool   `json:"available"`
	Configured       bool   `json:"configured"`
	APIKeyConfigured bool   `json:"api_key_configured"`
	Version          string `json:"version,omitempty"`
	Message          string `json:"message,omitempty"`
}

type PromptResult struct {
	Content         string
	SessionID       string
	StoredSessionID string
}

type Process interface {
	Input() io.WriteCloser
	Output() io.ReadCloser
	Errors() io.ReadCloser
	Wait() error
	Stop() error
}

type Prompter interface {
	Prompt(ctx context.Context, prompt string) (PromptResult, error)
}

// IsolatedPrompter runs one prompt with an explicit system context in a
// temporary Hermes home that does not inherit memory, skills, MCP servers, or
// browser state. Existing Prompt callers keep the normal A-share profile.
type IsolatedPrompter interface {
	PromptIsolated(ctx context.Context, systemContext, prompt string) (PromptResult, error)
}

type BrowserStatePrompter interface {
	PromptWithBrowserState(ctx context.Context, prompt, storageStatePath string) (PromptResult, error)
}

type Gateway interface {
	Prompter
	Status() Status
	ModelAPIKey() (string, error)
	SyncLLM(cfg appsettings.LLM, apiKeyUpdate *string) error
	Start(ctx context.Context) (Process, error)
}

type SettingsGateway interface {
	Gateway
	AgentSettings() (AgentSettings, error)
	SyncAgentSettings(AgentSettings) error
}

// ProfileGateway is implemented by runtimes that can keep one secret per
// named model profile without exposing it to the application settings JSON.
// It is optional so older test and embedded gateways remain compatible.
type ProfileGateway interface {
	SyncLLMProfile(cfg appsettings.LLM, profileID string, apiKeyUpdate *string) error
	StoreLLMProfileKey(profileID string, apiKeyUpdate *string) error
	ModelAPIKeyForProfile(profileID string) (string, error)
}

type AgentSettings struct {
	ReasoningEffort string          `json:"reasoning_effort" yaml:"-"`
	Skills          []SkillInfo     `json:"skills" yaml:"-"`
	MCPServers      []MCPServerInfo `json:"mcp_servers" yaml:"-"`
}

// ValidReasoningEfforts mirrors the levels supported by Hermes. "none" turns
// reasoning off; the remaining levels trade latency and token usage for depth.
var ValidReasoningEfforts = []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"}

func IsValidReasoningEffort(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return slices.Contains(ValidReasoningEfforts, value)
}

type SkillInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Enabled     bool   `json:"enabled"`
}

type MCPServerInfo struct {
	Name                     string            `json:"name"`
	Enabled                  bool              `json:"enabled"`
	Transport                string            `json:"transport"`
	Command                  string            `json:"command,omitempty"`
	Args                     []string          `json:"args,omitempty"`
	Env                      map[string]string `json:"env,omitempty"`
	URL                      string            `json:"url,omitempty"`
	Headers                  map[string]string `json:"headers,omitempty"`
	Timeout                  int               `json:"timeout,omitempty"`
	ConnectTimeout           int               `json:"connect_timeout,omitempty"`
	SupportsParallelToolCall bool              `json:"supports_parallel_tool_calls,omitempty"`
	raw                      map[string]any
}

type Config struct {
	RuntimeRoot string
	Home        string
	WorkDir     string
	PythonPath  string
}

type Runtime struct {
	runtimeRoot string
	home        string
	workDir     string
	pythonPath  string
	isolated    bool

	mu         sync.RWMutex
	configMu   sync.Mutex
	llm        appsettings.LLM
	configured bool
	hasAPIKey  bool
}

func NewRuntime(cfg Config) *Runtime {
	runtimeRoot := strings.TrimSpace(cfg.RuntimeRoot)
	home := strings.TrimSpace(cfg.Home)
	workDir := strings.TrimSpace(cfg.WorkDir)
	pythonPath := strings.TrimSpace(cfg.PythonPath)
	if pythonPath == "" && runtimeRoot != "" {
		pythonPath = runtimePython(runtimeRoot)
	}
	return &Runtime{
		runtimeRoot: runtimeRoot,
		home:        home,
		workDir:     workDir,
		pythonPath:  pythonPath,
	}
}

func (r *Runtime) Status() Status {
	r.mu.RLock()
	configured := r.configured
	hasAPIKey := r.hasAPIKey
	r.mu.RUnlock()

	status := Status{Configured: configured, APIKeyConfigured: hasAPIKey, Version: r.runtimeVersion()}
	if r.pythonPath == "" {
		status.Message = "未設定 Hermes 執行環境路徑"
		return status
	}
	info, err := os.Stat(r.pythonPath)
	if err != nil || info.IsDir() {
		status.Message = "Hermes 執行環境不可用"
		return status
	}
	status.Available = true
	if !configured {
		status.Message = "請先設定 Hermes 使用的模型"
	}
	return status
}

// ModelAPIKey returns the protected provider key for backend-only operations
// such as discovering models. Callers must never expose the returned value in
// responses or logs.
func (r *Runtime) ModelAPIKey() (string, error) {
	if strings.TrimSpace(r.home) == "" {
		return "", errors.New("Hermes 使用者目錄未設定")
	}
	return readEnvValue(filepath.Join(r.home, ".env"), modelAPIKeyEnvName)
}

// SyncLLM writes the app's model selection into Hermes' own config.yaml and
// writes secrets only into Hermes' .env. A nil apiKeyUpdate retains the
// existing MODEL_API_KEY; a pointer to an empty string clears it.
func (r *Runtime) SyncLLM(cfg appsettings.LLM, apiKeyUpdate *string) error {
	return r.syncLLM(cfg, "active", apiKeyUpdate)
}

// SyncLLMProfile activates a profile and makes its key the Hermes runtime key.
func (r *Runtime) SyncLLMProfile(cfg appsettings.LLM, profileID string, apiKeyUpdate *string) error {
	if strings.TrimSpace(profileID) == "" {
		profileID = "active"
	}
	return r.syncLLM(cfg, profileID, apiKeyUpdate)
}

func (r *Runtime) syncLLM(cfg appsettings.LLM, profileID string, apiKeyUpdate *string) error {
	if strings.TrimSpace(r.home) == "" {
		return errors.New("Hermes 使用者目錄未設定")
	}
	if err := os.MkdirAll(r.home, 0o700); err != nil {
		return fmt.Errorf("建立 Hermes 使用者目錄: %w", err)
	}
	r.configMu.Lock()
	defer r.configMu.Unlock()

	envPath := filepath.Join(r.home, ".env")
	profileKeyName := profileKeyEnvName(profileID)
	existingKey, err := readEnvValue(envPath, profileKeyName)
	if err != nil {
		return err
	}
	if profileID == "active" {
		// Legacy installations only have MODEL_API_KEY. Keep it as the
		// active profile's key until the first profile-aware save.
		if existingKey == "" {
			existingKey, err = readEnvValue(envPath, modelAPIKeyEnvName)
			if err != nil {
				return err
			}
		}
	}
	effectiveKey := existingKey
	if apiKeyUpdate != nil {
		effectiveKey = strings.TrimSpace(*apiKeyUpdate)
		if strings.ContainsAny(effectiveKey, "\r\n") {
			return errors.New("模型 API Key 不能包含換行")
		}
		if err := writeEnvValue(envPath, profileKeyName, effectiveKey); err != nil {
			return err
		}
	} else if _, statErr := os.Stat(envPath); errors.Is(statErr, os.ErrNotExist) {
		if err := writeEnvValue(envPath, profileKeyName, ""); err != nil {
			return err
		}
	}
	// MODEL_API_KEY is always the currently activated profile's key. This is
	// what the Hermes child process reads, while profile-specific variables
	// retain the other connections for later switching.
	if err := writeEnvValue(envPath, modelAPIKeyEnvName, effectiveKey); err != nil {
		return err
	}

	normalized := normalizeLLM(cfg)
	// Hermes resolves stale timeouts by provider ID. The app uses a custom
	// provider profile whose runtime ID can differ from the managed YAML key,
	// so also write the documented environment fallback to cover that path.
	if err := writeEnvValue(envPath, staleTimeoutEnvName, strconv.Itoa(normalized.ResponseTimeoutSeconds)); err != nil {
		return err
	}
	configText, err := r.renderMergedConfig(normalized)
	if err != nil {
		return err
	}
	if err := writeSecureFile(filepath.Join(r.home, "config.yaml"), []byte(configText)); err != nil {
		return fmt.Errorf("寫入 Hermes 模型設定: %w", err)
	}

	configured := normalized.Model != "" && normalized.BaseURL != "" && (normalized.Provider == "custom" || effectiveKey != "")
	r.mu.Lock()
	r.llm = normalized
	r.configured = configured
	r.hasAPIKey = effectiveKey != ""
	r.mu.Unlock()
	return nil
}

// StoreLLMProfileKey updates only a non-active profile's secret.
func (r *Runtime) StoreLLMProfileKey(profileID string, apiKeyUpdate *string) error {
	if strings.TrimSpace(r.home) == "" {
		return errors.New("Hermes 使用者目錄未設定")
	}
	if strings.TrimSpace(profileID) == "" {
		return errors.New("模型設定 ID 不能為空")
	}
	r.configMu.Lock()
	defer r.configMu.Unlock()
	if err := os.MkdirAll(r.home, 0o700); err != nil {
		return fmt.Errorf("建立 Hermes 使用者目錄: %w", err)
	}
	if apiKeyUpdate == nil {
		return nil
	}
	value := strings.TrimSpace(*apiKeyUpdate)
	if strings.ContainsAny(value, "\r\n") {
		return errors.New("模型 API Key 不能包含換行")
	}
	return writeEnvValue(filepath.Join(r.home, ".env"), profileKeyEnvName(profileID), value)
}

func (r *Runtime) ModelAPIKeyForProfile(profileID string) (string, error) {
	if strings.TrimSpace(r.home) == "" {
		return "", errors.New("Hermes 使用者目錄未設定")
	}
	if strings.TrimSpace(profileID) == "" || profileID == "active" {
		return r.ModelAPIKey()
	}
	value, err := readEnvValue(filepath.Join(r.home, ".env"), profileKeyEnvName(profileID))
	if err != nil {
		return "", err
	}
	return value, nil
}

func profileKeyEnvName(profileID string) string {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" || profileID == "active" {
		return modelAPIKeyEnvName
	}
	var b strings.Builder
	for _, r := range profileID {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return modelProfileKeyPrefix + b.String()
}

func (r *Runtime) AgentSettings() (AgentSettings, error) {
	if strings.TrimSpace(r.home) == "" {
		return AgentSettings{}, errors.New("Hermes 使用者目錄未設定")
	}
	config, err := r.readConfigMap()
	if err != nil {
		return AgentSettings{}, err
	}
	disabled := map[string]bool{}
	if skills, ok := stringMap(config["skills"]); ok {
		for _, name := range stringSlice(skills["disabled"]) {
			disabled[name] = true
		}
	}
	reasoningEffort := "medium"
	if agent, ok := stringMap(config["agent"]); ok {
		if value := strings.ToLower(strings.TrimSpace(stringValue(agent["reasoning_effort"]))); IsValidReasoningEffort(value) {
			reasoningEffort = value
		}
	}
	settings := AgentSettings{ReasoningEffort: reasoningEffort, Skills: discoverSkills(filepath.Join(r.home, "skills"))}
	for index := range settings.Skills {
		settings.Skills[index].Enabled = !disabled[settings.Skills[index].Name]
	}
	if servers, ok := stringMap(config["mcp_servers"]); ok {
		for name, value := range servers {
			serverConfig, ok := stringMap(value)
			if !ok {
				continue
			}
			server := MCPServerInfo{
				Name:                     name,
				Enabled:                  boolValue(serverConfig["enabled"], true),
				Command:                  stringValue(serverConfig["command"]),
				Args:                     stringSlice(serverConfig["args"]),
				Env:                      stringStringMap(serverConfig["env"]),
				URL:                      stringValue(serverConfig["url"]),
				Headers:                  stringStringMap(serverConfig["headers"]),
				Timeout:                  intValue(serverConfig["timeout"]),
				ConnectTimeout:           intValue(serverConfig["connect_timeout"]),
				SupportsParallelToolCall: boolValue(serverConfig["supports_parallel_tool_calls"], false),
				raw:                      cloneStringAnyMap(serverConfig),
			}
			server.Transport = strings.ToLower(strings.TrimSpace(stringValue(serverConfig["transport"])))
			if server.Transport == "" {
				if server.URL != "" {
					server.Transport = "http"
				} else {
					server.Transport = "stdio"
				}
			}
			settings.MCPServers = append(settings.MCPServers, server)
		}
	}
	slices.SortFunc(settings.MCPServers, func(a, b MCPServerInfo) int { return strings.Compare(a.Name, b.Name) })
	return settings, nil
}

func (r *Runtime) SyncAgentSettings(settings AgentSettings) error {
	if strings.TrimSpace(r.home) == "" {
		return errors.New("Hermes 使用者目錄未設定")
	}
	r.configMu.Lock()
	defer r.configMu.Unlock()
	config, err := r.readConfigMap()
	if err != nil {
		return err
	}
	reasoningEffort := strings.ToLower(strings.TrimSpace(settings.ReasoningEffort))
	if reasoningEffort == "" {
		reasoningEffort = "medium"
	}
	if !IsValidReasoningEffort(reasoningEffort) {
		return fmt.Errorf("無效的 Hermes 思考等級: %s", settings.ReasoningEffort)
	}
	agent, _ := stringMap(config["agent"])
	if agent == nil {
		agent = map[string]any{}
	}
	agent["reasoning_effort"] = reasoningEffort
	config["agent"] = agent
	skills, _ := stringMap(config["skills"])
	if skills == nil {
		skills = map[string]any{}
	}
	disabled := make([]string, 0)
	knownSkills := map[string]bool{}
	for _, skill := range settings.Skills {
		name := strings.TrimSpace(skill.Name)
		knownSkills[name] = true
		if !skill.Enabled {
			disabled = append(disabled, name)
		}
	}
	for _, name := range stringSlice(skills["disabled"]) {
		if name = strings.TrimSpace(name); name != "" && !knownSkills[name] {
			disabled = append(disabled, name)
		}
	}
	slices.Sort(disabled)
	disabled = slices.Compact(disabled)
	skills["disabled"] = disabled
	if _, ok := skills["creation_nudge_interval"]; !ok {
		skills["creation_nudge_interval"] = 0
	}
	config["skills"] = skills
	servers := map[string]any{}
	for _, server := range settings.MCPServers {
		entry := cloneStringAnyMap(server.raw)
		if entry == nil {
			entry = map[string]any{}
		}
		entry["enabled"] = server.Enabled
		if server.Transport == "stdio" {
			delete(entry, "url")
			delete(entry, "headers")
			delete(entry, "transport")
			entry["command"] = strings.TrimSpace(server.Command)
			if len(server.Args) > 0 {
				entry["args"] = server.Args
			} else {
				delete(entry, "args")
			}
			if len(server.Env) > 0 {
				entry["env"] = server.Env
			} else {
				delete(entry, "env")
			}
		} else {
			delete(entry, "command")
			delete(entry, "args")
			delete(entry, "env")
			entry["url"] = strings.TrimSpace(server.URL)
			if server.Transport == "sse" {
				entry["transport"] = "sse"
			} else {
				delete(entry, "transport")
			}
			if len(server.Headers) > 0 {
				entry["headers"] = server.Headers
			} else {
				delete(entry, "headers")
			}
		}
		if server.Timeout > 0 {
			entry["timeout"] = server.Timeout
		} else {
			delete(entry, "timeout")
		}
		if server.ConnectTimeout > 0 {
			entry["connect_timeout"] = server.ConnectTimeout
		} else {
			delete(entry, "connect_timeout")
		}
		if server.SupportsParallelToolCall {
			entry["supports_parallel_tool_calls"] = true
		} else {
			delete(entry, "supports_parallel_tool_calls")
		}
		servers[strings.TrimSpace(server.Name)] = entry
	}
	config["mcp_servers"] = servers
	return r.writeConfigMap(config)
}

func (r *Runtime) Start(ctx context.Context) (Process, error) {
	return r.start(ctx, "")
}

func (r *Runtime) start(ctx context.Context, browserStatePath string) (Process, error) {
	status := r.Status()
	if !status.Available {
		return nil, errors.New(firstNonEmpty(status.Message, "Hermes 執行環境不可用"))
	}
	if strings.TrimSpace(r.home) == "" {
		return nil, errors.New("Hermes 使用者目錄未設定")
	}
	processEnv, err := r.processEnvironment(browserStatePath)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, r.pythonPath, "-m", "tui_gateway.entry")
	cmd.Dir = firstExistingDirectory(r.workDir, r.home)
	cmd.Env = processEnv
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, fmt.Errorf("啟動 Hermes: %w", err)
	}
	return &commandProcess{cmd: cmd, stdin: stdin, stdout: stdout, stderr: stderr}, nil
}

func (r *Runtime) processEnvironment(browserStatePath string) ([]string, error) {
	modelAPIKey, err := readEnvValue(filepath.Join(r.home, ".env"), modelAPIKeyEnvName)
	if err != nil {
		return nil, err
	}
	baseEnvironment := os.Environ()
	if r.isolated {
		baseEnvironment = unsetEnv(baseEnvironment, "A_STOCK_AGENT_BROWSER_WRAPPER_DIR")
		baseEnvironment = unsetEnv(baseEnvironment, "AGENT_BROWSER_STATE")
		baseEnvironment = unsetEnv(baseEnvironment, "AGENT_BROWSER_PROFILE")
	}
	values := hermesEnvironment(baseEnvironment, r.home, r.workDir, filepath.Dir(r.pythonPath))
	if r.isolated {
		values = unsetEnv(values, "AGENT_BROWSER_STATE")
		values = unsetEnv(values, "AGENT_BROWSER_PROFILE")
	}
	// Hermes' TUI gateway resolves provider key_env values from the process
	// environment. Explicitly mirror the protected Hermes .env value and
	// override any ambient shell value so the application's setting is the
	// single source of truth, including after the key is cleared.
	values = setEnv(values, modelAPIKeyEnvName, modelAPIKey)
	if browserStatePath != "" {
		absolutePath, err := filepath.Abs(browserStatePath)
		if err != nil {
			return nil, fmt.Errorf("解析浏览器登录态路径: %w", err)
		}
		info, err := os.Stat(absolutePath)
		if err != nil || info.IsDir() {
			return nil, errors.New("雪球浏览器登录态不存在，请先在设置中登录")
		}
		values = setEnv(values, "AGENT_BROWSER_STATE", absolutePath)
		// agent-browser rejects storage_state and a persistent profile used at
		// the same time. The Hermes task session is already isolated, so this
		// browser run must use only the explicitly selected storage state.
		values = unsetEnv(values, "AGENT_BROWSER_PROFILE")
	}
	return values, nil
}

func (r *Runtime) Prompt(ctx context.Context, prompt string) (PromptResult, error) {
	return r.prompt(ctx, prompt, "")
}

func (r *Runtime) PromptIsolated(ctx context.Context, systemContext, prompt string) (PromptResult, error) {
	if strings.TrimSpace(systemContext) == "" {
		return PromptResult{}, errors.New("Hermes 隔離系統提示詞不能為空")
	}
	temporaryHome, err := os.MkdirTemp("", "easy-stock-hermes-isolated-*")
	if err != nil {
		return PromptResult{}, err
	}
	defer os.RemoveAll(temporaryHome)
	data, err := r.isolatedConfig(systemContext)
	if err != nil {
		return PromptResult{}, err
	}
	if err := writeSecureFile(filepath.Join(temporaryHome, "config.yaml"), data); err != nil {
		return PromptResult{}, err
	}
	key, err := r.ModelAPIKey()
	if err != nil {
		return PromptResult{}, err
	}
	if err := writeEnvValue(filepath.Join(temporaryHome, ".env"), modelAPIKeyEnvName, key); err != nil {
		return PromptResult{}, err
	}
	isolated := NewRuntime(Config{RuntimeRoot: r.runtimeRoot, Home: temporaryHome, WorkDir: r.workDir, PythonPath: r.pythonPath})
	isolated.isolated = true
	r.mu.RLock()
	isolated.llm, isolated.configured, isolated.hasAPIKey = r.llm, r.configured, r.hasAPIKey
	r.mu.RUnlock()
	return isolated.prompt(ctx, prompt, "")
}

func (r *Runtime) isolatedConfig(systemContext string) ([]byte, error) {
	existing, err := r.readConfigMap()
	if err != nil {
		return nil, err
	}
	config := map[string]any{}
	if model, ok := existing["model"]; ok {
		config["model"] = model
	}
	if providers, ok := existing["providers"]; ok {
		config["providers"] = providers
	}
	reasoningEffort := "medium"
	if existingAgent, ok := stringMap(existing["agent"]); ok {
		if value := strings.TrimSpace(stringValue(existingAgent["reasoning_effort"])); value != "" {
			reasoningEffort = value
		}
	}
	config["agent"] = map[string]any{"reasoning_effort": reasoningEffort, "system_prompt": strings.TrimSpace(systemContext)}
	config["memory"] = map[string]any{"memory_enabled": false, "user_profile_enabled": false, "nudge_interval": 0}
	config["skills"] = map[string]any{"disabled": []string{"*"}, "creation_nudge_interval": 0}
	config["mcp_servers"] = map[string]any{}
	config["curator"] = map[string]any{"enabled": false}
	config["security"] = map[string]any{"allow_lazy_installs": false}
	return yaml.Marshal(config)
}

func (r *Runtime) PromptWithBrowserState(ctx context.Context, prompt, storageStatePath string) (PromptResult, error) {
	if strings.TrimSpace(storageStatePath) == "" {
		return PromptResult{}, errors.New("浏览器登录态路径不能为空")
	}
	return r.prompt(ctx, prompt, strings.TrimSpace(storageStatePath))
}

func (r *Runtime) prompt(ctx context.Context, prompt, browserStatePath string) (PromptResult, error) {
	if strings.TrimSpace(prompt) == "" {
		return PromptResult{}, errors.New("Hermes 提示詞不能為空")
	}
	status := r.Status()
	if !status.Available {
		return PromptResult{}, errors.New(firstNonEmpty(status.Message, "Hermes 執行環境不可用"))
	}
	if !status.Configured {
		return PromptResult{}, errors.New(firstNonEmpty(status.Message, "請先設定 Hermes 使用的模型"))
	}

	process, err := r.start(ctx, browserStatePath)
	if err != nil {
		return PromptResult{}, err
	}
	promptContext, cancelPrompt := context.WithCancel(ctx)
	defer func() {
		cancelPrompt()
		_ = process.Stop()
		_ = process.Wait()
	}()
	diagnostics := newTailBuffer(hermesErrorTailSize)
	diagnosticsDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(diagnostics, process.Errors())
		close(diagnosticsDone)
	}()

	type lineResult struct {
		line []byte
		err  error
	}
	lines := make(chan lineResult, 1)
	go func() {
		scanner := bufio.NewScanner(process.Output())
		scanner.Buffer(make([]byte, 64<<10), 4<<20)
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			select {
			case lines <- lineResult{line: line}:
			case <-promptContext.Done():
				return
			}
		}
		select {
		case lines <- lineResult{err: scanner.Err()}:
		case <-promptContext.Done():
		}
	}()

	writeRPC := func(id, method string, params map[string]any) error {
		data, marshalErr := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
		if marshalErr != nil {
			return marshalErr
		}
		data = append(data, '\n')
		_, writeErr := process.Input().Write(data)
		return writeErr
	}

	created := false
	submitted := false
	result := PromptResult{}
	var streamed strings.Builder
	for {
		select {
		case <-ctx.Done():
			return PromptResult{}, ctx.Err()
		case item := <-lines:
			if item.err != nil {
				waitForDiagnostics(diagnosticsDone)
				return PromptResult{}, r.hermesFailure(fmt.Sprintf("Hermes 對話工作階段結束: %v", item.err), diagnostics.String())
			}
			if len(item.line) == 0 {
				waitForDiagnostics(diagnosticsDone)
				return PromptResult{}, r.hermesFailure("Hermes 對話工作階段意外結束", diagnostics.String())
			}
			var frame rpcFrame
			if err := json.Unmarshal(item.line, &frame); err != nil {
				continue
			}
			if frame.Error != nil {
				return PromptResult{}, r.hermesFailure(fmt.Sprintf("Hermes: %s", frame.Error.Message), diagnostics.String())
			}
			if eventType(frame) == "gateway.ready" && !created {
				created = true
				if err := writeRPC("1", "session.create", map[string]any{"client": "easy-stock", "cwd": r.workDir}); err != nil {
					return PromptResult{}, err
				}
				continue
			}
			if frame.ID == "1" && frame.Result != nil && !submitted {
				result.SessionID = stringValue(frame.Result["session_id"])
				result.StoredSessionID = firstNonEmpty(stringValue(frame.Result["stored_session_id"]), result.SessionID)
				if result.SessionID == "" {
					return PromptResult{}, errors.New("Hermes 未回傳工作階段 ID")
				}
				submitted = true
				if err := writeRPC("2", "prompt.submit", map[string]any{"session_id": result.SessionID, "text": prompt}); err != nil {
					return PromptResult{}, err
				}
				continue
			}
			switch eventType(frame) {
			case "message.delta":
				streamed.WriteString(firstNonEmpty(eventText(frame, "delta"), eventText(frame, "text")))
			case "message.complete":
				result.Content = strings.TrimSpace(firstNonEmpty(eventText(frame, "content"), eventText(frame, "text"), streamed.String()))
				if result.Content == "" {
					return PromptResult{}, errors.New("Hermes 沒有回傳有效內容")
				}
				return result, nil
			case "message.error", "session.error", "run.error":
				return PromptResult{}, r.hermesFailure(firstNonEmpty(eventText(frame, "message"), "Hermes 執行失敗"), diagnostics.String())
			}
		}
	}
}

type tailBuffer struct {
	mu    sync.Mutex
	limit int
	data  []byte
}

func newTailBuffer(limit int) *tailBuffer {
	return &tailBuffer{limit: limit}
}

func (b *tailBuffer) Write(value []byte) (int, error) {
	written := len(value)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.limit <= 0 {
		return written, nil
	}
	if len(value) >= b.limit {
		b.data = append(b.data[:0], value[len(value)-b.limit:]...)
		return written, nil
	}
	b.data = append(b.data, value...)
	if overflow := len(b.data) - b.limit; overflow > 0 {
		copy(b.data, b.data[overflow:])
		b.data = b.data[:b.limit]
	}
	return written, nil
}

func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(append([]byte(nil), b.data...))
}

func waitForDiagnostics(done <-chan struct{}) {
	select {
	case <-done:
	case <-time.After(150 * time.Millisecond):
	}
}

func (r *Runtime) hermesFailure(summary, diagnostic string) error {
	apiKey, _ := r.ModelAPIKey()
	summary = sanitizeHermesDiagnostic(summary, apiKey)
	diagnostic = sanitizeHermesDiagnostic(diagnostic, apiKey)
	if diagnostic == "" {
		return errors.New(summary)
	}
	return fmt.Errorf("%s\n執行環境詳細資訊：%s", summary, diagnostic)
}

func sanitizeHermesDiagnostic(value, apiKey string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\r\n", "\n"))
	if apiKey != "" {
		value = strings.ReplaceAll(value, apiKey, "[REDACTED]")
	}
	return diagnosticSecretPattern.ReplaceAllString(value, "$1[REDACTED]")
}

type commandProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	once   sync.Once
}

func (p *commandProcess) Input() io.WriteCloser { return p.stdin }
func (p *commandProcess) Output() io.ReadCloser { return p.stdout }
func (p *commandProcess) Errors() io.ReadCloser { return p.stderr }
func (p *commandProcess) Wait() error           { return p.cmd.Wait() }
func (p *commandProcess) Stop() error {
	var result error
	p.once.Do(func() {
		_ = p.stdin.Close()
		if p.cmd.Process != nil {
			result = p.cmd.Process.Kill()
		}
	})
	return result
}

type rpcFrame struct {
	ID     string         `json:"id"`
	Method string         `json:"method"`
	Params map[string]any `json:"params"`
	Result map[string]any `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func eventType(frame rpcFrame) string {
	if frame.Method == "event" {
		return stringValue(frame.Params["type"])
	}
	return frame.Method
}

func eventText(frame rpcFrame, key string) string {
	if value := stringValue(frame.Params[key]); value != "" {
		return value
	}
	payload, _ := frame.Params["payload"].(map[string]any)
	return stringValue(payload[key])
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func normalizeLLM(cfg appsettings.LLM) appsettings.LLM {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "" {
		provider = "openai"
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL(provider)
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = defaultModel(provider)
	}
	apiMode := strings.TrimSpace(cfg.APIMode)
	if apiMode == "" {
		if provider == "anthropic" {
			apiMode = "anthropic_messages"
		} else {
			apiMode = "chat_completions"
		}
	}
	if apiMode == "responses" {
		apiMode = "codex_responses"
	}
	return appsettings.LLM{Provider: provider, BaseURL: baseURL, Model: model, APIMode: apiMode, ResponseTimeoutSeconds: appsettings.NormalizeLLMResponseTimeoutSeconds(cfg.ResponseTimeoutSeconds)}
}

func renderConfig(cfg appsettings.LLM, workDir string) string {
	providerName := cfg.Provider
	if providerName == "custom" {
		providerName = "自訂模型"
	}
	var text strings.Builder
	fmt.Fprintf(&text, "model:\n  default: %s\n  provider: %s\n  base_url: %s\n  api_mode: %s\n\n", yamlString(cfg.Model), providerSlug, yamlString(cfg.BaseURL), yamlString(cfg.APIMode))
	fmt.Fprintf(&text, "providers:\n  %s:\n    name: %s\n    api: %s\n    key_env: %s\n    default_model: %s\n    transport: %s\n    stale_timeout_seconds: %d\n\n", providerSlug, yamlString(providerName), yamlString(cfg.BaseURL), modelAPIKeyEnvName, yamlString(cfg.Model), yamlString(transportForAPIMode(cfg.APIMode)), appsettings.NormalizeLLMResponseTimeoutSeconds(cfg.ResponseTimeoutSeconds))
	text.WriteString("agent:\n  reasoning_effort: medium\n  system_prompt: |-\n")
	for _, line := range strings.Split(systemPrompt, "\n") {
		fmt.Fprintf(&text, "    %s\n", line)
	}
	if strings.TrimSpace(workDir) != "" {
		fmt.Fprintf(&text, "\nterminal:\n  cwd: %s\n", yamlString(workDir))
	}
	text.WriteString("\nmemory:\n  memory_enabled: true\n  user_profile_enabled: false\n  nudge_interval: 0\nskills:\n  creation_nudge_interval: 0\ncurator:\n  enabled: false\nsecurity:\n  allow_lazy_installs: false\n")
	return text.String()
}

func (r *Runtime) renderMergedConfig(cfg appsettings.LLM) (string, error) {
	config, err := r.readConfigMap()
	if err != nil {
		return "", err
	}
	previousAgent, _ := stringMap(config["agent"])
	previousReasoningEffort := ""
	if previousAgent != nil {
		previousReasoningEffort = strings.ToLower(strings.TrimSpace(stringValue(previousAgent["reasoning_effort"])))
	}
	managed := map[string]any{}
	if err := yaml.Unmarshal([]byte(renderConfig(cfg, r.workDir)), &managed); err != nil {
		return "", fmt.Errorf("產生 Hermes 設定: %w", err)
	}
	for _, key := range []string{"model", "providers", "agent", "terminal", "memory", "curator", "security"} {
		if value, ok := managed[key]; ok {
			config[key] = value
		} else {
			delete(config, key)
		}
	}
	// Model/profile synchronization regenerates the managed agent section. Keep
	// the user's selected reasoning level instead of resetting it to medium.
	if IsValidReasoningEffort(previousReasoningEffort) {
		generatedAgent, _ := stringMap(config["agent"])
		if generatedAgent == nil {
			generatedAgent = map[string]any{}
		}
		generatedAgent["reasoning_effort"] = previousReasoningEffort
		config["agent"] = generatedAgent
	}
	existingSkills, _ := stringMap(config["skills"])
	if existingSkills == nil {
		existingSkills = map[string]any{}
	}
	existingSkills["creation_nudge_interval"] = 0
	config["skills"] = existingSkills
	data, err := yaml.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("編碼 Hermes 設定: %w", err)
	}
	return string(data), nil
}

func (r *Runtime) readConfigMap() (map[string]any, error) {
	config := map[string]any{}
	data, err := os.ReadFile(filepath.Join(r.home, "config.yaml"))
	if errors.Is(err, os.ErrNotExist) {
		return config, nil
	}
	if err != nil {
		return nil, fmt.Errorf("讀取 Hermes 設定: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return config, nil
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析 Hermes 設定: %w", err)
	}
	return config, nil
}

func (r *Runtime) writeConfigMap(config map[string]any) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("編碼 Hermes 設定: %w", err)
	}
	if err := writeSecureFile(filepath.Join(r.home, "config.yaml"), data); err != nil {
		return fmt.Errorf("寫入 Hermes 設定: %w", err)
	}
	return nil
}

func discoverSkills(root string) []SkillInfo {
	result := []SkillInfo{}
	seen := map[string]bool{}
	_ = filepath.WalkDir(root, func(skillPath string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || entry.Name() != "SKILL.md" {
			return nil
		}
		data, readErr := os.ReadFile(skillPath)
		if readErr != nil {
			return nil
		}
		name, description := skillFrontmatter(string(data))
		skillDir := filepath.Dir(skillPath)
		if name == "" {
			name = filepath.Base(skillDir)
		}
		if seen[name] {
			return nil
		}
		seen[name] = true
		category := "未分類"
		if relative, relativeErr := filepath.Rel(root, skillDir); relativeErr == nil {
			parts := strings.Split(filepath.ToSlash(relative), "/")
			if len(parts) > 1 && parts[0] != "." {
				category = parts[0]
			}
		}
		result = append(result, SkillInfo{Name: name, Description: description, Category: category, Enabled: true})
		return nil
	})
	slices.SortFunc(result, func(a, b SkillInfo) int { return strings.Compare(a.Name, b.Name) })
	return result
}

func skillFrontmatter(content string) (string, string) {
	content = strings.TrimPrefix(content, "\ufeff")
	lines := strings.Split(content, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return "", ""
	}
	end := -1
	for index, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			end = index + 1
			break
		}
	}
	if end < 0 {
		return "", ""
	}
	var metadata struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &metadata); err != nil {
		return "", ""
	}
	return strings.TrimSpace(metadata.Name), strings.TrimSpace(metadata.Description)
}

func cloneStringAnyMap(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	result := make(map[string]any, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}

func stringMap(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case map[any]any:
		result := map[string]any{}
		for key, item := range typed {
			result[fmt.Sprint(key)] = item
		}
		return result, true
	default:
		return nil, false
	}
}

func stringSlice(value any) []string {
	items, ok := value.([]any)
	if !ok {
		if values, stringOK := value.([]string); stringOK {
			return append([]string(nil), values...)
		}
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok {
			result = append(result, text)
		}
	}
	return result
}

func stringStringMap(value any) map[string]string {
	items, ok := stringMap(value)
	if !ok || len(items) == 0 {
		return nil
	}
	result := map[string]string{}
	for key, item := range items {
		if text, ok := item.(string); ok {
			result[key] = text
		}
	}
	return result
}

func boolValue(value any, fallback bool) bool {
	if boolean, ok := value.(bool); ok {
		return boolean
	}
	if text, ok := value.(string); ok {
		parsed, err := strconv.ParseBool(text)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func intValue(value any) int {
	switch number := value.(type) {
	case int:
		return number
	case int64:
		return int(number)
	case float64:
		return int(number)
	case string:
		parsed, _ := strconv.Atoi(number)
		return parsed
	default:
		return 0
	}
}

func transportForAPIMode(apiMode string) string {
	switch apiMode {
	case "anthropic_messages":
		return "anthropic_messages"
	case "codex_responses", "responses":
		return "codex_responses"
	default:
		return "chat_completions"
	}
}

func defaultBaseURL(provider string) string {
	switch provider {
	case "deepseek":
		return "https://api.deepseek.com"
	case "qwen":
		return "https://dashscope.aliyuncs.com/compatible-mode/v1"
	case "moonshot":
		return "https://api.moonshot.cn/v1"
	case "minimax":
		return "https://api.minimaxi.com/v1"
	case "zhipu":
		return "https://open.bigmodel.cn/api/paas/v4"
	case "siliconflow":
		return "https://api.siliconflow.cn/v1"
	case "anthropic":
		return "https://api.anthropic.com"
	case "custom":
		return ""
	default:
		return "https://api.openai.com/v1"
	}
}

func defaultModel(provider string) string {
	switch provider {
	case "deepseek":
		return "deepseek-chat"
	case "qwen":
		return "qwen-plus"
	case "moonshot":
		return "moonshot-v1-8k"
	case "minimax":
		return "MiniMax-Text-01"
	case "zhipu":
		return "glm-4-plus"
	case "siliconflow":
		return ""
	case "anthropic":
		return "claude-3-5-haiku-latest"
	case "custom":
		return ""
	default:
		return "gpt-4o-mini"
	}
}

func runtimePython(root string) string {
	return runtimePythonForOS(root, runtime.GOOS)
}

func runtimePythonForOS(root, goos string) string {
	if goos == "windows" {
		return filepath.Join(root, "python", "python.exe")
	}
	return filepath.Join(root, "venv", "bin", "python")
}

func hermesEnvironment(base []string, home, workDir, binDir string) []string {
	values := append([]string(nil), base...)
	values = setEnv(values, "HERMES_HOME", home)
	values = setEnv(values, "HERMES_SESSION_SOURCE", "easy-stock")
	values = setEnv(values, "PYTHONNOUSERSITE", "1")
	values = setEnv(values, "PYTHONUNBUFFERED", "1")
	values = setEnv(values, "NO_COLOR", "1")
	if strings.TrimSpace(home) != "" {
		values = setEnv(values, "AGENT_BROWSER_PROFILE", filepath.Join(home, "browser-profile"))
	}
	if strings.TrimSpace(workDir) != "" {
		values = setEnv(values, "TERMINAL_CWD", workDir)
	}
	pathValue := os.Getenv("PATH")
	if pathValue == "" {
		pathValue = filepath.Dir(binDir)
	}
	pathEntries := []string{binDir}
	if wrapperDir := strings.TrimSpace(environmentValue(base, "A_STOCK_AGENT_BROWSER_WRAPPER_DIR")); wrapperDir != "" {
		if info, err := os.Stat(wrapperDir); err == nil && info.IsDir() {
			pathEntries = append(pathEntries, wrapperDir)
		}
	}
	if strings.TrimSpace(workDir) != "" {
		workspaceBin := filepath.Join(workDir, "node_modules", ".bin")
		if info, err := os.Stat(workspaceBin); err == nil && info.IsDir() {
			pathEntries = append(pathEntries, workspaceBin)
		}
	}
	pathEntries = append(pathEntries, pathValue)
	values = setEnv(values, "PATH", strings.Join(pathEntries, string(os.PathListSeparator)))
	return values
}

func environmentValue(values []string, key string) string {
	prefix := key + "="
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			return strings.TrimPrefix(value, prefix)
		}
	}
	return ""
}

func setEnv(values []string, key, value string) []string {
	prefix := key + "="
	for index := range values {
		if strings.HasPrefix(values[index], prefix) {
			values[index] = prefix + value
			return values
		}
	}
	return append(values, prefix+value)
}

func unsetEnv(values []string, key string) []string {
	prefix := key + "="
	result := values[:0]
	for _, value := range values {
		if !strings.HasPrefix(value, prefix) {
			result = append(result, value)
		}
	}
	return result
}

func firstExistingDirectory(values ...string) string {
	for _, value := range values {
		if info, err := os.Stat(value); err == nil && info.IsDir() {
			return value
		}
	}
	return ""
}

func yamlString(value string) string { return strconv.Quote(value) }

func readEnvValue(path, key string) (string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("讀取 Hermes 環境設定: %w", err)
	}
	prefix := key + "="
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), prefix) {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), prefix))
		if unquoted, unquoteErr := strconv.Unquote(value); unquoteErr == nil {
			return unquoted, nil
		}
		return value, nil
	}
	return "", nil
}

func writeEnvValue(path, key, value string) error {
	lines := []string{}
	if data, err := os.ReadFile(path); err == nil {
		lines = strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("讀取 Hermes 環境設定: %w", err)
	}
	prefix := key + "="
	replacement := prefix + strconv.Quote(value)
	updated := false
	result := make([]string, 0, len(lines)+1)
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			if !updated {
				result = append(result, replacement)
				updated = true
			}
			continue
		}
		if line != "" {
			result = append(result, line)
		}
	}
	if !updated {
		result = append(result, replacement)
	}
	if err := writeSecureFile(path, []byte(strings.Join(result, "\n")+"\n")); err != nil {
		return fmt.Errorf("寫入 Hermes 環境設定: %w", err)
	}
	return nil
}

func writeSecureFile(path string, data []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".hermes-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func (r *Runtime) runtimeVersion() string {
	if strings.TrimSpace(r.runtimeRoot) == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(r.runtimeRoot, "runtime-manifest.json"))
	if err != nil {
		return ""
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(data, &manifest) != nil {
		return ""
	}
	return strings.TrimSpace(manifest.Version)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
