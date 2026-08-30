package foundation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

type taiwanContractFixture struct {
	TickSizes   map[string][][]float64 `json:"tick_sizes"`
	PriceLimits []struct {
		Reference float64 `json:"reference_price"`
		LimitPct  float64 `json:"limit_pct"`
		TickClass string  `json:"tick_class"`
		Upper     float64 `json:"upper"`
		Lower     float64 `json:"lower"`
	} `json:"price_limits"`
	Taxes []struct {
		Class     string  `json:"class"`
		TradeDate string  `json:"trade_date"`
		Rate      float64 `json:"rate"`
	} `json:"taxes"`
}

func TestTaiwanMarketContractV1(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "docs", "taiwan_market_contract_v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture taiwanContractFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	for class, cases := range fixture.TickSizes {
		for _, c := range cases {
			if got := TaiwanTickSize(c[0], class); got != c[1] {
				t.Errorf("tick %s %.2f: got %v want %v", class, c[0], got, c[1])
			}
		}
	}
	for _, c := range fixture.PriceLimits {
		up, down := TaiwanPriceLimits(c.Reference, c.LimitPct, c.TickClass)
		if up != c.Upper || down != c.Lower {
			t.Errorf("limits %.2f: got (%v,%v) want (%v,%v)", c.Reference, up, down, c.Upper, c.Lower)
		}
	}
	for _, c := range fixture.Taxes {
		date, err := time.Parse("2006-01-02", c.TradeDate)
		if err != nil {
			t.Fatal(err)
		}
		got, err := TaiwanSecuritiesTaxRate(c.Class, date)
		if err != nil || got != c.Rate {
			t.Errorf("tax %s: got %v,%v want %v", c.Class, got, err, c.Rate)
		}
	}
}
