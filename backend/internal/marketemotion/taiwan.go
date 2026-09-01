package marketemotion

import "easy-stock/backend/internal/foundation"

const TaiwanEmotionModelVersion = "taiwan_emotion_v1"

type TaiwanEmotionRaw struct {
	AdvanceRatio         *float64 `json:"advance_ratio"`
	AdvanceDeclineDiff   int      `json:"advance_decline_diff"`
	AdvancingAmountRatio *float64 `json:"advancing_amount_ratio"`
	Advancers            int      `json:"advancers"`
	Decliners            int      `json:"decliners"`
	Unchanged            int      `json:"unchanged"`
	NoTrade              int      `json:"no_trade"`
	Unknown              int      `json:"unknown"`
	UniverseCount        int      `json:"universe_count"`
	TradedCount          int      `json:"traded_count"`
	TotalAmountTWD       float64  `json:"total_amount_twd"`
	MissingAmountCount   int      `json:"missing_amount_count"`
}

type TaiwanEmotionComponents struct {
	BreadthParticipation       string `json:"breadth_participation"`
	CapitalParticipation       string `json:"capital_participation"`
	BreadthCapitalRelationship string `json:"breadth_capital_relationship"`
}

type TaiwanEmotionCoverage struct {
	DirectionCoverage *float64 `json:"direction_coverage"`
	AmountCoverage    *float64 `json:"amount_coverage"`
	LimitRuleCoverage *float64 `json:"limit_rule_coverage"`
}

type TaiwanEmotionScope struct {
	Scope                   string                  `json:"scope"`
	ModelVersion            string                  `json:"model_version"`
	AsOf                    *string                 `json:"as_of"`
	TargetLatestTradingDate string                  `json:"target_latest_trading_date"`
	Status                  string                  `json:"status"`
	Freshness               string                  `json:"freshness"`
	Confidence              string                  `json:"confidence"`
	State                   string                  `json:"state"`
	Raw                     TaiwanEmotionRaw        `json:"raw"`
	Components              TaiwanEmotionComponents `json:"components"`
	Coverage                TaiwanEmotionCoverage   `json:"coverage"`
	IncludedExchanges       []string                `json:"included_exchanges"`
	MissingExchanges        []string                `json:"missing_exchanges"`
	ClassificationBasis     string                  `json:"classification_basis"`
}

type TaiwanMarketEmotion struct {
	TWSE     TaiwanEmotionScope `json:"twse"`
	TPEX     TaiwanEmotionScope `json:"tpex"`
	Combined TaiwanEmotionScope `json:"combined"`
}

func CalculateTaiwanMarketEmotion(breadth foundation.TaiwanMarketBreadth) TaiwanMarketEmotion {
	return TaiwanMarketEmotion{
		TWSE:     calculateTaiwanEmotionScope(breadth.TWSE),
		TPEX:     calculateTaiwanEmotionScope(breadth.TPEX),
		Combined: calculateTaiwanEmotionScope(breadth.Combined),
	}
}

func calculateTaiwanEmotionScope(breadth foundation.TaiwanBreadthScope) TaiwanEmotionScope {
	result := TaiwanEmotionScope{
		Scope: breadth.Scope, ModelVersion: TaiwanEmotionModelVersion,
		AsOf: breadth.AsOf, TargetLatestTradingDate: breadth.TargetLatestTradingDate,
		Status: breadth.Status, Freshness: breadth.Freshness,
		IncludedExchanges:   append([]string(nil), breadth.IncludedExchanges...),
		MissingExchanges:    append([]string(nil), breadth.MissingExchanges...),
		ClassificationBasis: breadth.ClassificationBasis,
		Raw: TaiwanEmotionRaw{
			AdvanceRatio: breadth.AdvanceRatio, AdvanceDeclineDiff: breadth.AdvanceDeclineDiff,
			AdvancingAmountRatio: breadth.AdvancingAmountRatio,
			Advancers:            breadth.Advancers, Decliners: breadth.Decliners, Unchanged: breadth.Unchanged,
			NoTrade: breadth.NoTrade, Unknown: breadth.Unknown, UniverseCount: breadth.UniverseCount,
			TradedCount: breadth.TradedCount, TotalAmountTWD: breadth.TotalAmountTWD,
			MissingAmountCount: breadth.MissingAmountCount,
		},
	}
	result.Coverage = taiwanEmotionCoverage(breadth)
	result.Components.BreadthParticipation = participation(breadth.AdvanceRatio)
	result.Components.CapitalParticipation = participation(breadth.AdvancingAmountRatio)
	result.Components.BreadthCapitalRelationship = relationship(breadth.AdvanceRatio, breadth.AdvancingAmountRatio)
	result.State = taiwanEmotionState(breadth)
	result.Confidence = taiwanEmotionConfidence(breadth, result.Coverage)
	return result
}

func participation(ratio *float64) string {
	if ratio == nil {
		return "unavailable"
	}
	if *ratio > 0.5 {
		return "positive"
	}
	if *ratio < 0.5 {
		return "negative"
	}
	return "balanced"
}

func relationship(breadth, capital *float64) string {
	if breadth == nil || capital == nil {
		return "indeterminate"
	}
	if *breadth > 0.5 && *capital < 0.5 {
		return "breadth_positive_capital_negative"
	}
	if *breadth < 0.5 && *capital > 0.5 {
		return "breadth_negative_capital_positive"
	}
	if *breadth > 0.5 && *capital > 0.5 {
		return "aligned_positive"
	}
	if *breadth < 0.5 && *capital < 0.5 {
		return "aligned_negative"
	}
	return "mixed"
}

func taiwanEmotionState(breadth foundation.TaiwanBreadthScope) string {
	if breadth.Status == "unavailable" {
		return "unavailable"
	}
	if breadth.AdvanceRatio == nil || breadth.AdvancingAmountRatio == nil {
		return "indeterminate"
	}
	if *breadth.AdvanceRatio > 0.5 && *breadth.AdvancingAmountRatio > 0.5 {
		return "positive"
	}
	if *breadth.AdvanceRatio < 0.5 && *breadth.AdvancingAmountRatio < 0.5 {
		return "weak"
	}
	return "mixed"
}

func taiwanEmotionCoverage(breadth foundation.TaiwanBreadthScope) TaiwanEmotionCoverage {
	if breadth.UniverseCount <= 0 {
		return TaiwanEmotionCoverage{}
	}
	denominator := float64(breadth.UniverseCount)
	direction := float64(breadth.Advancers+breadth.Decliners+breadth.Unchanged) / denominator
	amount := float64(max(0, breadth.UniverseCount-breadth.MissingAmountCount)) / denominator
	limit := float64(max(0, breadth.UniverseCount-breadth.LimitUnknownCount)) / denominator
	return TaiwanEmotionCoverage{DirectionCoverage: &direction, AmountCoverage: &amount, LimitRuleCoverage: &limit}
}

func taiwanEmotionConfidence(breadth foundation.TaiwanBreadthScope, coverage TaiwanEmotionCoverage) string {
	if breadth.Status == "unavailable" || breadth.AdvanceRatio == nil || breadth.AdvancingAmountRatio == nil || coverage.DirectionCoverage == nil || coverage.AmountCoverage == nil {
		return "low"
	}
	direction, amount := *coverage.DirectionCoverage, *coverage.AmountCoverage
	if breadth.Status == "current" && direction >= 0.95 && amount >= 0.95 {
		return "high"
	}
	if direction >= 0.80 && amount >= 0.80 {
		return "medium"
	}
	return "low"
}
