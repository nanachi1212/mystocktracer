package taiwan

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"easy-stock/backend/internal/foundation"
)

type chipSnapshot struct {
	payload   monthlyResponse
	expiresAt time.Time
}

func (c *Client) Institutional(ctx context.Context, security foundation.SecurityIdentity, limit int) (foundation.InstitutionalHistory, error) {
	dates, err := c.tradingDates(ctx, security, limit)
	if err != nil {
		return foundation.InstitutionalHistory{}, err
	}
	data := make([]foundation.InstitutionalFlow, 0, len(dates))
	for _, date := range dates {
		row, rowErr := c.institutionalDay(ctx, security, date)
		if rowErr == nil {
			data = append(data, row)
		}
	}
	if len(data) == 0 {
		return foundation.InstitutionalHistory{}, fmt.Errorf("official institutional data is unavailable for %s", security.Canonical)
	}
	meta := data[len(data)-1].Meta
	if len(data) < len(dates) {
		meta.Status = "data_insufficient"
		meta.FallbackReason = fmt.Sprintf("official rows available for %d of %d requested trading dates", len(data), len(dates))
	}
	return foundation.InstitutionalHistory{Security: security, Data: data, Summary: institutionalSummary(data), Meta: meta}, nil
}

func (c *Client) Margin(ctx context.Context, security foundation.SecurityIdentity, limit int) (foundation.MarginHistory, error) {
	dates, err := c.tradingDates(ctx, security, limit)
	if err != nil {
		return foundation.MarginHistory{}, err
	}
	data := make([]foundation.MarginTrading, 0, len(dates))
	for _, date := range dates {
		row, rowErr := c.marginDay(ctx, security, date)
		if rowErr == nil {
			data = append(data, row)
		}
	}
	if len(data) == 0 {
		return foundation.MarginHistory{}, fmt.Errorf("official margin data is unavailable for %s", security.Canonical)
	}
	meta := data[len(data)-1].Meta
	if len(data) < len(dates) {
		meta.Status = "data_insufficient"
		meta.FallbackReason = fmt.Sprintf("official rows available for %d of %d requested trading dates", len(data), len(dates))
	}
	return foundation.MarginHistory{Security: security, Data: data, Meta: meta}, nil
}

func (c *Client) tradingDates(ctx context.Context, security foundation.SecurityIdentity, limit int) ([]time.Time, error) {
	if limit < 1 || limit > 60 {
		return nil, fmt.Errorf("limit must be between 1 and 60")
	}
	lines, err := c.KLine(ctx, security, limit)
	if err != nil {
		return nil, err
	}
	dates := make([]time.Time, 0, len(lines))
	for _, line := range lines {
		dates = append(dates, line.Time.In(taipei()))
	}
	return dates, nil
}

func (c *Client) institutionalDay(ctx context.Context, security foundation.SecurityIdentity, date time.Time) (foundation.InstitutionalFlow, error) {
	var sourceURL string
	if security.Exchange == "TWSE" {
		sourceURL = "https://www.twse.com.tw/rwd/zh/fund/T86?response=json&selectType=ALLBUT0999&date=" + date.Format("20060102")
	} else if security.Exchange == "TPEX" {
		sourceURL = "https://www.tpex.org.tw/www/zh-tw/insti/dailyTrade?response=json&type=Daily&sect=AL&date=" + date.Format("2006/01/02")
	} else {
		return foundation.InstitutionalFlow{}, fmt.Errorf("unsupported Taiwan exchange %q", security.Exchange)
	}
	payload, err := c.chipDay(ctx, sourceURL)
	if err != nil {
		return foundation.InstitutionalFlow{}, err
	}
	rows := payload.Data
	if len(rows) == 0 && len(payload.Tables) > 0 {
		rows = payload.Tables[0].Data
	}
	for _, row := range rows {
		if len(row) > 0 && strings.TrimSpace(row[0]) == security.Code {
			if security.Exchange == "TWSE" {
				return parseTWSEInstitutional(security, date, sourceURL, row)
			}
			return parseTPExInstitutional(security, date, sourceURL, row)
		}
	}
	return foundation.InstitutionalFlow{}, fmt.Errorf("%s has no institutional row on %s", security.Canonical, date.Format("2006-01-02"))
}

func parseTWSEInstitutional(s foundation.SecurityIdentity, date time.Time, sourceURL string, row []string) (foundation.InstitutionalFlow, error) {
	if len(row) < 19 {
		return foundation.InstitutionalFlow{}, fmt.Errorf("TWSE institutional row has %d columns", len(row))
	}
	v, err := integers(row[2:19])
	if err != nil {
		return foundation.InstitutionalFlow{}, err
	}
	return buildInstitutional(s, date, sourceURL, v[0], v[1], v[2], v[6], v[7], v[8], v[10]+v[13], v[11]+v[14], v[9], v[10], v[11], v[12], v[13], v[14], v[15]), nil
}

func parseTPExInstitutional(s foundation.SecurityIdentity, date time.Time, sourceURL string, row []string) (foundation.InstitutionalFlow, error) {
	if len(row) < 24 {
		return foundation.InstitutionalFlow{}, fmt.Errorf("TPEx institutional row has %d columns", len(row))
	}
	v, err := integers(row[2:24])
	if err != nil {
		return foundation.InstitutionalFlow{}, err
	}
	return buildInstitutional(s, date, sourceURL, v[0], v[1], v[2], v[9], v[10], v[11], v[18], v[19], v[20], v[12], v[13], v[14], v[15], v[16], v[17]), nil
}

func buildInstitutional(s foundation.SecurityIdentity, date time.Time, sourceURL string, fb, fs, fn, tb, ts, tn, db, ds, dn, pb, ps, pn, hb, hs, hn int64) foundation.InstitutionalFlow {
	computedForeign, computedTrust := fb-fs, tb-ts
	computedProp, computedHedge := pb-ps, hb-hs
	computedDealer := computedProp + computedHedge
	discrepancies := []string{}
	if computedForeign != fn {
		discrepancies = append(discrepancies, "foreign net differs from official")
	}
	if computedTrust != tn {
		discrepancies = append(discrepancies, "investment trust net differs from official")
	}
	if computedDealer != dn {
		discrepancies = append(discrepancies, "dealer aggregate differs from official")
	}
	if computedProp != pn {
		discrepancies = append(discrepancies, "dealer proprietary net differs from official")
	}
	if computedHedge != hn {
		discrepancies = append(discrepancies, "dealer hedge net differs from official")
	}
	meta := officialMeta(strings.ToLower(s.Exchange)+":institutional", sourceURL, date)
	meta.Status = "official"
	meta.AvailableFields = []string{"unit:shares", "raw_net", "computed_net"}
	return foundation.InstitutionalFlow{Canonical: s.Canonical, Code: s.Code, Name: s.Name, Exchange: s.Exchange, TradeDate: date.Format("2006-01-02"), Unit: "shares", ForeignBuy: fb, ForeignSell: fs, ForeignNet: computedForeign, ForeignOfficialNet: fn, InvestmentTrustBuy: tb, InvestmentTrustSell: ts, InvestmentTrustNet: computedTrust, InvestmentTrustOfficialNet: tn, DealerBuy: db, DealerSell: ds, DealerNet: computedDealer, DealerOfficialNet: dn, DealerProprietaryBuy: pb, DealerProprietarySell: ps, DealerProprietaryNet: computedProp, DealerHedgeBuy: hb, DealerHedgeSell: hs, DealerHedgeNet: computedHedge, RetrievedAt: time.Now(), Meta: meta, Discrepancies: discrepancies}
}

func (c *Client) marginDay(ctx context.Context, security foundation.SecurityIdentity, date time.Time) (foundation.MarginTrading, error) {
	var sourceURL string
	if security.Exchange == "TWSE" {
		sourceURL = "https://www.twse.com.tw/rwd/zh/marginTrading/MI_MARGN?response=json&selectType=ALL&date=" + date.Format("20060102")
	} else if security.Exchange == "TPEX" {
		sourceURL = "https://www.tpex.org.tw/www/zh-tw/margin/balance?response=json&date=" + date.Format("2006/01/02")
	} else {
		return foundation.MarginTrading{}, fmt.Errorf("unsupported Taiwan exchange %q", security.Exchange)
	}
	payload, err := c.chipDay(ctx, sourceURL)
	if err != nil {
		return foundation.MarginTrading{}, err
	}
	rows := payload.Data
	if len(rows) == 0 {
		for _, table := range payload.Tables {
			if len(table.Data) > 0 && len(table.Data[0]) >= 16 {
				rows = table.Data
				break
			}
		}
	}
	for _, row := range rows {
		if len(row) > 0 && strings.TrimSpace(row[0]) == security.Code {
			if security.Exchange == "TWSE" {
				return parseMargin(security, date, sourceURL, row, 2, 3, 4, 5, 6, 8, 9, 10, 11, 12, 15)
			}
			return parseMargin(security, date, sourceURL, row, 3, 4, 5, 2, 6, 12, 11, 13, 10, 14, 19)
		}
	}
	return foundation.MarginTrading{}, fmt.Errorf("%s has no margin row on %s", security.Canonical, date.Format("2006-01-02"))
}

func parseMargin(s foundation.SecurityIdentity, date time.Time, sourceURL string, row []string, mb, ms, mc, mprev, mbal, scover, ssell, sred, sprev, sbal, note int) (foundation.MarginTrading, error) {
	max := note
	if len(row) <= max {
		return foundation.MarginTrading{}, fmt.Errorf("margin row has %d columns", len(row))
	}
	values := []*int64{}
	for _, index := range []int{mprev, mb, ms, mc, mbal, sprev, ssell, scover, sred, sbal} {
		value, err := optionalLots(row[index])
		if err != nil {
			return foundation.MarginTrading{}, err
		}
		values = append(values, value)
	}
	marginChange := difference(values[4], values[0])
	shortChange := difference(values[9], values[5])
	var ratio *float64
	if values[4] != nil && *values[4] > 0 && values[9] != nil {
		// Taiwan market convention here: short balance / margin balance * 100.
		v := float64(*values[9]) / float64(*values[4]) * 100
		ratio = &v
	}
	meta := officialMeta(strings.ToLower(s.Exchange)+":margin", sourceURL, date)
	meta.Status = "official"
	meta.AvailableFields = []string{"normalized_unit:shares", "raw_unit:lots", "lot_multiplier:1000"}
	return foundation.MarginTrading{Canonical: s.Canonical, Code: s.Code, Name: s.Name, Exchange: s.Exchange, TradeDate: date.Format("2006-01-02"), Unit: "shares", MarginPreviousBalance: values[0], MarginBuy: values[1], MarginSell: values[2], MarginCashRedemption: values[3], MarginBalance: values[4], MarginChange: marginChange, ShortPreviousBalance: values[5], ShortSell: values[6], ShortCover: values[7], ShortStockRedemption: values[8], ShortBalance: values[9], ShortChange: shortChange, ShortMarginRatio: ratio, Note: strings.TrimSpace(row[note]), RetrievedAt: time.Now(), Meta: meta}, nil
}

func (c *Client) chipDay(ctx context.Context, sourceURL string) (monthlyResponse, error) {
	c.chipMu.Lock()
	defer c.chipMu.Unlock()
	if cached, ok := c.chipDays[sourceURL]; ok && time.Now().Before(cached.expiresAt) {
		return cached.payload, nil
	}
	var payload monthlyResponse
	if err := c.getJSON(ctx, sourceURL, &payload); err != nil {
		return payload, err
	}
	c.chipDays[sourceURL] = chipSnapshot{payload: payload, expiresAt: time.Now().Add(6 * time.Hour)}
	return payload, nil
}
func optionalLots(raw string) (*int64, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	value, err := integer(raw)
	if err != nil {
		return nil, err
	}
	value *= 1000
	return &value, nil
}
func difference(current, previous *int64) *int64 {
	if current == nil || previous == nil {
		return nil
	}
	value := *current - *previous
	return &value
}
func integer(raw string) (int64, error) {
	cleaned := strings.ReplaceAll(strings.TrimSpace(raw), ",", "")
	return strconv.ParseInt(cleaned, 10, 64)
}
func integers(values []string) ([]int64, error) {
	out := make([]int64, len(values))
	for index, value := range values {
		parsed, err := integer(value)
		if err != nil {
			return nil, err
		}
		out[index] = parsed
	}
	return out, nil
}

func institutionalSummary(data []foundation.InstitutionalFlow) foundation.InstitutionalSummary {
	summary := foundation.InstitutionalSummary{}
	for index := len(data) - 1; index >= 0; index-- {
		distance := len(data) - index
		if distance <= 5 {
			summary.ForeignNet5D += data[index].ForeignNet
			summary.InvestmentTrustNet5D += data[index].InvestmentTrustNet
			summary.DealerNet5D += data[index].DealerNet
		}
		if distance <= 20 {
			summary.ForeignNet20D += data[index].ForeignNet
			summary.InvestmentTrustNet20D += data[index].InvestmentTrustNet
			summary.DealerNet20D += data[index].DealerNet
		}
	}
	if len(data) > 0 {
		summary.ForeignConsecutiveBuyDays, summary.ForeignConsecutiveSellDays = streak(data, func(v foundation.InstitutionalFlow) int64 { return v.ForeignNet })
		summary.InvestmentTrustConsecutiveBuyDays, summary.InvestmentTrustConsecutiveSellDays = streak(data, func(v foundation.InstitutionalFlow) int64 { return v.InvestmentTrustNet })
	}
	return summary
}
func streak(data []foundation.InstitutionalFlow, value func(foundation.InstitutionalFlow) int64) (int, int) {
	buy, sell := 0, 0
	for index := len(data) - 1; index >= 0; index-- {
		current := value(data[index])
		if current > 0 && sell == 0 {
			buy++
			continue
		}
		if current < 0 && buy == 0 {
			sell++
			continue
		}
		break
	}
	return buy, sell
}
