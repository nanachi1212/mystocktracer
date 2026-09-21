package foundation

import "time"

type SourceMeta struct {
	Source          string     `json:"source"`
	SourceURL       string     `json:"source_url,omitempty"`
	AvailableFields []string   `json:"available_fields,omitempty"`
	FetchedAt       time.Time  `json:"fetched_at"`
	LatencyMS       int64      `json:"latency_ms"`
	Stale           bool       `json:"stale"`
	TradeDate       string     `json:"trade_date,omitempty"`
	SnapshotID      string     `json:"snapshot_id,omitempty"`
	NextRefreshAt   *time.Time `json:"next_refresh_at,omitempty"`
	FallbackReason  string     `json:"fallback_reason,omitempty"`
	CarryForward    bool       `json:"carry_forward,omitempty"`
	Status          string     `json:"status,omitempty"`
	Freshness       string     `json:"freshness,omitempty"`
	IsRealtime      bool       `json:"is_realtime"`
}

type Quote struct {
	Symbol        string     `json:"symbol"`
	Name          string     `json:"name"`
	Price         float64    `json:"price"`
	Open          float64    `json:"open"`
	PreviousClose float64    `json:"previous_close"`
	High          float64    `json:"high"`
	Low           float64    `json:"low"`
	Change        float64    `json:"change"`
	ChangePercent float64    `json:"change_percent"`
	TradeTime     time.Time  `json:"trade_time,omitempty"`
	Meta          SourceMeta `json:"meta"`
}

type KLine struct {
	Symbol        string     `json:"symbol"`
	Time          time.Time  `json:"time"`
	Open          float64    `json:"open"`
	High          float64    `json:"high"`
	Low           float64    `json:"low"`
	Close         float64    `json:"close"`
	PreviousClose float64    `json:"previous_close,omitempty"`
	Volume        float64    `json:"volume"`
	Amount        float64    `json:"amount"`
	TurnoverRate  float64    `json:"turnover_rate,omitempty"`
	ChangePercent float64    `json:"change_percent,omitempty"`
	Meta          SourceMeta `json:"meta"`
}

// StockCatalogEntry is one A-share from EastMoney's stock-selection catalog.
// Industry and Concepts are membership evidence used to build thematic
// constituent pools without maintaining stock-code lists in local rules.
// StockThemeAttribution is an authoritative or cached per-stock theme label.
// It keeps source provenance so downstream analysis can prefer Kaipanla data
// without conflating it with broad industry/catalog fallbacks.
// StockBusinessProfile describes what the company primarily does. It is kept
// separate from market concepts because a broad concept membership is not
// evidence that the stock is currently being traded as that theme.
// StockFundamentals contains the latest reported financial snapshot used by
// the non-short-term stock route. Values follow EastMoney's published F10
// units: amounts are CNY and percentage fields are percentage points.
// MarketLimitEvent represents one stock in a daily limit-event pool that is
// not necessarily still sealed at the close. It is used for final broken-board
// and limit-down pools while LimitUpEvent remains the richer sealed-limit-up
// record used by the ladder.
