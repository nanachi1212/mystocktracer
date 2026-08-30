package taiwan

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"easy-stock/backend/internal/foundation"
)

type twseDailyRow struct{ Date, Code, Name, TradeVolume, TradeValue, OpeningPrice, HighestPrice, LowestPrice, ClosingPrice, Change string }
type tpexQuoteRow struct{ Date, SecuritiesCompanyCode, CompanyName, Close, Change, Open, High, Low, TradingShares, TransactionAmount string }
type monthlyResponse struct {
	Stat   string     `json:"stat"`
	Data   [][]string `json:"data"`
	Tables []struct {
		Data [][]string `json:"data"`
	} `json:"tables"`
}
type twseIndexRow struct{ Date, OpeningIndex, HighestIndex, LowestIndex, ClosingIndex string }
type tpexIndexRow struct{ Date, Open, High, Low, Close, Change string }

func (c *Client) Quote(ctx context.Context, security foundation.SecurityIdentity) (foundation.Quote, error) {
	started := time.Now()
	var quote foundation.Quote
	var err error
	if security.Exchange == "TWSE" {
		quote, err = c.twseQuote(ctx, security)
	} else if security.Exchange == "TPEX" {
		quote, err = c.tpexQuote(ctx, security)
	} else {
		return quote, fmt.Errorf("unsupported Taiwan exchange %q", security.Exchange)
	}
	if err != nil {
		lines, fallbackErr := c.KLine(ctx, security, 1)
		if fallbackErr != nil || len(lines) == 0 {
			return quote, fmt.Errorf("official quote failed: %v; monthly fallback failed: %w", err, fallbackErr)
		}
		line := lines[len(lines)-1]
		quote = quoteFromLine(security, line)
		quote.Meta.FallbackReason = err.Error()
		quote.Meta.Status = "official_monthly_fallback"
	}
	quote.Meta.LatencyMS = time.Since(started).Milliseconds()
	return quote, nil
}

func (c *Client) twseQuote(ctx context.Context, security foundation.SecurityIdentity) (foundation.Quote, error) {
	sourceURL := c.twseBaseURL + "/exchangeReport/STOCK_DAY_ALL"
	var rows []twseDailyRow
	if err := c.getJSON(ctx, sourceURL, &rows); err != nil {
		return foundation.Quote{}, err
	}
	for _, row := range rows {
		if row.Code == security.Code {
			return buildQuote(security, row.Name, row.Date, row.OpeningPrice, row.HighestPrice, row.LowestPrice, row.ClosingPrice, row.Change, row.TradeVolume, row.TradeValue, "twse:STOCK_DAY_ALL", sourceURL)
		}
	}
	return foundation.Quote{}, fmt.Errorf("%s not found in TWSE daily quote", security.Code)
}

func (c *Client) tpexQuote(ctx context.Context, security foundation.SecurityIdentity) (foundation.Quote, error) {
	sourceURL := c.tpexBaseURL + "/tpex_mainboard_quotes"
	var rows []tpexQuoteRow
	if err := c.getJSON(ctx, sourceURL, &rows); err != nil {
		return foundation.Quote{}, err
	}
	for _, row := range rows {
		if row.SecuritiesCompanyCode == security.Code {
			return buildQuote(security, row.CompanyName, row.Date, row.Open, row.High, row.Low, row.Close, row.Change, row.TradingShares, row.TransactionAmount, "tpex:mainboard_quotes", sourceURL)
		}
	}
	return foundation.Quote{}, fmt.Errorf("%s not found in TPEx daily quote", security.Code)
}

func buildQuote(security foundation.SecurityIdentity, name, rawDate, open, high, low, close, change, volume, amount, source, sourceURL string) (foundation.Quote, error) {
	date, err := taiwanDate(rawDate)
	if err != nil {
		return foundation.Quote{}, err
	}
	price, err := number(close)
	if err != nil || price <= 0 {
		return foundation.Quote{}, fmt.Errorf("invalid close %q", close)
	}
	changeValue, _ := number(change)
	previous := price - changeValue
	openValue, _ := number(open)
	highValue, _ := number(high)
	lowValue, _ := number(low)
	meta := officialMeta(source, sourceURL, date)
	_ = volume
	_ = amount
	return foundation.Quote{Symbol: security.Canonical, Name: first(strings.TrimSpace(name), security.Name), Price: price, Open: openValue, High: highValue, Low: lowValue, PreviousClose: previous, Change: changeValue, ChangePercent: percent(changeValue, previous), TradeTime: marketClose(date), Meta: meta}, nil
}

func (c *Client) KLine(ctx context.Context, security foundation.SecurityIdentity, limit int) ([]foundation.KLine, error) {
	if limit < 1 || limit > 240 {
		return nil, fmt.Errorf("limit must be between 1 and 240")
	}
	now := time.Now().In(taipei())
	lines := make([]foundation.KLine, 0, limit)
	for month := 0; month < 18 && len(lines) < limit; month++ {
		date := now.AddDate(0, -month, 0)
		monthly, err := c.month(ctx, security, date)
		if err != nil {
			if len(lines) == 0 {
				return nil, err
			}
			continue
		}
		lines = append(lines, monthly...)
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("official monthly history returned no rows for %s", security.Canonical)
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].Time.Before(lines[j].Time) })
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}
	return lines, nil
}

func (c *Client) month(ctx context.Context, security foundation.SecurityIdentity, month time.Time) ([]foundation.KLine, error) {
	var sourceURL, source string
	if security.Exchange == "TWSE" {
		sourceURL = "https://www.twse.com.tw/exchangeReport/STOCK_DAY?response=json&date=" + month.Format("20060102") + "&stockNo=" + url.QueryEscape(security.Code)
		source = "twse:STOCK_DAY"
	} else if security.Exchange == "TPEX" {
		sourceURL = "https://www.tpex.org.tw/www/zh-tw/afterTrading/tradingStock?response=json&date=" + month.Format("2006/01/02") + "&code=" + url.QueryEscape(security.Code)
		source = "tpex:tradingStock"
	} else {
		return nil, fmt.Errorf("unsupported Taiwan exchange %q", security.Exchange)
	}
	var payload monthlyResponse
	if err := c.getJSON(ctx, sourceURL, &payload); err != nil {
		return nil, err
	}
	rows := payload.Data
	if len(rows) == 0 && len(payload.Tables) > 0 {
		rows = payload.Tables[0].Data
	}
	lines := make([]foundation.KLine, 0, len(rows))
	for _, row := range rows {
		if len(row) < 8 {
			continue
		}
		date, err := taiwanDate(row[0])
		if err != nil {
			continue
		}
		open, e1 := number(row[3])
		high, e2 := number(row[4])
		low, e3 := number(row[5])
		closeValue, e4 := number(row[6])
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
			continue
		}
		change, _ := number(row[7])
		volume, _ := number(row[1])
		amount, _ := number(row[2])
		previous := closeValue - change
		lines = append(lines, foundation.KLine{Symbol: security.Canonical, Time: marketClose(date), Open: open, High: high, Low: low, Close: closeValue, PreviousClose: previous, Volume: volume, Amount: amount, ChangePercent: percent(change, previous), Meta: officialMeta(source, sourceURL, date)})
	}
	return lines, nil
}

func (c *Client) Indexes(ctx context.Context) ([]foundation.MarketIndexSeries, foundation.SourceMeta, error) {
	twseURL := c.twseBaseURL + "/indicesReport/MI_5MINS_HIST"
	tpexURL := c.tpexBaseURL + "/tpex_index"
	var twseRows []twseIndexRow
	if err := c.getJSON(ctx, twseURL, &twseRows); err != nil {
		return nil, foundation.SourceMeta{}, err
	}
	var tpexRows []tpexIndexRow
	if err := c.getJSON(ctx, tpexURL, &tpexRows); err != nil {
		return nil, foundation.SourceMeta{}, err
	}
	twse := indexSeries("taiex", "發行量加權股價指數", "TWSE", twseURL, twseRows, func(r twseIndexRow) (string, string, string, string, string, string) {
		return r.Date, r.OpeningIndex, r.HighestIndex, r.LowestIndex, r.ClosingIndex, ""
	})
	tpex := indexSeries("tpex", "櫃買指數", "TPEX", tpexURL, tpexRows, func(r tpexIndexRow) (string, string, string, string, string, string) {
		return r.Date, r.Open, r.High, r.Low, r.Close, r.Change
	})
	if len(twse.Lines) == 0 || len(tpex.Lines) == 0 {
		return nil, foundation.SourceMeta{}, fmt.Errorf("official Taiwan index history is empty")
	}
	meta := officialMeta("twse+tpex:index", twseURL+" | "+tpexURL, twse.Lines[len(twse.Lines)-1].Time)
	return []foundation.MarketIndexSeries{twse, tpex}, meta, nil
}

func indexSeries[T any](id, name, exchange, sourceURL string, rows []T, values func(T) (string, string, string, string, string, string)) foundation.MarketIndexSeries {
	lines := make([]foundation.KLine, 0, len(rows))
	for _, row := range rows {
		rawDate, o, h, l, c, ch := values(row)
		date, err := taiwanDate(rawDate)
		if err != nil {
			continue
		}
		open, _ := number(o)
		high, _ := number(h)
		low, _ := number(l)
		closeValue, e := number(c)
		if e != nil {
			continue
		}
		change, _ := number(ch)
		previous := closeValue - change
		lines = append(lines, foundation.KLine{Symbol: id, Time: marketClose(date), Open: open, High: high, Low: low, Close: closeValue, PreviousClose: previous, ChangePercent: percent(change, previous), Meta: officialMeta(strings.ToLower(exchange)+":index", sourceURL, date)})
	}
	if len(lines) == 0 {
		return foundation.MarketIndexSeries{}
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].Time.Before(lines[j].Time) })
	for index := 1; index < len(lines); index++ {
		if lines[index].PreviousClose == 0 {
			lines[index].PreviousClose = lines[index-1].Close
			lines[index].ChangePercent = percent(lines[index].Close-lines[index].PreviousClose, lines[index].PreviousClose)
		}
	}
	last := lines[len(lines)-1]
	snapshot := foundation.MarketIndexSnapshot{ID: id, SecID: id, Code: id, Name: name, Region: "TW", Market: exchange, Currency: "TWD", Price: last.Close, Change: last.Close - last.PreviousClose, ChangePercent: last.ChangePercent, TradeTime: last.Time, Status: "official_close", Meta: last.Meta}
	return foundation.MarketIndexSeries{Index: snapshot, Lines: lines, Meta: last.Meta}
}

func quoteFromLine(security foundation.SecurityIdentity, line foundation.KLine) foundation.Quote {
	return foundation.Quote{Symbol: security.Canonical, Name: security.Name, Price: line.Close, Open: line.Open, PreviousClose: line.PreviousClose, High: line.High, Low: line.Low, Change: line.Close - line.PreviousClose, ChangePercent: line.ChangePercent, TradeTime: line.Time, Meta: line.Meta}
}
func officialMeta(source, sourceURL string, date time.Time) foundation.SourceMeta {
	return foundation.SourceMeta{Source: source, SourceURL: sourceURL, FetchedAt: time.Now(), TradeDate: date.Format("2006-01-02"), Stale: time.Since(date) > 96*time.Hour, Status: "official_close", IsRealtime: false}
}
func number(raw string) (float64, error) {
	cleaned := strings.NewReplacer(",", "", "+", "", "−", "-").Replace(strings.TrimSpace(raw))
	if cleaned == "" || cleaned == "--" {
		return 0, fmt.Errorf("missing number")
	}
	return strconv.ParseFloat(cleaned, 64)
}
func percent(change, previous float64) float64 {
	if previous == 0 {
		return 0
	}
	return change / previous * 100
}
func taipei() *time.Location {
	location, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		return time.FixedZone("Asia/Taipei", 8*60*60)
	}
	return location
}
func marketClose(date time.Time) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), 13, 30, 0, 0, taipei())
}
func taiwanDate(raw string) (time.Time, error) {
	value := strings.TrimSpace(raw)
	if len(value) == 7 {
		year, err := strconv.Atoi(value[:3])
		if err == nil {
			return time.ParseInLocation("20060102", fmt.Sprintf("%04d%s", year+1911, value[3:]), taipei())
		}
	}
	if strings.Count(value, "/") == 2 {
		parts := strings.Split(value, "/")
		if len(parts[0]) == 3 {
			year, err := strconv.Atoi(parts[0])
			if err == nil {
				value = fmt.Sprintf("%04d/%s/%s", year+1911, parts[1], parts[2])
			}
		}
		return time.ParseInLocation("2006/01/02", value, taipei())
	}
	return time.ParseInLocation("20060102", value, taipei())
}
