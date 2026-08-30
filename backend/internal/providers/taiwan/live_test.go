package taiwan

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"easy-stock/backend/internal/foundation"
)

func TestLiveOfficialDirectorySmoke(t *testing.T) {
	if os.Getenv("EASY_STOCK_TW_LIVE_TEST") != "1" {
		t.Skip("set EASY_STOCK_TW_LIVE_TEST=1 to query TWSE and TPEx")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	items, err := NewClient(Config{}).Directory(ctx)
	if err != nil {
		t.Fatalf("official directory unavailable: %v", err)
	}
	expected := map[string]struct {
		name, canonical, exchange string
		securityType              foundation.SecurityType
	}{
		"2330": {"台積電", "2330.TWSE", "TWSE", foundation.SecurityTypeStock},
		"2317": {"鴻海", "2317.TWSE", "TWSE", foundation.SecurityTypeStock},
		"2454": {"聯發科", "2454.TWSE", "TWSE", foundation.SecurityTypeStock},
		"6488": {"環球晶", "6488.TPEX", "TPEX", foundation.SecurityTypeStock},
		"0050": {"元大台灣50", "0050.TWSE", "TWSE", foundation.SecurityTypeETF},
	}
	for code, want := range expected {
		byCode := liveMatches(items, code)
		byName := liveMatches(items, want.name)
		if len(byCode) != 1 {
			t.Fatalf("code %s returned %d identities: %+v", code, len(byCode), byCode)
		}
		if len(byName) != 1 {
			t.Fatalf("name %s returned %d identities: %+v", want.name, len(byName), byName)
		}
		got := byCode[0]
		if got != byName[0] {
			t.Fatalf("code/name mismatch for %s: code=%+v name=%+v", code, got, byName[0])
		}
		if got.Canonical != want.canonical || got.Exchange != want.exchange || got.Type != want.securityType || got.Name != want.name {
			t.Fatalf("identity mismatch for %s: got=%+v want=%+v", code, got, want)
		}
		if got.Market != "TW" || got.Currency != "TWD" || got.Timezone != "Asia/Taipei" || got.SourceURL == "" || got.RetrievedAt.IsZero() {
			t.Fatalf("metadata incomplete for %s: %+v", code, got)
		}
	}
}

func liveMatches(items []foundation.SecurityIdentity, query string) []foundation.SecurityIdentity {
	query = strings.ToLower(strings.TrimSpace(query))
	matches := make([]foundation.SecurityIdentity, 0, 1)
	for _, item := range items {
		if strings.ToLower(strings.TrimSpace(item.Code)) == query || strings.ToLower(strings.TrimSpace(item.Name)) == query || strings.ToLower(strings.TrimSpace(item.FullName)) == query {
			matches = append(matches, item)
		}
	}
	return matches
}
