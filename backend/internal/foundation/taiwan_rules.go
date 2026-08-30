package foundation

import (
	"fmt"
	"math"
	"time"
)

const (
	TaiwanBoardLotShares  = 1000
	TaiwanOddLotMinShares = 1
	TaiwanSettlementDays  = 2
)

func TaiwanTickSize(price float64, securityType string) float64 {
	if securityType == "etf" {
		if price < 50 {
			return 0.01
		}
		return 0.05
	}
	switch {
	case price < 10:
		return 0.01
	case price < 50:
		return 0.05
	case price < 100:
		return 0.10
	case price < 500:
		return 0.50
	case price < 1000:
		return 1
	default:
		return 5
	}
}

func TaiwanPriceLimits(reference, limitPct float64, securityType string) (float64, float64) {
	upperRaw, lowerRaw := reference*(1+limitPct), reference*(1-limitPct)
	upperTick, lowerTick := TaiwanTickSize(upperRaw, securityType), TaiwanTickSize(lowerRaw, securityType)
	upper := math.Floor(upperRaw/upperTick+1e-9) * upperTick
	lower := math.Ceil(lowerRaw/lowerTick-1e-9) * lowerTick
	return math.Round(upper*100) / 100, math.Round(lower*100) / 100
}

func TaiwanSecuritiesTaxRate(class string, tradeDate time.Time) (float64, error) {
	switch class {
	case "ordinary_stock":
		return 0.003, nil
	case "day_trade_stock":
		if tradeDate.After(time.Date(2027, 12, 31, 23, 59, 59, 0, tradeDate.Location())) {
			return 0, fmt.Errorf("day-trade tax rule is unverified after 2027-12-31")
		}
		return 0.0015, nil
	case "domestic_etf", "foreign_component_etf":
		return 0.001, nil
	case "passive_bond_etf":
		if tradeDate.After(time.Date(2026, 12, 31, 23, 59, 59, 0, tradeDate.Location())) {
			return 0, fmt.Errorf("passive bond ETF tax rule is unverified after 2026-12-31")
		}
		return 0, nil
	default:
		return 0, fmt.Errorf("unsupported Taiwan tax class %q", class)
	}
}
