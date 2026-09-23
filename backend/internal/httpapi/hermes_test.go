package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"sync"

	"github.com/nanachi1212/mystocktracer/backend/internal/agent"
	"github.com/nanachi1212/mystocktracer/backend/internal/appsettings"
)

// This product-facing fixture implements only the capabilities exercised by HTTP tests.
// The scripted process below supplies synthetic adapter frames through pipes.
type fakeAgentRuntime struct {
	status        agent.Status
	promptResult  agent.PromptResult
	promptErr     error
	modelAPIKey   string
	modelKeyErr   error
	start         func(context.Context) (agent.Process, error)
	lastLLM       appsettings.LLM
	lastKey       *string
	agentSettings agent.Settings
}

func (runtime *fakeAgentRuntime) Status() agent.Status { return runtime.status }
func (runtime *fakeAgentRuntime) ModelAPIKey() (string, error) {
	return runtime.modelAPIKey, runtime.modelKeyErr
}
func (runtime *fakeAgentRuntime) Prompt(context.Context, string) (agent.PromptResult, error) {
	return runtime.promptResult, runtime.promptErr
}
func (runtime *fakeAgentRuntime) Start(ctx context.Context) (agent.Process, error) {
	if runtime.start != nil {
		return runtime.start(ctx)
	}
	return newScriptedAgentProcess(), nil
}
func (runtime *fakeAgentRuntime) SyncLLM(model appsettings.LLM, key *string) error {
	runtime.lastLLM = model
	if model.Model == "" && model.BaseURL == "" && key == nil {
		return nil
	}
	runtime.lastKey = nil
	if key != nil {
		copy := *key
		runtime.lastKey = &copy
		runtime.modelAPIKey = copy
		runtime.status.APIKeyConfigured = copy != ""
	}
	runtime.status.Configured = model.Model != "" && model.BaseURL != "" &&
		(model.Provider == "custom" || runtime.status.APIKeyConfigured)
	return nil
}
func (runtime *fakeAgentRuntime) AgentSettings() (agent.Settings, error) {
	return runtime.agentSettings, nil
}
func (runtime *fakeAgentRuntime) SyncAgentSettings(value agent.Settings) error {
	runtime.agentSettings = value
	return nil
}

type scriptedAgentProcess struct {
	input   *io.PipeWriter
	output  *io.PipeReader
	errors  *io.PipeReader
	closed  chan struct{}
	stopper sync.Once
	stop    func()
}

func newScriptedAgentProcess() *scriptedAgentProcess {
	incoming, input := io.Pipe()
	output, outgoing := io.Pipe()
	errors, errorWriter := io.Pipe()
	process := &scriptedAgentProcess{input: input, output: output, errors: errors, closed: make(chan struct{})}
	process.stop = func() {
		for _, closer := range []io.Closer{incoming, input, output, outgoing, errors, errorWriter} {
			_ = closer.Close()
		}
	}
	go func() {
		defer close(process.closed)
		defer process.stopper.Do(process.stop)
		writer := json.NewEncoder(outgoing)
		reader := json.NewDecoder(incoming)
		_ = writer.Encode(map[string]any{"jsonrpc": "2.0", "method": "event", "params": map[string]any{"type": "gateway.ready", "payload": map[string]any{}}})
		for {
			var request struct {
				ID     string `json:"id"`
				Method string `json:"method"`
			}
			if err := reader.Decode(&request); err != nil {
				return
			}
			switch request.Method {
			case "session.create":
				_ = writer.Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID,
					"result": map[string]any{"session_id": "synthetic-live", "stored_session_id": "synthetic-stored"}})
			case "prompt.submit":
				_ = writer.Encode(map[string]any{"jsonrpc": "2.0", "method": "event",
					"params": map[string]any{"type": "message.delta", "payload": map[string]any{"text": "台灣"}}})
				_ = writer.Encode(map[string]any{"jsonrpc": "2.0", "method": "event",
					"params": map[string]any{"type": "message.complete", "payload": map[string]any{"content": "台灣資料"}}})
				return
			}
		}
	}()
	return process
}

func (process *scriptedAgentProcess) Input() io.WriteCloser { return process.input }
func (process *scriptedAgentProcess) Output() io.ReadCloser { return process.output }
func (process *scriptedAgentProcess) Errors() io.ReadCloser { return process.errors }
func (process *scriptedAgentProcess) Wait() error           { <-process.closed; return nil }
func (process *scriptedAgentProcess) Stop() error           { process.stopper.Do(process.stop); return nil }
