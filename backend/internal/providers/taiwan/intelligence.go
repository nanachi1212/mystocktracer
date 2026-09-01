package taiwan

import (
	"context"
	"fmt"
	"strings"
	"time"

	"easy-stock/backend/internal/foundation"
	"easy-stock/backend/internal/marketemotion"
	"easy-stock/backend/internal/sector"
	"easy-stock/backend/internal/stockanalysis"
)

func (c *Client) StockIntelligence(ctx context.Context, canonical string, now time.Time) (stockanalysis.TaiwanStockIntelligence, error) {
	identities, err := c.intelligenceDirectory(ctx, canonical)
	if err != nil {
		return stockanalysis.TaiwanStockIntelligence{}, err
	}
	canonical = strings.TrimSpace(canonical)
	var identity *foundation.SecurityIdentity
	for i := range identities {
		if identities[i].Canonical == canonical {
			item := identities[i]
			identity = &item
			break
		}
	}
	if identity == nil {
		return stockanalysis.TaiwanStockIntelligence{}, fmt.Errorf("unknown canonical Taiwan security %q", canonical)
	}

	// One shared bulk snapshot supplies breadth, emotion, and industry context.
	target := c.calendar.LatestCompleted(now, marketCutoffHour, marketCutoffMinute)
	breadth, emotion, radar := c.intelligenceMarketContext(ctx, identities, target, now)

	quote, quoteErr := c.Quote(ctx, *identity)
	// Ask for one spare bar because an official monthly response may already
	// contain today's not-yet-completed observation.
	lines, lineErr := c.KLine(ctx, *identity, 22)
	var quotePtr *foundation.Quote
	if quoteErr == nil {
		quotePtr = &quote
	}
	if lineErr != nil {
		lines = nil
	} else {
		lines = completedIntelligenceLines(lines, target)
	}
	var fundamentalsPtr *foundation.TaiwanFundamentals
	if identity.Type == foundation.SecurityTypeStock {
		if value, e := c.Fundamentals(ctx, *identity, 24); e == nil {
			fundamentalsPtr = &value
		}
	}
	var institutionalPtr *foundation.InstitutionalHistory
	if value, e := c.Institutional(ctx, *identity, 20); e == nil {
		institutionalPtr = &value
	}
	var marginPtr *foundation.MarginHistory
	if value, e := c.Margin(ctx, *identity, 20); e == nil {
		marginPtr = &value
	}

	breadthScope, emotionScope, radarScope := breadth.TWSE, emotion.TWSE, radar.TWSE
	if identity.Exchange == "TPEX" {
		breadthScope, emotionScope, radarScope = breadth.TPEX, emotion.TPEX, radar.TPEX
	}
	return stockanalysis.NewTaiwanStockIntelligence(*identity, quotePtr, lines, fundamentalsPtr, institutionalPtr, marginPtr, breadthScope, emotionScope, radarScope), nil
}

func (c *Client) intelligenceMarketContext(ctx context.Context, identities []foundation.SecurityIdentity, target, now time.Time) (foundation.TaiwanMarketBreadth, marketemotion.TaiwanMarketEmotion, sector.TaiwanIndustryRadar) {
	section := c.refreshDaily(ctx, target, identityAllowlist(identities))
	rows := c.dailyOn(target)
	exchangeStatus := map[string]string{"TWSE": "current", "TPEX": "current"}
	for _, exchange := range []string{"TWSE", "TPEX"} {
		if strings.Contains(section.Error, exchange+":") || !hasExchangeRows(rows, exchange) {
			exchangeStatus[exchange] = "unavailable"
		}
	}
	breadth := foundation.CalculateTaiwanMarketBreadth(identities, rows, c.Freshness(now), exchangeStatus)
	return breadth, marketemotion.CalculateTaiwanMarketEmotion(breadth), sector.CalculateTaiwanIndustryRadar(identities, rows, breadth)
}

func completedIntelligenceLines(lines []foundation.KLine, target time.Time) []foundation.KLine {
	targetDate := target.In(taipei()).Format("2006-01-02")
	result := make([]foundation.KLine, 0, len(lines))
	for _, line := range lines {
		if line.Time.In(taipei()).Format("2006-01-02") <= targetDate {
			result = append(result, line)
		}
	}
	if len(result) > 21 {
		result = result[len(result)-21:]
	}
	return result
}

func (c *Client) intelligenceDirectory(ctx context.Context, canonical string) ([]foundation.SecurityIdentity, error) {
	switch {
	case strings.HasSuffix(strings.TrimSpace(canonical), ".TWSE"):
		stocks, err := c.twseStocks(ctx)
		if err != nil {
			return nil, fmt.Errorf("TWSE company directory: %w", err)
		}
		funds, err := c.twseETFs(ctx)
		if err != nil {
			return nil, fmt.Errorf("TWSE fund directory: %w", err)
		}
		return append(stocks, funds...), nil
	case strings.HasSuffix(strings.TrimSpace(canonical), ".TPEX"):
		stocks, err := c.tpexStocks(ctx)
		if err != nil {
			return nil, fmt.Errorf("TPEx company directory: %w", err)
		}
		return stocks, nil
	default:
		return nil, fmt.Errorf("canonical Taiwan security must end in .TWSE or .TPEX")
	}
}
