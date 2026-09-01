package stockanalysis

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"easy-stock/backend/internal/foundation"
	"easy-stock/backend/internal/sector"
)

func TestTaiwanPriceInterpretationSignedDirectionBoundaries(t *testing.T) {
	tests := []struct {
		name         string
		five, twenty *float64
		want         string
	}{
		{"both positive", f64(1), f64(2), "positive"}, {"both negative", f64(-1), f64(-2), "negative"},
		{"positive negative", f64(1), f64(-2), "mixed"}, {"negative positive", f64(-1), f64(2), "mixed"},
		{"zero five", f64(0), f64(2), "mixed"}, {"zero twenty", f64(1), f64(0), "mixed"},
		{"missing five", nil, f64(2), "indeterminate"}, {"missing twenty", f64(1), nil, "indeterminate"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := interpretTaiwanPrice(TaiwanPriceHistoryEvidence{TaiwanEvidenceStatus: TaiwanEvidenceStatus{Status: "available"}, Return5D: tt.five, Return20D: tt.twenty}, TaiwanQuoteEvidence{})
			if got.State != tt.want {
				t.Fatalf("state=%s want %s", got.State, tt.want)
			}
		})
	}
}

func TestTaiwanPriceUsesCompletedReturnsAndReportsFutureQuoteStatus(t *testing.T) {
	got := interpretTaiwanPrice(TaiwanPriceHistoryEvidence{TaiwanEvidenceStatus: TaiwanEvidenceStatus{Status: "available", AsOf: "2026-08-31"}, Return5D: f64(1), Return20D: f64(2)}, TaiwanQuoteEvidence{TaiwanEvidenceStatus: TaiwanEvidenceStatus{Status: "official_close", Freshness: "unavailable", AsOf: "2026-09-01"}})
	if got.State != "positive" || got.Details["quote_freshness"] != "unavailable" || len(got.Reasons) != 3 {
		t.Fatalf("future quote semantics=%+v", got)
	}
}

func TestTaiwanMarketRelationshipAndStatusPreservation(t *testing.T) {
	price := TaiwanInterpretationComponent{State: "positive", Status: "available"}
	if got := interpretPriceMarket(price, TaiwanInterpretationComponent{State: "weak", Status: "partial"}); got.State != "price_positive_market_weak" {
		t.Fatalf("relationship=%+v", got)
	}
	if got := interpretPriceMarket(TaiwanInterpretationComponent{State: "negative"}, TaiwanInterpretationComponent{State: "positive"}); got.State != "price_negative_market_positive" {
		t.Fatalf("relationship=%+v", got)
	}
	if got := interpretPriceMarket(TaiwanInterpretationComponent{State: "positive"}, TaiwanInterpretationComponent{State: "positive"}); got.State != "aligned_positive" {
		t.Fatalf("relationship=%+v", got)
	}
	if got := interpretPriceMarket(TaiwanInterpretationComponent{State: "negative"}, TaiwanInterpretationComponent{State: "weak"}); got.State != "aligned_negative" {
		t.Fatalf("relationship=%+v", got)
	}
	for _, state := range []string{"positive", "weak", "mixed"} {
		got := interpretTaiwanMarket(TaiwanMarketContextEvidence{TaiwanEvidenceStatus: TaiwanEvidenceStatus{Status: "partial", Freshness: "stale"}, State: state})
		if got.State != state || got.Status != "partial" || got.Freshness != "stale" {
			t.Fatalf("market=%+v", got)
		}
	}
	if got := interpretTaiwanMarket(TaiwanMarketContextEvidence{TaiwanEvidenceStatus: TaiwanEvidenceStatus{Status: "unavailable"}, State: "positive"}); got.State != "indeterminate" {
		t.Fatalf("unavailable market=%+v", got)
	}
}

func TestTaiwanIndustryInterpretationBoundariesAndETF(t *testing.T) {
	identity := TaiwanIdentityEvidence{SecurityType: foundation.SecurityTypeStock}
	for _, tt := range []struct {
		b, c *float64
		want string
	}{{f64(1), f64(2), "supportive"}, {f64(-1), f64(-2), "weak"}, {f64(1), f64(-2), "mixed"}, {f64(0), f64(2), "mixed"}, {nil, f64(2), "indeterminate"}} {
		got := interpretTaiwanIndustry(identity, TaiwanIndustryContextEvidence{TaiwanEvidenceStatus: TaiwanEvidenceStatus{Status: "partial"}, Industry: &sector.TaiwanIndustry{RelativeBreadth: tt.b, RelativeCapital: tt.c, BenchmarkScope: "TWSE"}})
		if got.State != tt.want {
			t.Fatalf("industry=%+v want %s", got, tt.want)
		}
	}
	got := interpretTaiwanIndustry(TaiwanIdentityEvidence{SecurityType: foundation.SecurityTypeETF}, TaiwanIndustryContextEvidence{})
	if got.State != "not_applicable" || got.Status != "not_applicable" {
		t.Fatalf("ETF industry=%+v", got)
	}
}

func TestTaiwanInstitutionalAndMarginUseLatestOfficialFacts(t *testing.T) {
	official := foundation.SourceMeta{Status: "official", AvailableFields: []string{"unit:shares", "raw_net", "computed_net"}}
	inst := TaiwanInstitutionalEvidence{TaiwanEvidenceStatus: TaiwanEvidenceStatus{Status: "available"}, Data: &foundation.InstitutionalHistory{Data: []foundation.InstitutionalFlow{
		{TradeDate: "2026-08-30", ForeignOfficialNet: -1, Meta: official},
		{TradeDate: "2026-08-31", ForeignOfficialNet: 1, InvestmentTrustOfficialNet: 1, DealerOfficialNet: 1, Meta: official},
	}}}
	if got := interpretTaiwanInstitutional(inst); got.State != "net_buy" || got.Details["foreign"] != "net_buy" {
		t.Fatalf("institutional=%+v", got)
	}
	inst.Data.Data[1].ForeignOfficialNet, inst.Data.Data[1].InvestmentTrustOfficialNet, inst.Data.Data[1].DealerOfficialNet = -1, -1, -1
	if got := interpretTaiwanInstitutional(inst); got.State != "net_sell" {
		t.Fatalf("institutional=%+v", got)
	}
	inst.Data.Data[1].ForeignOfficialNet, inst.Data.Data[1].InvestmentTrustOfficialNet, inst.Data.Data[1].DealerOfficialNet = 0, 0, 0
	if got := interpretTaiwanInstitutional(inst); got.State != "neutral" {
		t.Fatalf("institutional=%+v", got)
	}
	if got := interpretTaiwanInstitutional(TaiwanInstitutionalEvidence{}); got.State != "indeterminate" {
		t.Fatalf("missing institutional=%+v", got)
	}
	missingCategory := inst
	missingCategory.Data = &foundation.InstitutionalHistory{Data: []foundation.InstitutionalFlow{{TradeDate: "2026-08-31", ForeignOfficialNet: 1, InvestmentTrustOfficialNet: 1}}}
	if got := interpretTaiwanInstitutional(missingCategory); got.State != "indeterminate" || got.Status != "data_insufficient" {
		t.Fatalf("missing category became zero: %+v", got)
	}
	marginChange, shortChange := int64(1), int64(-1)
	margin := TaiwanMarginEvidence{TaiwanEvidenceStatus: TaiwanEvidenceStatus{Status: "available"}, Data: &foundation.MarginHistory{Data: []foundation.MarginTrading{{TradeDate: "2026-08-31", MarginChange: &marginChange, ShortChange: &shortChange}}}}
	got := interpretTaiwanMargin(margin)
	if got.Details["margin_balance"] != "margin_increased" || got.Details["short_balance"] != "short_decreased" {
		t.Fatalf("margin=%+v", got)
	}
	zero := int64(0)
	margin.Data.Data[0].MarginChange = &zero
	if got := interpretTaiwanMargin(margin); got.Details["margin_balance"] != "unchanged" {
		t.Fatalf("zero margin=%+v", got)
	}
	margin.Data.Data[0].ShortChange = nil
	if got := interpretTaiwanMargin(margin); got.Details["short_balance"] != "indeterminate" || got.Details["margin_balance"] != "unchanged" {
		t.Fatalf("missing short=%+v", got)
	}
	shortPositive := int64(1)
	margin.Data.Data[0].ShortChange = &shortPositive
	if got := interpretTaiwanMargin(margin); got.Details["short_balance"] != "short_increased" {
		t.Fatalf("short increase=%+v", got)
	}
	margin.Data.Data[0].ShortChange = &zero
	if got := interpretTaiwanMargin(margin); got.Details["short_balance"] != "unchanged" {
		t.Fatalf("short zero=%+v", got)
	}
	if got := interpretTaiwanMargin(TaiwanMarginEvidence{}); got.State != "indeterminate" {
		t.Fatalf("missing margin=%+v", got)
	}
}

func TestTaiwanFundamentalsETFAndOfficialYoY(t *testing.T) {
	stock := TaiwanIdentityEvidence{SecurityType: foundation.SecurityTypeStock}
	value := f64(3)
	fund := TaiwanFundamentalsEvidence{TaiwanEvidenceStatus: TaiwanEvidenceStatus{Status: "available"}, Data: &foundation.TaiwanFundamentals{Revenue: []foundation.MonthlyRevenue{{Period: "2026-07", OfficialYoY: value}}}}
	if got := interpretTaiwanFundamentals(stock, fund); got.State != "positive" {
		t.Fatalf("fundamentals=%+v", got)
	}
	negative := f64(-3)
	fund.Data.Revenue[0].OfficialYoY = negative
	if got := interpretTaiwanFundamentals(stock, fund); got.State != "negative" {
		t.Fatalf("fundamentals=%+v", got)
	}
	zero := f64(0)
	fund.Data.Revenue[0].OfficialYoY = zero
	if got := interpretTaiwanFundamentals(stock, fund); got.State != "flat" {
		t.Fatalf("fundamentals=%+v", got)
	}
	fund.Data.Revenue[0].OfficialYoY = nil
	if got := interpretTaiwanFundamentals(stock, fund); got.State != "indeterminate" {
		t.Fatalf("missing YoY=%+v", got)
	}
	if got := interpretTaiwanFundamentals(stock, TaiwanFundamentalsEvidence{TaiwanEvidenceStatus: TaiwanEvidenceStatus{Status: "unavailable"}}); got.State != "indeterminate" {
		t.Fatalf("unavailable fundamentals=%+v", got)
	}
	if got := interpretTaiwanFundamentals(TaiwanIdentityEvidence{SecurityType: foundation.SecurityTypeETF}, fund); got.State != "not_applicable" {
		t.Fatalf("ETF fundamentals=%+v", got)
	}
}

func TestTaiwanInterpretationIsPureDeterministicAndHasNoForbiddenOutput(t *testing.T) {
	input := TaiwanStockIntelligence{Symbol: "2330.TWSE", Identity: TaiwanIdentityEvidence{SecurityType: foundation.SecurityTypeStock}, PriceHistory: TaiwanPriceHistoryEvidence{TaiwanEvidenceStatus: TaiwanEvidenceStatus{Status: "available"}, Return5D: f64(1), Return20D: f64(2)}, MarketContext: TaiwanMarketContextEvidence{TaiwanEvidenceStatus: TaiwanEvidenceStatus{Status: "current"}, State: "positive"}}
	before, _ := json.Marshal(input)
	one, two := CalculateTaiwanStockInterpretation(input), CalculateTaiwanStockInterpretation(input)
	after, _ := json.Marshal(input)
	if !reflect.DeepEqual(one, two) || !reflect.DeepEqual(after, before) {
		t.Fatal("calculator is not pure and deterministic")
	}
	raw, _ := json.Marshal(one)
	for _, forbidden := range []string{"overall_score", "recommendation", "action", "target_price", "position", "conviction"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("forbidden output %q in %s", forbidden, raw)
		}
	}
}

func f64(value float64) *float64 { return &value }
