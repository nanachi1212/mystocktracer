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
	"github.com/nanachi1212/mystocktracer/backend/internal/runtimelog"
)

const (
	maxAgentMessageBytes = 4 << 20
	maxAgentErrorBytes   = 8 << 10
)

var agentWebSocketUpgrader = websocket.Upgrader{
	ReadBufferSize:  64 << 10,
	WriteBufferSize: 64 << 10,
	CheckOrigin: func(request *http.Request) bool {
		origin := strings.TrimSpace(request.Header.Get("Origin"))
		return origin == "" || origin == "null" || strings.HasPrefix(origin, "file://") || isAllowedOrigin(origin)
	},
}

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
		_ = sendAgentError(connection, "無法啟動 Hermes 對話")
		return
	}
	bridge := newAgentBridge(process)
	defer bridge.close()
	go bridge.forwardClient(connection)
	bridge.forwardRuntime(connection)
}

func (s *Server) agentAvailability() (string, int) {
	if s.agentRuntime == nil {
		return "Hermes 對話服務不可用", http.StatusServiceUnavailable
	}
	status := s.agentRuntime.Status()
	if !status.Available {
		return firstNonEmpty(status.Message, "Hermes 執行環境不可用"), http.StatusServiceUnavailable
	}
	if !status.Configured {
		return firstNonEmpty(status.Message, "請先設定 Hermes 使用的模型"), http.StatusPreconditionFailed
	}
	return "", http.StatusOK
}

type agentBridge struct {
	process AgentProcess
	stop    sync.Once
	wait    sync.Once
	waitErr error
	errors  *boundedText
}

func newAgentBridge(process AgentProcess) *agentBridge {
	bridge := &agentBridge{process: process, errors: &boundedText{limit: maxAgentErrorBytes}}
	go bridge.captureErrors()
	return bridge
}

func (b *agentBridge) forwardClient(connection *websocket.Conn) {
	for {
		messageType, payload, err := connection.ReadMessage()
		if err != nil {
			b.stopProcess()
			return
		}
		if messageType != websocket.TextMessage {
			continue
		}
		if len(payload) > maxAgentMessageBytes {
			b.stopProcess()
			return
		}
		payload = append(payload, '\n')
		if _, err := b.process.Input().Write(payload); err != nil {
			b.stopProcess()
			return
		}
	}
}

func (b *agentBridge) forwardRuntime(connection *websocket.Conn) {
	scanner := bufio.NewScanner(b.process.Output())
	scanner.Buffer(make([]byte, 64<<10), maxAgentMessageBytes)
	for scanner.Scan() {
		message := append([]byte(nil), scanner.Bytes()...)
		if len(strings.TrimSpace(string(message))) == 0 {
			continue
		}
		if err := connection.WriteMessage(websocket.TextMessage, message); err != nil {
			return
		}
	}
	processError := b.waitProcess()
	if scanner.Err() != nil {
		processError = scanner.Err()
	}
	if processError != nil && !errors.Is(processError, io.EOF) {
		_ = sendAgentError(connection, "Hermes 對話意外結束")
	}
}

func (b *agentBridge) captureErrors() {
	scanner := bufio.NewScanner(b.process.Errors())
	scanner.Buffer(make([]byte, 16<<10), 256<<10)
	for scanner.Scan() {
		b.errors.append(scanner.Text())
	}
}

func (b *agentBridge) stopProcess() { b.stop.Do(func() { _ = b.process.Stop() }) }
func (b *agentBridge) waitProcess() error {
	b.wait.Do(func() { b.waitErr = b.process.Wait() })
	return b.waitErr
}
func (b *agentBridge) close() {
	b.stopProcess()
	_ = b.waitProcess()
}

type boundedText struct {
	mu    sync.Mutex
	value string
	limit int
}

func (b *boundedText) append(value string) {
	value = strings.TrimSpace(runtimelog.Redact(value))
	if value == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.value += "\n" + value
	if len(b.value) > b.limit {
		b.value = b.value[len(b.value)-b.limit:]
	}
}

func sendAgentError(connection *websocket.Conn, message string) error {
	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  "event",
		"params": map[string]any{
			"type":    "gateway.error",
			"payload": map[string]string{"message": strings.TrimSpace(runtimelog.Redact(message))},
		},
	})
	if err != nil {
		return err
	}
	return connection.WriteMessage(websocket.TextMessage, payload)
}
