package main

import (
	"os"
	"path/filepath"
	"testing"
)

func createSavedProfile(t *testing.T, root, name string) string {
	t.Helper()
	directory := filepath.Join(root, name)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "settings.json"), []byte(`{"llm":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return directory
}

func TestProfileSelectionPreservesHistoricalDirectories(t *testing.T) {
	cases := []struct {
		name, expected string
		profiles       []string
	}{
		{name: "fresh", expected: "mystocktracer"},
		{name: "oldest only", profiles: []string{"a-stock-ai"}, expected: "a-stock-ai"},
		{name: "later historical profile", profiles: []string{"a-stock-ai", "easy-stock"}, expected: "easy-stock"},
		{name: "canonical profile", profiles: []string{"a-stock-ai", "easy-stock", "mystocktracer"}, expected: "mystocktracer"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			for _, name := range test.profiles {
				createSavedProfile(t, root, name)
			}
			if got := preferredDataDir(root); got != filepath.Join(root, test.expected) {
				t.Fatalf("selected %q, expected %q", got, test.expected)
			}
			for _, name := range test.profiles {
				if _, err := os.Stat(filepath.Join(root, name, "settings.json")); err != nil {
					t.Fatalf("selection changed profile %s: %v", name, err)
				}
			}
		})
	}
}

func TestRuntimeEnvironmentPriorityAndExplicitEmptyValue(t *testing.T) {
	t.Setenv("A_STOCK_ADDR", "127.0.0.1:21111")
	if got := runtimeEnv("ADDR"); got != "127.0.0.1:21111" {
		t.Fatalf("legacy fallback: %q", got)
	}
	t.Setenv("MYSTOCKTRACER_ADDR", "127.0.0.1:22222")
	if got := runtimeEnv("ADDR"); got != "127.0.0.1:22222" {
		t.Fatalf("canonical precedence: %q", got)
	}
	t.Setenv("MYSTOCKTRACER_ADDR", "")
	if got := runtimeEnv("ADDR"); got != "" {
		t.Fatalf("explicit empty canonical value was ignored: %q", got)
	}
}

func TestRuntimeStorageNamesRemainStable(t *testing.T) {
	root := t.TempDir()
	paths := map[string]string{
		"MYSTOCKTRACER_SETTINGS_PATH":       "settings.json",
		"MYSTOCKTRACER_TAIWAN_WATCHLIST_DB": "taiwan-watchlist.db",
		"MYSTOCKTRACER_TAIWAN_PORTFOLIO_DB": "taiwan-portfolio.db",
	}
	for key, name := range paths {
		t.Setenv(key, filepath.Join(root, name))
	}
	got := loadRuntimeConfig()
	for field, path := range map[string]string{
		"settings": got.settingsPath, "watchlist": got.watchlistDBPath, "portfolio": got.portfolioDBPath,
	} {
		if filepath.Dir(path) != root {
			t.Fatalf("%s path escaped configured directory: %q", field, path)
		}
	}
	if filepath.Base(got.settingsPath) != paths["MYSTOCKTRACER_SETTINGS_PATH"] ||
		filepath.Base(got.watchlistDBPath) != paths["MYSTOCKTRACER_TAIWAN_WATCHLIST_DB"] ||
		filepath.Base(got.portfolioDBPath) != paths["MYSTOCKTRACER_TAIWAN_PORTFOLIO_DB"] {
		t.Fatalf("storage filenames changed: %+v", got)
	}
}
