package stockanalysis

import (
	"reflect"
	"testing"
	"time"

	"easy-stock/backend/internal/foundation"
	"easy-stock/backend/internal/marketemotion"
	"easy-stock/backend/internal/sector"
)

func TestTaiwanStockIntelligencePreservesEvidenceAndContexts(t *testing.T) {
	identity := foundation.SecurityIdentity{Canonical: "2330.TWSE", Code: "2330", Name: "台積電", Exchange: "TWSE", Currency: "TWD", Timezone: "Asia/Taipei", Type: foundation.SecurityTypeStock, Industry: "24"}
	meta := foundation.SourceMeta{Source: "twse:official", SourceURL: "official", TradeDate: "2026-08-31", Freshness: "current", Status: "official_close", FetchedAt: time.Unix(1, 0)}
	quote := foundation.Quote{Symbol: identity.Canonical, Price: 100, Change: 2, Meta: meta}
	lines := make([]foundation.KLine, 21)
	for i := range lines {
		lines[i] = foundation.KLine{Symbol: identity.Canonical, Time: time.Date(2026, 8, i+1, 0, 0, 0, 0, time.UTC), Close: float64(80 + i), Meta: meta}
	}
	fund := foundation.TaiwanFundamentals{Security: identity, Meta: meta}
	inst := foundation.InstitutionalHistory{Security: identity, Data: []foundation.InstitutionalFlow{{ForeignNet: -10}}, Meta: meta}
	margin := foundation.MarginHistory{Security: identity, Data: []foundation.MarginTrading{{MarginBalance: i64ptr(0)}}, Meta: meta}
	ar, aa := .6, .7
	breadth := foundation.TaiwanBreadthScope{Scope: "TWSE", Status: "current", Freshness: "current", AdvanceRatio: &ar, AdvancingAmountRatio: &aa}
	emotion := marketemotion.TaiwanEmotionScope{Scope: "TWSE", State: "positive", Confidence: "high"}
	radar := sector.TaiwanIndustryScope{Status: "current", Freshness: "current", TaxonomyStatus: "current", Industries: []sector.TaiwanIndustry{{IndustryID: "TWSE:24", IndustryCode: "24", IndustryName: "半導體業", TaxonomySource: "TWSE official"}}}
	got := NewTaiwanStockIntelligence(identity, &quote, lines, &fund, &inst, &margin, breadth, emotion, radar)
	if got.ModelVersion != TaiwanStockIntelligenceVersion || got.Quote.Data.Price != quote.Price || got.MarketContext.AdvanceRatio != breadth.AdvanceRatio || got.IndustryContext.Industry.IndustryID != "TWSE:24" {
		t.Fatalf("raw facts changed: %+v", got)
	}
	if got.PriceHistory.Return5D == nil || got.PriceHistory.Return20D == nil || got.Institutional.Data.Data[0].ForeignNet != -10 || *got.Margin.Data.Data[0].MarginBalance != 0 {
		t.Fatalf("evidence missing/zero semantics: %+v", got)
	}
	if !reflect.DeepEqual(got, NewTaiwanStockIntelligence(identity, &quote, lines, &fund, &inst, &margin, breadth, emotion, radar)) {
		t.Fatal("not deterministic")
	}
}

func TestTaiwanReturnRequiresNPlusOneCompletedCloses(t *testing.T) {
	lines := make([]foundation.KLine, 20)
	for i := range lines {
		lines[i].Close = float64(i + 1)
	}
	if periodReturn(lines, 20) != nil {
		t.Fatal("20-session return must require 21 closes")
	}
	lines = append(lines, foundation.KLine{Close: 21})
	got := periodReturn(lines, 20)
	if got == nil || *got != 2000 {
		t.Fatalf("20-session return=%v", got)
	}
}

func TestTaiwanStockIntelligenceMissingStaleUnclassifiedAndInsufficientHistory(t *testing.T) {
	identity := foundation.SecurityIdentity{Canonical: "9103.TWSE", Exchange: "TWSE", Type: foundation.SecurityTypeStock, Industry: "91"}
	meta := foundation.SourceMeta{Status: "official_monthly_fallback", Freshness: "stale", FallbackReason: "daily unavailable"}
	quote := foundation.Quote{Symbol: identity.Canonical, Meta: meta}
	lines := []foundation.KLine{{Close: 10, Meta: meta}}
	got := NewTaiwanStockIntelligence(identity, &quote, lines, nil, nil, nil, foundation.TaiwanBreadthScope{Scope: "TWSE", Status: "partial", Freshness: "current"}, marketemotion.TaiwanEmotionScope{}, sector.TaiwanIndustryScope{Status: "partial", TaxonomyStatus: "partial"})
	if got.Quote.Freshness != "stale" || got.Quote.Data.Meta.FallbackReason == "" || got.PriceHistory.Return5D != nil || got.PriceHistory.Return20D != nil {
		t.Fatalf("fallback/history semantics=%+v", got)
	}
	if got.Fundamentals.Status != "unavailable" || got.Institutional.Data != nil || got.Margin.Data != nil || got.IndustryContext.Status != "unavailable" {
		t.Fatalf("missing became value: %+v", got)
	}
}

func TestTaiwanFutureQuoteObservationStaysUnavailable(t *testing.T) {
	identity := foundation.SecurityIdentity{Canonical: "2330.TWSE", Exchange: "TWSE", Type: foundation.SecurityTypeStock}
	meta := foundation.SourceMeta{TradeDate: "2026-09-01", Freshness: "unavailable", Status: "official_close"}
	quote := foundation.Quote{Symbol: identity.Canonical, Price: 100, Meta: meta}
	got := NewTaiwanStockIntelligence(identity, &quote, nil, nil, nil, nil, foundation.TaiwanBreadthScope{Scope: "TWSE", TargetLatestTradingDate: "2026-08-31", Status: "current"}, marketemotion.TaiwanEmotionScope{}, sector.TaiwanIndustryScope{})
	if got.Quote.Data.Meta.TradeDate != "2026-09-01" || got.Quote.TargetLatestCompletedTradingDate != "2026-08-31" || got.Quote.Freshness != "unavailable" || got.MarketContext.Status != "current" {
		t.Fatalf("future observation contaminated completed context: %+v", got)
	}
}

func TestTaiwanStockIntelligenceETFHasNoStockOnlySections(t *testing.T) {
	identity := foundation.SecurityIdentity{Canonical: "0050.TWSE", Exchange: "TWSE", Type: foundation.SecurityTypeETF, Industry: "ETF"}
	got := NewTaiwanStockIntelligence(identity, nil, nil, nil, nil, nil, foundation.TaiwanBreadthScope{Scope: "TWSE"}, marketemotion.TaiwanEmotionScope{}, sector.TaiwanIndustryScope{})
	if got.Identity.SecurityType != foundation.SecurityTypeETF || got.Fundamentals.Status != "not_applicable" || got.IndustryContext.Status != "unavailable" {
		t.Fatalf("ETF semantics=%+v", got)
	}
}

func i64ptr(value int64) *int64 { return &value }
