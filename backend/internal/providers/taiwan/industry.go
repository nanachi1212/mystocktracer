package taiwan

import (
	"context"
	"strings"
	"time"

	"easy-stock/backend/internal/foundation"
	"easy-stock/backend/internal/sector"
)

// IndustryRadar acquires one directory and one bulk daily snapshot, then groups
// the already-acquired market facts in memory. It never calls Quote or KLine.
func (c *Client) IndustryRadar(ctx context.Context, now time.Time) (sector.TaiwanIndustryRadar, error) {
	target := c.calendar.LatestCompleted(now, marketCutoffHour, marketCutoffMinute)
	identities, err := c.Directory(ctx)
	if err != nil {
		return sector.TaiwanIndustryRadar{}, err
	}
	section := c.refreshDaily(ctx, target, identityAllowlist(identities))
	rows := c.dailyOn(target)
	status := map[string]string{"TWSE": "current", "TPEX": "current"}
	for _, exchange := range []string{"TWSE", "TPEX"} {
		if strings.Contains(section.Error, exchange+":") || !hasExchangeRows(rows, exchange) {
			status[exchange] = "unavailable"
		}
	}
	breadth := foundation.CalculateTaiwanMarketBreadth(identities, rows, c.Freshness(now), status)
	return sector.CalculateTaiwanIndustryRadar(identities, rows, breadth), nil
}
