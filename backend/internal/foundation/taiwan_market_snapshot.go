package foundation

import "time"

type TaiwanDailySnapshot struct {
	Canonical string       `json:"canonical"`
	Code      string       `json:"code"`
	Name      string       `json:"name"`
	Exchange  string       `json:"exchange"`
	Type      SecurityType `json:"security_type"`
	TradeDate string       `json:"trade_date"`
	Open      *float64     `json:"open"`
	High      *float64     `json:"high"`
	Low       *float64     `json:"low"`
	Close     *float64     `json:"close"`
	Change    *float64     `json:"change"`
	NoTrade   bool         `json:"no_trade"`
	Volume    *int64       `json:"volume"`
	Amount    *float64     `json:"amount"`
	Unit      string       `json:"unit"`
	Currency  string       `json:"currency"`
	Meta      SourceMeta   `json:"meta"`
}

type TaiwanFreshness struct {
	DailyAsOf               *string `json:"daily_as_of"`
	InstitutionalAsOf       *string `json:"institutional_as_of"`
	MarginAsOf              *string `json:"margin_as_of"`
	TargetLatestTradingDate string  `json:"target_latest_trading_date"`
	DailyStatus             string  `json:"daily_status"`
	InstitutionalStatus     string  `json:"institutional_status"`
	MarginStatus            string  `json:"margin_status"`
	DailyDaysBehind         *int    `json:"daily_days_behind,omitempty"`
	InstitutionalDaysBehind *int    `json:"institutional_days_behind,omitempty"`
	MarginDaysBehind        *int    `json:"margin_days_behind,omitempty"`
	Timezone                string  `json:"timezone"`
	Cutoff                  string  `json:"cutoff"`
}

type TaiwanRefreshSection struct {
	Status    string `json:"status"`
	TradeDate string `json:"trade_date,omitempty"`
	Rows      int    `json:"rows"`
	Error     string `json:"error,omitempty"`
}

type TaiwanMarketRefresh struct {
	Daily         TaiwanRefreshSection `json:"daily"`
	Institutional TaiwanRefreshSection `json:"institutional"`
	Margin        TaiwanRefreshSection `json:"margin"`
	OverallStatus string               `json:"overall_status"`
	Freshness     TaiwanFreshness      `json:"freshness"`
}

type TaiwanTradingCalendar struct {
	Holidays map[string]bool
}

func (c TaiwanTradingCalendar) IsTradingDay(day time.Time) bool {
	weekday := day.Weekday()
	return weekday != time.Saturday && weekday != time.Sunday && !c.Holidays[day.Format("2006-01-02")]
}

func (c TaiwanTradingCalendar) LatestCompleted(now time.Time, cutoffHour, cutoffMinute int) time.Time {
	local := now.In(time.FixedZone("Asia/Taipei", 8*60*60))
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
	cutoff := time.Date(local.Year(), local.Month(), local.Day(), cutoffHour, cutoffMinute, 0, 0, local.Location())
	if local.Before(cutoff) || !c.IsTradingDay(day) {
		day = day.AddDate(0, 0, -1)
	}
	for !c.IsTradingDay(day) {
		day = day.AddDate(0, 0, -1)
	}
	return day
}
