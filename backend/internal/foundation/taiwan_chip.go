package foundation

import "time"

type InstitutionalFlow struct {
	Canonical                  string     `json:"canonical"`
	Code                       string     `json:"code"`
	Name                       string     `json:"name"`
	Exchange                   string     `json:"exchange"`
	TradeDate                  string     `json:"trade_date"`
	Unit                       string     `json:"unit"`
	ForeignBuy                 int64      `json:"foreign_buy"`
	ForeignSell                int64      `json:"foreign_sell"`
	ForeignNet                 int64      `json:"foreign_net"`
	ForeignOfficialNet         int64      `json:"foreign_official_net"`
	InvestmentTrustBuy         int64      `json:"investment_trust_buy"`
	InvestmentTrustSell        int64      `json:"investment_trust_sell"`
	InvestmentTrustNet         int64      `json:"investment_trust_net"`
	InvestmentTrustOfficialNet int64      `json:"investment_trust_official_net"`
	DealerBuy                  int64      `json:"dealer_buy"`
	DealerSell                 int64      `json:"dealer_sell"`
	DealerNet                  int64      `json:"dealer_net"`
	DealerOfficialNet          int64      `json:"dealer_official_net"`
	DealerProprietaryBuy       int64      `json:"dealer_proprietary_buy"`
	DealerProprietarySell      int64      `json:"dealer_proprietary_sell"`
	DealerProprietaryNet       int64      `json:"dealer_proprietary_net"`
	DealerHedgeBuy             int64      `json:"dealer_hedge_buy"`
	DealerHedgeSell            int64      `json:"dealer_hedge_sell"`
	DealerHedgeNet             int64      `json:"dealer_hedge_net"`
	PublishedAt                *time.Time `json:"published_at,omitempty"`
	RetrievedAt                time.Time  `json:"retrieved_at"`
	Meta                       SourceMeta `json:"meta"`
	Discrepancies              []string   `json:"discrepancies,omitempty"`
}

type InstitutionalSummary struct {
	ForeignNet5D                       int64 `json:"foreign_net_5d"`
	ForeignNet20D                      int64 `json:"foreign_net_20d"`
	InvestmentTrustNet5D               int64 `json:"investment_trust_net_5d"`
	InvestmentTrustNet20D              int64 `json:"investment_trust_net_20d"`
	DealerNet5D                        int64 `json:"dealer_net_5d"`
	DealerNet20D                       int64 `json:"dealer_net_20d"`
	ForeignConsecutiveBuyDays          int   `json:"foreign_consecutive_buy_days"`
	ForeignConsecutiveSellDays         int   `json:"foreign_consecutive_sell_days"`
	InvestmentTrustConsecutiveBuyDays  int   `json:"investment_trust_consecutive_buy_days"`
	InvestmentTrustConsecutiveSellDays int   `json:"investment_trust_consecutive_sell_days"`
}

type InstitutionalHistory struct {
	Security SecurityIdentity     `json:"security"`
	Data     []InstitutionalFlow  `json:"data"`
	Summary  InstitutionalSummary `json:"summary"`
	Meta     SourceMeta           `json:"meta"`
}

type MarginTrading struct {
	Canonical             string     `json:"canonical"`
	Code                  string     `json:"code"`
	Name                  string     `json:"name"`
	Exchange              string     `json:"exchange"`
	TradeDate             string     `json:"trade_date"`
	Unit                  string     `json:"unit"`
	MarginPreviousBalance *int64     `json:"margin_previous_balance"`
	MarginBuy             *int64     `json:"margin_buy"`
	MarginSell            *int64     `json:"margin_sell"`
	MarginCashRedemption  *int64     `json:"margin_cash_redemption"`
	MarginBalance         *int64     `json:"margin_balance"`
	MarginChange          *int64     `json:"margin_change"`
	ShortPreviousBalance  *int64     `json:"short_previous_balance"`
	ShortSell             *int64     `json:"short_sell"`
	ShortCover            *int64     `json:"short_cover"`
	ShortStockRedemption  *int64     `json:"short_stock_redemption"`
	ShortBalance          *int64     `json:"short_balance"`
	ShortChange           *int64     `json:"short_change"`
	ShortMarginRatio      *float64   `json:"short_margin_ratio"`
	Note                  string     `json:"note,omitempty"`
	PublishedAt           *time.Time `json:"published_at,omitempty"`
	RetrievedAt           time.Time  `json:"retrieved_at"`
	Meta                  SourceMeta `json:"meta"`
	Discrepancies         []string   `json:"discrepancies,omitempty"`
}

type MarginHistory struct {
	Security SecurityIdentity `json:"security"`
	Data     []MarginTrading  `json:"data"`
	Meta     SourceMeta       `json:"meta"`
}
