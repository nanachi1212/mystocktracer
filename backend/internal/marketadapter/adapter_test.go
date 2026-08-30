package marketadapter

import "testing"

func TestRegistryKeepsChinaAndTaiwanAdaptersSeparate(t *testing.T) {
	registry, err := NewRegistry(ChinaAdapter{}, TaiwanAdapter{})
	if err != nil {
		t.Fatal(err)
	}
	if got := registry.Markets(); len(got) != 2 || got[0] != "CN" || got[1] != "TW" {
		t.Fatalf("Markets() = %v", got)
	}
	cn, _ := registry.Adapter("cn")
	if got, err := cn.NormalizeSymbol("600000"); err != nil || got.Canonical != "600000.SH" {
		t.Fatalf("CN normalization = %+v, %v", got, err)
	}
	tw, _ := registry.Adapter("TW")
	if got, err := tw.NormalizeSymbol("2330.twse"); err != nil || got.Canonical != "2330.TWSE" || got.Currency != "TWD" {
		t.Fatalf("TW normalization = %+v, %v", got, err)
	}
	if _, err := tw.NormalizeSymbol("2330"); err == nil {
		t.Fatal("bare Taiwan code must be resolved by the official directory")
	}
}
