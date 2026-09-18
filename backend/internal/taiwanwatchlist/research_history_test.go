package taiwanwatchlist

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func researchHistoryRecord(runID, canonical string, createdAt time.Time) ResearchHistoryRecord {
	return ResearchHistoryRecord{
		RunID: runID, Canonical: canonical, SecurityName: "台積電", CreatedAt: createdAt,
		EvidenceAsOf: "2026-09-17", ResearchVersion: "taiwan_ai_research_v2", PayloadVersion: "taiwan_ai_research_v2",
		ModelProvider: "test", ModelName: "test-model", EvidenceSnapshot: json.RawMessage(`{"research_version":"taiwan_ai_research_v2"}`),
		ResearchResult: json.RawMessage(`{"model_version":"taiwan_ai_research_v2","status":"available"}`),
		Provenance:     json.RawMessage(`{"price":{"status":"available"}}`), Validity: json.RawMessage(`{"valid":true,"completeness":"complete"}`), Completeness: "complete",
	}
}

func TestResearchHistoryPersistsOrdersPaginatesAndIsolatesSymbols(t *testing.T) {
	path := filepath.Join(t.TempDir(), "taiwan.db")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 9, 17, 1, 0, 0, 0, time.UTC)
	for index, runID := range []string{"research-run-0001", "research-run-0002", "research-run-0003"} {
		if _, inserted, err := store.SaveResearchHistory(t.Context(), researchHistoryRecord(runID, "2330.TWSE", t0.Add(time.Duration(index)*time.Minute))); err != nil || !inserted {
			t.Fatalf("save %s inserted=%v err=%v", runID, inserted, err)
		}
	}
	if _, inserted, err := store.SaveResearchHistory(t.Context(), researchHistoryRecord("research-run-other", "6488.TPEX", t0)); err != nil || !inserted {
		t.Fatalf("save other inserted=%v err=%v", inserted, err)
	}
	duplicate, inserted, err := store.SaveResearchHistory(t.Context(), researchHistoryRecord("research-run-0003", "2330.TWSE", t0.Add(time.Hour)))
	if err != nil || inserted || duplicate.CreatedAt != t0.Add(2*time.Minute) {
		t.Fatalf("duplicate=%+v inserted=%v err=%v", duplicate, inserted, err)
	}
	page, err := store.ListResearchHistory(t.Context(), "2330.TWSE", 2, 0)
	if err != nil || page.Total != 3 || len(page.Runs) != 2 || page.Runs[0].RunID != "research-run-0003" || !page.Runs[0].HasPrevious {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	secondPage, err := store.ListResearchHistory(t.Context(), "2330.TWSE", 2, 2)
	if err != nil || len(secondPage.Runs) != 1 || secondPage.Runs[0].RunID != "research-run-0001" || secondPage.Runs[0].HasPrevious {
		t.Fatalf("second page=%+v err=%v", secondPage, err)
	}
	previous, err := store.PreviousResearchHistory(t.Context(), "2330.TWSE", "research-run-0003")
	if err != nil || previous.RunID != "research-run-0002" {
		t.Fatalf("previous=%+v err=%v", previous, err)
	}
	if _, err := store.GetResearchHistory(t.Context(), "2330.TWSE", "research-run-other"); !errors.Is(err, ErrResearchHistoryNotFound) {
		t.Fatalf("cross-symbol get err=%v", err)
	}
	recent, err := store.ListRecentResearchHistory(t.Context(), 3)
	if err != nil || len(recent) != 3 || recent[0].RunID != "research-run-other" || recent[1].RunID != "research-run-0003" {
		t.Fatalf("recent=%+v err=%v", recent, err)
	}
	if _, err := store.ListRecentResearchHistory(t.Context(), 0); err == nil {
		t.Fatal("unbounded recent-history query was accepted")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	record, err := reopened.GetResearchHistory(t.Context(), "2330.TWSE", "research-run-0002")
	if err != nil || record.ModelName != "test-model" || string(record.EvidenceSnapshot) == "" {
		t.Fatalf("restarted record=%+v err=%v", record, err)
	}
}

func TestResearchHistoryOldSchemaMigrationPreservesExistingWatchlist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE taiwan_watchlist (canonical TEXT PRIMARY KEY, code TEXT NOT NULL, name TEXT NOT NULL, exchange TEXT NOT NULL, security_type TEXT NOT NULL, created_at TEXT NOT NULL);
		INSERT INTO taiwan_watchlist VALUES ('2330.TWSE','2330','台積電','TWSE','stock','2026-09-01T00:00:00Z');`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if entry, err := store.Get(t.Context(), "2330.TWSE"); err != nil || entry.Name != "台積電" {
		t.Fatalf("entry=%+v err=%v", entry, err)
	}
	if _, inserted, err := store.SaveResearchHistory(t.Context(), researchHistoryRecord("research-run-migrated", "2330.TWSE", time.Now().UTC())); err != nil || !inserted {
		t.Fatalf("migration save inserted=%v err=%v", inserted, err)
	}
}

func TestResearchHistoryRejectsInvalidOrOversizedSnapshots(t *testing.T) {
	store := openTestStore(t)
	record := researchHistoryRecord("research-run-invalid", "2330.TWSE", time.Now().UTC())
	record.EvidenceSnapshot = json.RawMessage(`not-json`)
	if _, _, err := store.SaveResearchHistory(t.Context(), record); err == nil {
		t.Fatal("invalid JSON was accepted")
	}
	record = researchHistoryRecord("research-run-oversized", "2330.TWSE", time.Now().UTC())
	record.EvidenceSnapshot = json.RawMessage(`"` + string(bytes.Repeat([]byte{'a'}, maxResearchEvidenceBytes)) + `"`)
	if _, _, err := store.SaveResearchHistory(t.Context(), record); err == nil {
		t.Fatal("oversized evidence was accepted")
	}
}

func TestResearchHistoryConcurrentDuplicateRunIsInsertedOnce(t *testing.T) {
	store := openTestStore(t)
	record := researchHistoryRecord("research-run-concurrent", "2330.TWSE", time.Now().UTC())
	var inserted atomic.Int32
	var wg sync.WaitGroup
	errorsSeen := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, created, err := store.SaveResearchHistory(t.Context(), record)
			if err != nil {
				errorsSeen <- err
				return
			}
			if created {
				inserted.Add(1)
			}
		}()
	}
	wg.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		t.Fatal(err)
	}
	page, err := store.ListResearchHistory(t.Context(), "2330.TWSE", 10, 0)
	if err != nil || inserted.Load() != 1 || page.Total != 1 {
		t.Fatalf("inserted=%d page=%+v err=%v", inserted.Load(), page, err)
	}
}
