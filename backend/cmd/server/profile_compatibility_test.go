package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStandaloneProfileWithoutSettings(t *testing.T) {
	for _, profile := range []string{"mystocktracer", "easy-stock", "a-stock-ai"} {
		for _, member := range []string{"taiwan-watchlist.db", "taiwan-portfolio.db", "hermes-home/config.yaml"} {
			t.Run(profile+"/"+member, func(t *testing.T) {
				root := t.TempDir()
				file := filepath.Join(root, profile, filepath.FromSlash(member))
				if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(file, []byte("synthetic state"), 0o600); err != nil {
					t.Fatal(err)
				}
				if got := preferredDataDir(root); got != filepath.Join(root, profile) {
					t.Fatalf("profile=%s", got)
				}
				if contents, err := os.ReadFile(file); err != nil || string(contents) != "synthetic state" {
					t.Fatalf("source changed: %v", err)
				}
			})
		}
	}
}

func TestCanonicalDatabaseWinsOverHistoricalSettings(t *testing.T) {
	root := t.TempDir()
	for _, member := range []string{"mystocktracer/taiwan-watchlist.db", "easy-stock/settings.json", "a-stock-ai/settings.json"} {
		file := filepath.Join(root, filepath.FromSlash(member))
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("synthetic"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if got := preferredDataDir(root); got != filepath.Join(root, "mystocktracer") {
		t.Fatalf("canonical DB profile was ignored: %s", got)
	}
}
