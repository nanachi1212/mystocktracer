package sector

import (
	"reflect"
	"testing"

	"easy-stock/backend/internal/foundation"
)

func TestTaiwanIndustryRadarMetricsUniverseAndRelativeValues(t *testing.T) {
	identities := []foundation.SecurityIdentity{
		{Canonical: "2330.TWSE", Exchange: "TWSE", Type: foundation.SecurityTypeStock, Industry: "24"},
		{Canonical: "2303.TWSE", Exchange: "TWSE", Type: foundation.SecurityTypeStock, Industry: "24"},
		{Canonical: "2881.TWSE", Exchange: "TWSE", Type: foundation.SecurityTypeStock, Industry: "17"},
		{Canonical: "9999.TWSE", Exchange: "TWSE", Type: foundation.SecurityTypeStock, Industry: "99"},
		{Canonical: "0050.TWSE", Exchange: "TWSE", Type: foundation.SecurityTypeETF, Industry: "股票型"},
		{Canonical: "????.TWSE", Exchange: "TWSE", Type: "unknown", Industry: "24"},
		{Canonical: "6488.TPEX", Exchange: "TPEX", Type: foundation.SecurityTypeStock, Industry: "24"},
	}
	rows := []foundation.TaiwanDailySnapshot{
		row("2330.TWSE", "TWSE", 1, 100, 10, false), row("2303.TWSE", "TWSE", 1, 50, 10, false),
		row("2881.TWSE", "TWSE", -1, 100, 10, false), row("6488.TPEX", "TPEX", -1, 80, 10, false),
	}
	breadth := foundation.CalculateTaiwanMarketBreadth(identities, rows, freshness("current"), map[string]string{"TWSE": "current", "TPEX": "current"})
	got := CalculateTaiwanIndustryRadar(identities, rows, breadth)
	if got.TWSE.EligibleUniverseCount != 4 || got.TWSE.ClassifiedCount != 3 || got.TWSE.UnclassifiedCount != 1 || len(got.TWSE.Industries) != 2 {
		t.Fatalf("TWSE classification=%+v", got.TWSE)
	}
	if got.Combined.ClassifiedCount != 4 || got.Combined.UnclassifiedCount != 1 || len(got.Combined.Industries) != 3 {
		t.Fatalf("combined=%+v", got.Combined)
	}
	semi := findIndustry(t, got.TWSE, "TWSE:24")
	if semi.ConstituentCount != 2 || semi.Advancers != 2 || semi.Decliners != 0 || semi.TradedCount != 2 || semi.AdvanceRatio == nil || *semi.AdvanceRatio != 1 {
		t.Fatalf("semiconductor=%+v", semi)
	}
	if semi.RelativeBreadth == nil || *semi.RelativeBreadth <= 0 {
		t.Fatalf("expected stronger than market: %+v", semi)
	}
	if semi.BenchmarkScope != "TWSE" || findIndustry(t, got.Combined, "TWSE:24").BenchmarkScope != "COMBINED" {
		t.Fatalf("benchmark scopes are not explicit")
	}
	finance := findIndustry(t, got.TWSE, "TWSE:17")
	if finance.RelativeBreadth == nil || *finance.RelativeBreadth >= 0 {
		t.Fatalf("expected weaker than market: %+v", finance)
	}
	if semi.Advancers+semi.Decliners+semi.Unchanged+semi.NoTrade+semi.Unknown != semi.ConstituentCount {
		t.Fatal("industry conservation failed")
	}
}

func TestTaiwanIndustryRadarUnknownNoTradeMissingAndNullDenominators(t *testing.T) {
	identities := []foundation.SecurityIdentity{
		{Canonical: "A.TWSE", Exchange: "TWSE", Type: foundation.SecurityTypeStock, Industry: "24"},
		{Canonical: "B.TWSE", Exchange: "TWSE", Type: foundation.SecurityTypeStock, Industry: "24"},
		{Canonical: "C.TWSE", Exchange: "TWSE", Type: foundation.SecurityTypeStock, Industry: "24"},
	}
	zero := int64(0)
	rows := []foundation.TaiwanDailySnapshot{{Canonical: "A.TWSE", Exchange: "TWSE", NoTrade: true, Volume: &zero}, {Canonical: "B.TWSE", Exchange: "TWSE", Volume: i64(10)}}
	breadth := foundation.CalculateTaiwanMarketBreadth(identities, rows, freshness("current"), map[string]string{"TWSE": "current", "TPEX": "unavailable"})
	first := CalculateTaiwanIndustryRadar(identities, rows, breadth)
	second := CalculateTaiwanIndustryRadar(identities, rows, breadth)
	if !reflect.DeepEqual(first, second) {
		t.Fatal("calculator is not deterministic/idempotent")
	}
	item := findIndustry(t, first.TWSE, "TWSE:24")
	if item.NoTrade != 1 || item.Unknown != 2 || item.MissingAmountCount != 3 || item.AdvanceRatio != nil || item.AdvancingAmountRatio != nil {
		t.Fatalf("null/missing semantics=%+v", item)
	}
	if first.Combined.Status != "partial" || !reflect.DeepEqual(first.Combined.IncludedExchanges, []string{"TWSE"}) || !reflect.DeepEqual(first.Combined.MissingExchanges, []string{"TPEX"}) {
		t.Fatalf("partial=%+v", first.Combined)
	}
}

func TestTaiwanIndustryRadarStaleUnavailableTaxonomyAndEqualMarket(t *testing.T) {
	identities := []foundation.SecurityIdentity{{Canonical: "A.TWSE", Exchange: "TWSE", Type: foundation.SecurityTypeStock, Industry: "24"}, {Canonical: "B.TPEX", Exchange: "TPEX", Type: foundation.SecurityTypeStock, Industry: "99"}}
	rows := []foundation.TaiwanDailySnapshot{row("A.TWSE", "TWSE", 1, 10, 1, false)}
	breadth := foundation.CalculateTaiwanMarketBreadth(identities, rows, freshness("stale"), map[string]string{"TWSE": "stale", "TPEX": "unavailable"})
	got := CalculateTaiwanIndustryRadar(identities, rows, breadth)
	if got.TWSE.Freshness != "stale" || got.TWSE.SnapshotStatus != "stale" {
		t.Fatalf("stale not inherited: %+v", got.TWSE)
	}
	item := findIndustry(t, got.TWSE, "TWSE:24")
	if item.RelativeBreadth == nil || *item.RelativeBreadth != 0 || item.RelativeCapital == nil || *item.RelativeCapital != 0 {
		t.Fatalf("equal market=%+v", item)
	}
	if got.TPEX.Status != "unavailable" || got.TPEX.TaxonomyStatus != "unavailable" || got.TPEX.UnclassifiedCount != 1 {
		t.Fatalf("taxonomy unavailable=%+v", got.TPEX)
	}
	if got.TWSE.ClassificationBasis != "current_reference" {
		t.Fatal("PIT disclosure missing")
	}
}

func TestTaiwanIndustryRadarNullRelativeRankingAndStatusReasons(t *testing.T) {
	identities := []foundation.SecurityIdentity{
		{Canonical: "A.TWSE", Exchange: "TWSE", Type: foundation.SecurityTypeStock, Industry: "24"},
		{Canonical: "B.TWSE", Exchange: "TWSE", Type: foundation.SecurityTypeStock, Industry: "17"},
		{Canonical: "C.TWSE", Exchange: "TWSE", Type: foundation.SecurityTypeStock, Industry: "91"},
	}
	rows := []foundation.TaiwanDailySnapshot{{Canonical: "A.TWSE", Exchange: "TWSE", Volume: i64(10)}, {Canonical: "B.TWSE", Exchange: "TWSE", Volume: i64(10)}}
	breadth := foundation.CalculateTaiwanMarketBreadth(identities, rows, freshness("current"), map[string]string{"TWSE": "current", "TPEX": "unavailable"})
	got := CalculateTaiwanIndustryRadar(identities, rows, breadth)
	if got.TWSE.SnapshotStatus != "current" || got.TWSE.TaxonomyStatus != "partial" || got.TWSE.Status != "partial" {
		t.Fatalf("status reasons are conflated: %+v", got.TWSE)
	}
	if len(got.TWSE.Industries) != 2 || got.TWSE.Industries[0].IndustryID != "TWSE:17" || got.TWSE.Industries[1].IndustryID != "TWSE:24" {
		t.Fatalf("null ranking must be deterministic by industry id: %+v", got.TWSE.Industries)
	}
	for _, item := range got.TWSE.Industries {
		if item.RelativeBreadth != nil || item.RelativeCapital != nil {
			t.Fatalf("missing market/industry baseline must remain null: %+v", item)
		}
	}
}

func TestTaiwanOfficialMappingsCoverCurrentDirectoryCodeSets(t *testing.T) {
	for _, code := range []string{"01", "02", "03", "04", "05", "06", "08", "09", "10", "11", "12", "14", "15", "16", "17", "18", "20", "21", "22", "23", "24", "25", "26", "27", "28", "29", "30", "31", "35", "36", "37", "38"} {
		if _, ok := officialTaiwanIndustry("TWSE", code); !ok {
			t.Fatalf("TWSE live official code %s missing", code)
		}
	}
	for _, code := range []string{"02", "03", "04", "05", "06", "10", "14", "15", "16", "17", "20", "21", "22", "23", "24", "25", "26", "27", "28", "29", "30", "31", "32", "33", "35", "36", "37", "38"} {
		if _, ok := officialTaiwanIndustry("TPEX", code); !ok {
			t.Fatalf("TPEX live official code %s missing", code)
		}
	}
}

func TestOfficialTaiwanIndustryIdentityKeepsTaxonomiesSeparate(t *testing.T) {
	twse, ok1 := officialTaiwanIndustry("TWSE", "17")
	tpex, ok2 := officialTaiwanIndustry("TPEX", "17")
	if !ok1 || !ok2 || twse.name != "金融保險" || tpex.name != "金融業" || twse.source == tpex.source {
		t.Fatalf("taxonomy identities: %+v %+v", twse, tpex)
	}
	if _, ok := officialTaiwanIndustry("TWSE", "91"); ok {
		t.Fatal("unsupported code must not be guessed")
	}
}

func row(id, exchange string, change, amount float64, volume int64, noTrade bool) foundation.TaiwanDailySnapshot {
	return foundation.TaiwanDailySnapshot{Canonical: id, Exchange: exchange, Change: &change, Amount: &amount, Volume: &volume, NoTrade: noTrade}
}
func i64(v int64) *int64 { return &v }
func freshness(status string) foundation.TaiwanFreshness {
	date := "2026-08-31"
	return foundation.TaiwanFreshness{DailyAsOf: &date, TargetLatestTradingDate: date, DailyStatus: status}
}
func findIndustry(t *testing.T, scope TaiwanIndustryScope, id string) TaiwanIndustry {
	t.Helper()
	for _, item := range scope.Industries {
		if item.IndustryID == id {
			return item
		}
	}
	t.Fatalf("industry %s not found", id)
	return TaiwanIndustry{}
}
