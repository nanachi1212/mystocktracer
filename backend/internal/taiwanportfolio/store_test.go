package taiwanportfolio

import (
	"context"
	"database/sql"
	"math"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestStoreUpsertDeleteAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "portfolio.db")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	first, existed, err := store.Upsert(ctx, Holding{Canonical: "2330.TWSE", DisplayName: "台積電", Shares: 1000, AverageCost: 900, Note: "長期"})
	if err != nil || existed {
		t.Fatalf("first upsert: existed=%v err=%v", existed, err)
	}
	updated, existed, err := store.Upsert(ctx, Holding{Canonical: "2330.twse", DisplayName: "台積電", Shares: 1200, AverageCost: 910})
	if err != nil || !existed {
		t.Fatalf("duplicate upsert: existed=%v err=%v", existed, err)
	}
	if updated.CreatedAt != first.CreatedAt || updated.Shares != 1200 {
		t.Fatalf("unexpected update: %+v", updated)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	items, err := store.List(ctx)
	if err != nil || len(items) != 1 || items[0].Canonical != "2330.TWSE" || items[0].Shares != 1200 {
		t.Fatalf("restart list: %+v err=%v", items, err)
	}
	deleted, err := store.Delete(ctx, "2330.TWSE")
	if err != nil || !deleted {
		t.Fatalf("delete: deleted=%v err=%v", deleted, err)
	}
	deleted, err = store.Delete(ctx, "2330.TWSE")
	if err != nil || deleted {
		t.Fatalf("delete absent: deleted=%v err=%v", deleted, err)
	}
}

func TestStoreRejectsInvalidNumericValues(t *testing.T) {
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, holding := range []Holding{
		{Canonical: "2330.TWSE", Shares: 0, AverageCost: 1},
		{Canonical: "2330.TWSE", Shares: math.NaN(), AverageCost: 1},
		{Canonical: "2330.TWSE", Shares: 1, AverageCost: -1},
		{Canonical: "2330.TWSE", Shares: 1, AverageCost: math.Inf(1)},
	} {
		if _, _, err := store.Upsert(context.Background(), holding); err == nil {
			t.Fatalf("expected rejection: %+v", holding)
		}
	}
	items, _ := store.List(context.Background())
	if len(items) != 0 {
		t.Fatalf("invalid rows were persisted: %+v", items)
	}
}

func TestStoreMigratesLegacySchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE taiwan_portfolio_holdings (canonical_symbol TEXT PRIMARY KEY, shares REAL NOT NULL, average_cost REAL NOT NULL); INSERT INTO taiwan_portfolio_holdings VALUES ('6488.TPEX', 200, 350)`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	items, err := store.List(context.Background())
	if err != nil || len(items) != 1 || items[0].Canonical != "6488.TPEX" || items[0].CreatedAt.IsZero() || items[0].UpdatedAt.IsZero() {
		t.Fatalf("migration result: %+v err=%v", items, err)
	}
}
