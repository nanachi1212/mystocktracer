package foundation

import (
	"testing"
	"time"
)

func TestLatestAvailableStatementIsStrictAndRevisionAware(t *testing.T) {
	first := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	revised := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)
	history := []FinancialStatementPeriod{
		{PeriodEnd: "2026-03-31", Revision: "original", AvailableAt: &first},
		{PeriodEnd: "2026-03-31", Revision: "revised", AvailableAt: &revised},
	}
	for _, query := range []time.Time{first.AddDate(0, 0, -1), first} {
		if got := LatestAvailableStatement(history, query); got != nil {
			t.Fatalf("statement must not be available at %s: %+v", query, got)
		}
	}
	if got := LatestAvailableStatement(history, first.AddDate(0, 0, 1)); got == nil || got.Revision != "original" {
		t.Fatalf("next trading day should see original: %+v", got)
	}
	if got := LatestAvailableStatement(history, revised); got == nil || got.Revision != "original" {
		t.Fatalf("revision day must still see original: %+v", got)
	}
	if got := LatestAvailableStatement(history, revised.AddDate(0, 0, 1)); got == nil || got.Revision != "revised" {
		t.Fatalf("day after revision should see revised: %+v", got)
	}
}
