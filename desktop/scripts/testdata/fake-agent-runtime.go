package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type request struct {
	ID     string `json:"id"`
	Method string `json:"method"`
}

func main() {
	for i := 0; i < 2048; i++ {
		fmt.Fprintln(os.Stderr, "fake-runtime-stderr-line")
	}
	write(map[string]any{"jsonrpc": "2.0", "method": "event", "params": map[string]any{"type": "gateway.ready", "payload": map[string]any{}}})
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var input request
		if json.Unmarshal(scanner.Bytes(), &input) != nil {
			continue
		}
		switch input.Method {
		case "session.create", "session.resume":
			write(map[string]any{"jsonrpc": "2.0", "id": input.ID, "result": map[string]any{"session_id": "runtime-smoke", "stored_session_id": "stored-smoke"}})
		case "prompt.submit":
			write(event("message.delta", map[string]any{"text": "packaged "}))
			write(event("message.delta", map[string]any{"text": "stream ok"}))
			write(event("message.complete", map[string]any{"content": "packaged stream ok"}))
		}
	}
}

func event(kind string, payload map[string]any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "method": "event", "params": map[string]any{"type": kind, "payload": payload}}
}
func write(value any) { data, _ := json.Marshal(value); fmt.Println(string(data)) }
