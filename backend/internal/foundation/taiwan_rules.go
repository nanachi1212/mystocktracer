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

type TaiwanSecurityRuleProfile struct {
	SecurityType    string   `json:"security_type"`
	LotSizeClass    string   `json:"lot_size_class"`
	BoardLotShares  *int     `json:"board_lot_shares"`
	OddLotMinShares int      `json:"odd_lot_min_shares"`
	TickSizeClass   string   `json:"tick_size_class"`
	PriceLimitClass string   `json:"price_limit_class"`
	PriceLimitPct   *float64 `json:"price_limit_pct"`
	TaxClass        string   `json:"tax_class"`
	SettlementDays  int      `json:"settlement_business_days"`
	Status          string   `json:"status"`
	Reason          string   `json:"reason,omitempty"`
	SourceURL       string   `json:"source_url"`
}

func TaiwanRuleProfile(security SecurityIdentity) TaiwanSecurityRuleProfile {
	profile := TaiwanSecurityRuleProfile{SecurityType: string(security.Type), OddLotMinShares: TaiwanOddLotMinShares, SettlementDays: TaiwanSettlementDays, SourceURL: security.SourceURL}
	if security.Type == SecurityTypeStock {
		lot, limit := TaiwanBoardLotShares, 0.10
		profile.LotSizeClass, profile.BoardLotShares = "board_lot", &lot
		profile.TickSizeClass, profile.PriceLimitClass, profile.PriceLimitPct = "ordinary_stock", "ordinary_10_percent", &limit
		profile.TaxClass, profile.Status = "ordinary_stock", "official"
		return profile
	}
	if security.Type != SecurityTypeETF || security.TaiwanMetadata == nil {
		profile.Status, profile.Reason = "data_insufficient", "official Taiwan rule metadata is unavailable"
		return profile
	}
	metadata := security.TaiwanMetadata
	profile.TickSizeClass = "etf"
	profile.TaxClass = "domestic_etf"
	if metadata.ComponentScope == "foreign" {
		lot := TaiwanBoardLotShares
		profile.LotSizeClass, profile.BoardLotShares = "board_lot", &lot
		profile.PriceLimitClass, profile.TaxClass = "no_limit", "foreign_component_etf"
	} else if metadata.ComponentScope == "domestic" && metadata.Strategy == "ordinary" {
		lot := TaiwanBoardLotShares
		profile.LotSizeClass, profile.BoardLotShares = "board_lot", &lot
		limit := 0.10
		profile.PriceLimitClass, profile.PriceLimitPct = "ordinary_10_percent", &limit
	} else if metadata.ComponentScope == "domestic" && metadata.LeverageMultiplier != nil {
		limit := 0.10 * math.Abs(*metadata.LeverageMultiplier)
		profile.PriceLimitClass, profile.PriceLimitPct = "leveraged_domestic", &limit
	} else {
		profile.Status, profile.Reason = "data_insufficient", "official ETF multiplier or component scope is unavailable"
		return profile
	}
	if metadata.Strategy == "passive_bond" {
		profile.TaxClass = "passive_bond_etf"
	}
	profile.Status = "official"
	return profile
}

type BrokerCommissionConfig struct {
	Rate     *float64 `json:"commission_rate,omitempty"`
	Discount float64  `json:"commission_discount,omitempty"`
	Minimum  *float64 `json:"minimum_commission,omitempty"`
	Source   string   `json:"source"`
}

func (config BrokerCommissionConfig) Commission(tradeValue float64) (*float64, error) {
	if config.Rate == nil {
		return nil, nil
	}
	if *config.Rate < 0 || config.Discount < 0 || (config.Minimum != nil && *config.Minimum < 0) {
		return nil, fmt.Errorf("broker commission values must be non-negative")
	}
	discount := config.Discount
	if discount == 0 {
		discount = 1
	}
	fee := tradeValue * *config.Rate * discount
	if config.Minimum != nil && fee < *config.Minimum {
		fee = *config.Minimum
	}
	return &fee, nil
}

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
