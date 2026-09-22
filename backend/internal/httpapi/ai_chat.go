package httpapi

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

const (
	maxAgentMessageBytes = 256 << 10
	maxAgentPromptBytes  = 64 << 10
	maxAgentReplyBytes   = 1 << 20
	agentProtocolVersion = 1
)

var agentWebSocketUpgrader = websocket.Upgrader{ReadBufferSize: 64 << 10, WriteBufferSize: 64 << 10, CheckOrigin: func(request *http.Request) bool {
	origin := strings.TrimSpace(request.Header.Get("Origin"))
	return origin == "" || origin == "null" || strings.HasPrefix(origin, "file://") || isAllowedOrigin(origin)
}}

// aiChatWebSocket exposes the mystocktracer protocol only. Third-party runtime
// frames are translated here and are never part of the renderer contract.
func (s *Server) aiChatWebSocket(w http.ResponseWriter, r *http.Request) {
	if message, status := s.agentAvailability(); status != http.StatusOK {
		writeError(w, status, message)
		return
	}
	connection, err := agentWebSocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer connection.Close()
	connection.SetReadLimit(maxAgentMessageBytes)
	process, err := s.agentRuntime.Start(r.Context())
	if err != nil {
		_ = writeAgentEvent(connection, agentEvent{Type: "runtime.error", Error: &agentError{Code: "runtime_start_failed", Message: "無法啟動 AI 執行環境"}})
		return
	}
	bridge := &agentBridge{process: process, connection: connection}
	defer bridge.close()
	go func() { _, _ = io.Copy(io.Discard, process.Errors()) }()
	go bridge.forwardClient()
	bridge.forwardRuntime()
}

func (s *Server) agentAvailability() (string, int) {
	if s.agentRuntime == nil {
		return "AI 對話服務不可用", http.StatusServiceUnavailable
	}
	status := s.agentRuntime.Status()
	if !status.Available {
		return firstNonEmpty(status.Message, "AI 執行環境不可用"), http.StatusServiceUnavailable
	}
	if !status.Configured {
		return firstNonEmpty(status.Message, "請先設定 AI 模型"), http.StatusPreconditionFailed
	}
	return "", http.StatusOK
}

type agentEvent struct {
	Version int            `json:"version"`
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload,omitempty"`
	Error   *agentError    `json:"error,omitempty"`
}
type agentError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type agentClientEvent struct {
	Version int             `json:"version"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}
type agentBridge struct {
	process           AgentProcess
	connection        *websocket.Conn
	writeMu           sync.Mutex
	stop              sync.Once
	wait              sync.Once
	waitErr           error
	sessionID         string
	runtimeSessionID  string
	started, resuming bool
	content           strings.Builder
}

func (b *agentBridge) forwardClient() {
	for {
		messageType, payload, err := b.connection.ReadMessage()
		if err != nil {
			b.stopProcess()
			return
		}
		if messageType != websocket.TextMessage || len(payload) > maxAgentMessageBytes {
			b.stopProcess()
			return
		}
		var event agentClientEvent
		if json.Unmarshal(payload, &event) != nil || event.Version != agentProtocolVersion {
			_ = b.send(agentEvent{Type: "runtime.error", Error: &agentError{Code: "invalid_message", Message: "AI 對話訊息格式無效"}})
			b.stopProcess()
			return
		}
		switch event.Type {
		case "session.start":
			b.startSession(event.Payload)
		case "prompt.submit":
			b.submitPrompt(event.Payload)
		case "session.interrupt":
			if b.runtimeSessionID != "" {
				b.writeRuntime("mystocktracer-interrupt", "session.interrupt", map[string]any{"session_id": b.runtimeSessionID})
			}
		default:
			_ = b.send(agentEvent{Type: "runtime.error", Error: &agentError{Code: "unknown_message", Message: "不支援的 AI 對話訊息"}})
			b.stopProcess()
			return
		}
	}
}
func (b *agentBridge) startSession(payload json.RawMessage) {
	if b.started {
		_ = b.send(agentEvent{Type: "runtime.error", Error: &agentError{Code: "session_started", Message: "AI 對話已建立"}})
		return
	}
	var input struct {
		ResumeSessionID string `json:"resume_session_id"`
		SeedMessages    []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"seed_messages"`
	}
	if json.Unmarshal(payload, &input) != nil || len(input.SeedMessages) > 32 || len(input.ResumeSessionID) > 256 {
		_ = b.send(agentEvent{Type: "runtime.error", Error: &agentError{Code: "invalid_session", Message: "AI 對話工作階段無效"}})
		b.stopProcess()
		return
	}
	b.started, b.resuming = true, input.ResumeSessionID != ""
	if b.resuming {
		b.writeRuntime("mystocktracer-session", "session.resume", map[string]any{"session_id": input.ResumeSessionID})
		return
	}
	params := map[string]any{"client": "mystocktracer-desktop"}
	if len(input.SeedMessages) > 0 {
		params["messages"] = input.SeedMessages
	}
	b.writeRuntime("mystocktracer-session", "session.create", params)
}
func (b *agentBridge) submitPrompt(payload json.RawMessage) {
	var input struct {
		SessionID string `json:"session_id"`
		Text      string `json:"text"`
	}
	if json.Unmarshal(payload, &input) != nil || input.SessionID == "" || input.SessionID != b.sessionID || len(input.Text) > maxAgentPromptBytes {
		_ = b.send(agentEvent{Type: "runtime.error", Error: &agentError{Code: "invalid_prompt", Message: "AI 提示內容無效"}})
		return
	}
	b.content.Reset()
	b.writeRuntime("mystocktracer-prompt", "prompt.submit", map[string]any{"session_id": b.runtimeSessionID, "text": input.Text})
}

func (b *agentBridge) forwardRuntime() {
	scanner := bufio.NewScanner(b.process.Output())
	scanner.Buffer(make([]byte, 64<<10), maxAgentMessageBytes)
	for scanner.Scan() {
		b.handleRuntimeFrame(scanner.Bytes())
	}
	if err := b.waitProcess(); err != nil && !errors.Is(err, io.EOF) {
		_ = b.send(agentEvent{Type: "runtime.error", Error: &agentError{Code: "runtime_ended", Message: "AI 執行環境意外結束"}})
	}
}
func (b *agentBridge) handleRuntimeFrame(payload []byte) {
	var frame struct {
		ID     string `json:"id"`
		Params struct {
			Type    string         `json:"type"`
			Payload map[string]any `json:"payload"`
		} `json:"params"`
		Result map[string]any `json:"result"`
		Error  *struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(payload, &frame) != nil {
		_ = b.send(agentEvent{Type: "runtime.error", Error: &agentError{Code: "invalid_runtime_frame", Message: "AI 執行環境傳回無效資料"}})
		return
	}
	if frame.ID == "mystocktracer-session" {
		if frame.Error != nil && b.resuming {
			b.resuming = false
			b.writeRuntime("mystocktracer-session-new", "session.create", map[string]any{"client": "mystocktracer-desktop"})
			return
		}
		if frame.Error != nil {
			_ = b.send(agentEvent{Type: "runtime.error", Error: &agentError{Code: "session_unavailable", Message: "無法建立 AI 對話"}})
			return
		}
		b.readyFromResult(frame.Result)
		return
	}
	if frame.ID == "mystocktracer-session-new" {
		b.readyFromResult(frame.Result)
		return
	}
	switch frame.Params.Type {
	case "gateway.ready":
		_ = b.send(agentEvent{Type: "runtime.ready", Payload: map[string]any{"protocol": agentProtocolVersion}})
	case "message.delta":
		if text, ok := frame.Params.Payload["text"].(string); ok {
			if b.content.Len()+len(text) > maxAgentReplyBytes {
				_ = b.send(agentEvent{Type: "runtime.error", Error: &agentError{Code: "reply_too_large", Message: "AI 回覆超過允許大小"}})
				b.stopProcess()
				return
			}
			b.content.WriteString(text)
			_ = b.send(agentEvent{Type: "message.delta", Payload: map[string]any{"content": b.content.String()}})
		}
	case "message.complete":
		content, _ := frame.Params.Payload["content"].(string)
		if content == "" {
			content = b.content.String()
		}
		_ = b.send(agentEvent{Type: "message.complete", Payload: map[string]any{"content": content}})
	}
}
func (b *agentBridge) readyFromResult(result map[string]any) {
	runtimeID, _ := result["session_id"].(string)
	storedID, _ := result["stored_session_id"].(string)
	if runtimeID == "" {
		_ = b.send(agentEvent{Type: "runtime.error", Error: &agentError{Code: "invalid_session", Message: "AI 對話工作階段無效"}})
		return
	}
	if storedID == "" {
		storedID = runtimeID
	}
	b.runtimeSessionID, b.sessionID = runtimeID, storedID
	_ = b.send(agentEvent{Type: "session.ready", Payload: map[string]any{"session_id": storedID}})
}
func (b *agentBridge) writeRuntime(id, method string, params map[string]any) {
	data, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	if err == nil {
		_, _ = b.process.Input().Write(append(data, '\n'))
	}
}
func (b *agentBridge) send(event agentEvent) error {
	event.Version = agentProtocolVersion
	b.writeMu.Lock()
	defer b.writeMu.Unlock()
	return writeAgentEvent(b.connection, event)
}
func writeAgentEvent(connection *websocket.Conn, event agentEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return connection.WriteMessage(websocket.TextMessage, data)
}
func (b *agentBridge) stopProcess() { b.stop.Do(func() { _ = b.process.Stop() }) }
func (b *agentBridge) waitProcess() error {
	b.wait.Do(func() { b.waitErr = b.process.Wait() })
	return b.waitErr
}
func (b *agentBridge) close() { b.stopProcess(); _ = b.waitProcess() }
