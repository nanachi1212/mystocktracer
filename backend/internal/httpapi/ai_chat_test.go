package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nanachi1212/mystocktracer/backend/internal/agent"
)

func TestChatSocketTranslatesAdapterFramesToProductEvents(t *testing.T) {
	runtime := &fakeAgentRuntime{status: agent.Status{Available: true, Configured: true, APIKeyConfigured: true}}
	server := httptest.NewServer(NewServer(Config{AgentRuntime: runtime}))
	defer server.Close()
	connection, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/api/v1/ai/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	_ = connection.SetReadDeadline(time.Now().Add(5 * time.Second))
	read := func(expected string) agentEvent {
		t.Helper()
		var event agentEvent
		if err := connection.ReadJSON(&event); err != nil {
			t.Fatal(err)
		}
		if event.Version != agentProtocolVersion || event.Type != expected {
			t.Fatalf("received %+v, expected %s v%d", event, expected, agentProtocolVersion)
		}
		return event
	}
	read("runtime.ready")
	if err := connection.WriteJSON(agentClientEvent{Version: agentProtocolVersion, Type: "session.start", Payload: []byte(`{}`)}); err != nil {
		t.Fatal(err)
	}
	ready := read("session.ready")
	id, ok := ready.Payload["session_id"].(string)
	if !ok || id == "" {
		t.Fatalf("session ID missing: %+v", ready)
	}
	if err := connection.WriteJSON(map[string]any{"version": agentProtocolVersion, "type": "prompt.submit",
		"payload": map[string]any{"session_id": id, "text": "分析合成台股資料"}}); err != nil {
		t.Fatal(err)
	}
	if delta := read("message.delta"); delta.Payload["content"] != "台灣" {
		t.Fatalf("streamed content changed: %+v", delta)
	}
	if complete := read("message.complete"); complete.Payload["content"] != "台灣資料" {
		t.Fatalf("final content changed: %+v", complete)
	}
}

func TestChatSocketRejectsUnavailableAndUnconfiguredRuntimes(t *testing.T) {
	for _, test := range []struct {
		name     string
		status   agent.Status
		expected int
	}{
		{name: "unavailable", status: agent.Status{Message: "執行環境不可用"}, expected: http.StatusServiceUnavailable},
		{name: "unconfigured", status: agent.Status{Available: true}, expected: http.StatusPreconditionFailed},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := NewServer(Config{AgentRuntime: &fakeAgentRuntime{status: test.status}})
			response := httptest.NewRecorder()
			server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/ai/ws", nil))
			if response.Code != test.expected || !strings.Contains(response.Body.String(), "error") {
				t.Fatalf("status=%d payload=%s", response.Code, response.Body.String())
			}
		})
	}
}
