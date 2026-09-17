package taiwanwatchlist

import (
	"path/filepath"
	"testing"
	"time"

	"easy-stock/backend/internal/foundation"
)

func addAlertTestSecurity(t *testing.T, store *Store, canonical, code, name, exchange string) {
	t.Helper()
	if _, err := store.Add(t.Context(), Entry{Canonical: canonical, Code: code, Name: name, Exchange: exchange, SecurityType: "stock"}); err != nil {
		t.Fatal(err)
	}
}

func alertEvent(id, symbol string, publishedAt time.Time) foundation.TaiwanCorporateEvent {
	return foundation.TaiwanCorporateEvent{
		ID: id, Symbol: symbol, Title: "公告 " + id, Category: "重大訊息", PublishedAt: &publishedAt,
		Provider: "provider-a", Source: "official-feed", SourceURL: "https://example.com/" + id,
		RetrievedAt: publishedAt.Add(time.Minute), Status: foundation.TaiwanCorporateEventsAvailable,
	}
}

func alertFeed(events ...foundation.TaiwanCorporateEvent) foundation.TaiwanCorporateEventFeed {
	return foundation.TaiwanCorporateEventFeed{Status: foundation.TaiwanCorporateEventsAvailable, Provider: "provider-a", Source: "official-feed", Events: events}
}

func TestAlertInboxBaselineNewDuplicateLateAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watchlist.db")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	addAlertTestSecurity(t, store, "2330.TWSE", "2330", "台積電", "TWSE")
	t0 := time.Date(2026, 9, 17, 1, 0, 0, 0, time.UTC)
	baselineTime := t0.Add(-time.Minute)
	if _, err := store.ApplyCorporateEventsWithPreference(t.Context(), "2330.TWSE", alertFeed(alertEvent("baseline", "2330.TWSE", baselineTime)), t0, true); err != nil {
		t.Fatal(err)
	}
	page, err := store.ListAlerts(t.Context(), AlertFilterAll, 50, 0)
	if err != nil || len(page.Alerts) != 0 || page.UnreadCount != 0 {
		t.Fatalf("baseline alerts = %+v, %v", page, err)
	}

	newTime := t0.Add(time.Minute)
	change, err := store.ApplyCorporateEventsWithPreference(t.Context(), "2330.TWSE", alertFeed(alertEvent("new", "2330.TWSE", newTime)), t0.Add(2*time.Minute), true)
	if err != nil || len(change.NewEvents) != 1 {
		t.Fatalf("new event change = %+v, %v", change, err)
	}
	if _, err := store.ApplyCorporateEventsWithPreference(t.Context(), "2330.TWSE", alertFeed(alertEvent("new", "2330.TWSE", newTime)), t0.Add(3*time.Minute), true); err != nil {
		t.Fatal(err)
	}
	lateTime := t0.Add(-time.Hour)
	if _, err := store.ApplyCorporateEventsWithPreference(t.Context(), "2330.TWSE", alertFeed(alertEvent("late", "2330.TWSE", lateTime)), t0.Add(4*time.Minute), true); err != nil {
		t.Fatal(err)
	}
	page, err = store.ListAlerts(t.Context(), AlertFilterAll, 50, 0)
	if err != nil || len(page.Alerts) != 1 || page.UnreadCount != 1 || page.Alerts[0].EventID != "new" || page.Alerts[0].SecurityName != "台積電" {
		t.Fatalf("deduped alerts = %+v, %v", page, err)
	}
	if ok, err := store.MarkAlertRead(t.Context(), page.Alerts[0].ID, t0.Add(5*time.Minute)); err != nil || !ok {
		t.Fatalf("mark read = %v, %v", ok, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	page, err = store.ListAlerts(t.Context(), AlertFilterRead, 50, 0)
	if err != nil || len(page.Alerts) != 1 || !page.Alerts[0].Read || page.UnreadCount != 0 {
		t.Fatalf("restarted alerts = %+v, %v", page, err)
	}
}

func TestAlertPreferenceDisabledDoesNotBackfillOnReenable(t *testing.T) {
	store := openTestStore(t)
	addAlertTestSecurity(t, store, "2330.TWSE", "2330", "台積電", "TWSE")
	t0 := time.Date(2026, 9, 17, 2, 0, 0, 0, time.UTC)
	baselineTime := t0.Add(-time.Minute)
	if _, err := store.ApplyCorporateEventsWithPreference(t.Context(), "2330.TWSE", alertFeed(alertEvent("baseline", "2330.TWSE", baselineTime)), t0, true); err != nil {
		t.Fatal(err)
	}
	disabledTime := t0.Add(time.Minute)
	if _, err := store.ApplyCorporateEventsWithPreference(t.Context(), "2330.TWSE", alertFeed(alertEvent("disabled", "2330.TWSE", disabledTime)), t0.Add(2*time.Minute), false); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ApplyCorporateEventsWithPreference(t.Context(), "2330.TWSE", alertFeed(alertEvent("disabled", "2330.TWSE", disabledTime)), t0.Add(3*time.Minute), true); err != nil {
		t.Fatal(err)
	}
	enabledTime := t0.Add(4 * time.Minute)
	if _, err := store.ApplyCorporateEventsWithPreference(t.Context(), "2330.TWSE", alertFeed(alertEvent("enabled", "2330.TWSE", enabledTime)), t0.Add(5*time.Minute), true); err != nil {
		t.Fatal(err)
	}
	page, err := store.ListAlerts(t.Context(), AlertFilterAll, 50, 0)
	if err != nil || len(page.Alerts) != 1 || page.Alerts[0].EventID != "enabled" {
		t.Fatalf("re-enabled alerts = %+v, %v", page, err)
	}
}

func TestAlertsAreIsolatedBySymbolAndMarkAllRead(t *testing.T) {
	store := openTestStore(t)
	addAlertTestSecurity(t, store, "2330.TWSE", "2330", "台積電", "TWSE")
	addAlertTestSecurity(t, store, "6488.TPEX", "6488", "環球晶", "TPEX")
	t0 := time.Date(2026, 9, 17, 3, 0, 0, 0, time.UTC)
	for _, symbol := range []string{"2330.TWSE", "6488.TPEX"} {
		baselineTime := t0.Add(-time.Minute)
		if _, err := store.ApplyCorporateEventsWithPreference(t.Context(), symbol, alertFeed(alertEvent("baseline-"+symbol, symbol, baselineTime)), t0, true); err != nil {
			t.Fatal(err)
		}
		newTime := t0.Add(time.Minute)
		if _, err := store.ApplyCorporateEventsWithPreference(t.Context(), symbol, alertFeed(alertEvent("new-"+symbol, symbol, newTime)), t0.Add(2*time.Minute), true); err != nil {
			t.Fatal(err)
		}
	}
	page, err := store.ListAlerts(t.Context(), AlertFilterUnread, 1, 0)
	if err != nil || page.Total != 2 || page.UnreadCount != 2 || len(page.Alerts) != 1 {
		t.Fatalf("paginated alerts = %+v, %v", page, err)
	}
	updated, err := store.MarkAllAlertsRead(t.Context(), t0.Add(3*time.Minute))
	if err != nil || updated != 2 {
		t.Fatalf("mark all read = %d, %v", updated, err)
	}
	page, err = store.ListAlerts(t.Context(), AlertFilterUnread, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 0 || page.UnreadCount != 0 {
		t.Fatalf("unread after mark all = %+v", page)
	}
}

func TestPartialStaleAndUnavailableFeedsCreateNoAlerts(t *testing.T) {
	store := openTestStore(t)
	addAlertTestSecurity(t, store, "2330.TWSE", "2330", "台積電", "TWSE")
	t0 := time.Date(2026, 9, 17, 4, 0, 0, 0, time.UTC)
	baselineTime := t0.Add(-time.Minute)
	if _, err := store.ApplyCorporateEventsWithPreference(t.Context(), "2330.TWSE", alertFeed(alertEvent("baseline", "2330.TWSE", baselineTime)), t0, true); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{foundation.TaiwanCorporateEventsPartial, foundation.TaiwanCorporateEventsStale, foundation.TaiwanCorporateEventsUnavailable} {
		candidateTime := t0.Add(time.Minute)
		feed := alertFeed(alertEvent(status, "2330.TWSE", candidateTime))
		feed.Status = status
		if _, err := store.ApplyCorporateEventsWithPreference(t.Context(), "2330.TWSE", feed, t0.Add(2*time.Minute), true); err != nil {
			t.Fatal(err)
		}
	}
	page, err := store.ListAlerts(t.Context(), AlertFilterAll, 50, 0)
	if err != nil || len(page.Alerts) != 0 {
		t.Fatalf("failure-state alerts = %+v, %v", page, err)
	}
	recoveredTime := t0.Add(3 * time.Minute)
	if _, err := store.ApplyCorporateEventsWithPreference(t.Context(), "2330.TWSE", alertFeed(alertEvent("recovered", "2330.TWSE", recoveredTime)), t0.Add(4*time.Minute), true); err != nil {
		t.Fatal(err)
	}
	page, err = store.ListAlerts(t.Context(), AlertFilterAll, 50, 0)
	if err != nil || len(page.Alerts) != 1 || page.Alerts[0].EventID != "recovered" {
		t.Fatalf("recovered alerts = %+v, %v", page, err)
	}
}
