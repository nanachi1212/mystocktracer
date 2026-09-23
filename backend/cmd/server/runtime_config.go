package main

import (
	"os"
	"path/filepath"
	"strings"
)

type runtimeConfig struct {
	address                string
	token                  string
	settingsPath           string
	watchlistDBPath        string
	portfolioDBPath        string
	cashflowCacheDirectory string
	logDirectory           string
	hermesRuntimeRoot      string
	hermesHome             string
	hermesWorkDir          string
	hermesPython           string
	toAlphaMOPSEnabled     bool
	toAlphaMOPSEndpoint    string
	version                string
}

func loadRuntimeConfig() runtimeConfig {
	dataDirectory := ""
	if configDirectory, err := os.UserConfigDir(); err == nil {
		dataDirectory = preferredDataDir(configDirectory)
	}
	workDirectory, _ := os.Getwd()
	return runtimeConfig{
		address:                envOrDefault("ADDR", "127.0.0.1:20081"),
		token:                  runtimeEnv("TOKEN"),
		settingsPath:           envOrDataPath("SETTINGS_PATH", dataDirectory, "settings.json"),
		watchlistDBPath:        envOrDataPath("TAIWAN_WATCHLIST_DB", dataDirectory, "taiwan-watchlist.db"),
		portfolioDBPath:        envOrDataPath("TAIWAN_PORTFOLIO_DB", dataDirectory, "taiwan-portfolio.db"),
		cashflowCacheDirectory: envOrDataPath("CASHFLOW_CACHE", dataDirectory, "cashflow-cache"),
		logDirectory:           envOrDataPath("LOG_DIR", dataDirectory, "logs"),
		hermesRuntimeRoot:      resolveHermesRuntimeRoot(),
		hermesHome:             envOrDataPath("HERMES_HOME", dataDirectory, "hermes-home"),
		hermesWorkDir:          envOrDefault("HERMES_WORKDIR", workDirectory),
		hermesPython:           runtimeEnv("HERMES_PYTHON"),
		toAlphaMOPSEnabled:     envBool("TOALPHA_MOPS_ENABLED"),
		toAlphaMOPSEndpoint:    runtimeEnv("TOALPHA_MOPS_ENDPOINT"),
		version:                envOrDefault("APP_VERSION", "development"),
	}
}

func runtimeEnv(suffix string) string {
	if value, ok := os.LookupEnv("MYSTOCKTRACER_" + suffix); ok {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(os.Getenv("A_STOCK_" + suffix))
}

func envOrDefault(suffix, fallback string) string {
	if value := runtimeEnv(suffix); value != "" {
		return value
	}
	return fallback
}

func envOrDataPath(suffix, dataDirectory, name string) string {
	if value := runtimeEnv(suffix); value != "" {
		return value
	}
	return dataPath(dataDirectory, name)
}

func envBool(suffix string) bool {
	value := runtimeEnv(suffix)
	return value == "1" || strings.EqualFold(value, "true") || strings.EqualFold(value, "yes")
}

// preferredDataDir picks the data directory for a standalone backend run. The desktop app never
// relies on it — it passes every path explicitly — so this only covers development and direct
// `go run` usage. Phase B4 made "mystocktracer" the canonical name; the two historical names are
// still read in place when they already hold settings, so an existing local profile keeps working.
// Nothing is copied or renamed here: adopting a directory is the whole behaviour.
func preferredDataDir(configDirectory string) string {
	canonical := filepath.Join(configDirectory, "mystocktracer")
	for _, candidate := range []string{canonical, filepath.Join(configDirectory, "easy-stock"), filepath.Join(configDirectory, "a-stock-ai")} {
		if isFile(filepath.Join(candidate, "settings.json")) {
			return candidate
		}
	}
	return canonical
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dataPath(directory, name string) string {
	if directory == "" {
		return ""
	}
	return filepath.Join(directory, name)
}

func resolveHermesRuntimeRoot() string {
	if configured := runtimeEnv("HERMES_RUNTIME_ROOT"); configured != "" {
		return configured
	}
	var candidates []string
	if executable, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Clean(filepath.Join(filepath.Dir(executable), "..", "hermes-runtime")))
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(cwd, "desktop", "resources", "hermes-runtime"),
			filepath.Join(cwd, "..", "desktop", "resources", "hermes-runtime"),
		)
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return ""
}
