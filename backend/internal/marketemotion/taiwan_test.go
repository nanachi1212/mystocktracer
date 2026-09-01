package marketemotion

import (
	"reflect"
	"testing"

	"easy-stock/backend/internal/foundation"
)

func emotionRatio(value float64) *float64 { return &value }
func emotionDate(value string) *string    { return &value }

func emotionBreadth(scope string, advanceRatio, amountRatio *float64) foundation.TaiwanBreadthScope {
	return foundation.TaiwanBreadthScope{Scope: scope, Status: "current", Freshness: "current", AsOf: emotionDate("2026-08-31"), TargetLatestTradingDate: "2026-08-31", Universe: "stock_breadth", UniverseCount: 100, TradedCount: 98, Advancers: 60, Decliners: 30, Unchanged: 8, NoTrade: 1, Unknown: 1, AdvanceDeclineDiff: 30, AdvanceRatio: advanceRatio, TotalAmountTWD: 1000, AdvancingAmountRatio: amountRatio, MissingAmountCount: 1, LimitUnknownCount: 100, IncludedExchanges: []string{scope}, ClassificationBasis: "current_reference"}
}

func TestTaiwanEmotionDirectionAndDivergence(t *testing.T) {
	tests := []struct {
		name, wantState, wantBreadth, wantCapital, wantRelationship string
		breadthRatio, amountRatio                                   *float64
	}{
		{"broad strong market", "positive", "positive", "positive", "aligned_positive", emotionRatio(.70), emotionRatio(.65)},
		{"broad weak market", "weak", "negative", "negative", "aligned_negative", emotionRatio(.30), emotionRatio(.35)},
		{"breadth positive amount negative divergence", "mixed", "positive", "negative", "breadth_positive_capital_negative", emotionRatio(.60), emotionRatio(.40)},
		{"breadth negative amount positive divergence", "mixed", "negative", "positive", "breadth_negative_capital_positive", emotionRatio(.40), emotionRatio(.60)},
		{"balanced inputs", "mixed", "balanced", "balanced", "mixed", emotionRatio(.50), emotionRatio(.50)},
		{"missing advance ratio", "indeterminate", "unavailable", "positive", "indeterminate", nil, emotionRatio(.60)},
		{"missing amount ratio", "indeterminate", "positive", "unavailable", "indeterminate", emotionRatio(.60), nil},
		{"zero denominator", "indeterminate", "unavailable", "unavailable", "indeterminate", nil, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := calculateTaiwanEmotionScope(emotionBreadth("TWSE", test.breadthRatio, test.amountRatio))
			if got.State != test.wantState || got.Components.BreadthParticipation != test.wantBreadth || got.Components.CapitalParticipation != test.wantCapital || got.Components.BreadthCapitalRelationship != test.wantRelationship {
				t.Fatalf("got=%+v", got)
			}
		})
	}
}

func TestTaiwanEmotionStateBoundariesAreMathematicalMajorities(t *testing.T) {
	for _, test := range []struct {
		name, want      string
		breadth, amount float64
	}{
		{"both just above half", "positive", .500001, .500001}, {"both exactly half", "mixed", .5, .5}, {"both just below half", "weak", .499999, .499999}, {"one exactly half", "mixed", .5, .7},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := calculateTaiwanEmotionScope(emotionBreadth("TWSE", emotionRatio(test.breadth), emotionRatio(test.amount)))
			if got.State != test.want {
				t.Fatalf("state=%s want=%s", got.State, test.want)
			}
		})
	}
}

func TestTaiwanEmotionConfidenceCoveragePolicy(t *testing.T) {
	high := emotionBreadth("TWSE", emotionRatio(.6), emotionRatio(.4))
	if got := calculateTaiwanEmotionScope(high); got.Confidence != "high" || got.State != "mixed" || got.Coverage.DirectionCoverage == nil || *got.Coverage.DirectionCoverage != .98 || got.Coverage.AmountCoverage == nil || *got.Coverage.AmountCoverage != .99 {
		t.Fatalf("high=%+v", got)
	}
	medium := high
	medium.AdvanceRatio, medium.AdvancingAmountRatio = emotionRatio(.6), emotionRatio(.6)
	medium.Unknown, medium.NoTrade, medium.Unchanged, medium.Advancers, medium.Decliners, medium.MissingAmountCount = 10, 5, 5, 45, 35, 15
	if got := calculateTaiwanEmotionScope(medium); got.Confidence != "medium" || got.State != "positive" {
		t.Fatalf("medium=%+v", got)
	}
	low := high
	low.AdvanceRatio, low.AdvancingAmountRatio = emotionRatio(.4), emotionRatio(.4)
	low.Unknown, low.NoTrade, low.Unchanged, low.Advancers, low.Decliners, low.MissingAmountCount = 25, 5, 5, 35, 30, 25
	if got := calculateTaiwanEmotionScope(low); got.Confidence != "low" || got.State != "weak" {
		t.Fatalf("low=%+v", got)
	}
	stale := high
	stale.Status, stale.Freshness = "stale", "stale"
	if got := calculateTaiwanEmotionScope(stale); got.Confidence != "medium" || got.Freshness != "stale" {
		t.Fatalf("stale=%+v", got)
	}
}

func TestTaiwanEmotionScopesFreshnessPartialAndUnavailable(t *testing.T) {
	twse := emotionBreadth("TWSE", emotionRatio(.6), emotionRatio(.7))
	tpex := emotionBreadth("TPEX", nil, nil)
	tpex.Status, tpex.Freshness, tpex.IncludedExchanges, tpex.MissingExchanges = "unavailable", "unavailable", nil, []string{"TPEX"}
	combined := twse
	combined.Scope, combined.Status, combined.Freshness, combined.IncludedExchanges, combined.MissingExchanges = "COMBINED", "partial", "partial", []string{"TWSE"}, []string{"TPEX"}
	got := CalculateTaiwanMarketEmotion(foundation.TaiwanMarketBreadth{TWSE: twse, TPEX: tpex, Combined: combined})
	if got.TWSE.Scope != "TWSE" || got.TPEX.Scope != "TPEX" || got.Combined.Scope != "COMBINED" || got.TPEX.State != "unavailable" || got.TPEX.Confidence != "low" || got.Combined.Status != "partial" || got.Combined.Freshness != "partial" || got.Combined.Confidence != "medium" {
		t.Fatalf("got=%+v", got)
	}
	if !reflect.DeepEqual(got.Combined.IncludedExchanges, []string{"TWSE"}) || !reflect.DeepEqual(got.Combined.MissingExchanges, []string{"TPEX"}) {
		t.Fatalf("exchanges=%+v", got.Combined)
	}
}

func TestTaiwanEmotionPreservesRawFactsAndPITDisclosure(t *testing.T) {
	scope := emotionBreadth("COMBINED", emotionRatio(.625), emotionRatio(.55))
	scope.Advancers, scope.Decliners, scope.Unchanged, scope.NoTrade, scope.Unknown = 50, 30, 10, 5, 5
	scope.AdvanceDeclineDiff, scope.TradedCount, scope.TotalAmountTWD, scope.MissingAmountCount, scope.TargetLatestTradingDate = 20, 90, 12345, 7, "2026-09-01"
	before := scope
	one, two := calculateTaiwanEmotionScope(scope), calculateTaiwanEmotionScope(scope)
	if !reflect.DeepEqual(one, two) || !reflect.DeepEqual(scope, before) {
		t.Fatal("calculator must be deterministic and must not mutate M2A input")
	}
	if one.Raw.AdvanceRatio != scope.AdvanceRatio || one.Raw.AdvancingAmountRatio != scope.AdvancingAmountRatio || one.Raw.AdvanceDeclineDiff != 20 || one.Raw.Advancers != 50 || one.Raw.Decliners != 30 || one.Raw.Unknown != 5 || one.Raw.UniverseCount != 100 || one.Raw.MissingAmountCount != 7 {
		t.Fatalf("raw=%+v breadth=%+v", one.Raw, scope)
	}
	if one.AsOf == nil || *one.AsOf != "2026-08-31" || one.TargetLatestTradingDate != "2026-09-01" || one.ClassificationBasis != "current_reference" || one.ModelVersion != TaiwanEmotionModelVersion {
		t.Fatalf("disclosure=%+v", one)
	}
}

func TestTaiwanEmotionDoesNotRequireLimitMetrics(t *testing.T) {
	scope := emotionBreadth("TWSE", emotionRatio(.6), emotionRatio(.6))
	scope.LimitUnknownCount = scope.UniverseCount
	got := calculateTaiwanEmotionScope(scope)
	if got.State != "positive" || got.Coverage.LimitRuleCoverage == nil || *got.Coverage.LimitRuleCoverage != 0 {
		t.Fatalf("got=%+v", got)
	}
	empty := calculateTaiwanEmotionScope(foundation.TaiwanBreadthScope{Scope: "TWSE", Status: "current", Freshness: "current"})
	if empty.Coverage.DirectionCoverage != nil || empty.Coverage.AmountCoverage != nil || empty.Coverage.LimitRuleCoverage != nil {
		t.Fatalf("zero universe coverage must be null: %+v", empty.Coverage)
	}
}
