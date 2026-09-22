// Package hermesadapter adapts the optional Nous Research Hermes Agent runtime
// to mystocktracer's product-owned agent contracts.
package hermesadapter

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
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/nanachi1212/mystocktracer/backend/internal/agent"
	"github.com/nanachi1212/mystocktracer/backend/internal/appsettings"
	"gopkg.in/yaml.v3"
)

const modelKey = "MODEL_API_KEY"
const profileKeyPrefix = "MODEL_API_KEY_PROFILE_"

type Config struct{ RuntimeRoot, Home, WorkDir, PythonPath string }
type Runtime struct {
	runtimeRoot, home, workDir, python string
	mu                                 sync.RWMutex
	configMu                           sync.Mutex
	llm                                appsettings.LLM
	configured, hasKey                 bool
}

func New(cfg Config) *Runtime {
	python := strings.TrimSpace(cfg.PythonPath)
	if python == "" && cfg.RuntimeRoot != "" {
		if runtime.GOOS == "windows" {
			python = filepath.Join(cfg.RuntimeRoot, "python", "python.exe")
		} else {
			python = filepath.Join(cfg.RuntimeRoot, "venv", "bin", "python")
		}
	}
	return &Runtime{runtimeRoot: strings.TrimSpace(cfg.RuntimeRoot), home: strings.TrimSpace(cfg.Home), workDir: strings.TrimSpace(cfg.WorkDir), python: python}
}
func (r *Runtime) Status() agent.Status {
	r.mu.RLock()
	configured, hasKey := r.configured, r.hasKey
	r.mu.RUnlock()
	status := agent.Status{Configured: configured, APIKeyConfigured: hasKey, Version: r.version()}
	info, err := os.Stat(r.python)
	if r.python == "" || err != nil || info.IsDir() {
		status.Message = "AI 執行環境不可用"
		return status
	}
	status.Available = true
	if !configured {
		status.Message = "請先設定 AI 模型"
	}
	return status
}
func (r *Runtime) ModelAPIKey() (string, error) {
	return envValue(filepath.Join(r.home, ".env"), modelKey)
}
func (r *Runtime) ModelAPIKeyForProfile(id string) (string, error) {
	if id == "" || id == "active" {
		return r.ModelAPIKey()
	}
	return envValue(filepath.Join(r.home, ".env"), profileEnvName(id))
}
func (r *Runtime) StoreLLMProfileKey(id string, update *string) error {
	if id == "" {
		return errors.New("模型設定 ID 不能為空")
	}
	if update == nil {
		return nil
	}
	return r.writeKey(profileEnvName(id), *update)
}
func (r *Runtime) SyncLLM(cfg appsettings.LLM, update *string) error {
	return r.SyncLLMProfile(cfg, "active", update)
}
func (r *Runtime) SyncLLMProfile(cfg appsettings.LLM, profileID string, update *string) error {
	if strings.TrimSpace(r.home) == "" {
		return errors.New("AI 設定目錄未設定")
	}
	r.configMu.Lock()
	defer r.configMu.Unlock()
	if err := os.MkdirAll(r.home, 0o700); err != nil {
		return err
	}
	keyName := profileEnvName(profileID)
	current, err := envValue(filepath.Join(r.home, ".env"), keyName)
	if err != nil {
		return err
	}
	if profileID == "" || profileID == "active" {
		if current == "" {
			current, err = r.ModelAPIKey()
			if err != nil {
				return err
			}
		}
	}
	if update != nil {
		current = strings.TrimSpace(*update)
		if strings.ContainsAny(current, "\r\n") {
			return errors.New("模型 API Key 不能包含換行")
		}
		if err := r.writeKey(keyName, current); err != nil {
			return err
		}
	}
	if err := r.writeKey(modelKey, current); err != nil {
		return err
	}
	llm := normalizeLLM(cfg)
	text, err := r.mergeModelConfig(llm)
	if err != nil {
		return err
	}
	if err := atomicSecretFile(filepath.Join(r.home, "config.yaml"), []byte(text)); err != nil {
		return err
	}
	r.mu.Lock()
	r.llm, r.hasKey = llm, current != ""
	r.configured = llm.Model != "" && llm.BaseURL != "" && (llm.Provider == "custom" || current != "")
	r.mu.Unlock()
	return nil
}
func (r *Runtime) writeKey(key, value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return errors.New("secret 不能包含換行")
	}
	return writeEnv(filepath.Join(r.home, ".env"), key, strings.TrimSpace(value))
}

func (r *Runtime) Start(ctx context.Context) (agent.Process, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	status := r.Status()
	if !status.Available {
		return nil, errors.New(status.Message)
	}
	if strings.TrimSpace(r.home) == "" {
		return nil, errors.New("AI 設定目錄未設定")
	}
	key, err := r.ModelAPIKey()
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, r.python, "-m", "tui_gateway.entry")
	cmd.Dir = existingDir(r.workDir, r.home)
	cmd.Env = runtimeEnvironment(r.home, r.workDir, filepath.Dir(r.python), key)
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		_ = in.Close()
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = in.Close()
		_ = out.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		_ = in.Close()
		_ = out.Close()
		_ = stderr.Close()
		return nil, fmt.Errorf("啟動 AI 執行環境: %w", err)
	}
	return &process{cmd: cmd, in: in, out: out, err: stderr}, nil
}
func (r *Runtime) Prompt(ctx context.Context, prompt string) (agent.PromptResult, error) {
	return r.prompt(ctx, prompt)
}
func (r *Runtime) PromptIsolated(ctx context.Context, systemContext, prompt string) (agent.PromptResult, error) {
	if err := ctx.Err(); err != nil {
		return agent.PromptResult{}, err
	}
	if strings.TrimSpace(systemContext) == "" {
		return agent.PromptResult{}, errors.New("隔離系統提示詞不能為空")
	}
	temporary, err := os.MkdirTemp("", "mystocktracer-ai-isolated-*")
	if err != nil {
		return agent.PromptResult{}, err
	}
	defer os.RemoveAll(temporary)
	config, err := r.isolatedConfig(systemContext)
	if err != nil {
		return agent.PromptResult{}, err
	}
	if err := atomicSecretFile(filepath.Join(temporary, "config.yaml"), config); err != nil {
		return agent.PromptResult{}, err
	}
	key, err := r.ModelAPIKey()
	if err != nil {
		return agent.PromptResult{}, err
	}
	if err := writeEnv(filepath.Join(temporary, ".env"), modelKey, key); err != nil {
		return agent.PromptResult{}, err
	}
	isolated := New(Config{RuntimeRoot: r.runtimeRoot, Home: temporary, WorkDir: r.workDir, PythonPath: r.python})
	r.mu.RLock()
	isolated.llm, isolated.configured, isolated.hasKey = r.llm, r.configured, r.hasKey
	r.mu.RUnlock()
	return isolated.prompt(ctx, prompt)
}
func (r *Runtime) prompt(ctx context.Context, text string) (agent.PromptResult, error) {
	if strings.TrimSpace(text) == "" {
		return agent.PromptResult{}, errors.New("AI 提示詞不能為空")
	}
	status := r.Status()
	if !status.Available || !status.Configured {
		return agent.PromptResult{}, errors.New(status.Message)
	}
	p, err := r.Start(ctx)
	if err != nil {
		return agent.PromptResult{}, err
	}
	defer func() { _ = p.Stop(); _ = p.Wait() }()
	go func() { _, _ = io.Copy(io.Discard, p.Errors()) }()
	lines := make(chan []byte, 1)
	go func() {
		scanner := bufio.NewScanner(p.Output())
		scanner.Buffer(make([]byte, 64<<10), 4<<20)
		for scanner.Scan() {
			lines <- append([]byte(nil), scanner.Bytes()...)
		}
		close(lines)
	}()
	write := func(id, method string, params map[string]any) error {
		data, e := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
		if e != nil {
			return e
		}
		_, e = p.Input().Write(append(data, '\n'))
		return e
	}
	var result agent.PromptResult
	var content strings.Builder
	created := false
	for {
		select {
		case <-ctx.Done():
			return agent.PromptResult{}, ctx.Err()
		case line, ok := <-lines:
			if !ok {
				return agent.PromptResult{}, errors.New("AI 執行環境意外結束")
			}
			var frame rpc
			if json.Unmarshal(line, &frame) != nil {
				return agent.PromptResult{}, errors.New("AI 執行環境傳回無效資料")
			}
			if frame.Error != nil {
				return agent.PromptResult{}, errors.New("AI 執行環境請求失敗")
			}
			if frame.event() == "gateway.ready" && !created {
				created = true
				if e := write("session", "session.create", map[string]any{"client": "mystocktracer"}); e != nil {
					return agent.PromptResult{}, e
				}
				continue
			}
			if frame.ID == "session" && frame.Result != nil {
				result.SessionID = frame.text(frame.Result, "session_id")
				result.StoredSessionID = frame.text(frame.Result, "stored_session_id")
				if result.SessionID == "" {
					return agent.PromptResult{}, errors.New("AI 工作階段無效")
				}
				if result.StoredSessionID == "" {
					result.StoredSessionID = result.SessionID
				}
				if e := write("prompt", "prompt.submit", map[string]any{"session_id": result.SessionID, "text": text}); e != nil {
					return agent.PromptResult{}, e
				}
				continue
			}
			switch frame.event() {
			case "message.delta":
				content.WriteString(first(frame.eventText("delta"), frame.eventText("text")))
			case "message.complete":
				result.Content = strings.TrimSpace(first(frame.eventText("content"), frame.eventText("text"), content.String()))
				if result.Content == "" {
					return agent.PromptResult{}, errors.New("AI 沒有回傳有效內容")
				}
				return result, nil
			case "message.error", "session.error", "run.error":
				return agent.PromptResult{}, errors.New("AI 執行環境請求失敗")
			}
		}
	}
}

func (r *Runtime) AgentSettings() (agent.Settings, error) {
	config, err := r.readConfig()
	if err != nil {
		return agent.Settings{}, err
	}
	settings := agent.Settings{ReasoningEffort: "medium", Skills: discoverSkills(filepath.Join(r.home, "skills"))}
	if value := strings.ToLower(textAt(config, "agent", "reasoning_effort")); agent.IsValidReasoningEffort(value) {
		settings.ReasoningEffort = value
	}
	disabled := map[string]bool{}
	for _, name := range stringList(mapAt(config, "skills")["disabled"]) {
		disabled[name] = true
	}
	for i := range settings.Skills {
		settings.Skills[i].Enabled = !disabled[settings.Skills[i].Name]
	}
	for name, raw := range mapAt(config, "mcp_servers") {
		entry := asMap(raw)
		server := agent.MCPServerSetting{Name: name, Enabled: boolAt(entry, "enabled", true), Transport: strings.ToLower(asString(entry["transport"])), Command: asString(entry["command"]), Args: stringList(entry["args"]), Env: stringMap(entry["env"]), URL: asString(entry["url"]), Headers: stringMap(entry["headers"]), Timeout: intAt(entry, "timeout"), ConnectTimeout: intAt(entry, "connect_timeout"), SupportsParallelToolCall: boolAt(entry, "supports_parallel_tool_calls", false)}
		if server.Transport == "" {
			if server.URL != "" {
				server.Transport = "http"
			} else {
				server.Transport = "stdio"
			}
		}
		settings.MCPServers = append(settings.MCPServers, server)
	}
	sort.Slice(settings.MCPServers, func(i, j int) bool { return settings.MCPServers[i].Name < settings.MCPServers[j].Name })
	return settings, nil
}
func (r *Runtime) SyncAgentSettings(settings agent.Settings) error {
	if !agent.IsValidReasoningEffort(strings.ToLower(strings.TrimSpace(settings.ReasoningEffort))) {
		return errors.New("無效的思考等級")
	}
	r.configMu.Lock()
	defer r.configMu.Unlock()
	config, err := r.readConfig()
	if err != nil {
		return err
	}
	config["agent"] = map[string]any{"reasoning_effort": strings.ToLower(strings.TrimSpace(settings.ReasoningEffort)), "system_prompt": generalSystemPrompt}
	disabled := []string{}
	for _, s := range settings.Skills {
		if !s.Enabled {
			disabled = append(disabled, s.Name)
		}
	}
	config["skills"] = map[string]any{"disabled": disabled, "creation_nudge_interval": 0}
	servers := map[string]any{}
	for _, s := range settings.MCPServers {
		if strings.TrimSpace(s.Name) == "" {
			continue
		}
		entry := map[string]any{"enabled": s.Enabled, "timeout": s.Timeout, "connect_timeout": s.ConnectTimeout, "supports_parallel_tool_calls": s.SupportsParallelToolCall}
		if s.Transport == "stdio" {
			entry["command"] = strings.TrimSpace(s.Command)
			entry["args"] = s.Args
			entry["env"] = s.Env
		} else {
			entry["url"] = strings.TrimSpace(s.URL)
			entry["headers"] = s.Headers
			if s.Transport == "sse" {
				entry["transport"] = "sse"
			}
		}
		servers[strings.TrimSpace(s.Name)] = entry
	}
	config["mcp_servers"] = servers
	return r.writeConfig(config)
}

const generalSystemPrompt = "你是 mystocktracer 的 AI 研究助手。預設使用繁體中文，清楚區分事實、推論與待驗證事項；不得編造即時市場資料或承諾投資報酬。"

func (r *Runtime) isolatedConfig(system string) ([]byte, error) {
	existing, err := r.readConfig()
	if err != nil {
		return nil, err
	}
	config := map[string]any{"model": existing["model"], "providers": existing["providers"], "agent": map[string]any{"reasoning_effort": "medium", "system_prompt": strings.TrimSpace(system)}, "memory": map[string]any{"memory_enabled": false, "user_profile_enabled": false, "nudge_interval": 0}, "skills": map[string]any{"disabled": []string{"*"}, "creation_nudge_interval": 0}, "mcp_servers": map[string]any{}, "curator": map[string]any{"enabled": false}, "security": map[string]any{"allow_lazy_installs": false}}
	return yaml.Marshal(config)
}
func (r *Runtime) mergeModelConfig(llm appsettings.LLM) (string, error) {
	config, err := r.readConfig()
	if err != nil {
		return "", err
	}
	config["model"] = map[string]any{"default": llm.Model, "provider": "mystocktracer", "base_url": llm.BaseURL, "api_mode": llm.APIMode}
	config["providers"] = map[string]any{"mystocktracer": map[string]any{"name": llm.Provider, "api": llm.BaseURL, "key_env": modelKey, "default_model": llm.Model, "transport": transport(llm.APIMode), "stale_timeout_seconds": llm.ResponseTimeoutSeconds}}
	agentConfig := mapAt(config, "agent")
	if agentConfig == nil {
		agentConfig = map[string]any{}
	}
	if !agent.IsValidReasoningEffort(asString(agentConfig["reasoning_effort"])) {
		agentConfig["reasoning_effort"] = "medium"
	}
	agentConfig["system_prompt"] = generalSystemPrompt
	config["agent"] = agentConfig
	config["memory"] = map[string]any{"memory_enabled": true, "user_profile_enabled": false, "nudge_interval": 0}
	config["curator"] = map[string]any{"enabled": false}
	config["security"] = map[string]any{"allow_lazy_installs": false}
	data, err := yaml.Marshal(config)
	return string(data), err
}
func normalizeLLM(value appsettings.LLM) appsettings.LLM {
	value.Provider = strings.ToLower(strings.TrimSpace(value.Provider))
	if value.Provider == "" {
		value.Provider = "openai"
	}
	value.BaseURL = strings.TrimRight(strings.TrimSpace(value.BaseURL), "/")
	value.Model = strings.TrimSpace(value.Model)
	value.APIMode = strings.TrimSpace(value.APIMode)
	if value.APIMode == "responses" {
		value.APIMode = "codex_responses"
	}
	if value.APIMode == "" {
		value.APIMode = "chat_completions"
	}
	value.ResponseTimeoutSeconds = appsettings.NormalizeLLMResponseTimeoutSeconds(value.ResponseTimeoutSeconds)
	return value
}
func transport(mode string) string {
	if mode == "anthropic_messages" {
		return "anthropic_messages"
	}
	if mode == "codex_responses" {
		return "codex_responses"
	}
	return "chat_completions"
}
func profileEnvName(id string) string {
	if id == "" || id == "active" {
		return modelKey
	}
	var b strings.Builder
	for _, c := range id {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
			b.WriteRune(c)
		} else {
			b.WriteByte('_')
		}
	}
	return profileKeyPrefix + b.String()
}
func (r *Runtime) readConfig() (map[string]any, error) {
	out := map[string]any{}
	data, err := os.ReadFile(filepath.Join(r.home, "config.yaml"))
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	if len(bytesTrim(data)) == 0 {
		return out, nil
	}
	if err := yaml.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}
func (r *Runtime) writeConfig(config map[string]any) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}
	return atomicSecretFile(filepath.Join(r.home, "config.yaml"), data)
}
func (r *Runtime) version() string {
	data, err := os.ReadFile(filepath.Join(r.runtimeRoot, "runtime-manifest.json"))
	if err != nil {
		return ""
	}
	var m struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(data, &m) != nil {
		return ""
	}
	return strings.TrimSpace(m.Version)
}

type process struct {
	cmd      *exec.Cmd
	in       io.WriteCloser
	out, err io.ReadCloser
	stopOnce sync.Once
	waitOnce sync.Once
	stopErr  error
	waitErr  error
}

func (p *process) Input() io.WriteCloser { return p.in }
func (p *process) Output() io.ReadCloser { return p.out }
func (p *process) Errors() io.ReadCloser { return p.err }
func (p *process) Wait() error           { p.waitOnce.Do(func() { p.waitErr = p.cmd.Wait() }); return p.waitErr }
func (p *process) Stop() error {
	p.stopOnce.Do(func() {
		if p.cmd.Process != nil {
			p.stopErr = p.cmd.Process.Kill()
		}
	})
	return p.stopErr
}

type rpc struct {
	ID     string         `json:"id"`
	Method string         `json:"method"`
	Params map[string]any `json:"params"`
	Result map[string]any `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (f rpc) event() string {
	if t := asString(f.Params["type"]); t != "" {
		return t
	}
	return f.Method
}
func (f rpc) eventText(key string) string {
	if v := asString(f.Params[key]); v != "" {
		return v
	}
	return asString(asMap(f.Params["payload"])[key])
}
func (f rpc) text(m map[string]any, k string) string { return asString(m[k]) }
func envValue(path, key string) (string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	prefix := key + "="
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			value := strings.TrimSpace(strings.TrimPrefix(line, prefix))
			if plain, e := strconv.Unquote(value); e == nil {
				return plain, nil
			}
			return value, nil
		}
	}
	return "", nil
}
func writeEnv(path, key, value string) error {
	lines := []string{}
	if data, err := os.ReadFile(path); err == nil {
		lines = strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	prefix := key + "="
	replacement := prefix + strconv.Quote(value)
	done := false
	out := make([]string, 0, len(lines)+1)
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			if !done {
				out = append(out, replacement)
				done = true
			}
			continue
		}
		if line != "" {
			out = append(out, line)
		}
	}
	if !done {
		out = append(out, replacement)
	}
	return atomicSecretFile(path, []byte(strings.Join(out, "\n")+"\n"))
}
func atomicSecretFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".mystocktracer-*.tmp")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Chmod(temp, 0o600); err != nil {
		return err
	}
	if err = os.Rename(temp, path); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}
func runtimeEnvironment(home, work, bin, key string) []string {
	env := os.Environ()
	env = putEnv(env, "HERMES_HOME", home)
	env = putEnv(env, "MODEL_API_KEY", key)
	env = putEnv(env, "PYTHONNOUSERSITE", "1")
	env = putEnv(env, "PYTHONUNBUFFERED", "1")
	env = putEnv(env, "NO_COLOR", "1")
	env = putEnv(env, "HERMES_SESSION_SOURCE", "mystocktracer")
	if work != "" {
		env = putEnv(env, "TERMINAL_CWD", work)
	}
	env = putEnv(env, "PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return env
}
func putEnv(values []string, key, value string) []string {
	prefix := key + "="
	for i := range values {
		if strings.HasPrefix(values[i], prefix) {
			values[i] = prefix + value
			return values
		}
	}
	return append(values, prefix+value)
}
func existingDir(values ...string) string {
	for _, value := range values {
		if info, err := os.Stat(value); err == nil && info.IsDir() {
			return value
		}
	}
	return ""
}
func discoverSkills(root string) []agent.SkillSetting {
	out := []agent.SkillSetting{}
	seen := map[string]bool{}
	_ = filepath.WalkDir(root, func(path string, e os.DirEntry, err error) error {
		if err != nil || e.IsDir() || e.Name() != "SKILL.md" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var meta struct {
			Name        string `yaml:"name"`
			Description string `yaml:"description"`
		}
		parts := strings.Split(string(data), "---")
		if len(parts) > 2 {
			_ = yaml.Unmarshal([]byte(parts[1]), &meta)
		}
		if meta.Name == "" {
			meta.Name = filepath.Base(filepath.Dir(path))
		}
		if seen[meta.Name] {
			return nil
		}
		seen[meta.Name] = true
		out = append(out, agent.SkillSetting{Name: meta.Name, Description: meta.Description, Category: "未分類", Enabled: true})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
func asMap(v any) map[string]any {
	if x, ok := v.(map[string]any); ok {
		return x
	}
	if x, ok := v.(map[any]any); ok {
		out := map[string]any{}
		for k, v := range x {
			out[fmt.Sprint(k)] = v
		}
		return out
	}
	return nil
}
func mapAt(m map[string]any, key string) map[string]any { return asMap(m[key]) }
func textAt(m map[string]any, keys ...string) string {
	if len(keys) == 0 {
		return ""
	}
	for _, key := range keys[:len(keys)-1] {
		m = mapAt(m, key)
		if m == nil {
			return ""
		}
	}
	return asString(m[keys[len(keys)-1]])
}
func asString(v any) string { x, _ := v.(string); return strings.TrimSpace(x) }
func stringList(v any) []string {
	var out []string
	switch x := v.(type) {
	case []any:
		for _, v := range x {
			if s := asString(v); s != "" {
				out = append(out, s)
			}
		}
	case []string:
		out = append(out, x...)
	}
	return out
}
func stringMap(v any) map[string]string {
	out := map[string]string{}
	for k, v := range asMap(v) {
		if s := asString(v); s != "" {
			out[k] = s
		}
	}
	return out
}
func boolAt(m map[string]any, key string, fallback bool) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return fallback
}
func intAt(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}
func first(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
func bytesTrim(v []byte) []byte { return []byte(strings.TrimSpace(string(v))) }

var _ agent.StatusReporter = (*Runtime)(nil)
var _ agent.Prompter = (*Runtime)(nil)
var _ agent.IsolatedPrompter = (*Runtime)(nil)
var _ agent.ChatRuntime = (*Runtime)(nil)
var _ agent.SecretReader = (*Runtime)(nil)
var _ agent.ModelConfigurator = (*Runtime)(nil)
var _ agent.ProfileConfigurator = (*Runtime)(nil)
var _ agent.CapabilityConfigurator = (*Runtime)(nil)
