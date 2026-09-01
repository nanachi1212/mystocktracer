package foundation

import (
	"reflect"
	"testing"
)

func breadthFloat(value float64) *float64 { return &value }
func breadthInt(value int64) *int64       { return &value }

func breadthIdentity(code, exchange string, securityType SecurityType) SecurityIdentity {
	identity := SecurityIdentity{Canonical: code + "." + exchange, Code: code, Exchange: exchange, Type: securityType, SourceURL: "https://official.test"}
	profile := TaiwanRuleProfile(identity)
	identity.RuleProfile = &profile
	return identity
}

func breadthRow(identity SecurityIdentity, change *float64, volume *int64, amount float64) TaiwanDailySnapshot {
	return TaiwanDailySnapshot{Canonical: identity.Canonical, Exchange: identity.Exchange, Type: identity.Type, TradeDate: "2026-08-31", Close: breadthFloat(100), Change: change, Volume: volume, Amount: breadthFloat(amount)}
}

func TestTaiwanMarketBreadthDeterministicSemantics(t *testing.T) {
	twseA := breadthIdentity("1101", "TWSE", SecurityTypeStock)
	twseB := breadthIdentity("1102", "TWSE", SecurityTypeStock)
	tpexA := breadthIdentity("6488", "TPEX", SecurityTypeStock)
	etf := breadthIdentity("0050", "TWSE", SecurityTypeETF)
	unknownType := breadthIdentity("9999", "TWSE", SecurityType("unknown"))
	identities := []SecurityIdentity{twseA, twseB, tpexA, etf, unknownType}
	fresh := TaiwanFreshness{DailyAsOf: ptrBreadth("2026-08-31"), TargetLatestTradingDate: "2026-08-31", DailyStatus: "current", DailyDaysBehind: intBreadth(0)}

	t.Run("all advancers and ETF excluded", func(t *testing.T) {
		rows := []TaiwanDailySnapshot{breadthRow(twseA, breadthFloat(1), breadthInt(10), 100), breadthRow(twseB, breadthFloat(2), breadthInt(20), 200), breadthRow(etf, breadthFloat(1), breadthInt(30), 300)}
		got := CalculateTaiwanMarketBreadth(identities, rows, fresh, map[string]string{"TWSE": "current", "TPEX": "unavailable"}).TWSE
		if got.UniverseCount != 2 || got.Advancers != 2 || got.TotalAmountTWD != 300 {
			t.Fatalf("got=%+v", got)
		}
	})

	t.Run("all decliners", func(t *testing.T) {
		rows := []TaiwanDailySnapshot{breadthRow(twseA, breadthFloat(-1), breadthInt(10), 100), breadthRow(twseB, breadthFloat(-2), breadthInt(20), 200)}
		got := CalculateTaiwanMarketBreadth(identities, rows, fresh, map[string]string{"TWSE": "current", "TPEX": "unavailable"}).TWSE
		if got.Decliners != 2 || got.AdvanceDeclineDiff != -2 || got.AdvanceRatio == nil || *got.AdvanceRatio != 0 {
			t.Fatalf("got=%+v", got)
		}
	})

	t.Run("mixed explicit zero missing no trade malformed and amounts", func(t *testing.T) {
		zero := breadthFloat(0)
		rows := []TaiwanDailySnapshot{breadthRow(twseA, zero, breadthInt(10), 100), breadthRow(twseB, nil, breadthInt(0), 0)}
		got := CalculateTaiwanMarketBreadth(identities, rows, fresh, map[string]string{"TWSE": "current", "TPEX": "unavailable"}).TWSE
		if got.Unchanged != 1 || got.NoTrade != 1 || got.Unknown != 0 || got.TradedCount != 1 || got.UnchangedAmountTWD != 100 {
			t.Fatalf("got=%+v", got)
		}
	})

	t.Run("malformed unknown and zero denominator null", func(t *testing.T) {
		rows := []TaiwanDailySnapshot{breadthRow(twseA, nil, breadthInt(10), 25)}
		got := CalculateTaiwanMarketBreadth([]SecurityIdentity{twseA}, rows, fresh, map[string]string{"TWSE": "current", "TPEX": "unavailable"}).TWSE
		if got.Unknown != 1 || got.TradedCount != 1 || got.AdvanceRatio != nil || got.AdvancingAmountRatio != nil || got.UnknownAmountTWD != 25 {
			t.Fatalf("got=%+v", got)
		}
	})

	t.Run("TWSE TPEX and combined recompute raw counts", func(t *testing.T) {
		rows := []TaiwanDailySnapshot{breadthRow(twseA, breadthFloat(1), breadthInt(10), 100), breadthRow(twseB, breadthFloat(-1), breadthInt(10), 300), breadthRow(tpexA, breadthFloat(1), breadthInt(10), 600)}
		got := CalculateTaiwanMarketBreadth(identities, rows, fresh, map[string]string{"TWSE": "current", "TPEX": "current"})
		if got.TWSE.AdvanceRatio == nil || *got.TWSE.AdvanceRatio != .5 || got.TPEX.AdvanceRatio == nil || *got.TPEX.AdvanceRatio != 1 || got.Combined.AdvanceRatio == nil || *got.Combined.AdvanceRatio != 2.0/3.0 || got.Combined.TotalAmountTWD != 1000 {
			t.Fatalf("got=%+v", got)
		}
	})

	t.Run("stale snapshot inherited", func(t *testing.T) {
		stale := fresh
		stale.DailyStatus, stale.TargetLatestTradingDate, stale.DailyDaysBehind = "stale", "2026-09-01", intBreadth(1)
		got := CalculateTaiwanMarketBreadth(identities, nil, stale, map[string]string{"TWSE": "stale", "TPEX": "stale"})
		if got.TWSE.Freshness != "stale" || got.TWSE.DaysBehind == nil || *got.TWSE.DaysBehind != 1 {
			t.Fatalf("got=%+v", got.TWSE)
		}
	})

	t.Run("unavailable and partial failure are isolated", func(t *testing.T) {
		rows := []TaiwanDailySnapshot{breadthRow(twseA, breadthFloat(1), breadthInt(10), 100), breadthRow(twseB, breadthFloat(-1), breadthInt(10), 200)}
		got := CalculateTaiwanMarketBreadth(identities, rows, fresh, map[string]string{"TWSE": "current", "TPEX": "unavailable"})
		if got.TWSE.Status != "current" || got.TPEX.Status != "unavailable" || got.TPEX.Freshness != "unavailable" || got.TPEX.Unknown != 1 || got.Combined.Status != "partial" || got.Combined.Freshness != "partial" || !reflect.DeepEqual(got.Combined.IncludedExchanges, []string{"TWSE"}) || !reflect.DeepEqual(got.Combined.MissingExchanges, []string{"TPEX"}) {
			t.Fatalf("got=%+v", got)
		}
	})

	t.Run("idempotent calculation and sanity equation", func(t *testing.T) {
		rows := []TaiwanDailySnapshot{breadthRow(twseA, breadthFloat(1), breadthInt(10), 100), breadthRow(twseB, nil, breadthInt(10), 50), breadthRow(tpexA, breadthFloat(0), breadthInt(0), 0)}
		one := CalculateTaiwanMarketBreadth(identities, rows, fresh, map[string]string{"TWSE": "current", "TPEX": "current"})
		two := CalculateTaiwanMarketBreadth(identities, rows, fresh, map[string]string{"TWSE": "current", "TPEX": "current"})
		if !reflect.DeepEqual(one, two) {
			t.Fatalf("not idempotent: one=%+v two=%+v", one, two)
		}
		for _, scope := range []TaiwanBreadthScope{one.TWSE, one.TPEX, one.Combined} {
			if scope.Advancers+scope.Decliners+scope.Unchanged+scope.NoTrade+scope.Unknown != scope.UniverseCount {
				t.Fatalf("sanity failed: %+v", scope)
			}
		}
	})

	t.Run("safe limits and data insufficient limit unknown", func(t *testing.T) {
		up := breadthRow(twseA, breadthFloat(10), breadthInt(10), 100)
		up.Close = breadthFloat(110)
		insufficient := twseB
		insufficient.RuleProfile = &TaiwanSecurityRuleProfile{Status: "data_insufficient"}
		rows := []TaiwanDailySnapshot{up, breadthRow(insufficient, breadthFloat(1), breadthInt(10), 100)}
		got := CalculateTaiwanMarketBreadth([]SecurityIdentity{twseA, insufficient}, rows, fresh, map[string]string{"TWSE": "current", "TPEX": "unavailable"}).TWSE
		if got.LimitUpCount != 1 || got.LimitDownCount != 0 || got.LimitUnknownCount != 1 {
			t.Fatalf("got=%+v", got)
		}
	})

	t.Run("safe limit down", func(t *testing.T) {
		down := breadthRow(twseA, breadthFloat(-10), breadthInt(10), 100)
		down.Close = breadthFloat(90)
		got := CalculateTaiwanMarketBreadth([]SecurityIdentity{twseA}, []TaiwanDailySnapshot{down}, fresh, map[string]string{"TWSE": "current", "TPEX": "unavailable"}).TWSE
		if got.LimitDownCount != 1 || got.LimitUpCount != 0 {
			t.Fatalf("got=%+v", got)
		}
	})

	t.Run("TPEX only", func(t *testing.T) {
		got := CalculateTaiwanMarketBreadth([]SecurityIdentity{tpexA}, []TaiwanDailySnapshot{breadthRow(tpexA, breadthFloat(1), breadthInt(10), 100)}, fresh, map[string]string{"TWSE": "unavailable", "TPEX": "current"})
		if got.TPEX.Advancers != 1 || got.Combined.UniverseCount != 1 || !reflect.DeepEqual(got.Combined.IncludedExchanges, []string{"TPEX"}) {
			t.Fatalf("got=%+v", got)
		}
	})

	t.Run("unknown SecurityType fails closed", func(t *testing.T) {
		got := CalculateTaiwanMarketBreadth([]SecurityIdentity{unknownType}, []TaiwanDailySnapshot{breadthRow(unknownType, breadthFloat(1), breadthInt(10), 100)}, fresh, map[string]string{"TWSE": "current", "TPEX": "unavailable"}).TWSE
		if got.UniverseCount != 0 || got.Advancers != 0 {
			t.Fatalf("got=%+v", got)
		}
	})

	t.Run("amount conservation exposes missing and preserves zero", func(t *testing.T) {
		advance := breadthRow(twseA, breadthFloat(1), breadthInt(10), 100)
		unknown := breadthRow(twseB, nil, breadthInt(10), 25)
		zero := breadthRow(tpexA, breadthFloat(-1), breadthInt(10), 0)
		missingIdentity := breadthIdentity("6489", "TPEX", SecurityTypeStock)
		missing := breadthRow(missingIdentity, breadthFloat(1), breadthInt(10), 0)
		missing.Amount = nil
		got := CalculateTaiwanMarketBreadth([]SecurityIdentity{twseA, twseB, tpexA, missingIdentity}, []TaiwanDailySnapshot{advance, unknown, zero, missing}, fresh, map[string]string{"TWSE": "current", "TPEX": "current"}).Combined
		bucketTotal := got.AdvancingAmountTWD + got.DecliningAmountTWD + got.UnchangedAmountTWD + got.NoTradeAmountTWD + got.UnknownAmountTWD
		if got.TotalAmountTWD != 125 || bucketTotal != got.TotalAmountTWD || got.UnknownAmountTWD != 25 || got.MissingAmountCount != 1 {
			t.Fatalf("got=%+v bucket_total=%v", got, bucketTotal)
		}
	})
}

func ptrBreadth(value string) *string { return &value }
func intBreadth(value int) *int       { return &value }
