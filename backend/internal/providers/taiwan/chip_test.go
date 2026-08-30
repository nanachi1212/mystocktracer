package taiwan

import (
	"testing"
	"time"

	"easy-stock/backend/internal/foundation"
)

func TestInstitutionalParsingAndDerivedCalculations(t *testing.T) {
	security := foundation.SecurityIdentity{Canonical: "2330.TWSE", Code: "2330", Name: "台積電", Exchange: "TWSE"}
	row := []string{"2330", "台積電", "10,027,572", "6,995,917", "3,031,655", "0", "0", "0", "109,915", "1,874,910", "-1,764,995", "-23,367", "70,050", "15,400", "54,650", "113,681", "191,698", "-78,017", "1,243,293"}
	flow, err := parseTWSEInstitutional(security, time.Date(2026, 8, 28, 0, 0, 0, 0, taipei()), "official", row)
	if err != nil {
		t.Fatal(err)
	}
	if flow.ForeignNet != 3_031_655 || flow.InvestmentTrustNet != -1_764_995 || flow.DealerNet != -23_367 || flow.DealerOfficialNet != -23_367 {
		t.Fatalf("unexpected flow: %+v", flow)
	}
	if len(flow.Discrepancies) != 0 {
		t.Fatalf("unexpected discrepancy: %v", flow.Discrepancies)
	}
	summary := institutionalSummary([]foundation.InstitutionalFlow{{ForeignNet: -1, InvestmentTrustNet: 2, DealerNet: 3}, flow})
	if summary.ForeignNet5D != 3_031_654 || summary.InvestmentTrustNet5D != -1_764_993 || summary.DealerNet5D != -23_364 || summary.ForeignConsecutiveBuyDays != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestMarginNormalizesLotsToSharesAndCalculatesChanges(t *testing.T) {
	security := foundation.SecurityIdentity{Canonical: "6488.TPEX", Code: "6488", Name: "環球晶", Exchange: "TPEX"}
	row := []string{"6488", "環球晶", "11,604", "1,132", "432", "0", "12,304", "216", "10.29", "119,528", "113", "2", "14", "0", "101", "1", "0.08", "119,528", "19", "11 A"}
	margin, err := parseMargin(security, time.Now(), "official", row, 3, 4, 5, 2, 6, 12, 11, 13, 10, 14, 19)
	if err != nil {
		t.Fatal(err)
	}
	if *margin.MarginBalance != 12_304_000 || *margin.MarginChange != 700_000 || *margin.ShortBalance != 101_000 || *margin.ShortChange != -12_000 {
		t.Fatalf("unexpected margin: %+v", margin)
	}
	if margin.ShortMarginRatio == nil || *margin.ShortMarginRatio < 0.82 || *margin.ShortMarginRatio > 0.83 {
		t.Fatalf("unexpected ratio: %v", margin.ShortMarginRatio)
	}
}
