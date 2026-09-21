package foundation

import "time"

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
