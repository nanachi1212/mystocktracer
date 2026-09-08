package taiwan

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"easy-stock/backend/internal/foundation"
)

const (
	marketCutoffHour   = 17
	marketCutoffMinute = 30
)

func (c *Client) RefreshMarketWide(ctx context.Context, target time.Time) foundation.TaiwanMarketRefresh {
	identities, directoryErr := c.Directory(ctx)
	if directoryErr != nil {
		failed := foundation.TaiwanRefreshSection{Status: "failed", Error: directoryErr.Error()}
		return foundation.TaiwanMarketRefresh{Daily: failed, Institutional: failed, Margin: failed, OverallStatus: "failed", Freshness: c.Freshness(c.now())}
	}
	byExchange := identityAllowlist(identities)
	result := foundation.TaiwanMarketRefresh{
		Daily:         c.refreshDaily(ctx, target, byExchange),
		Institutional: c.refreshInstitutional(ctx, target, byExchange),
		Margin:        c.refreshMargin(ctx, target, byExchange),
	}
	result.OverallStatus = refreshOverall(result.Daily, result.Institutional, result.Margin)
	result.Freshness = c.Freshness(c.now())
	return result
}

func identityAllowlist(items []foundation.SecurityIdentity) map[string]map[string]foundation.SecurityIdentity {
	out := map[string]map[string]foundation.SecurityIdentity{"TWSE": {}, "TPEX": {}}
	for _, item := range items {
		if exchange, ok := out[item.Exchange]; ok {
			exchange[item.Code] = item
		}
	}
	return out
}

func (c *Client) refreshDaily(ctx context.Context, target time.Time, allow map[string]map[string]foundation.SecurityIdentity) foundation.TaiwanRefreshSection {
	dateKey := target.Format("2006-01-02")
	existing := c.dailyOn(target)
	if len(existing) > 0 && hasExchangeRows(existing, "TWSE") && hasExchangeRows(existing, "TPEX") {
		return sectionResult(dateKey, len(existing), nil)
	}

	c.dailyFlightMu.Lock()
	if flight, active := c.dailyFlights[dateKey]; active {
		c.dailyFlightMu.Unlock()
		flight.wg.Wait()
		return flight.section
	}
	flight := &dailyFlightCall{}
	flight.wg.Add(1)
	c.dailyFlights[dateKey] = flight
	c.dailyFlightMu.Unlock()

	defer func() {
		c.dailyFlightMu.Lock()
		delete(c.dailyFlights, dateKey)
		c.dailyFlightMu.Unlock()
		flight.wg.Done()
	}()

	date := target.In(taipei()).Format("20060102")
	twseURL := c.twseReportBaseURL + "/rwd/zh/afterTrading/MI_INDEX?response=json&type=ALL&date=" + date
	tpexURL := c.tpexReportBaseURL + "/www/zh-tw/afterTrading/dailyQuotes?response=json&date=" + target.In(taipei()).Format("2006/01/02")
	rows := []foundation.TaiwanDailySnapshot{}
	errors := []string{}
	var twse, tpex monthlyResponse
	if err := c.getJSON(ctx, twseURL, &twse); err != nil {
		errors = append(errors, "TWSE: "+err.Error())
	} else {
		rows = append(rows, parseTWSEDailySnapshot(twse, target, allow["TWSE"], twseURL)...)
	}
	if err := c.getJSON(ctx, tpexURL, &tpex); err != nil {
		errors = append(errors, "TPEx: "+err.Error())
	} else {
		rows = append(rows, parseTPExDailySnapshot(tpex, target, allow["TPEX"], tpexURL)...)
	}
	section := c.storeDaily(target, rows, errors)
	flight.section = section
	return section
}

func parseTWSEDailySnapshot(payload monthlyResponse, target time.Time, allow map[string]foundation.SecurityIdentity, sourceURL string) []foundation.TaiwanDailySnapshot {
	rows := payload.Data
	if len(rows) == 0 {
		for _, table := range payload.Tables {
			if len(table.Data) > 0 && len(table.Data[0]) >= 9 {
				rows = table.Data
				break
			}
		}
	}
	return parseDailyRows(rows, target, allow, "TWSE", sourceURL, 0, 8, 5, 6, 7, 2, 4, 10, 9)
}

func parseTPExDailySnapshot(payload monthlyResponse, target time.Time, allow map[string]foundation.SecurityIdentity, sourceURL string) []foundation.TaiwanDailySnapshot {
	rows := payload.Data
	if len(rows) == 0 {
		for _, table := range payload.Tables {
			if len(table.Data) > 0 && len(table.Data[0]) >= 10 {
				rows = table.Data
				break
			}
		}
	}
	return parseDailyRows(rows, target, allow, "TPEX", sourceURL, 0, 2, 4, 5, 6, 8, 9, 3, -1)
}

func parseDailyRows(rows [][]string, target time.Time, allow map[string]foundation.SecurityIdentity, exchange, sourceURL string, codeAt, closeAt, openAt, highAt, lowAt, volumeAt, amountAt, changeAt, signAt int) []foundation.TaiwanDailySnapshot {
	out := make([]foundation.TaiwanDailySnapshot, 0, len(rows))
	max := amountAt
	for _, row := range rows {
		if len(row) <= max {
			continue
		}
		identity, ok := allow[strings.TrimSpace(row[codeAt])]
		if !ok {
			continue
		}
		open, openOK := optionalNumber(row[openAt])
		high, highOK := optionalNumber(row[highAt])
		low, lowOK := optionalNumber(row[lowAt])
		closeValue, closeOK := optionalNumber(row[closeAt])
		volume, volumeOK := optionalInteger(row[volumeAt])
		amount, amountOK := optionalNumber(row[amountAt])
		var change *float64
		if changeAt >= 0 && len(row) > changeAt {
			change, _ = optionalNumber(row[changeAt])
			if change != nil && signAt >= 0 && len(row) > signAt {
				sign := strings.NewReplacer("−", "-", "－", "-", "﹣", "-").Replace(row[signAt])
				if strings.Contains(sign, "-") {
					value := -math.Abs(*change)
					change = &value
				} else if strings.Contains(sign, "+") {
					value := math.Abs(*change)
					change = &value
				}
			}
		}
		noTrade := !openOK && !highOK && !lowOK && !closeOK && !volumeOK && !amountOK && change == nil &&
			missingToken(row[openAt]) && missingToken(row[highAt]) && missingToken(row[lowAt]) && missingToken(row[closeAt]) && missingToken(row[volumeAt]) && missingToken(row[amountAt])
		if !openOK && !highOK && !lowOK && !closeOK && !volumeOK && !amountOK && !noTrade {
			continue
		}
		meta := officialMeta(strings.ToLower(exchange)+":market-wide-daily", sourceURL, target)
		meta.Status = "official"
		meta.AvailableFields = []string{"volume:shares", "amount:TWD", "price:raw", "change:official"}
		out = append(out, foundation.TaiwanDailySnapshot{Canonical: identity.Canonical, Code: identity.Code, Name: identity.Name, Exchange: exchange, Type: identity.Type, TradeDate: target.Format("2006-01-02"), Open: open, High: high, Low: low, Close: closeValue, Change: change, NoTrade: noTrade, Volume: volume, Amount: amount, Unit: "shares", Currency: "TWD", Meta: meta})
	}
	return out
}

func optionalNumber(raw string) (*float64, bool) {
	value := strings.NewReplacer("－", "-", "﹣", "-").Replace(strings.TrimSpace(raw))
	if missingToken(value) {
		return nil, false
	}
	parsed, err := number(value)
	if err != nil {
		return nil, false
	}
	return &parsed, true
}

func missingToken(raw string) bool {
	value := strings.TrimSpace(raw)
	return value == "" || value == "-" || value == "--" || value == "N/A"
}

func optionalInteger(raw string) (*int64, bool) {
	value := strings.TrimSpace(raw)
	if value == "" || value == "-" || value == "--" || value == "N/A" {
		return nil, false
	}
	parsed, err := integer(value)
	if err != nil {
		return nil, false
	}
	return &parsed, true
}

func (c *Client) refreshInstitutional(ctx context.Context, target time.Time, allow map[string]map[string]foundation.SecurityIdentity) foundation.TaiwanRefreshSection {
	rows := []foundation.InstitutionalFlow{}
	errors := []string{}
	for _, exchange := range []string{"TWSE", "TPEX"} {
		url := c.institutionalURL(exchange, target)
		payload, err := c.chipDay(ctx, url)
		if err != nil {
			errors = append(errors, exchange+": "+err.Error())
			continue
		}
		data := payload.Data
		if len(data) == 0 && len(payload.Tables) > 0 {
			data = payload.Tables[0].Data
		}
		for _, row := range data {
			if len(row) == 0 {
				continue
			}
			identity, ok := allow[exchange][strings.TrimSpace(row[0])]
			if !ok {
				continue
			}
			var parsed foundation.InstitutionalFlow
			if exchange == "TWSE" {
				parsed, err = parseTWSEInstitutional(identity, target, url, row)
			} else {
				parsed, err = parseTPExInstitutional(identity, target, url, row)
			}
			if err == nil {
				rows = append(rows, parsed)
			}
		}
	}
	return c.storeInstitutional(target, rows, errors)
}

func (c *Client) institutionalURL(exchange string, target time.Time) string {
	if exchange == "TWSE" {
		return c.twseReportBaseURL + "/rwd/zh/fund/T86?response=json&selectType=ALLBUT0999&date=" + target.Format("20060102")
	}
	return c.tpexReportBaseURL + "/www/zh-tw/insti/dailyTrade?response=json&type=Daily&sect=AL&date=" + target.Format("2006/01/02")
}

func (c *Client) refreshMargin(ctx context.Context, target time.Time, allow map[string]map[string]foundation.SecurityIdentity) foundation.TaiwanRefreshSection {
	rows := []foundation.MarginTrading{}
	errors := []string{}
	for _, exchange := range []string{"TWSE", "TPEX"} {
		url := c.marginURL(exchange, target)
		payload, err := c.chipDay(ctx, url)
		if err != nil {
			errors = append(errors, exchange+": "+err.Error())
			continue
		}
		data := payload.Data
		if len(data) == 0 {
			for _, table := range payload.Tables {
				if len(table.Data) > 0 && len(table.Data[0]) >= 16 {
					data = table.Data
					break
				}
			}
		}
		for _, row := range data {
			if len(row) == 0 {
				continue
			}
			identity, ok := allow[exchange][strings.TrimSpace(row[0])]
			if !ok {
				continue
			}
			var parsed foundation.MarginTrading
			if exchange == "TWSE" {
				parsed, err = parseMargin(identity, target, url, row, 2, 3, 4, 5, 6, 8, 9, 10, 11, 12, 15)
			} else {
				parsed, err = parseMargin(identity, target, url, row, 3, 4, 5, 2, 6, 12, 11, 13, 10, 14, 19)
			}
			if err == nil {
				rows = append(rows, parsed)
			}
		}
	}
	return c.storeMargin(target, rows, errors)
}

func (c *Client) marginURL(exchange string, target time.Time) string {
	if exchange == "TWSE" {
		return c.twseReportBaseURL + "/rwd/zh/marginTrading/MI_MARGN?response=json&selectType=ALL&date=" + target.Format("20060102")
	}
	return c.tpexReportBaseURL + "/www/zh-tw/margin/balance?response=json&date=" + target.Format("2006/01/02")
}

func (c *Client) storeDaily(day time.Time, rows []foundation.TaiwanDailySnapshot, errs []string) foundation.TaiwanRefreshSection {
	key := day.Format("2006-01-02")
	c.snapshotMu.Lock()
	defer c.snapshotMu.Unlock()
	if len(rows) > 0 {
		c.dailyDays[key] = rows
	}
	return sectionResult(key, len(rows), errs)
}
func (c *Client) storeInstitutional(day time.Time, rows []foundation.InstitutionalFlow, errs []string) foundation.TaiwanRefreshSection {
	key := day.Format("2006-01-02")
	c.snapshotMu.Lock()
	defer c.snapshotMu.Unlock()
	if len(rows) > 0 {
		c.instDays[key] = rows
	}
	return sectionResult(key, len(rows), errs)
}
func (c *Client) storeMargin(day time.Time, rows []foundation.MarginTrading, errs []string) foundation.TaiwanRefreshSection {
	key := day.Format("2006-01-02")
	c.snapshotMu.Lock()
	defer c.snapshotMu.Unlock()
	if len(rows) > 0 {
		c.marginDays[key] = rows
	}
	return sectionResult(key, len(rows), errs)
}

func sectionResult(day string, rows int, errs []string) foundation.TaiwanRefreshSection {
	status := "success"
	if len(errs) > 0 {
		if rows > 0 {
			status = "partial"
		} else {
			status = "failed"
		}
	}
	return foundation.TaiwanRefreshSection{Status: status, TradeDate: day, Rows: rows, Error: strings.Join(errs, "; ")}
}
func refreshOverall(sections ...foundation.TaiwanRefreshSection) string {
	successes, failures := 0, 0
	for _, s := range sections {
		if s.Status == "success" {
			successes++
		}
		if s.Status == "failed" {
			failures++
		}
		if s.Status == "partial" {
			successes++
			failures++
		}
	}
	if failures == 0 {
		return "success"
	}
	if successes > 0 {
		return "partial"
	}
	return "failed"
}

func (c *Client) Freshness(now time.Time) foundation.TaiwanFreshness {
	c.snapshotMu.RLock()
	defer c.snapshotMu.RUnlock()
	target := c.calendar.LatestCompleted(now, marketCutoffHour, marketCutoffMinute)
	daily := latestKey(c.dailyDays)
	inst := latestKey(c.instDays)
	margin := latestKey(c.marginDays)
	return foundation.TaiwanFreshness{DailyAsOf: ptrString(daily), InstitutionalAsOf: ptrString(inst), MarginAsOf: ptrString(margin), TargetLatestTradingDate: target.Format("2006-01-02"), DailyStatus: freshnessStatus(daily, target), InstitutionalStatus: freshnessStatus(inst, target), MarginStatus: freshnessStatus(margin, target), DailyDaysBehind: daysBehind(daily, target, c.calendar), InstitutionalDaysBehind: daysBehind(inst, target, c.calendar), MarginDaysBehind: daysBehind(margin, target, c.calendar), Timezone: "Asia/Taipei", Cutoff: "17:30"}
}
func latestKey[T any](values map[string][]T) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return ""
	}
	return keys[len(keys)-1]
}
func ptrString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
func freshnessStatus(value string, target time.Time) string {
	if value == "" {
		return "unavailable"
	}
	if value == target.Format("2006-01-02") {
		return "current"
	}
	return "stale"
}
func daysBehind(value string, target time.Time, calendar foundation.TaiwanTradingCalendar) *int {
	if value == "" {
		return nil
	}
	day, err := time.ParseInLocation("2006-01-02", value, taipei())
	if err != nil {
		return nil
	}
	count := 0
	for day.Before(target) {
		day = day.AddDate(0, 0, 1)
		if calendar.IsTradingDay(day) {
			count++
		}
	}
	return &count
}

func (c *Client) AvailableMarketDates() (daily, institutional, margin []string) {
	c.snapshotMu.RLock()
	defer c.snapshotMu.RUnlock()
	for key := range c.dailyDays {
		daily = append(daily, key)
	}
	for key := range c.instDays {
		institutional = append(institutional, key)
	}
	for key := range c.marginDays {
		margin = append(margin, key)
	}
	sort.Strings(daily)
	sort.Strings(institutional)
	sort.Strings(margin)
	return
}

func (c *Client) DailyAsOf(query time.Time) ([]foundation.TaiwanDailySnapshot, error) {
	c.snapshotMu.RLock()
	defer c.snapshotMu.RUnlock()
	key := latestNotAfter(c.dailyDays, query)
	if key == "" {
		return nil, fmt.Errorf("daily snapshot unavailable as of %s", query.Format("2006-01-02"))
	}
	return append([]foundation.TaiwanDailySnapshot(nil), c.dailyDays[key]...), nil
}

func (c *Client) dailyOn(query time.Time) []foundation.TaiwanDailySnapshot {
	c.snapshotMu.RLock()
	defer c.snapshotMu.RUnlock()
	return append([]foundation.TaiwanDailySnapshot(nil), c.dailyDays[query.Format("2006-01-02")]...)
}
func (c *Client) instOn(query time.Time) []foundation.InstitutionalFlow {
	c.snapshotMu.RLock()
	defer c.snapshotMu.RUnlock()
	return append([]foundation.InstitutionalFlow(nil), c.instDays[query.Format("2006-01-02")]...)
}
func (c *Client) marginOn(query time.Time) []foundation.MarginTrading {
	c.snapshotMu.RLock()
	defer c.snapshotMu.RUnlock()
	return append([]foundation.MarginTrading(nil), c.marginDays[query.Format("2006-01-02")]...)
}
func latestNotAfter[T any](values map[string][]T, query time.Time) string {
	limit := query.Format("2006-01-02")
	best := ""
	for key := range values {
		if key <= limit && key > best {
			best = key
		}
	}
	return best
}
