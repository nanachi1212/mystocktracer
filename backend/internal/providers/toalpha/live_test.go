package toalpha

import (
	"os"
	"testing"
	"time"

	"github.com/nanachi1212/mystocktracer/backend/internal/foundation"
)

func TestLiveToAlphaMOPSMaterialNews(t *testing.T) {
	if os.Getenv("A_STOCK_LIVE_TEST") != "1" {
		t.Skip("set A_STOCK_LIVE_TEST=1 to call the public ToAlpha MOPS endpoint")
	}
	client := NewClient(Config{Enabled: true, Timeout: 15 * time.Second})
	feed := client.CorporateEvents(t.Context(), "2330.TWSE", 30, 5)
	if feed.Status != foundation.TaiwanCorporateEventsAvailable && feed.Status != foundation.TaiwanCorporateEventsPartial && feed.Status != foundation.TaiwanCorporateEventsNoEvents {
		if _, err := client.fetchMaterialNews(t.Context(), "2330", 30, 5); err != nil {
			t.Logf("live protocol diagnostic: %v", err)
		}
		t.Fatalf("live ToAlpha MOPS feed unavailable: %+v", feed)
	}
	if feed.Provider != foundation.TaiwanCorporateEventProviderToAlpha || feed.Source == "" {
		t.Fatalf("live provenance missing: %+v", feed)
	}
	if len(feed.Events) > 5 {
		t.Fatalf("live response exceeded requested bound: %d", len(feed.Events))
	}
}
