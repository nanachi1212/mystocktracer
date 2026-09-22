package agent

import (
	"context"
	"errors"
	"strings"
	"time"
)

const ConnectionProbeMarker = "MYSTOCKTRACER_AI_OK"

var (
	ErrRuntimeUnavailable  = errors.New("AI 執行環境不可用")
	ErrRuntimeUnconfigured = errors.New("請先設定 AI 使用的模型")
	ErrProbeMismatch       = errors.New("AI 執行環境已啟動，但模型未回傳預期的探測標記")
)

type ConnectionProbeResult struct {
	Latency  time.Duration
	Response string
}

func ProbeConnection(ctx context.Context, runtime interface {
	StatusReporter
	Prompter
}) (ConnectionProbeResult, error) {
	status := runtime.Status()
	if !status.Available {
		return ConnectionProbeResult{}, ErrRuntimeUnavailable
	}
	if !status.Configured {
		return ConnectionProbeResult{}, ErrRuntimeUnconfigured
	}
	started := time.Now()
	result, err := runtime.Prompt(ctx, "這是 mystocktracer 的模型連線探測。請僅回覆 "+ConnectionProbeMarker+"，不要加入其他文字。")
	if err != nil {
		return ConnectionProbeResult{}, err
	}
	content := strings.TrimSpace(result.Content)
	if !strings.Contains(strings.ToUpper(content), ConnectionProbeMarker) {
		return ConnectionProbeResult{}, ErrProbeMismatch
	}
	return ConnectionProbeResult{Latency: time.Since(started), Response: truncateRunes(content, 200)}, nil
}

func truncateRunes(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit]) + "…"
}
