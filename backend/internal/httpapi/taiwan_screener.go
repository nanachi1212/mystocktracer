package httpapi

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"easy-stock/backend/internal/foundation"
)

// taiwanScreenerRow is the wire shape of one screened Taiwan security. M7A covers market-snapshot
// fields (price/volume/amount); M7D additively joins institutional and margin fields — all nullable,
// all backend-authoritative, none fabricated. No fundamentals, AI interpretation, or Watchlist state.
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
	// M7D — institutional (nil when the security has no institutional row for the target date, or
	// when the institutional domain was not requested at all; a genuine reported net of 0 is 0, never nil).
	ForeignNet       *int64 `json:"foreign_net"`
	TrustNet         *int64 `json:"trust_net"`
	DealerNet        *int64 `json:"dealer_net"`
	InstitutionalNet *int64 `json:"institutional_net"`
	// M7D — margin/short (values reused directly from the existing MarginTrading model; nil stays nil).
	MarginBalance    *int64   `json:"margin_balance"`
	MarginChange     *int64   `json:"margin_change"`
	ShortBalance     *int64   `json:"short_balance"`
	ShortChange      *int64   `json:"short_change"`
	ShortMarginRatio *float64 `json:"short_margin_ratio"`
}

type taiwanScreenerResponse struct {
	Scope      string              `json:"scope"`
	AsOf       *string             `json:"as_of"`
	Freshness  string              `json:"freshness"`
	Total      int                 `json:"total"`
	Offset     int                 `json:"offset"`
	Limit      int                 `json:"limit"`
	Securities []taiwanScreenerRow `json:"securities"`
	// M7D — additive, present only when the corresponding domain was actually requested (a filter or
	// sort key engaged it). Sourced directly from the existing TaiwanFreshness model — trade-date
	// AsOf/status/days-behind only. Never a published_at/available_at timestamp: M7D.0 established
	// that the official payloads carry no such field, and 17:30 remains only a candidate-date
	// selector, never a publication guarantee.
	InstitutionalAsOf       *string `json:"institutional_as_of,omitempty"`
	InstitutionalStatus     string  `json:"institutional_status,omitempty"`
	InstitutionalDaysBehind *int    `json:"institutional_days_behind,omitempty"`
	MarginAsOf              *string `json:"margin_as_of,omitempty"`
	MarginStatus            string  `json:"margin_status,omitempty"`
	MarginDaysBehind        *int    `json:"margin_days_behind,omitempty"`
}

type taiwanScreenerQuery struct {
	scope                              string // "", "twse", "tpex", or "combined" — already validated/lowercased
	minPrice, maxPrice                 *float64
	minChangePercent, maxChangePercent *float64
	minVolume, maxVolume               *float64
	minAmount, maxAmount               *float64
	// M7D institutional filters.
	minForeignNet, maxForeignNet             *float64
	minTrustNet, maxTrustNet                 *float64
	minDealerNet, maxDealerNet               *float64
	minInstitutionalNet, maxInstitutionalNet *float64
	// M7D margin filters.
	minMarginBalance, maxMarginBalance       *float64
	minMarginChange, maxMarginChange         *float64
	minShortBalance, maxShortBalance         *float64
	minShortChange, maxShortChange           *float64
	minShortMarginRatio, maxShortMarginRatio *float64
	sort                                     string
	order                                    string // "asc" | "desc"
}

var taiwanScreenerSortKeys = map[string]bool{
	"price": true, "change_percent": true, "volume": true, "amount": true,
	"foreign_net": true, "trust_net": true, "dealer_net": true, "institutional_net": true,
	"margin_balance": true, "margin_change": true, "short_balance": true, "short_change": true, "short_margin_ratio": true,
}

// taiwanScreenerNeedsInstitutional/taiwanScreenerNeedsMargin decide whether this specific request
// actually engages that domain (an active min/max filter, or a sort key from that domain). This is
// the sole gate for fetching institutional/margin data — a vanilla M7A-style request (no advanced
// params) must never trigger either, preserving the legacy daily-only request cost exactly.
func taiwanScreenerNeedsInstitutional(q taiwanScreenerQuery) bool {
	switch q.sort {
	case "foreign_net", "trust_net", "dealer_net", "institutional_net":
		return true
	}
	return q.minForeignNet != nil || q.maxForeignNet != nil ||
		q.minTrustNet != nil || q.maxTrustNet != nil ||
		q.minDealerNet != nil || q.maxDealerNet != nil ||
		q.minInstitutionalNet != nil || q.maxInstitutionalNet != nil
}

func taiwanScreenerNeedsMargin(q taiwanScreenerQuery) bool {
	switch q.sort {
	case "margin_balance", "margin_change", "short_balance", "short_change", "short_margin_ratio":
		return true
	}
	return q.minMarginBalance != nil || q.maxMarginBalance != nil ||
		q.minMarginChange != nil || q.maxMarginChange != nil ||
		q.minShortBalance != nil || q.maxShortBalance != nil ||
		q.minShortChange != nil || q.maxShortChange != nil ||
		q.minShortMarginRatio != nil || q.maxShortMarginRatio != nil
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

	// M7D lazy domain loading: institutional/margin are fetched ONLY when this request actually
	// engages them. A domain fetch failure (or an unconfigured provider) never fails the whole
	// request — it only leaves that domain's fields/freshness absent; base daily rows stay usable.
	var institutional map[string]foundation.InstitutionalFlow
	var institutionalFreshness foundation.TaiwanFreshness
	institutionalRequested := taiwanScreenerNeedsInstitutional(query)
	if institutionalRequested && s.taiwanScreenerInstitutional != nil {
		instRows, instFreshness, instErr := s.taiwanScreenerInstitutional.ScreenerInstitutional(ctx, time.Now())
		institutionalFreshness = instFreshness
		if instErr == nil {
			institutional = make(map[string]foundation.InstitutionalFlow, len(instRows))
			for _, row := range instRows {
				institutional[row.Canonical] = row
			}
		}
	}

	var margin map[string]foundation.MarginTrading
	var marginFreshness foundation.TaiwanFreshness
	marginRequested := taiwanScreenerNeedsMargin(query)
	if marginRequested && s.taiwanScreenerMargin != nil {
		marginRows, mFreshness, marginErr := s.taiwanScreenerMargin.ScreenerMargin(ctx, time.Now())
		marginFreshness = mFreshness
		if marginErr == nil {
			margin = make(map[string]foundation.MarginTrading, len(marginRows))
			for _, row := range marginRows {
				margin[row.Canonical] = row
			}
		}
	}

	filtered := filterAndSortTaiwanScreener(rows, institutional, margin, query)
	total := len(filtered)
	page := paginateTaiwanScreener(filtered, offset, limit)

	scopeLabel := "COMBINED"
	if query.scope != "" && query.scope != "combined" {
		scopeLabel = strings.ToUpper(query.scope)
	}
	response := taiwanScreenerResponse{
		Scope: scopeLabel, AsOf: freshness.DailyAsOf, Freshness: freshness.DailyStatus,
		Total: total, Offset: offset, Limit: limit, Securities: page,
	}
	if institutionalRequested {
		response.InstitutionalAsOf = institutionalFreshness.InstitutionalAsOf
		response.InstitutionalStatus = institutionalFreshness.InstitutionalStatus
		response.InstitutionalDaysBehind = institutionalFreshness.InstitutionalDaysBehind
		if response.InstitutionalStatus == "" {
			response.InstitutionalStatus = "unavailable"
		}
	}
	if marginRequested {
		response.MarginAsOf = marginFreshness.MarginAsOf
		response.MarginStatus = marginFreshness.MarginStatus
		response.MarginDaysBehind = marginFreshness.MarginDaysBehind
		if response.MarginStatus == "" {
			response.MarginStatus = "unavailable"
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": response})
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
	if !taiwanScreenerSortKeys[query.sort] {
		return query, 0, 0, "sort must be one of price, change_percent, volume, amount, foreign_net, trust_net, dealer_net, institutional_net, margin_balance, margin_change, short_balance, short_change, short_margin_ratio"
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

	// M7D institutional range params — reuse the exact same optional-float parsing and min<=max
	// validation already proven for the M7A fields above; no new validation logic invented.
	institutionalRanges := []struct {
		minKey, maxKey string
		min, max       **float64
		label          string
	}{
		{"min_foreign_net", "max_foreign_net", &query.minForeignNet, &query.maxForeignNet, "min_foreign_net must not be greater than max_foreign_net"},
		{"min_trust_net", "max_trust_net", &query.minTrustNet, &query.maxTrustNet, "min_trust_net must not be greater than max_trust_net"},
		{"min_dealer_net", "max_dealer_net", &query.minDealerNet, &query.maxDealerNet, "min_dealer_net must not be greater than max_dealer_net"},
		{"min_institutional_net", "max_institutional_net", &query.minInstitutionalNet, &query.maxInstitutionalNet, "min_institutional_net must not be greater than max_institutional_net"},
	}
	for _, item := range institutionalRanges {
		if *item.min, err = parseOptionalScreenerFloat(r, item.minKey); err != nil {
			return query, 0, 0, err.Error()
		}
		if *item.max, err = parseOptionalScreenerFloat(r, item.maxKey); err != nil {
			return query, 0, 0, err.Error()
		}
		if rangeInvalid(*item.min, *item.max) {
			return query, 0, 0, item.label
		}
	}

	// M7D margin range params — same pattern.
	marginRanges := []struct {
		minKey, maxKey string
		min, max       **float64
		label          string
	}{
		{"min_margin_balance", "max_margin_balance", &query.minMarginBalance, &query.maxMarginBalance, "min_margin_balance must not be greater than max_margin_balance"},
		{"min_margin_change", "max_margin_change", &query.minMarginChange, &query.maxMarginChange, "min_margin_change must not be greater than max_margin_change"},
		{"min_short_balance", "max_short_balance", &query.minShortBalance, &query.maxShortBalance, "min_short_balance must not be greater than max_short_balance"},
		{"min_short_change", "max_short_change", &query.minShortChange, &query.maxShortChange, "min_short_change must not be greater than max_short_change"},
		{"min_short_margin_ratio", "max_short_margin_ratio", &query.minShortMarginRatio, &query.maxShortMarginRatio, "min_short_margin_ratio must not be greater than max_short_margin_ratio"},
	}
	for _, item := range marginRanges {
		if *item.min, err = parseOptionalScreenerFloat(r, item.minKey); err != nil {
			return query, 0, 0, err.Error()
		}
		if *item.max, err = parseOptionalScreenerFloat(r, item.maxKey); err != nil {
			return query, 0, 0, err.Error()
		}
		if rangeInvalid(*item.min, *item.max) {
			return query, 0, 0, item.label
		}
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
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return nil, fmt.Errorf("%s must be a number", key)
	}
	return &value, nil
}

func rangeInvalid(min, max *float64) bool {
	return min != nil && max != nil && *min > *max
}

// filterAndSortTaiwanScreener applies scope/range filters and deterministic sorting entirely in
// memory over the already-fetched bulk snapshot rows (plus the optionally-fetched institutional/
// margin maps, each keyed by exact canonical) — no additional provider calls are made here.
//
// Filter semantics: a row is excluded from a given range filter only if that filter was actually
// requested (min/max present) AND the row's value for that field is unavailable (nil) — a filter
// that was never requested never excludes a row for that field, and an unavailable value is never
// treated as 0.
func filterAndSortTaiwanScreener(rows []foundation.TaiwanDailySnapshot, institutional map[string]foundation.InstitutionalFlow, margin map[string]foundation.MarginTrading, query taiwanScreenerQuery) []taiwanScreenerRow {
	filtered := make([]taiwanScreenerRow, 0, len(rows))
	for _, raw := range rows {
		if query.scope != "" && query.scope != "combined" && !strings.EqualFold(raw.Exchange, query.scope) {
			continue
		}
		row := toTaiwanScreenerRow(raw, institutional, margin)
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
		if !passesIntRange(row.ForeignNet, query.minForeignNet, query.maxForeignNet) {
			continue
		}
		if !passesIntRange(row.TrustNet, query.minTrustNet, query.maxTrustNet) {
			continue
		}
		if !passesIntRange(row.DealerNet, query.minDealerNet, query.maxDealerNet) {
			continue
		}
		if !passesIntRange(row.InstitutionalNet, query.minInstitutionalNet, query.maxInstitutionalNet) {
			continue
		}
		if !passesIntRange(row.MarginBalance, query.minMarginBalance, query.maxMarginBalance) {
			continue
		}
		if !passesIntRange(row.MarginChange, query.minMarginChange, query.maxMarginChange) {
			continue
		}
		if !passesIntRange(row.ShortBalance, query.minShortBalance, query.maxShortBalance) {
			continue
		}
		if !passesIntRange(row.ShortChange, query.minShortChange, query.maxShortChange) {
			continue
		}
		if !passesRange(row.ShortMarginRatio, query.minShortMarginRatio, query.maxShortMarginRatio) {
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

func toTaiwanScreenerRow(row foundation.TaiwanDailySnapshot, institutional map[string]foundation.InstitutionalFlow, margin map[string]foundation.MarginTrading) taiwanScreenerRow {
	out := taiwanScreenerRow{
		Canonical: row.Canonical, Code: row.Code, Name: row.Name, Exchange: row.Exchange, SecurityType: string(row.Type),
		TradeDate: row.TradeDate, Price: row.Close, Change: row.Change,
		ChangePercent: taiwanScreenerChangePercent(row.Close, row.Change),
		Volume:        row.Volume, Amount: row.Amount,
	}
	// A nil map (domain never fetched) and a present-but-missing canonical (security absent from
	// that date's bulk payload) both correctly leave every field nil below — indexing a nil map is
	// safe in Go and the two-value form still reports ok=false, so this never needs special-casing.
	if flow, ok := institutional[row.Canonical]; ok {
		foreign, trust, dealer := flow.ForeignNet, flow.InvestmentTrustNet, flow.DealerNet
		total := foreign + trust + dealer
		out.ForeignNet, out.TrustNet, out.DealerNet, out.InstitutionalNet = &foreign, &trust, &dealer, &total
	}
	if trading, ok := margin[row.Canonical]; ok {
		// MarginTrading's own fields are already *int64/*float64 (nil-safe at the provider layer),
		// reused directly — never recomputed here.
		out.MarginBalance, out.MarginChange = trading.MarginBalance, trading.MarginChange
		out.ShortBalance, out.ShortChange, out.ShortMarginRatio = trading.ShortBalance, trading.ShortChange, trading.ShortMarginRatio
	}
	return out
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
		return int64ToFloatPointer(row.Volume)
	case "amount":
		return row.Amount
	case "foreign_net":
		return int64ToFloatPointer(row.ForeignNet)
	case "trust_net":
		return int64ToFloatPointer(row.TrustNet)
	case "dealer_net":
		return int64ToFloatPointer(row.DealerNet)
	case "institutional_net":
		return int64ToFloatPointer(row.InstitutionalNet)
	case "margin_balance":
		return int64ToFloatPointer(row.MarginBalance)
	case "margin_change":
		return int64ToFloatPointer(row.MarginChange)
	case "short_balance":
		return int64ToFloatPointer(row.ShortBalance)
	case "short_change":
		return int64ToFloatPointer(row.ShortChange)
	case "short_margin_ratio":
		return row.ShortMarginRatio
	}
	return nil
}

func int64ToFloatPointer(value *int64) *float64 {
	if value == nil {
		return nil
	}
	converted := float64(*value)
	return &converted
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
