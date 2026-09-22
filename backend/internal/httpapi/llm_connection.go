package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/nanachi1212/mystocktracer/backend/internal/agent"
	"github.com/nanachi1212/mystocktracer/backend/internal/appsettings"
)

const llmProbeMarker = agent.ConnectionProbeMarker

type llmConnectionTestResult struct {
	OK        bool   `json:"ok"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	APIMode   string `json:"api_mode"`
	Runtime   string `json:"runtime"`
	LatencyMS int64  `json:"latency_ms"`
	Response  string `json:"response"`
}

func (s *Server) settingsLLMTest(w http.ResponseWriter, r *http.Request) {
	if s.agentRuntime == nil {
		writeError(w, http.StatusServiceUnavailable, "AI 模型執行環境不可用")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.modelResponseTimeout())
	defer cancel()
	result, err := agent.ProbeConnection(ctx, s.agentRuntime)
	if err != nil {
		if errors.Is(err, agent.ErrRuntimeUnavailable) {
			writeError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		if errors.Is(err, agent.ErrRuntimeUnconfigured) {
			writeError(w, http.StatusPreconditionFailed, err.Error())
			return
		}
		if errors.Is(err, agent.ErrProbeMismatch) {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeUpstreamError(w, "llm_connection_probe", "AI 模型連線失敗", err)
		return
	}

	cfg := s.settingsStore.Snapshot().LLM
	apiMode := strings.TrimSpace(cfg.APIMode)
	if apiMode == "responses" {
		apiMode = "codex_responses"
	}
	if apiMode == "" {
		if strings.TrimSpace(cfg.Provider) == "anthropic" {
			apiMode = "anthropic_messages"
		} else {
			apiMode = "chat_completions"
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": llmConnectionTestResult{
		OK:        true,
		Provider:  firstNonEmpty(strings.TrimSpace(cfg.Provider), "openai"),
		Model:     strings.TrimSpace(cfg.Model),
		APIMode:   apiMode,
		Runtime:   "agent-runtime",
		LatencyMS: result.Latency.Milliseconds(),
		Response:  result.Response,
	}})
}

// modelResponseTimeout keeps synchronous AI endpoints from cancelling a
// request before the configured provider timeout has elapsed. The small
// grace period covers the final retry/error frame and process cleanup.
func (s *Server) modelResponseTimeout() time.Duration {
	seconds := appsettings.DefaultLLMResponseTimeoutSeconds
	if s.settingsStore != nil {
		seconds = appsettings.NormalizeLLMResponseTimeoutSeconds(s.settingsStore.Snapshot().LLM.ResponseTimeoutSeconds)
	}
	return time.Duration(seconds)*time.Second + 15*time.Second
}
