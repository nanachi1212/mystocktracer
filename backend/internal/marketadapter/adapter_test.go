package marketadapter

import "testing"

func TestRegistryExposesTaiwanAdapter(t *testing.T) {
	registry, err := NewRegistry(TaiwanAdapter{})
	if err != nil {
		t.Fatal(err)
	}
	if got := registry.Markets(); len(got) != 1 || got[0] != "TW" {
		t.Fatalf("Markets() = %v", got)
	}
	tw, _ := registry.Adapter("TW")
	if got, err := tw.NormalizeSymbol("2330.twse"); err != nil || got.Canonical != "2330.TWSE" || got.Currency != "TWD" {
		t.Fatalf("TW normalization = %+v, %v", got, err)
	}
	if _, err := tw.NormalizeSymbol("2330"); err == nil {
		t.Fatal("bare Taiwan code must be resolved by the official directory")
	}
}
