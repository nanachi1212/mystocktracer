package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"easy-stock/backend/internal/foundation"
)

func (s *Server) taiwanSecurity(ctxQuery string, r *http.Request) (foundation.SecurityIdentity, error) {
	items, _, _, _, err := s.taiwanDirectories.load(r.Context(), s.taiwanDirectory)
	if err != nil {
		return foundation.SecurityIdentity{}, err
	}
	matches := filterTaiwanDirectory(items, strings.TrimSpace(ctxQuery))
	if len(matches) != 1 {
		return foundation.SecurityIdentity{}, fmt.Errorf("Taiwan security must resolve to one canonical identity; got %d", len(matches))
	}
	return matches[0], nil
}

func (s *Server) taiwanQuotesHandler(w http.ResponseWriter, r *http.Request) {
	if s.taiwanMarket == nil {
		writeError(w, http.StatusServiceUnavailable, "Taiwan market provider is unavailable")
		return
	}
	raw := strings.TrimSpace(r.URL.Query().Get("symbols"))
	if raw == "" {
		writeError(w, http.StatusBadRequest, "symbols is required")
		return
	}
	parts := strings.Split(raw, ",")
	if len(parts) > 10 {
		writeError(w, http.StatusBadRequest, "at most 10 Taiwan symbols are supported")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	quotes := make([]foundation.Quote, 0, len(parts))
	for _, part := range parts {
		security, err := s.taiwanSecurity(part, r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		quote, err := s.taiwanMarket.Quote(ctx, security)
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		quotes = append(quotes, quote)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": quotes})
}

func (s *Server) taiwanKLineHandler(w http.ResponseWriter, r *http.Request) {
	if s.taiwanMarket == nil {
		writeError(w, http.StatusServiceUnavailable, "Taiwan market provider is unavailable")
		return
	}
	security, err := s.taiwanSecurity(r.URL.Query().Get("symbol"), r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	limit, err := marketLimitQuery(r, 120, 240)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	lines, err := s.taiwanMarket.KLine(ctx, security, limit)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": lines})
}

func (s *Server) taiwanIndexesHandler(w http.ResponseWriter, r *http.Request) {
	if s.taiwanMarket == nil {
		writeError(w, http.StatusServiceUnavailable, "Taiwan market provider is unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	data, meta, err := s.taiwanMarket.Indexes(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "meta": meta})
}

func (s *Server) taiwanInstitutionalHandler(w http.ResponseWriter, r *http.Request) {
	if s.taiwanChip == nil {
		writeError(w, http.StatusServiceUnavailable, "Taiwan chip provider is unavailable")
		return
	}
	security, err := s.taiwanSecurity(r.URL.Query().Get("symbol"), r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	limit, err := marketLimitQuery(r, 20, 60)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	data, err := s.taiwanChip.Institutional(ctx, security, limit)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (s *Server) taiwanMarginHandler(w http.ResponseWriter, r *http.Request) {
	if s.taiwanChip == nil {
		writeError(w, http.StatusServiceUnavailable, "Taiwan chip provider is unavailable")
		return
	}
	security, err := s.taiwanSecurity(r.URL.Query().Get("symbol"), r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	limit, err := marketLimitQuery(r, 20, 60)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	data, err := s.taiwanChip.Margin(ctx, security, limit)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (s *Server) taiwanDataStatusHandler(w http.ResponseWriter, _ *http.Request) {
	if s.taiwanSnapshot == nil {
		writeError(w, http.StatusServiceUnavailable, "Taiwan snapshot provider is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": s.taiwanSnapshot.Freshness(time.Now())})
}

func (s *Server) taiwanMarketBreadthHandler(w http.ResponseWriter, r *http.Request) {
	if s.taiwanBreadth == nil {
		writeError(w, http.StatusServiceUnavailable, "Taiwan market breadth provider is unavailable")
		return
	}
	scope := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("scope")))
	if scope != "" && scope != "twse" && scope != "tpex" && scope != "combined" {
		writeError(w, http.StatusBadRequest, "scope must be twse, tpex, or combined")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	data, err := s.taiwanBreadth.MarketBreadth(ctx, time.Now())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	switch scope {
	case "":
		writeJSON(w, http.StatusOK, map[string]any{"data": data})
	case "twse":
		writeJSON(w, http.StatusOK, map[string]any{"data": data.TWSE})
	case "tpex":
		writeJSON(w, http.StatusOK, map[string]any{"data": data.TPEX})
	case "combined":
		writeJSON(w, http.StatusOK, map[string]any{"data": data.Combined})
	}
}

func (s *Server) taiwanMarketEmotionHandler(w http.ResponseWriter, r *http.Request) {
	if s.taiwanEmotion == nil {
		writeError(w, http.StatusServiceUnavailable, "Taiwan market emotion provider is unavailable")
		return
	}
	scope := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("scope")))
	if scope != "" && scope != "twse" && scope != "tpex" && scope != "combined" {
		writeError(w, http.StatusBadRequest, "scope must be twse, tpex, or combined")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	data, err := s.taiwanEmotion.MarketEmotion(ctx, time.Now())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	switch scope {
	case "":
		writeJSON(w, http.StatusOK, map[string]any{"data": data})
	case "twse":
		writeJSON(w, http.StatusOK, map[string]any{"data": data.TWSE})
	case "tpex":
		writeJSON(w, http.StatusOK, map[string]any{"data": data.TPEX})
	case "combined":
		writeJSON(w, http.StatusOK, map[string]any{"data": data.Combined})
	}
}

func (s *Server) taiwanIndustryRadarHandler(w http.ResponseWriter, r *http.Request) {
	if s.taiwanIndustryRadar == nil {
		writeError(w, http.StatusServiceUnavailable, "Taiwan industry radar provider is unavailable")
		return
	}
	scope := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("scope")))
	if scope != "" && scope != "twse" && scope != "tpex" && scope != "combined" {
		writeError(w, http.StatusBadRequest, "scope must be twse, tpex, or combined")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	data, err := s.taiwanIndustryRadar.IndustryRadar(ctx, time.Now())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	switch scope {
	case "":
		writeJSON(w, http.StatusOK, map[string]any{"data": data})
	case "twse":
		writeJSON(w, http.StatusOK, map[string]any{"data": data.TWSE})
	case "tpex":
		writeJSON(w, http.StatusOK, map[string]any{"data": data.TPEX})
	case "combined":
		writeJSON(w, http.StatusOK, map[string]any{"data": data.Combined})
	}
}

func (s *Server) taiwanStockIntelligenceHandler(w http.ResponseWriter, r *http.Request) {
	if s.taiwanIntelligence == nil {
		writeError(w, http.StatusServiceUnavailable, "Taiwan stock intelligence provider is unavailable")
		return
	}
	symbol := strings.TrimSpace(r.PathValue("symbol"))
	if symbol == "" {
		writeError(w, http.StatusBadRequest, "canonical Taiwan symbol is required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	data, err := s.taiwanIntelligence.StockIntelligence(ctx, symbol, time.Now())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (s *Server) taiwanFundamentalsHandler(w http.ResponseWriter, r *http.Request) {
	if s.taiwanFundamentals == nil {
		writeError(w, http.StatusServiceUnavailable, "Taiwan fundamentals provider is unavailable")
		return
	}
	security, err := s.taiwanSecurity(r.URL.Query().Get("symbol"), r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	months := 24
	if raw := strings.TrimSpace(r.URL.Query().Get("months")); raw != "" {
		months, err = strconv.Atoi(raw)
		if err != nil || months < 1 || months > 36 {
			writeError(w, http.StatusBadRequest, "months must be between 1 and 36")
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	data, err := s.taiwanFundamentals.Fundamentals(ctx, security, months)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}
