package taiwanwatchlist

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestStoreListsEmptyByDefault(t *testing.T) {
	store := openTestStore(t)
	entries, err := store.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty watchlist, got %+v", entries)
	}
}

func TestStoreAddPersistsEntry(t *testing.T) {
	store := openTestStore(t)
	saved, err := store.Add(context.Background(), Entry{Canonical: "2330.TWSE", Code: "2330", Name: "台積電", Exchange: "TWSE", SecurityType: "stock"})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Canonical != "2330.TWSE" || saved.Code != "2330" || saved.Name != "台積電" || saved.Exchange != "TWSE" || saved.SecurityType != "stock" {
		t.Fatalf("unexpected saved entry: %+v", saved)
	}
	if saved.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be set")
	}
}

func TestStoreReadsBackAfterAdd(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.Add(context.Background(), Entry{Canonical: "2330.TWSE", Code: "2330", Name: "台積電", Exchange: "TWSE", SecurityType: "stock"}); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(context.Background(), "2330.TWSE")
	if err != nil {
		t.Fatal(err)
	}
	if got.Canonical != "2330.TWSE" || got.Name != "台積電" {
		t.Fatalf("unexpected entry read back: %+v", got)
	}
}

func TestStoreAddIsIdempotentForSameCanonical(t *testing.T) {
	store := openTestStore(t)
	first, err := store.Add(context.Background(), Entry{Canonical: "2330.TWSE", Code: "2330", Name: "台積電", Exchange: "TWSE", SecurityType: "stock"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Add(context.Background(), Entry{Canonical: "2330.twse", Code: "2330", Name: "台積電", Exchange: "TWSE", SecurityType: "stock"})
	if err != nil {
		t.Fatal(err)
	}
	if !first.CreatedAt.Equal(second.CreatedAt) {
		t.Fatalf("expected the original CreatedAt to be kept, first=%v second=%v", first.CreatedAt, second.CreatedAt)
	}
	entries, err := store.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly one row after duplicate add, got %+v", entries)
	}
}

func TestStoreListOrdersByCreatedAtThenCanonical(t *testing.T) {
	store := openTestStore(t)
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if _, err := store.Add(context.Background(), Entry{Canonical: "6488.TPEX", Code: "6488", Name: "環球晶", Exchange: "TPEX", SecurityType: "stock", CreatedAt: base.Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Add(context.Background(), Entry{Canonical: "2330.TWSE", Code: "2330", Name: "台積電", Exchange: "TWSE", SecurityType: "stock", CreatedAt: base}); err != nil {
		t.Fatal(err)
	}
	entries, err := store.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Canonical != "2330.TWSE" || entries[1].Canonical != "6488.TPEX" {
		t.Fatalf("expected 2330.TWSE (earlier created_at) before 6488.TPEX, got %+v", entries)
	}
}

func TestStoreRemoveDeletesExisting(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.Add(context.Background(), Entry{Canonical: "2330.TWSE", Code: "2330", Name: "台積電", Exchange: "TWSE", SecurityType: "stock"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Remove(context.Background(), "2330.TWSE"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(context.Background(), "2330.TWSE"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows after remove, got %v", err)
	}
}

func TestStoreRemoveAbsentSymbolIsSafe(t *testing.T) {
	store := openTestStore(t)
	if err := store.Remove(context.Background(), "9999.TWSE"); err != nil {
		t.Fatalf("expected removing an absent symbol to be a safe no-op, got %v", err)
	}
}

func TestStoreCanonicalSymbolIsUniqueAcrossExchanges(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.Add(context.Background(), Entry{Canonical: "2330.TWSE", Code: "2330", Name: "台積電", Exchange: "TWSE", SecurityType: "stock"}); err != nil {
		t.Fatal(err)
	}
	// A different exchange with the same code is a different canonical identity —
	// it must be able to coexist, proving code alone is not the identity key.
	if _, err := store.Add(context.Background(), Entry{Canonical: "2330.TPEX", Code: "2330", Name: "假設同代碼上櫃證券", Exchange: "TPEX", SecurityType: "stock"}); err != nil {
		t.Fatal(err)
	}
	entries, err := store.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2330.TWSE and 2330.TPEX to coexist as distinct entries, got %+v", entries)
	}
}

func TestStorePersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "taiwan-watchlist.db")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Add(context.Background(), Entry{Canonical: "6488.TPEX", Code: "6488", Name: "環球晶", Exchange: "TPEX", SecurityType: "stock"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close(); _ = os.Remove(path) })
	entries, err := reopened.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Canonical != "6488.TPEX" {
		t.Fatalf("expected 6488.TPEX to survive reopen, got %+v", entries)
	}
}
