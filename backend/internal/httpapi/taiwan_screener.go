package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"easy-stock/backend/internal/foundation"
)

// taiwanScreenerRow is the wire shape of one screened Taiwan security. M7A is market-snapshot
// screening only — no fundamentals, institutional, margin, AI interpretation, or Watchlist state.
type taiwanScreenerRow struct {
	Canonical     string   `json:"canonical"`
	Code          string   `json:"code"`
	Name          string   `json:"name"`
	Exchange      string   `json:"exchange"`
	SecurityType  string   `json:"security_type"`
	TradeDate     string   `json:"trade_date"`
	Price         *float64 `json:"price"`
	Change        *float64 `json:"change"`
	ChangePercent *float64 `json:"change_percent"`
	Volume        *int64   `json:"volume"`
	Amount        *float64 `json:"amount"`
}

type taiwanScreenerResponse struct {
	Scope      string              `json:"scope"`
	AsOf       *string             `json:"as_of"`
	Freshness  string              `json:"freshness"`
	Total      int                 `json:"total"`
	Offset     int                 `json:"offset"`
	Limit      int                 `json:"limit"`
	Securities []taiwanScreenerRow `json:"securities"`
}

type taiwanScreenerQuery struct {
	scope                              string // "", "twse", "tpex", or "combined" — already validated/lowercased
	minPrice, maxPrice                 *float64
	minChangePercent, maxChangePercent *float64
	minVolume, maxVolume               *float64
	minAmount, maxAmount               *float64
	sort                               string // "price" | "change_percent" | "volume" | "amount"
	order                              string // "asc" | "desc"
}

func (s *Server) taiwanScreenerHandler(w http.ResponseWriter, r *http.Request) {
	if s.taiwanScreener == nil {
		writeError(w, http.StatusServiceUnavailable, "Taiwan screener provider is unavailable")
		return
	}
	query, offset, limit, errMessage := parseTaiwanScreenerQuery(r)
	if errMessage != "" {
		writeError(w, http.StatusBadRequest, errMessage)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	// Reuses the existing bulk market-wide snapshot (one TWSE + one TPEx request total, the same
	// path MarketBreadth already uses) — never one request per security, regardless of how many
	// rows are ultimately filtered/paginated below.
	rows, freshness, err := s.taiwanScreener.ScreenerSnapshot(ctx, time.Now())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	filtered := filterAndSortTaiwanScreener(rows, query)
	total := len(filtered)
	page := paginateTaiwanScreener(filtered, offset, limit)

	scopeLabel := "COMBINED"
	if query.scope != "" && query.scope != "combined" {
		scopeLabel = strings.ToUpper(query.scope)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": taiwanScreenerResponse{
		Scope: scopeLabel, AsOf: freshness.DailyAsOf, Freshness: freshness.DailyStatus,
		Total: total, Offset: offset, Limit: limit, Securities: page,
	}})
}

// parseTaiwanScreenerQuery validates and parses every query parameter up front, returning a
// non-empty message (with the handler's 400 response) on the first invalid input.
func parseTaiwanScreenerQuery(r *http.Request) (taiwanScreenerQuery, int, int, string) {
	query := taiwanScreenerQuery{}

	query.scope = strings.ToLower(strings.TrimSpace(r.URL.Query().Get("scope")))
	if query.scope != "" && query.scope != "twse" && query.scope != "tpex" && query.scope != "combined" {
		return query, 0, 0, "scope must be twse, tpex, or combined"
	}

	query.sort = strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort")))
	if query.sort == "" {
		query.sort = "amount"
	}
	if query.sort != "price" && query.sort != "change_percent" && query.sort != "volume" && query.sort != "amount" {
		return query, 0, 0, "sort must be price, change_percent, volume, or amount"
	}

	query.order = strings.ToLower(strings.TrimSpace(r.URL.Query().Get("order")))
	if query.order == "" {
		query.order = "desc"
	}
	if query.order != "asc" && query.order != "desc" {
		return query, 0, 0, "order must be asc or desc"
	}

	var err error
	if query.minPrice, err = parseOptionalScreenerFloat(r, "min_price"); err != nil {
		return query, 0, 0, err.Error()
	}
	if query.maxPrice, err = parseOptionalScreenerFloat(r, "max_price"); err != nil {
		return query, 0, 0, err.Error()
	}
	if rangeInvalid(query.minPrice, query.maxPrice) {
		return query, 0, 0, "min_price must not be greater than max_price"
	}

	if query.minChangePercent, err = parseOptionalScreenerFloat(r, "min_change_percent"); err != nil {
		return query, 0, 0, err.Error()
	}
	if query.maxChangePercent, err = parseOptionalScreenerFloat(r, "max_change_percent"); err != nil {
		return query, 0, 0, err.Error()
	}
	if rangeInvalid(query.minChangePercent, query.maxChangePercent) {
		return query, 0, 0, "min_change_percent must not be greater than max_change_percent"
	}

	if query.minVolume, err = parseOptionalScreenerFloat(r, "min_volume"); err != nil {
		return query, 0, 0, err.Error()
	}
	if query.maxVolume, err = parseOptionalScreenerFloat(r, "max_volume"); err != nil {
		return query, 0, 0, err.Error()
	}
	if rangeInvalid(query.minVolume, query.maxVolume) {
		return query, 0, 0, "min_volume must not be greater than max_volume"
	}

	if query.minAmount, err = parseOptionalScreenerFloat(r, "min_amount"); err != nil {
		return query, 0, 0, err.Error()
	}
	if query.maxAmount, err = parseOptionalScreenerFloat(r, "max_amount"); err != nil {
		return query, 0, 0, err.Error()
	}
	if rangeInvalid(query.minAmount, query.maxAmount) {
		return query, 0, 0, "min_amount must not be greater than max_amount"
	}

	limit, err := marketLimitQuery(r, 50, 200)
	if err != nil {
		return query, 0, 0, err.Error()
	}
	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		value, convErr := strconv.Atoi(raw)
		if convErr != nil || value < 0 {
			return query, 0, 0, "offset must be zero or a positive integer"
		}
		offset = value
	}

	return query, offset, limit, ""
}

func parseOptionalScreenerFloat(r *http.Request, key string) (*float64, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, fmt.Errorf("%s must be a number", key)
	}
	return &value, nil
}

func rangeInvalid(min, max *float64) bool {
	return min != nil && max != nil && *min > *max
}

// filterAndSortTaiwanScreener applies scope/range filters and deterministic sorting entirely in
// memory over the already-fetched bulk snapshot rows — no additional provider calls are made here.
//
// Filter semantics: a row is excluded from a given range filter only if that filter was actually
// requested (min/max present) AND the row's value for that field is unavailable (nil) — a filter
// that was never requested never excludes a row for that field, and an unavailable value is never
// treated as 0.
func filterAndSortTaiwanScreener(rows []foundation.TaiwanDailySnapshot, query taiwanScreenerQuery) []taiwanScreenerRow {
	filtered := make([]taiwanScreenerRow, 0, len(rows))
	for _, raw := range rows {
		if query.scope != "" && query.scope != "combined" && !strings.EqualFold(raw.Exchange, query.scope) {
			continue
		}
		row := toTaiwanScreenerRow(raw)
		if !passesRange(row.Price, query.minPrice, query.maxPrice) {
			continue
		}
		if !passesRange(row.ChangePercent, query.minChangePercent, query.maxChangePercent) {
			continue
		}
		if !passesIntRange(row.Volume, query.minVolume, query.maxVolume) {
			continue
		}
		if !passesRange(row.Amount, query.minAmount, query.maxAmount) {
			continue
		}
		filtered = append(filtered, row)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		vi := taiwanScreenerSortValue(filtered[i], query.sort)
		vj := taiwanScreenerSortValue(filtered[j], query.sort)
		// Unavailable sort values are placed at the end consistently for both asc and desc —
		// a nil value is never treated as "less than everything" or "greater than everything"
		// depending on direction, it simply always sorts after any available value.
		if vi == nil && vj == nil {
			return filtered[i].Canonical < filtered[j].Canonical
		}
		if vi == nil {
			return false
		}
		if vj == nil {
			return true
		}
		if *vi == *vj {
			return filtered[i].Canonical < filtered[j].Canonical
		}
		if query.order == "desc" {
			return *vi > *vj
		}
		return *vi < *vj
	})
	return filtered
}

func toTaiwanScreenerRow(row foundation.TaiwanDailySnapshot) taiwanScreenerRow {
	return taiwanScreenerRow{
		Canonical: row.Canonical, Code: row.Code, Name: row.Name, Exchange: row.Exchange, SecurityType: string(row.Type),
		TradeDate: row.TradeDate, Price: row.Close, Change: row.Change,
		ChangePercent: taiwanScreenerChangePercent(row.Close, row.Change),
		Volume:        row.Volume, Amount: row.Amount,
	}
}

// taiwanScreenerChangePercent derives change percent from the official exchange-reported price
// change: TWSE/TPEx daily quote feeds define change (漲跌價差) as close - previous_close, so
// previous_close = close - change and change_percent = change / previous_close * 100. This is
// confirmed by how `parseDailyRows` (snapshot.go) populates Change directly from each exchange's
// own reported price-change column — it is not something this backend derives itself elsewhere.
// A missing close/change, or a non-positive previous_close (e.g. a data anomaly), never fabricates
// a percentage — it returns nil (unavailable), never 0.
func taiwanScreenerChangePercent(closeValue, change *float64) *float64 {
	if closeValue == nil || change == nil {
		return nil
	}
	previousClose := *closeValue - *change
	if previousClose <= 0 {
		return nil
	}
	percent := *change / previousClose * 100
	return &percent
}

func taiwanScreenerSortValue(row taiwanScreenerRow, field string) *float64 {
	switch field {
	case "price":
		return row.Price
	case "change_percent":
		return row.ChangePercent
	case "volume":
		if row.Volume == nil {
			return nil
		}
		value := float64(*row.Volume)
		return &value
	case "amount":
		return row.Amount
	}
	return nil
}

func passesRange(value *float64, min, max *float64) bool {
	if min == nil && max == nil {
		return true
	}
	if value == nil {
		return false
	}
	if min != nil && *value < *min {
		return false
	}
	if max != nil && *value > *max {
		return false
	}
	return true
}

func passesIntRange(value *int64, min, max *float64) bool {
	if min == nil && max == nil {
		return true
	}
	if value == nil {
		return false
	}
	converted := float64(*value)
	return passesRange(&converted, min, max)
}

func paginateTaiwanScreener(rows []taiwanScreenerRow, offset, limit int) []taiwanScreenerRow {
	if offset >= len(rows) {
		return []taiwanScreenerRow{}
	}
	end := offset + limit
	if end > len(rows) {
		end = len(rows)
	}
	return append([]taiwanScreenerRow(nil), rows[offset:end]...)
}
