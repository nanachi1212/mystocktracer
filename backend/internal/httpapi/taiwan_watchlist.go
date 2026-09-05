package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"easy-stock/backend/internal/foundation"
	"easy-stock/backend/internal/taiwanwatchlist"
)

// taiwanWatchlistSecurity is the wire shape of one saved Taiwan security.
// Quote/price data is intentionally absent — the watchlist persists identity
// only; live prices remain backend-authoritative via /api/v1/tw/quotes (M6B).
type taiwanWatchlistSecurity struct {
	Canonical    string    `json:"canonical"`
	Code         string    `json:"code"`
	Name         string    `json:"name"`
	Exchange     string    `json:"exchange"`
	SecurityType string    `json:"security_type"`
	CreatedAt    time.Time `json:"created_at"`
}

func toTaiwanWatchlistSecurity(entry taiwanwatchlist.Entry) taiwanWatchlistSecurity {
	return taiwanWatchlistSecurity{
		Canonical: entry.Canonical, Code: entry.Code, Name: entry.Name,
		Exchange: entry.Exchange, SecurityType: entry.SecurityType, CreatedAt: entry.CreatedAt,
	}
}

func (s *Server) taiwanWatchlistListHandler(w http.ResponseWriter, r *http.Request) {
	if s.watchlistStore == nil {
		writeError(w, http.StatusServiceUnavailable, "Taiwan watchlist storage is unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	entries, err := s.watchlistStore.List(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load Taiwan watchlist")
		return
	}
	securities := make([]taiwanWatchlistSecurity, 0, len(entries))
	for _, entry := range entries {
		securities = append(securities, toTaiwanWatchlistSecurity(entry))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"securities": securities}})
}

type taiwanWatchlistAddRequest struct {
	Symbol string `json:"symbol"`
}

func (s *Server) taiwanWatchlistAddHandler(w http.ResponseWriter, r *http.Request) {
	if s.watchlistStore == nil {
		writeError(w, http.StatusServiceUnavailable, "Taiwan watchlist storage is unavailable")
		return
	}
	if s.taiwanDirectory == nil {
		writeError(w, http.StatusServiceUnavailable, "Taiwan directory provider is unavailable")
		return
	}
	var payload taiwanWatchlistAddRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "malformed request body")
		return
	}
	identity, statusCode, message := s.resolveTaiwanWatchlistSymbol(r.Context(), payload.Symbol)
	if message != "" {
		writeError(w, statusCode, message)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	saved, err := s.watchlistStore.Add(ctx, taiwanwatchlist.Entry{
		Canonical: identity.Canonical, Code: identity.Code, Name: identity.Name,
		Exchange: identity.Exchange, SecurityType: string(identity.Type),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save Taiwan watchlist entry")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"security": toTaiwanWatchlistSecurity(saved)}})
}

func (s *Server) taiwanWatchlistRemoveHandler(w http.ResponseWriter, r *http.Request) {
	if s.watchlistStore == nil {
		writeError(w, http.StatusServiceUnavailable, "Taiwan watchlist storage is unavailable")
		return
	}
	symbol := strings.ToUpper(strings.TrimSpace(r.PathValue("symbol")))
	if symbol == "" {
		writeError(w, http.StatusBadRequest, "canonical Taiwan symbol is required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	// Removing an already-absent symbol is treated as a safe no-op (idempotent),
	// not a 404 — the end state the caller wants (symbol not in the watchlist) is
	// already true, and the watchlist is the sole owner of this identity.
	if err := s.watchlistStore.Remove(ctx, symbol); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove Taiwan watchlist entry")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"canonical": symbol, "removed": true}})
}

// resolveTaiwanWatchlistSymbol validates and resolves a client-supplied symbol
// against the backend-authoritative Taiwan security directory. The client's
// name/exchange/security_type (if any) are never trusted for persistence —
// only the resolved directory identity is saved.
//
// Returns a non-empty message (with statusCode) on any failure, distinguishing:
//   - malformed symbol (empty, or no recognizable {code}.{EXCHANGE} shape)
//   - unsupported exchange (well-formed but not TWSE/TPEX)
//   - not found (well-formed TWSE/TPEX shape, but absent from the directory)
//   - upstream directory failure (surfaced the same way taiwanSecurity() already does)
func (s *Server) resolveTaiwanWatchlistSymbol(ctx context.Context, rawSymbol string) (*foundation.SecurityIdentity, int, string) {
	symbol := strings.ToUpper(strings.TrimSpace(rawSymbol))
	if symbol == "" {
		return nil, http.StatusBadRequest, "symbol is required"
	}
	dot := strings.LastIndex(symbol, ".")
	if dot <= 0 || dot == len(symbol)-1 {
		return nil, http.StatusBadRequest, "malformed Taiwan security symbol; expected {code}.{TWSE|TPEX}"
	}
	exchange := symbol[dot+1:]
	if exchange != "TWSE" && exchange != "TPEX" {
		return nil, http.StatusBadRequest, "unsupported exchange; Taiwan securities must be TWSE or TPEX"
	}
	items, _, _, _, err := s.taiwanDirectories.load(ctx, s.taiwanDirectory)
	if err != nil {
		return nil, http.StatusBadGateway, err.Error()
	}
	for i := range items {
		if strings.EqualFold(items[i].Canonical, symbol) {
			identity := items[i]
			return &identity, 0, ""
		}
	}
	return nil, http.StatusNotFound, "Taiwan security not found"
}
