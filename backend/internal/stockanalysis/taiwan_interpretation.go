package stockanalysis

import (
	"fmt"
	"sort"

	"easy-stock/backend/internal/foundation"
)

const TaiwanStockInterpretationVersion = "taiwan_stock_interpretation_v1"

type TaiwanInterpretationComponent struct {
	State        string            `json:"state"`
	Status       string            `json:"status"`
	Freshness    string            `json:"freshness,omitempty"`
	AsOf         string            `json:"as_of,omitempty"`
	Reasons      []string          `json:"reasons"`
	EvidenceKeys []string          `json:"evidence_keys,omitempty"`
	Details      map[string]string `json:"details,omitempty"`
}

type TaiwanInterpretationComponents struct {
	Price                   TaiwanInterpretationComponent `json:"price"`
	Market                  TaiwanInterpretationComponent `json:"market"`
	PriceMarketRelationship TaiwanInterpretationComponent `json:"price_market_relationship"`
	Industry                TaiwanInterpretationComponent `json:"industry"`
	Institutional           TaiwanInterpretationComponent `json:"institutional"`
	Margin                  TaiwanInterpretationComponent `json:"margin"`
	Fundamentals            TaiwanInterpretationComponent `json:"fundamentals"`
}

type TaiwanInterpretationDataQuality struct {
	AvailableComponents     []string `json:"available_components"`
	IndeterminateComponents []string `json:"indeterminate_components"`
	UnavailableComponents   []string `json:"unavailable_components"`
	StaleComponents         []string `json:"stale_components"`
	PartialComponents       []string `json:"partial_components"`
}

type TaiwanStockInterpretation struct {
	ModelVersion string                          `json:"model_version"`
	Symbol       string                          `json:"symbol"`
	Components   TaiwanInterpretationComponents  `json:"components"`
	DataQuality  TaiwanInterpretationDataQuality `json:"data_quality"`
}

// CalculateTaiwanStockInterpretation is a pure interpretation policy over M4A evidence.
func CalculateTaiwanStockInterpretation(input TaiwanStockIntelligence) TaiwanStockInterpretation {
	result := TaiwanStockInterpretation{ModelVersion: TaiwanStockInterpretationVersion, Symbol: input.Symbol}
	result.Components.Price = interpretTaiwanPrice(input.PriceHistory, input.Quote)
	result.Components.Market = interpretTaiwanMarket(input.MarketContext)
	result.Components.PriceMarketRelationship = interpretPriceMarket(result.Components.Price, result.Components.Market)
	result.Components.Industry = interpretTaiwanIndustry(input.Identity, input.IndustryContext)
	result.Components.Institutional = interpretTaiwanInstitutional(input.Institutional)
	result.Components.Margin = interpretTaiwanMargin(input.Margin)
	result.Components.Fundamentals = interpretTaiwanFundamentals(input.Identity, input.Fundamentals)
	result.DataQuality = interpretationDataQuality(result.Components)
	return result
}

func interpretTaiwanPrice(value TaiwanPriceHistoryEvidence, quote TaiwanQuoteEvidence) TaiwanInterpretationComponent {
	c := component("indeterminate", value.Status, value.Freshness, value.AsOf, "price_history_summary.return_5d_percent", "price_history_summary.return_20d_percent", "quote.status", "quote.freshness")
	c.Details = map[string]string{"quote_status": quote.Status, "quote_freshness": quote.Freshness}
	if value.Return5D == nil || value.Return20D == nil {
		c.Status = "data_insufficient"
		c.Reasons = []string{"completed 5-session and 20-session returns are both required"}
		return c
	}
	c.Reasons = []string{fmt.Sprintf("return_5d_percent %s 0", signWord(*value.Return5D)), fmt.Sprintf("return_20d_percent %s 0", signWord(*value.Return20D))}
	if quote.Status != "" || quote.Freshness != "" {
		c.Reasons = append(c.Reasons, fmt.Sprintf("quote status is %s with %s freshness and is not used in completed-return direction", quote.Status, quote.Freshness))
	}
	switch {
	case *value.Return5D > 0 && *value.Return20D > 0:
		c.State = "positive"
	case *value.Return5D < 0 && *value.Return20D < 0:
		c.State = "negative"
	default:
		c.State = "mixed"
	}
	return c
}

func interpretTaiwanMarket(value TaiwanMarketContextEvidence) TaiwanInterpretationComponent {
	c := component(value.State, value.Status, value.Freshness, value.AsOf, "market_context.state", "market_context.confidence", "market_context.status", "market_context.freshness")
	if value.Status == "unavailable" || value.State == "" {
		c.State = "indeterminate"
		c.Reasons = []string{"M2B market state is unavailable"}
		return c
	}
	c.Reasons = []string{fmt.Sprintf("M2B market state is %s", value.State), fmt.Sprintf("M2B confidence is %s", value.Confidence)}
	return c
}

func interpretPriceMarket(price, market TaiwanInterpretationComponent) TaiwanInterpretationComponent {
	c := component("indeterminate", "data_insufficient", "", "", "interpretation.components.price.state", "market_context.state")
	if price.State == "indeterminate" || market.State == "indeterminate" {
		c.Reasons = []string{"price and market states are both required"}
		return c
	}
	c.Status = "available"
	c.Reasons = []string{fmt.Sprintf("price state is %s", price.State), fmt.Sprintf("market state is %s", market.State)}
	marketPositive := market.State == "positive" || market.State == "strong"
	marketNegative := market.State == "weak" || market.State == "stressed" || market.State == "negative"
	switch {
	case price.State == "positive" && marketPositive:
		c.State = "aligned_positive"
	case price.State == "negative" && marketNegative:
		c.State = "aligned_negative"
	case price.State == "positive" && marketNegative:
		c.State = "price_positive_market_weak"
	case price.State == "negative" && marketPositive:
		c.State = "price_negative_market_positive"
	default:
		c.State = "mixed"
	}
	return c
}

func interpretTaiwanIndustry(identity TaiwanIdentityEvidence, value TaiwanIndustryContextEvidence) TaiwanInterpretationComponent {
	c := component("indeterminate", value.Status, value.Freshness, value.AsOf, "industry_context.industry.relative_breadth", "industry_context.industry.relative_capital", "industry_context.industry.benchmark_scope")
	if identity.SecurityType != foundation.SecurityTypeStock {
		c.Status, c.State, c.Reasons = "not_applicable", "not_applicable", []string{"industry interpretation is not applicable to non-stock security types"}
		return c
	}
	if value.Status == "unavailable" {
		c.Reasons = []string{"M3 industry evidence is unavailable"}
		return c
	}
	if value.Industry == nil || value.Industry.RelativeBreadth == nil || value.Industry.RelativeCapital == nil {
		c.Status = "data_insufficient"
		c.Reasons = []string{"industry relative breadth and relative capital are both required"}
		return c
	}
	c.Reasons = []string{fmt.Sprintf("relative_breadth %s 0 versus %s", signWord(*value.Industry.RelativeBreadth), value.Industry.BenchmarkScope), fmt.Sprintf("relative_capital %s 0 versus %s", signWord(*value.Industry.RelativeCapital), value.Industry.BenchmarkScope)}
	switch {
	case *value.Industry.RelativeBreadth > 0 && *value.Industry.RelativeCapital > 0:
		c.State = "supportive"
	case *value.Industry.RelativeBreadth < 0 && *value.Industry.RelativeCapital < 0:
		c.State = "weak"
	default:
		c.State = "mixed"
	}
	return c
}

func interpretTaiwanInstitutional(value TaiwanInstitutionalEvidence) TaiwanInterpretationComponent {
	c := component("indeterminate", value.Status, value.Freshness, value.AsOf, "institutional.data.data[].foreign_official_net", "institutional.data.data[].investment_trust_official_net", "institutional.data.data[].dealer_official_net")
	if value.Status == "unavailable" {
		c.Reasons = []string{"institutional section is unavailable"}
		return c
	}
	if value.Data == nil || len(value.Data.Data) == 0 {
		c.Status, c.Reasons = "data_insufficient", []string{"latest official institutional flow is unavailable"}
		return c
	}
	latest := latestInstitutional(value.Data.Data)
	c.AsOf = latest.TradeDate
	if latest.Meta.Status != "official" || !containsInterpretationField(latest.Meta.AvailableFields, "raw_net") {
		c.Status = "data_insufficient"
		c.Reasons = []string{"a complete official observation containing foreign, investment trust, and dealer net values is required"}
		return c
	}
	c.Details = map[string]string{"foreign": signedFlow(latest.ForeignOfficialNet), "investment_trust": signedFlow(latest.InvestmentTrustOfficialNet), "dealer": signedFlow(latest.DealerOfficialNet)}
	c.Reasons = []string{fmt.Sprintf("foreign_official_net %s 0", integerSignWord(latest.ForeignOfficialNet)), fmt.Sprintf("investment_trust_official_net %s 0", integerSignWord(latest.InvestmentTrustOfficialNet)), fmt.Sprintf("dealer_official_net %s 0", integerSignWord(latest.DealerOfficialNet)), "no cross-category total is inferred"}
	c.State = consistentFlowState(c.Details)
	return c
}

func interpretTaiwanMargin(value TaiwanMarginEvidence) TaiwanInterpretationComponent {
	c := component("indeterminate", value.Status, value.Freshness, value.AsOf, "margin.data.data[].margin_change", "margin.data.data[].short_change")
	if value.Status == "unavailable" {
		c.Reasons = []string{"margin section is unavailable"}
		return c
	}
	if value.Data == nil || len(value.Data.Data) == 0 {
		c.Status, c.Reasons = "data_insufficient", []string{"latest official margin observation is unavailable"}
		return c
	}
	latest := latestMargin(value.Data.Data)
	c.AsOf = latest.TradeDate
	c.Details = map[string]string{"margin_balance": "indeterminate", "short_balance": "indeterminate"}
	if latest.MarginChange != nil {
		c.Details["margin_balance"] = changeState(*latest.MarginChange, "margin")
	}
	if latest.ShortChange != nil {
		c.Details["short_balance"] = changeState(*latest.ShortChange, "short")
	}
	if latest.MarginChange == nil && latest.ShortChange == nil {
		c.Status, c.Reasons = "data_insufficient", []string{"margin_change and short_change are unavailable"}
		return c
	}
	c.State = "reported"
	c.Reasons = []string{fmt.Sprintf("margin_change is %s", c.Details["margin_balance"]), fmt.Sprintf("short_change is %s", c.Details["short_balance"]), "balance changes are interpreted independently; no bullish or bearish meaning is inferred"}
	return c
}

func interpretTaiwanFundamentals(identity TaiwanIdentityEvidence, value TaiwanFundamentalsEvidence) TaiwanInterpretationComponent {
	c := component("indeterminate", value.Status, value.Freshness, value.AsOf, "fundamentals.data.monthly_revenue[].official_yoy_percent")
	if identity.SecurityType != foundation.SecurityTypeStock {
		c.Status, c.State, c.Reasons = "not_applicable", "not_applicable", []string{"ordinary-stock fundamentals are not applicable"}
		return c
	}
	if value.Status == "unavailable" {
		c.Reasons = []string{"fundamentals section is unavailable"}
		return c
	}
	if value.Data == nil || len(value.Data.Revenue) == 0 {
		c.Status, c.Reasons = "data_insufficient", []string{"official monthly revenue YoY is unavailable"}
		return c
	}
	latest := value.Data.Revenue[len(value.Data.Revenue)-1]
	for _, item := range value.Data.Revenue {
		if item.Period > latest.Period {
			latest = item
		}
	}
	if latest.OfficialYoY == nil {
		c.Status, c.Reasons = "data_insufficient", []string{"latest monthly revenue has no official YoY value"}
		return c
	}
	c.AsOf = latest.Period
	c.Reasons = []string{fmt.Sprintf("official monthly revenue YoY %s 0", signWord(*latest.OfficialYoY))}
	switch {
	case *latest.OfficialYoY > 0:
		c.State = "positive"
	case *latest.OfficialYoY < 0:
		c.State = "negative"
	default:
		c.State = "flat"
	}
	return c
}

func interpretationDataQuality(c TaiwanInterpretationComponents) TaiwanInterpretationDataQuality {
	q := TaiwanInterpretationDataQuality{}
	items := map[string]TaiwanInterpretationComponent{"price": c.Price, "market": c.Market, "price_market_relationship": c.PriceMarketRelationship, "industry": c.Industry, "institutional": c.Institutional, "margin": c.Margin, "fundamentals": c.Fundamentals}
	for name, item := range items {
		switch {
		case item.Status == "unavailable" || item.Status == "not_applicable":
			q.UnavailableComponents = append(q.UnavailableComponents, name)
		case item.State == "indeterminate":
			q.IndeterminateComponents = append(q.IndeterminateComponents, name)
		default:
			q.AvailableComponents = append(q.AvailableComponents, name)
		}
		if item.Freshness == "stale" {
			q.StaleComponents = append(q.StaleComponents, name)
		}
		if item.Status == "partial" {
			q.PartialComponents = append(q.PartialComponents, name)
		}
	}
	sort.Strings(q.AvailableComponents)
	sort.Strings(q.IndeterminateComponents)
	sort.Strings(q.UnavailableComponents)
	sort.Strings(q.StaleComponents)
	sort.Strings(q.PartialComponents)
	return q
}

func component(state, status, freshness, asOf string, keys ...string) TaiwanInterpretationComponent {
	return TaiwanInterpretationComponent{State: state, Status: status, Freshness: freshness, AsOf: asOf, Reasons: []string{}, EvidenceKeys: keys}
}
func signWord(v float64) string {
	if v > 0 {
		return ">"
	}
	if v < 0 {
		return "<"
	}
	return "="
}
func integerSignWord(v int64) string { return signWord(float64(v)) }
func signedFlow(v int64) string {
	if v > 0 {
		return "net_buy"
	}
	if v < 0 {
		return "net_sell"
	}
	return "neutral"
}
func changeState(v int64, prefix string) string {
	if v > 0 {
		return prefix + "_increased"
	}
	if v < 0 {
		return prefix + "_decreased"
	}
	return "unchanged"
}
func consistentFlowState(values map[string]string) string {
	first := ""
	for _, v := range values {
		if first == "" {
			first = v
		} else if v != first {
			return "mixed"
		}
	}
	return first
}
func latestInstitutional(values []foundation.InstitutionalFlow) foundation.InstitutionalFlow {
	latest := values[0]
	for _, v := range values[1:] {
		if v.TradeDate > latest.TradeDate {
			latest = v
		}
	}
	return latest
}
func latestMargin(values []foundation.MarginTrading) foundation.MarginTrading {
	latest := values[0]
	for _, v := range values[1:] {
		if v.TradeDate > latest.TradeDate {
			latest = v
		}
	}
	return latest
}
func appendUnique(values []string, target string) []string {
	for _, v := range values {
		if v == target {
			return values
		}
	}
	return append(values, target)
}
func containsInterpretationField(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
