package foundation

import "math"

type TaiwanBreadthScope struct {
	Scope                   string   `json:"scope"`
	Status                  string   `json:"status"`
	AsOf                    *string  `json:"as_of"`
	TargetLatestTradingDate string   `json:"target_latest_trading_date"`
	Freshness               string   `json:"freshness"`
	DaysBehind              *int     `json:"days_behind,omitempty"`
	Universe                string   `json:"universe"`
	UniverseCount           int      `json:"universe_count"`
	TradedCount             int      `json:"traded_count"`
	Advancers               int      `json:"advancers"`
	Decliners               int      `json:"decliners"`
	Unchanged               int      `json:"unchanged"`
	NoTrade                 int      `json:"no_trade"`
	Unknown                 int      `json:"unknown"`
	AdvanceDeclineDiff      int      `json:"advance_decline_diff"`
	AdvanceRatio            *float64 `json:"advance_ratio"`
	TotalAmountTWD          float64  `json:"total_amount_twd"`
	AdvancingAmountTWD      float64  `json:"advancing_amount_twd"`
	DecliningAmountTWD      float64  `json:"declining_amount_twd"`
	UnchangedAmountTWD      float64  `json:"unchanged_amount_twd"`
	NoTradeAmountTWD        float64  `json:"no_trade_amount_twd"`
	UnknownAmountTWD        float64  `json:"unknown_amount_twd"`
	MissingAmountCount      int      `json:"missing_amount_count"`
	AdvancingAmountRatio    *float64 `json:"advancing_amount_ratio"`
	LimitUpCount            int      `json:"limit_up_count"`
	LimitDownCount          int      `json:"limit_down_count"`
	LimitUnknownCount       int      `json:"limit_unknown_count"`
	IncludedExchanges       []string `json:"included_exchanges"`
	MissingExchanges        []string `json:"missing_exchanges"`
	ClassificationBasis     string   `json:"classification_basis"`
}

type TaiwanMarketBreadth struct {
	TWSE     TaiwanBreadthScope `json:"twse"`
	TPEX     TaiwanBreadthScope `json:"tpex"`
	Combined TaiwanBreadthScope `json:"combined"`
}

func CalculateTaiwanMarketBreadth(identities []SecurityIdentity, rows []TaiwanDailySnapshot, freshness TaiwanFreshness, exchangeStatus map[string]string) TaiwanMarketBreadth {
	twse := calculateBreadthScope("TWSE", identities, rows, freshness, exchangeStatus["TWSE"])
	tpex := calculateBreadthScope("TPEX", identities, rows, freshness, exchangeStatus["TPEX"])
	included, missing := []string{}, []string{}
	for _, scope := range []TaiwanBreadthScope{twse, tpex} {
		if scope.Status == "unavailable" {
			missing = append(missing, scope.Scope)
		} else {
			included = append(included, scope.Scope)
		}
	}
	combinedIdentities := make([]SecurityIdentity, 0, len(identities))
	for _, identity := range identities {
		if contains(included, identity.Exchange) {
			combinedIdentities = append(combinedIdentities, identity)
		}
	}
	combinedStatus := freshness.DailyStatus
	if len(included) == 0 {
		combinedStatus = "unavailable"
	} else if len(missing) > 0 {
		combinedStatus = "partial"
	}
	combined := calculateBreadthScope("COMBINED", combinedIdentities, rows, freshness, combinedStatus)
	combined.Freshness = combinedStatus
	combined.IncludedExchanges, combined.MissingExchanges = included, missing
	return TaiwanMarketBreadth{TWSE: twse, TPEX: tpex, Combined: combined}
}

func calculateBreadthScope(scope string, identities []SecurityIdentity, rows []TaiwanDailySnapshot, freshness TaiwanFreshness, status string) TaiwanBreadthScope {
	result := TaiwanBreadthScope{
		Scope: scope, Status: status, AsOf: freshness.DailyAsOf,
		TargetLatestTradingDate: freshness.TargetLatestTradingDate,
		Freshness:               freshness.DailyStatus, DaysBehind: freshness.DailyDaysBehind,
		Universe: "stock_breadth", ClassificationBasis: "current_reference",
	}
	if status == "" {
		result.Status = freshness.DailyStatus
	}
	result.Freshness = result.Status
	rowByCanonical := make(map[string]TaiwanDailySnapshot, len(rows))
	for _, row := range rows {
		rowByCanonical[row.Canonical] = row
	}
	for _, identity := range identities {
		if identity.Type != SecurityTypeStock || (scope != "COMBINED" && identity.Exchange != scope) {
			continue
		}
		result.UniverseCount++
		row, ok := rowByCanonical[identity.Canonical]
		if result.Status == "unavailable" || !ok {
			result.Unknown++
			result.LimitUnknownCount++
			continue
		}
		bucket := "unknown"
		if row.NoTrade || (row.Volume != nil && *row.Volume == 0) {
			bucket = "no_trade"
		} else if row.Change != nil {
			result.TradedCount++
			switch {
			case *row.Change > 0:
				bucket = "advance"
			case *row.Change < 0:
				bucket = "decline"
			default:
				bucket = "unchanged"
			}
		} else if row.Volume != nil && *row.Volume > 0 {
			result.TradedCount++
		}
		result.addBucket(bucket, row.Amount)
		result.addLimit(identity, row)
	}
	result.AdvanceDeclineDiff = result.Advancers - result.Decliners
	if denominator := result.Advancers + result.Decliners; denominator > 0 {
		ratio := float64(result.Advancers) / float64(denominator)
		result.AdvanceRatio = &ratio
	}
	if denominator := result.AdvancingAmountTWD + result.DecliningAmountTWD; denominator > 0 {
		ratio := result.AdvancingAmountTWD / denominator
		result.AdvancingAmountRatio = &ratio
	}
	if scope != "COMBINED" {
		if result.Status == "unavailable" {
			result.MissingExchanges = []string{scope}
		} else {
			result.IncludedExchanges = []string{scope}
		}
	}
	return result
}

func (result *TaiwanBreadthScope) addBucket(bucket string, amount *float64) {
	if amount != nil {
		result.TotalAmountTWD += *amount
	} else {
		result.MissingAmountCount++
	}
	switch bucket {
	case "advance":
		result.Advancers++
		if amount != nil {
			result.AdvancingAmountTWD += *amount
		}
	case "decline":
		result.Decliners++
		if amount != nil {
			result.DecliningAmountTWD += *amount
		}
	case "unchanged":
		result.Unchanged++
		if amount != nil {
			result.UnchangedAmountTWD += *amount
		}
	case "no_trade":
		result.NoTrade++
		if amount != nil {
			result.NoTradeAmountTWD += *amount
		}
	default:
		result.Unknown++
		if amount != nil {
			result.UnknownAmountTWD += *amount
		}
	}
}

func (result *TaiwanBreadthScope) addLimit(identity SecurityIdentity, row TaiwanDailySnapshot) {
	profile := identity.RuleProfile
	if profile == nil {
		derived := TaiwanRuleProfile(identity)
		profile = &derived
	}
	if profile.Status != "official" || profile.PriceLimitPct == nil || row.Close == nil || row.Change == nil {
		result.LimitUnknownCount++
		return
	}
	reference := *row.Close - *row.Change
	if reference <= 0 {
		result.LimitUnknownCount++
		return
	}
	upper, lower := TaiwanPriceLimits(reference, *profile.PriceLimitPct, string(identity.Type))
	tolerance := TaiwanTickSize(*row.Close, string(identity.Type)) / 2
	if math.Abs(*row.Close-upper) <= tolerance {
		result.LimitUpCount++
	} else if math.Abs(*row.Close-lower) <= tolerance {
		result.LimitDownCount++
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
