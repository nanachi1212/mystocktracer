package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPreferredDataDirUsesLegacyWhenRenamedDirectoryIsUnconfigured(t *testing.T) {
	configDir := t.TempDir()
	legacy := filepath.Join(configDir, "a-stock-ai")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "settings.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := preferredDataDir(configDir); got != legacy {
		t.Fatalf("preferredDataDir() = %q, want %q", got, legacy)
	}
}

func TestPreferredDataDirUsesRenamedDirectoryWhenConfigured(t *testing.T) {
	configDir := t.TempDir()
	current := filepath.Join(configDir, "easy-stock")
	legacy := filepath.Join(configDir, "a-stock-ai")
	for _, directory := range []string{current, legacy} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "settings.json"), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if got := preferredDataDir(configDir); got != current {
		t.Fatalf("preferredDataDir() = %q, want %q", got, current)
	}
}

func TestRuntimeEnvPrefersMystocktracerNameAndFallsBackToLegacy(t *testing.T) {
	t.Setenv("A_STOCK_ADDR", "127.0.0.1:21001")
	if got := runtimeEnv("ADDR"); got != "127.0.0.1:21001" {
		t.Fatalf("legacy fallback = %q", got)
	}
	t.Setenv("MYSTOCKTRACER_ADDR", "127.0.0.1:22001")
	if got := runtimeEnv("ADDR"); got != "127.0.0.1:22001" {
		t.Fatalf("canonical env did not win: %q", got)
	}
	t.Setenv("MYSTOCKTRACER_ADDR", "")
	if got := runtimeEnv("ADDR"); got != "" {
		t.Fatalf("explicit empty canonical env must not fall back: %q", got)
	}
}

func TestLoadRuntimeConfigKeepsPersistedDataFilenames(t *testing.T) {
	configDirectory := t.TempDir()
	t.Setenv("MYSTOCKTRACER_SETTINGS_PATH", filepath.Join(configDirectory, "settings.json"))
	t.Setenv("MYSTOCKTRACER_TAIWAN_WATCHLIST_DB", filepath.Join(configDirectory, "taiwan-watchlist.db"))
	t.Setenv("MYSTOCKTRACER_TAIWAN_PORTFOLIO_DB", filepath.Join(configDirectory, "taiwan-portfolio.db"))
	config := loadRuntimeConfig()
	if filepath.Base(config.settingsPath) != "settings.json" ||
		filepath.Base(config.watchlistDBPath) != "taiwan-watchlist.db" ||
		filepath.Base(config.portfolioDBPath) != "taiwan-portfolio.db" {
		t.Fatalf("persisted paths changed: %+v", config)
	}
}
