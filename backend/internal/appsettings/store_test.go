package appsettings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestStorePersistsSecretsWithPrivatePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "settings.json")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("open settings: %v", err)
	}
	if _, err := store.Update(func(values *Values) error {
		values.LLM.Provider = "openai"
		values.LLM.APIKey = "sk-secret-value"
		return nil
	}); err != nil {
		t.Fatalf("update settings: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat settings: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("settings permissions = %o, want 600", info.Mode().Perm())
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen settings: %v", err)
	}
	values := reopened.Snapshot()
	if values.LLM.APIKey != "sk-secret-value" {
		t.Fatalf("settings did not persist: %+v", values)
	}
}

func TestStoreMigratesSingleLLMToSelectableProfiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	legacy := map[string]any{"llm": map[string]any{"provider": "custom", "base_url": "https://model.example/v1", "model": "gpt-5.6-sol", "api_mode": "codex_responses"}}
	data, _ := json.Marshal(legacy)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	values := store.Snapshot()
	if len(values.LLMProfiles) != 1 || values.LLMProfiles[0].Model != "gpt-5.6-sol" || values.ActiveLLMProfileID != values.LLMProfiles[0].ID {
		t.Fatalf("migration=%+v", values)
	}
	if values.LLM.ResponseTimeoutSeconds != DefaultLLMResponseTimeoutSeconds {
		t.Fatalf("response timeout = %d, want default %d", values.LLM.ResponseTimeoutSeconds, DefaultLLMResponseTimeoutSeconds)
	}
}

func TestStorePersistsBrokerSpecificCommissionWithoutInventingDefault(t *testing.T) {
	store, err := Open("")
	if err != nil {
		t.Fatal(err)
	}
	if store.Snapshot().BrokerCommission.Rate != nil {
		t.Fatal("unset broker rate must remain nil")
	}
	rate, minimum := .001425, 20.0
	values, err := store.Update(func(values *Values) error {
		values.BrokerCommission = BrokerCommission{Rate: &rate, Discount: .6, Minimum: &minimum, Source: "broker_config"}
		return nil
	})
	if err != nil || values.BrokerCommission.Rate == nil || *values.BrokerCommission.Rate != rate {
		t.Fatalf("commission = %+v, %v", values.BrokerCommission, err)
	}
}

func TestTaiwanCorporateEventAlertPreferenceDefaultsEnabledAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if !store.Snapshot().TaiwanAlerts.CorporateEventsEnabled {
		t.Fatal("corporate-event alerts must default enabled for backward-compatible change detection")
	}
	if _, err := store.Update(func(values *Values) error {
		values.TaiwanAlerts.CorporateEventsEnabled = false
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Snapshot().TaiwanAlerts.CorporateEventsEnabled {
		t.Fatal("disabled corporate-event alert preference did not persist")
	}
}

func TestLegacySettingsWithoutTaiwanAlertsKeepSafeDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"llm":{"provider":"custom"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if !store.Snapshot().TaiwanAlerts.CorporateEventsEnabled {
		t.Fatal("legacy settings unexpectedly disabled Taiwan alerts")
	}
}

func TestInvalidJSONDoesNotOverwriteExistingSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	original := []byte(`{"llm":`)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); err == nil || !strings.Contains(err.Error(), "decode settings") {
		t.Fatalf("Open() error = %v, want decode failure", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Fatalf("invalid settings were modified: %q", after)
	}
}

func TestLegacyUnknownFieldsAreReadButNotRegenerated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	legacy := []byte(`{"llm":{"provider":"custom","model":"gpt-test"},"tushare_token":"removed","review_automation":{"enabled":true}}`)
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if store.Snapshot().LLM.Model != "gpt-test" {
		t.Fatalf("known legacy value was not loaded: %+v", store.Snapshot())
	}
	if _, err := store.Update(func(values *Values) error {
		values.TaiwanAlerts.CorporateEventsEnabled = false
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, removed := range []string{"tushare_token", "review_automation"} {
		if strings.Contains(string(written), removed) {
			t.Fatalf("retired field %q was regenerated: %s", removed, written)
		}
	}
}

func TestStoreConcurrentSnapshotsAndUpdates(t *testing.T) {
	store, err := Open("")
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		group.Add(1)
		go func(id int) {
			defer group.Done()
			for iteration := 0; iteration < 100; iteration++ {
				if id%2 == 0 {
					_, _ = store.Update(func(values *Values) error {
						values.BrokerCommission.Source = "concurrent"
						return nil
					})
					continue
				}
				snapshot := store.Snapshot()
				if len(snapshot.LLMProfiles) == 0 {
					t.Errorf("snapshot lost normalized profiles")
					return
				}
			}
		}(worker)
	}
	group.Wait()
}
