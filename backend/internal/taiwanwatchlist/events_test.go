package taiwanwatchlist

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"easy-stock/backend/internal/foundation"
)

func TestOldSchemaMigrationPreservesWatchlistAndEventStateAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watchlist.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE taiwan_watchlist (canonical TEXT PRIMARY KEY, code TEXT NOT NULL, name TEXT NOT NULL, exchange TEXT NOT NULL, security_type TEXT NOT NULL, created_at TEXT NOT NULL);
		CREATE INDEX taiwan_watchlist_created ON taiwan_watchlist(created_at ASC);
		INSERT INTO taiwan_watchlist VALUES ('2330.TWSE','2330','台積電','TWSE','stock','2026-09-01T00:00:00Z');`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if entry, err := store.Get(t.Context(), "2330.TWSE"); err != nil || entry.Name != "台積電" {
		t.Fatalf("preserved entry = %+v, %v", entry, err)
	}
	t0 := time.Date(2026, 9, 16, 1, 0, 0, 0, time.UTC)
	published := t0.Add(-time.Minute)
	if _, err := store.ApplyCorporateEvents(t.Context(), "2330.TWSE", eventFeed(event("baseline", &published)), t0); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if entry, err := store.Get(t.Context(), "2330.TWSE"); err != nil || entry.Name != "台積電" {
		t.Fatalf("restarted entry = %+v, %v", entry, err)
	}
	result, err := store.ApplyCorporateEvents(t.Context(), "2330.TWSE", eventFeed(event("baseline", &published)), t0.Add(time.Hour))
	if err != nil || result.Baseline || len(result.NewEvents) != 0 {
		t.Fatalf("restarted state = %+v, %v", result, err)
	}
	newTime := t0.Add(2 * time.Hour)
	if _, err := store.ApplyCorporateEvents(t.Context(), "2330.TWSE", eventFeed(event("new-after-migration", &newTime)), t0.Add(3*time.Hour)); err != nil {
		t.Fatal(err)
	}
	page, err := store.ListAlerts(t.Context(), AlertFilterAll, 10, 0)
	if err != nil || len(page.Alerts) != 1 || page.Alerts[0].SecurityName != "台積電" {
		t.Fatalf("migrated alert inbox = %+v, %v", page, err)
	}
}

func TestCorporateEventChangeDetectionAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watchlist.db")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 9, 16, 1, 0, 0, 0, time.UTC)
	old := t0.Add(-time.Hour)
	feed := eventFeed(event("old", &old))
	first, err := store.ApplyCorporateEvents(t.Context(), "2330.TWSE", feed, t0)
	if err != nil || !first.Baseline || len(first.NewEvents) != 0 {
		t.Fatalf("first observation = %+v, %v", first, err)
	}
	duplicate, err := store.ApplyCorporateEvents(t.Context(), "2330.TWSE", feed, t0.Add(time.Minute))
	if err != nil || duplicate.Baseline || len(duplicate.NewEvents) != 0 {
		t.Fatalf("duplicate observation = %+v, %v", duplicate, err)
	}
	newTime := t0.Add(2 * time.Minute)
	lateTime := t0.Add(-2 * time.Hour)
	changed, err := store.ApplyCorporateEvents(t.Context(), "2330.TWSE", eventFeed(event("old", &old), event("new", &newTime), event("late", &lateTime)), t0.Add(3*time.Minute))
	if err != nil || len(changed.NewEvents) != 1 || changed.NewEvents[0].ID != "new" {
		t.Fatalf("changed observation = %+v, %v", changed, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	restarted, err := store.ApplyCorporateEvents(t.Context(), "2330.TWSE", eventFeed(event("new", &newTime)), t0.Add(4*time.Minute))
	if err != nil || restarted.Baseline || len(restarted.NewEvents) != 0 {
		t.Fatalf("restart observation = %+v, %v", restarted, err)
	}
}

func TestCorporateEventProviderOutageDoesNotAdvanceState(t *testing.T) {
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	t0 := time.Date(2026, 9, 16, 1, 0, 0, 0, time.UTC)
	firstEvent := t0.Add(-time.Minute)
	if _, err := store.ApplyCorporateEvents(t.Context(), "2330.TWSE", eventFeed(event("baseline", &firstEvent)), t0); err != nil {
		t.Fatal(err)
	}
	outage := foundation.TaiwanCorporateEventFeed{Status: foundation.TaiwanCorporateEventsUnavailable, Provider: "toalpha", Events: []foundation.TaiwanCorporateEvent{}}
	state, err := store.ApplyCorporateEvents(t.Context(), "2330.TWSE", outage, t0.Add(time.Hour))
	if err != nil || state.LastSuccessfulSync == nil || !state.LastSuccessfulSync.Equal(t0) {
		t.Fatalf("outage state = %+v, %v", state, err)
	}
	recoveryTime := t0.Add(30 * time.Minute)
	recovered, err := store.ApplyCorporateEvents(t.Context(), "2330.TWSE", eventFeed(event("recovered", &recoveryTime)), t0.Add(2*time.Hour))
	if err != nil || len(recovered.NewEvents) != 1 {
		t.Fatalf("recovery state = %+v, %v", recovered, err)
	}
}

func TestCorporateEventPartialDoesNotAdvanceState(t *testing.T) {
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	t0 := time.Date(2026, 9, 16, 1, 0, 0, 0, time.UTC)
	baselineTime := t0.Add(-time.Minute)
	if _, err := store.ApplyCorporateEvents(t.Context(), "2330.TWSE", eventFeed(event("baseline", &baselineTime)), t0); err != nil {
		t.Fatal(err)
	}
	partialTime := t0.Add(10 * time.Minute)
	partial := eventFeed(event("partial-event", &partialTime))
	partial.Status = foundation.TaiwanCorporateEventsPartial
	state, err := store.ApplyCorporateEvents(t.Context(), "2330.TWSE", partial, t0.Add(20*time.Minute))
	if err != nil || state.LastSuccessfulSync == nil || !state.LastSuccessfulSync.Equal(t0) {
		t.Fatalf("partial state = %+v, %v", state, err)
	}
	recovered, err := store.ApplyCorporateEvents(t.Context(), "2330.TWSE", eventFeed(event("partial-event", &partialTime)), t0.Add(30*time.Minute))
	if err != nil || len(recovered.NewEvents) != 1 {
		t.Fatalf("recovered state = %+v, %v", recovered, err)
	}
}

func eventFeed(events ...foundation.TaiwanCorporateEvent) foundation.TaiwanCorporateEventFeed {
	return foundation.TaiwanCorporateEventFeed{Status: foundation.TaiwanCorporateEventsAvailable, Provider: "toalpha", Events: events}
}

func event(id string, publishedAt *time.Time) foundation.TaiwanCorporateEvent {
	return foundation.TaiwanCorporateEvent{ID: id, Symbol: "2330.TWSE", Title: id, PublishedAt: publishedAt, Provider: "toalpha"}
}
