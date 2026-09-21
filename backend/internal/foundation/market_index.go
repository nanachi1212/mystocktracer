package foundation

import "time"

// MarketIndexSnapshot / MarketIndexSeries are the market-neutral index shapes the Taiwan index
// endpoint (GET /api/v1/tw/indexes) returns. They were extracted from the removed A-share
// market-overview type set in Phase B1; only the index shapes survived because they are the only
// ones the Taiwan product consumes.
type MarketIndexSnapshot struct {
	ID            string     `json:"id"`
	SecID         string     `json:"secid"`
	Code          string     `json:"code"`
	Name          string     `json:"name"`
	Region        string     `json:"region"`
	Market        string     `json:"market"`
	Currency      string     `json:"currency"`
	Price         float64    `json:"price"`
	Change        float64    `json:"change"`
	ChangePercent float64    `json:"change_percent"`
	TradeTime     time.Time  `json:"trade_time,omitempty"`
	Status        string     `json:"status"`
	Meta          SourceMeta `json:"meta"`
}

type MarketIndexSeries struct {
	Index MarketIndexSnapshot `json:"index"`
	Lines []KLine             `json:"lines"`
	Meta  SourceMeta          `json:"meta"`
}
