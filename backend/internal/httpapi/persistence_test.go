package httpapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPersistencePolicyAtStartup(t *testing.T) {
	root := t.TempDir()
	brokenSettings := filepath.Join(root, "broken-settings.json")
	if err := os.WriteFile(brokenSettings, []byte(`{"llm":`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, problem string
		config        Config
	}{
		{name: "strict settings", problem: "open settings", config: Config{SettingsPath: brokenSettings, StrictPersistence: true}},
		{name: "strict database", problem: "taiwan watchlist database", config: Config{WatchlistDBPath: root, TaiwanPortfolioDBPath: root, StrictPersistence: true}},
		{name: "memory fallback", config: Config{SettingsPath: brokenSettings}},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := NewServer(test.config)
			t.Cleanup(func() { _ = server.Close() })
			err := server.StartupError()
			if test.problem == "" {
				if err != nil {
					t.Fatalf("non-strict startup failed: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.problem) {
				t.Fatalf("missing startup diagnosis %q: %v", test.problem, err)
			}
			if test.name == "strict database" && !strings.Contains(err.Error(), "Taiwan portfolio database") {
				t.Fatalf("portfolio failure was hidden: %v", err)
			}
		})
	}
}
