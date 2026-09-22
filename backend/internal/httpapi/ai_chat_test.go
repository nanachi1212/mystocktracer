package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/nanachi1212/mystocktracer/backend/internal/agent"
)

func TestAIChatUsesProductProtocolOverWebSocket(t *testing.T) {
	gateway := &fakeAgentRuntime{status: agent.Status{Available: true, Configured: true, APIKeyConfigured: true}}
	httpServer := httptest.NewServer(NewServer(Config{AgentRuntime: gateway}))
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/v1/ai/ws"
	connection, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer connection.Close()

	readFrame := func() map[string]any {
		_, payload, readErr := connection.ReadMessage()
		if readErr != nil {
			t.Fatalf("read websocket frame: %v", readErr)
		}
		var frame map[string]any
		if json.Unmarshal(payload, &frame) != nil {
			t.Fatalf("invalid frame: %s", payload)
		}
		return frame
	}
	if frame := readFrame(); frame["type"] != "runtime.ready" || frame["version"] != float64(1) {
		t.Fatalf("first frame = %+v, want product runtime.ready event", frame)
	}
	if err := connection.WriteJSON(map[string]any{"version": 1, "type": "session.start", "payload": map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	if frame := readFrame(); frame["type"] != "session.ready" {
		t.Fatalf("session response = %+v", frame)
	}
	if err := connection.WriteJSON(map[string]any{"version": 1, "type": "prompt.submit", "payload": map[string]any{"session_id": "stored-1", "text": "你好"}}); err != nil {
		t.Fatal(err)
	}
	_ = readFrame()
	complete := readFrame()
	if complete["type"] != "message.complete" {
		t.Fatalf("complete frame = %+v", complete)
	}
}

func TestAIChatRequiresAvailableRuntime(t *testing.T) {
	server := NewServer(Config{AgentRuntime: &fakeAgentRuntime{status: agent.Status{Message: "AI 執行環境不可用"}}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/ws", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "AI 執行環境") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
