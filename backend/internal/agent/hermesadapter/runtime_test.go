package hermesadapter

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nanachi1212/mystocktracer/backend/internal/appsettings"
)

func TestProcessStopAndWaitAreConcurrentSafe(t *testing.T) {
	if os.Getenv("MYSTOCKTRACER_PROCESS_HELPER") == "1" {
		time.Sleep(30 * time.Second)
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestProcessStopAndWaitAreConcurrentSafe")
	cmd.Env = append(os.Environ(), "MYSTOCKTRACER_PROCESS_HELPER=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	p := &process{cmd: cmd, in: stdin, out: stdout, err: stderr}
	go func() { _, _ = io.Copy(io.Discard, p.Errors()) }()
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(2)
		go func() { defer group.Done(); _ = p.Stop() }()
		go func() { defer group.Done(); _ = p.Wait() }()
	}
	done := make(chan struct{})
	go func() { group.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent Stop/Wait deadlocked")
	}
}

func TestProfileSecretsRetainAndClearWithoutEnteringSettingsJSON(t *testing.T) {
	root := t.TempDir()
	python, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	runtime := New(Config{Home: root, PythonPath: python})
	key := "test-secret"
	config := appsettings.LLM{Provider: "openai", BaseURL: "https://example.test/v1", Model: "test-model"}
	if err := runtime.SyncLLMProfile(config, "primary", &key); err != nil {
		t.Fatal(err)
	}
	if err := runtime.SyncLLMProfile(config, "primary", nil); err != nil {
		t.Fatal(err)
	}
	stored, err := runtime.ModelAPIKeyForProfile("primary")
	if err != nil || stored != key {
		t.Fatalf("retained key=%q err=%v", stored, err)
	}
	empty := ""
	if err := runtime.SyncLLMProfile(config, "primary", &empty); err != nil {
		t.Fatal(err)
	}
	stored, err = runtime.ModelAPIKeyForProfile("primary")
	if err != nil || stored != "" {
		t.Fatalf("cleared key=%q err=%v", stored, err)
	}
	data, err := os.ReadFile(filepath.Join(root, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "test-secret") {
		t.Fatal("secret leaked into config.yaml")
	}
}

func TestIsolatedConfigHasNoGeneralCapabilities(t *testing.T) {
	runtime := New(Config{Home: t.TempDir()})
	data, err := runtime.isolatedConfig("Taiwan evidence-only system context")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, required := range []string{"memory_enabled: false", "disabled:\n        - '*'", "mcp_servers: {}", "allow_lazy_installs: false", "Taiwan evidence-only system context"} {
		if !strings.Contains(text, required) {
			t.Fatalf("isolated config missing %q:\n%s", required, text)
		}
	}
	if strings.Contains(text, generalSystemPrompt) {
		t.Fatal("isolated config inherited normal chat system prompt")
	}
}

func TestIsolatedPromptRespectsCancellationBeforeStartingRuntime(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runtime := New(Config{Home: t.TempDir(), PythonPath: "missing-python"})
	_, err := runtime.PromptIsolated(ctx, "Taiwan evidence-only", "bounded evidence")
	if err == nil {
		t.Fatal("expected cancelled isolated prompt to fail")
	}
}
