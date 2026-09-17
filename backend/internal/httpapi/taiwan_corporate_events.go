package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"easy-stock/backend/internal/foundation"
	"easy-stock/backend/internal/taiwanwatchlist"
)

type corporateEventSyncRequest struct {
	Symbols []string `json:"symbols"`
}

type corporateEventSyncItem struct {
	Feed   foundation.TaiwanCorporateEventFeed      `json:"feed"`
	Change taiwanwatchlist.CorporateEventSyncResult `json:"change"`
}

func (s *Server) taiwanCorporateEventsHandler(w http.ResponseWriter, r *http.Request) {
	if s.taiwanCorporateEvents == nil {
		writeError(w, http.StatusServiceUnavailable, "Taiwan corporate-event provider is unavailable")
		return
	}
	symbol := strings.ToUpper(strings.TrimSpace(r.PathValue("symbol")))
	if !validTaiwanCanonical(symbol) {
		writeError(w, http.StatusBadRequest, "canonical Taiwan symbol must end in .TWSE or .TPEX")
		return
	}
	ctx, cancel := contextWithTimeout(r, 15*time.Second)
	defer cancel()
	identity, statusCode, message := s.resolveTaiwanWatchlistSymbol(ctx, symbol)
	if message != "" {
		writeError(w, statusCode, message)
		return
	}
	if identity.Type != foundation.SecurityTypeStock {
		writeJSON(w, http.StatusOK, map[string]any{"data": unsupportedCorporateEventFeed("MOPS announcements are supported only for Taiwan stocks")})
		return
	}
	feed := s.taiwanCorporateEvents.CorporateEvents(ctx, symbol, 90, 12)
	writeJSON(w, http.StatusOK, map[string]any{"data": feed})
}

// taiwanCorporateEventSyncHandler supports both watchlist and portfolio flows.
// An empty symbols array means the saved Taiwan watchlist; callers such as a
// Taiwan portfolio can submit its canonical holdings explicitly. Provider
// outages remain data state and never erase the last successful observation.
func (s *Server) taiwanCorporateEventSyncHandler(w http.ResponseWriter, r *http.Request) {
	if s.taiwanCorporateEvents == nil || s.watchlistStore == nil {
		writeError(w, http.StatusServiceUnavailable, "Taiwan corporate-event tracking is unavailable")
		return
	}
	var request corporateEventSyncRequest
	if r.Body != nil {
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		if err := decoder.Decode(&request); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "invalid corporate-event sync request")
			return
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "invalid corporate-event sync request")
			return
		}
	}
	ctx, cancel := contextWithTimeout(r, 45*time.Second)
	defer cancel()
	symbols := normalizeTaiwanEventSymbols(request.Symbols)
	if len(request.Symbols) > 0 {
		for _, raw := range request.Symbols {
			if !validTaiwanCanonical(strings.ToUpper(strings.TrimSpace(raw))) {
				writeError(w, http.StatusBadRequest, "symbols must contain only canonical Taiwan stocks ending in .TWSE or .TPEX")
				return
			}
		}
		for _, symbol := range symbols {
			identity, statusCode, message := s.resolveTaiwanWatchlistSymbol(ctx, symbol)
			if message != "" {
				writeError(w, statusCode, message)
				return
			}
			if identity.Type != foundation.SecurityTypeStock {
				writeError(w, http.StatusBadRequest, "corporate-event sync supports only Taiwan stocks")
				return
			}
		}
	}
	if len(symbols) == 0 {
		entries, err := s.watchlistStore.List(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to read Taiwan watchlist")
			return
		}
		for _, entry := range entries {
			if entry.SecurityType == string(foundation.SecurityTypeStock) {
				symbols = append(symbols, entry.Canonical)
			}
		}
	}
	if len(symbols) > 20 {
		writeError(w, http.StatusBadRequest, "corporate-event sync supports at most 20 symbols")
		return
	}
	items := make(map[string]corporateEventSyncItem, len(symbols))
	var itemsMu sync.Mutex
	var firstErr error
	var errMu sync.Mutex
	sem := make(chan struct{}, 3)
	alertsEnabled := true
	if s.settingsStore != nil {
		alertsEnabled = s.settingsStore.Snapshot().TaiwanAlerts.CorporateEventsEnabled
	}
	var wg sync.WaitGroup
	for _, symbol := range symbols {
		symbol := symbol
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			feed := s.taiwanCorporateEvents.CorporateEvents(ctx, symbol, 90, 12)
			change, err := s.watchlistStore.ApplyCorporateEventsWithPreference(ctx, symbol, feed, time.Now().UTC(), alertsEnabled)
			if err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errMu.Unlock()
				return
			}
			itemsMu.Lock()
			items[symbol] = corporateEventSyncItem{Feed: feed, Change: change}
			itemsMu.Unlock()
		}()
	}
	wg.Wait()
	if firstErr != nil {
		writeError(w, http.StatusInternalServerError, "failed to persist Taiwan corporate-event state")
		return
	}
	if ctx.Err() != nil && len(items) != len(symbols) {
		writeError(w, http.StatusGatewayTimeout, "Taiwan corporate-event sync timed out")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"symbols": symbols, "items": items}})
}

func unsupportedCorporateEventFeed(reason string) foundation.TaiwanCorporateEventFeed {
	return foundation.TaiwanCorporateEventFeed{Status: foundation.TaiwanCorporateEventsUnsupported, Provider: foundation.TaiwanCorporateEventProviderToAlpha, Source: foundation.TaiwanCorporateEventSourceMOPS, Reason: reason, Events: []foundation.TaiwanCorporateEvent{}}
}

func normalizeTaiwanEventSymbols(values []string) []string {
	set := map[string]struct{}{}
	for _, value := range values {
		value = strings.ToUpper(strings.TrimSpace(value))
		if validTaiwanCanonical(value) {
			set[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func validTaiwanCanonical(value string) bool {
	dot := strings.LastIndex(value, ".")
	return dot > 0 && dot < len(value)-1 && (value[dot+1:] == "TWSE" || value[dot+1:] == "TPEX")
}
