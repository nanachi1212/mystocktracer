package stockanalysis

import (
	"time"

	"easy-stock/backend/internal/foundation"
	"easy-stock/backend/internal/marketemotion"
	"easy-stock/backend/internal/sector"
)

const TaiwanStockIntelligenceVersion = "taiwan_stock_intelligence_v1"

type TaiwanEvidenceStatus struct {
	Status    string                 `json:"status"`
	Freshness string                 `json:"freshness,omitempty"`
	AsOf      string                 `json:"as_of,omitempty"`
	Source    string                 `json:"source,omitempty"`
	SourceURL string                 `json:"source_url,omitempty"`
	FetchedAt *time.Time             `json:"fetched_at,omitempty"`
	Reason    string                 `json:"reason,omitempty"`
	Meta      *foundation.SourceMeta `json:"meta,omitempty"`
}

type TaiwanIdentityEvidence struct {
	CanonicalSymbol        string                  `json:"canonical_symbol"`
	Code                   string                  `json:"code"`
	Name                   string                  `json:"name"`
	Exchange               string                  `json:"exchange"`
	Currency               string                  `json:"currency"`
	Timezone               string                  `json:"timezone"`
	SecurityType           foundation.SecurityType `json:"security_type"`
	IndustryCode           string                  `json:"industry_code,omitempty"`
	IndustryID             string                  `json:"industry_id,omitempty"`
	IndustryName           string                  `json:"industry_name,omitempty"`
	IndustryTaxonomySource string                  `json:"industry_taxonomy_source,omitempty"`
	ClassificationBasis    string                  `json:"classification_basis"`
}

type TaiwanQuoteEvidence struct {
	TaiwanEvidenceStatus
	TargetLatestCompletedTradingDate string            `json:"target_latest_completed_trading_date,omitempty"`
	Data                             *foundation.Quote `json:"data,omitempty"`
}

type TaiwanPriceHistoryEvidence struct {
	TaiwanEvidenceStatus
	RequestedWindow int                `json:"requested_window"`
	AvailableWindow int                `json:"available_window"`
	LatestBarDate   string             `json:"latest_bar_date,omitempty"`
	Return5D        *float64           `json:"return_5d_percent"`
	Return20D       *float64           `json:"return_20d_percent"`
	Bars            []foundation.KLine `json:"bars,omitempty"`
}

type TaiwanFundamentalsEvidence struct {
	TaiwanEvidenceStatus
	Data *foundation.TaiwanFundamentals `json:"data,omitempty"`
}
type TaiwanInstitutionalEvidence struct {
	TaiwanEvidenceStatus
	Data *foundation.InstitutionalHistory `json:"data,omitempty"`
}
type TaiwanMarginEvidence struct {
	TaiwanEvidenceStatus
	Data *foundation.MarginHistory `json:"data,omitempty"`
}

type TaiwanMarketContextEvidence struct {
	TaiwanEvidenceStatus
	Scope                string   `json:"scope"`
	AdvanceRatio         *float64 `json:"advance_ratio"`
	AdvancingAmountRatio *float64 `json:"advancing_amount_ratio"`
	State                string   `json:"state,omitempty"`
	Confidence           string   `json:"confidence,omitempty"`
}

type TaiwanIndustryContextEvidence struct {
	TaiwanEvidenceStatus
	Industry       *sector.TaiwanIndustry `json:"industry,omitempty"`
	TaxonomyStatus string                 `json:"taxonomy_status,omitempty"`
}

type TaiwanStockIntelligence struct {
	ModelVersion    string                        `json:"model_version"`
	Symbol          string                        `json:"symbol"`
	Identity        TaiwanIdentityEvidence        `json:"identity"`
	Quote           TaiwanQuoteEvidence           `json:"quote"`
	PriceHistory    TaiwanPriceHistoryEvidence    `json:"price_history_summary"`
	Fundamentals    TaiwanFundamentalsEvidence    `json:"fundamentals"`
	Institutional   TaiwanInstitutionalEvidence   `json:"institutional"`
	Margin          TaiwanMarginEvidence          `json:"margin"`
	MarketContext   TaiwanMarketContextEvidence   `json:"market_context"`
	IndustryContext TaiwanIndustryContextEvidence `json:"industry_context"`
}

func NewTaiwanStockIntelligence(identity foundation.SecurityIdentity, quote *foundation.Quote, lines []foundation.KLine, fundamentals *foundation.TaiwanFundamentals, institutional *foundation.InstitutionalHistory, margin *foundation.MarginHistory, breadth foundation.TaiwanBreadthScope, emotion marketemotion.TaiwanEmotionScope, radar sector.TaiwanIndustryScope) TaiwanStockIntelligence {
	result := TaiwanStockIntelligence{ModelVersion: TaiwanStockIntelligenceVersion, Symbol: identity.Canonical,
		Identity: TaiwanIdentityEvidence{CanonicalSymbol: identity.Canonical, Code: identity.Code, Name: identity.Name, Exchange: identity.Exchange, Currency: identity.Currency, Timezone: identity.Timezone, SecurityType: identity.Type, IndustryCode: identity.Industry, ClassificationBasis: "current_reference"}}
	result.Quote = quoteEvidence(quote)
	result.Quote.TargetLatestCompletedTradingDate = breadth.TargetLatestTradingDate
	result.PriceHistory = historyEvidence(lines, 20)
	result.Fundamentals = fundamentalsEvidence(fundamentals, identity.Type == foundation.SecurityTypeStock)
	result.Institutional = institutionalEvidence(institutional)
	result.Margin = marginEvidence(margin)
	result.MarketContext = TaiwanMarketContextEvidence{TaiwanEvidenceStatus: TaiwanEvidenceStatus{Status: breadth.Status, Freshness: breadth.Freshness, AsOf: stringValue(breadth.AsOf), Source: "M2A/M2B Taiwan market context"}, Scope: identity.Exchange, AdvanceRatio: breadth.AdvanceRatio, AdvancingAmountRatio: breadth.AdvancingAmountRatio, State: emotion.State, Confidence: emotion.Confidence}
	result.IndustryContext = TaiwanIndustryContextEvidence{TaiwanEvidenceStatus: TaiwanEvidenceStatus{Status: "unavailable", Freshness: radar.Freshness, AsOf: stringValue(radar.AsOf), Source: "M3 Taiwan industry radar"}, TaxonomyStatus: radar.TaxonomyStatus}
	if identity.Type != foundation.SecurityTypeStock {
		result.IndustryContext.Reason = "industry context is not applicable to non-stock security types"
	} else {
		id := identity.Exchange + ":" + identity.Industry
		for i := range radar.Industries {
			if radar.Industries[i].IndustryID == id {
				item := radar.Industries[i]
				result.IndustryContext.Industry = &item
				result.IndustryContext.Status = radar.Status
				result.Identity.IndustryID, result.Identity.IndustryName, result.Identity.IndustryTaxonomySource = item.IndustryID, item.IndustryName, item.TaxonomySource
				break
			}
		}
		if result.IndustryContext.Industry == nil {
			result.IndustryContext.Reason = "official industry classification is unavailable"
		}
	}
	return result
}

func quoteEvidence(value *foundation.Quote) TaiwanQuoteEvidence {
	if value == nil {
		return TaiwanQuoteEvidence{TaiwanEvidenceStatus: TaiwanEvidenceStatus{Status: "unavailable", Reason: "quote unavailable"}}
	}
	meta := value.Meta
	return TaiwanQuoteEvidence{TaiwanEvidenceStatus: statusFromMeta(meta), Data: value}
}
func historyEvidence(lines []foundation.KLine, requested int) TaiwanPriceHistoryEvidence {
	result := TaiwanPriceHistoryEvidence{RequestedWindow: requested, AvailableWindow: len(lines), Return5D: periodReturn(lines, 5), Return20D: periodReturn(lines, 20)}
	if len(lines) == 0 {
		result.Status = "unavailable"
		result.Reason = "price history unavailable"
		return result
	}
	latest := lines[len(lines)-1]
	result.Status = "available"
	result.Freshness = latest.Meta.Freshness
	result.AsOf = latest.Time.Format("2006-01-02")
	result.Source = latest.Meta.Source
	result.SourceURL = latest.Meta.SourceURL
	result.LatestBarDate = result.AsOf
	result.Bars = append([]foundation.KLine(nil), lines...)
	result.Meta = &latest.Meta
	return result
}
func periodReturn(lines []foundation.KLine, window int) *float64 {
	// An N-session return needs N+1 completed closes.
	if len(lines) <= window || lines[len(lines)-1-window].Close <= 0 {
		return nil
	}
	value := (lines[len(lines)-1].Close/lines[len(lines)-1-window].Close - 1) * 100
	return &value
}
func fundamentalsEvidence(value *foundation.TaiwanFundamentals, applicable bool) TaiwanFundamentalsEvidence {
	if !applicable {
		return TaiwanFundamentalsEvidence{TaiwanEvidenceStatus: TaiwanEvidenceStatus{Status: "not_applicable", Reason: "ordinary-stock fundamentals are not applicable"}}
	}
	if value == nil {
		return TaiwanFundamentalsEvidence{TaiwanEvidenceStatus: TaiwanEvidenceStatus{Status: "unavailable", Reason: "fundamentals unavailable"}}
	}
	meta := value.Meta
	return TaiwanFundamentalsEvidence{TaiwanEvidenceStatus: statusFromMeta(meta), Data: value}
}
func institutionalEvidence(value *foundation.InstitutionalHistory) TaiwanInstitutionalEvidence {
	if value == nil {
		return TaiwanInstitutionalEvidence{TaiwanEvidenceStatus: TaiwanEvidenceStatus{Status: "unavailable", Reason: "institutional data unavailable"}}
	}
	meta := value.Meta
	return TaiwanInstitutionalEvidence{TaiwanEvidenceStatus: statusFromMeta(meta), Data: value}
}
func marginEvidence(value *foundation.MarginHistory) TaiwanMarginEvidence {
	if value == nil {
		return TaiwanMarginEvidence{TaiwanEvidenceStatus: TaiwanEvidenceStatus{Status: "unavailable", Reason: "margin data unavailable"}}
	}
	meta := value.Meta
	return TaiwanMarginEvidence{TaiwanEvidenceStatus: statusFromMeta(meta), Data: value}
}
func statusFromMeta(meta foundation.SourceMeta) TaiwanEvidenceStatus {
	status := meta.Status
	if status == "" {
		status = "available"
	}
	fetched := meta.FetchedAt
	return TaiwanEvidenceStatus{Status: status, Freshness: meta.Freshness, AsOf: meta.TradeDate, Source: meta.Source, SourceURL: meta.SourceURL, FetchedAt: &fetched, Meta: &meta}
}
func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
