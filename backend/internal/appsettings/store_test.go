package appsettings

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestSettingsFileRoundTripAndPrivateState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile", "settings.json")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	initial := store.Snapshot()
	if initial.BrokerCommission.Rate != nil || !initial.TaiwanAlerts.CorporateEventsEnabled {
		t.Fatalf("unexpected defaults: %+v", initial)
	}
	rate, minimum := 0.001425, 20.0
	_, err = store.Update(func(value *Values) error {
		value.LLM.Provider = "custom"
		value.LLM.APIKey = "synthetic-private-key"
		value.BrokerCommission = BrokerCommission{Rate: &rate, Discount: 0.6, Minimum: &minimum, Source: "broker_config"}
		value.TaiwanAlerts.CorporateEventsEnabled = false
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("settings mode %o", info.Mode().Perm())
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got := reopened.Snapshot()
	if got.LLM.APIKey != "synthetic-private-key" || got.BrokerCommission.Rate == nil ||
		*got.BrokerCommission.Rate != rate || got.BrokerCommission.Source != "broker_config" ||
		got.TaiwanAlerts.CorporateEventsEnabled || got.UpdatedAt.IsZero() {
		t.Fatalf("round trip changed settings: %+v", got)
	}
}

func TestHistoricalSettingsReadAndWriteCompatibility(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	oldFile := []byte(`{"llm":{"provider":"custom","base_url":"https://model.example/v1","model":"synthetic-model","api_mode":"codex_responses"},"tushare_token":"retired","review_automation":{"enabled":true}}`)
	if err := os.WriteFile(path, oldFile, 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got := store.Snapshot()
	if len(got.LLMProfiles) != 1 || got.ActiveLLMProfileID != got.LLMProfiles[0].ID ||
		got.LLMProfiles[0].Model != "synthetic-model" || got.LLM.ResponseTimeoutSeconds != DefaultLLMResponseTimeoutSeconds ||
		!got.TaiwanAlerts.CorporateEventsEnabled {
		t.Fatalf("old profile not normalized: %+v", got)
	}
	if _, err := store.Update(func(value *Values) error {
		value.TaiwanAlerts.CorporateEventsEnabled = false
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, retired := range []string{"tushare_token", "review_automation"} {
		if bytes.Contains(written, []byte(retired)) {
			t.Fatalf("removed setting %s returned in %s", retired, written)
		}
	}
	if again, err := Open(path); err != nil || again.Snapshot().TaiwanAlerts.CorporateEventsEnabled {
		t.Fatalf("alert preference did not persist: %v", err)
	}
}

func TestRetiredAnthropicDefaultModelIsUpgraded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	oldFile := []byte(`{"llm":{"provider":"anthropic","base_url":"https://api.anthropic.com","model":"claude-3-5-haiku-latest","api_mode":"anthropic_messages"},"llm_profiles":[{"id":"p1","name":"a","provider":"anthropic","base_url":"https://api.anthropic.com","model":"claude-3-5-haiku-latest","api_mode":"anthropic_messages"},{"id":"p2","name":"b","provider":"custom","base_url":"https://model.example/v1","model":"synthetic-model","api_mode":"chat_completions"}],"active_llm_profile_id":"p1"}`)
	if err := os.WriteFile(path, oldFile, 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got := store.Snapshot()
	if got.LLM.Model != "claude-haiku-4-5" || got.LLMProfiles[0].Model != "claude-haiku-4-5" || got.LLMProfiles[1].Model != "synthetic-model" {
		t.Fatalf("retired default model not upgraded: %+v", got)
	}
}

func TestUnreadableSettingsDoNotChangeOnOpen(t *testing.T) {
	for _, input := range []string{"", `{"llm":`} {
		t.Run(input, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Open(path); err == nil || !strings.Contains(err.Error(), "decode settings") {
				t.Fatalf("expected decode failure, got %v", err)
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != input {
				t.Fatalf("invalid input was modified: %q, %v", after, err)
			}
		})
	}
}

func TestRejectedUpdateLeavesSnapshotAndFileIntact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Update(func(value *Values) error { value.BrokerCommission.Source = "before"; return nil }); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	_, err = store.Update(func(value *Values) error { value.BrokerCommission.Source = "after"; return errors.New("rejected") })
	if err == nil || store.Snapshot().BrokerCommission.Source != "before" {
		t.Fatalf("rejected mutation became visible: %v", err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("rejected mutation rewrote settings file")
	}
}

func TestSnapshotsAreIndependentDuringConcurrentUpdates(t *testing.T) {
	store, err := Open("")
	if err != nil {
		t.Fatal(err)
	}
	first := store.Snapshot()
	first.LLMProfiles[0].Name = "mutated copy"
	if store.Snapshot().LLMProfiles[0].Name == "mutated copy" {
		t.Fatal("snapshot exposes mutable profile slice")
	}
	var workers sync.WaitGroup
	for worker := 0; worker < 6; worker++ {
		workers.Add(1)
		go func(id int) {
			defer workers.Done()
			for attempt := 0; attempt < 50; attempt++ {
				if id%2 == 0 {
					if _, err := store.Update(func(value *Values) error { value.BrokerCommission.Source = "test"; return nil }); err != nil {
						t.Errorf("update: %v", err)
					}
				} else if len(store.Snapshot().LLMProfiles) == 0 {
					t.Error("reader saw an incomplete profile")
				}
			}
		}(worker)
	}
	workers.Wait()
}
