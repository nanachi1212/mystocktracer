package taiwan

import (
	"context"
	"strings"
	"time"

	"easy-stock/backend/internal/foundation"
)

func (c *Client) MarketBreadth(ctx context.Context, now time.Time) (foundation.TaiwanMarketBreadth, error) {
	target := c.calendar.LatestCompleted(now, marketCutoffHour, marketCutoffMinute)
	identities, err := c.Directory(ctx)
	if err != nil {
		return foundation.TaiwanMarketBreadth{}, err
	}
	section := c.refreshDaily(ctx, target, identityAllowlist(identities))
	rows := c.dailyOn(target)
	status := map[string]string{"TWSE": "current", "TPEX": "current"}
	for _, exchange := range []string{"TWSE", "TPEX"} {
		if strings.Contains(section.Error, exchange+":") || !hasExchangeRows(rows, exchange) {
			status[exchange] = "unavailable"
		}
	}
	freshness := c.Freshness(now)
	return foundation.CalculateTaiwanMarketBreadth(identities, rows, freshness, status), nil
}

func hasExchangeRows(rows []foundation.TaiwanDailySnapshot, exchange string) bool {
	for _, row := range rows {
		if row.Exchange == exchange {
			return true
		}
	}
	return false
}

// ScreenerSnapshot returns the market-wide daily rows for `now`'s latest completed trading day,
// reusing the exact same bulk refresh path as MarketBreadth (one TWSE + one TPEx request total,
// never per-security) so a Screener never causes N+1 provider calls. Filtering/sorting/pagination
// happen entirely in the httpapi layer over these in-memory rows — this method does no filtering.
func (c *Client) ScreenerSnapshot(ctx context.Context, now time.Time) ([]foundation.TaiwanDailySnapshot, foundation.TaiwanFreshness, error) {
	target := c.calendar.LatestCompleted(now, marketCutoffHour, marketCutoffMinute)
	identities, err := c.Directory(ctx)
	if err != nil {
		return nil, foundation.TaiwanFreshness{}, err
	}
	c.refreshDaily(ctx, target, identityAllowlist(identities))
	rows := c.dailyOn(target)
	return rows, c.Freshness(now), nil
}

// ScreenerInstitutional returns the market-wide institutional-flow rows for `now`'s latest completed
// trading day, reusing the exact same bulk refresh path already used by RefreshMarketWide/MarketBreadth
// (one TWSE + one TPEx request total) — never one request per security. M7D calls this ONLY when a
// Screener request actually needs an institutional field (filter or sort); it must never be invoked
// unconditionally, so a vanilla M7A-style request never pays this cost.
//
// PIT note: this exposes only trade-date freshness (via Freshness()) — it does not assign
// InstitutionalFlow.PublishedAt and makes no claim about an exact official publication timestamp,
// per the M7D.0 availability gate (no such timestamp exists in the official payloads).
func (c *Client) ScreenerInstitutional(ctx context.Context, now time.Time) ([]foundation.InstitutionalFlow, foundation.TaiwanFreshness, error) {
	target := c.calendar.LatestCompleted(now, marketCutoffHour, marketCutoffMinute)
	identities, err := c.Directory(ctx)
	if err != nil {
		return nil, foundation.TaiwanFreshness{}, err
	}
	c.refreshInstitutional(ctx, target, identityAllowlist(identities))
	rows := c.instOn(target)
	return rows, c.Freshness(now), nil
}

// ScreenerMargin returns the market-wide margin/short rows for `now`'s latest completed trading day,
// with the exact same bounded bulk-request and PIT properties as ScreenerInstitutional above. Called
// ONLY when a Screener request actually needs a margin field.
func (c *Client) ScreenerMargin(ctx context.Context, now time.Time) ([]foundation.MarginTrading, foundation.TaiwanFreshness, error) {
	target := c.calendar.LatestCompleted(now, marketCutoffHour, marketCutoffMinute)
	identities, err := c.Directory(ctx)
	if err != nil {
		return nil, foundation.TaiwanFreshness{}, err
	}
	c.refreshMargin(ctx, target, identityAllowlist(identities))
	rows := c.marginOn(target)
	return rows, c.Freshness(now), nil
}
