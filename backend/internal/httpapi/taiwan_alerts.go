package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nanachi1212/mystocktracer/backend/internal/taiwanwatchlist"
)

const (
	defaultTaiwanAlertLimit = 50
	maxTaiwanAlertLimit     = 100
	maxTaiwanAlertOffset    = 10000
)

func (s *Server) taiwanAlertsListHandler(w http.ResponseWriter, r *http.Request) {
	if s.watchlistStore == nil {
		writeError(w, http.StatusServiceUnavailable, "Taiwan alert storage is unavailable")
		return
	}
	limit, ok := boundedAlertQueryInt(w, r, "limit", defaultTaiwanAlertLimit, 1, maxTaiwanAlertLimit)
	if !ok {
		return
	}
	offset, ok := boundedAlertQueryInt(w, r, "offset", 0, 0, maxTaiwanAlertOffset)
	if !ok {
		return
	}
	filter := strings.TrimSpace(r.URL.Query().Get("status"))
	if filter == "" {
		filter = taiwanwatchlist.AlertFilterAll
	}
	if filter != taiwanwatchlist.AlertFilterAll && filter != taiwanwatchlist.AlertFilterUnread && filter != taiwanwatchlist.AlertFilterRead {
		writeError(w, http.StatusBadRequest, "status must be all, unread, or read")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	page, err := s.watchlistStore.ListAlerts(ctx, filter, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load Taiwan alerts")
		return
	}
	enabled := true
	if s.settingsStore != nil {
		enabled = s.settingsStore.Snapshot().TaiwanAlerts.CorporateEventsEnabled
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"alerts": page.Alerts, "unread_count": page.UnreadCount, "total": page.Total,
		"limit": page.Limit, "offset": page.Offset, "corporate_events_enabled": enabled,
	}})
}

func (s *Server) taiwanAlertReadHandler(w http.ResponseWriter, r *http.Request) {
	if s.watchlistStore == nil {
		writeError(w, http.StatusServiceUnavailable, "Taiwan alert storage is unavailable")
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("id")), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "alert id must be a positive integer")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	updated, err := s.watchlistStore.MarkAlertRead(ctx, id, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to mark Taiwan alert read")
		return
	}
	if !updated {
		writeError(w, http.StatusNotFound, "Taiwan alert not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"id": id, "read": true}})
}

func (s *Server) taiwanAlertsReadAllHandler(w http.ResponseWriter, r *http.Request) {
	if s.watchlistStore == nil {
		writeError(w, http.StatusServiceUnavailable, "Taiwan alert storage is unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	updated, err := s.watchlistStore.MarkAllAlertsRead(ctx, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to mark all Taiwan alerts read")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"updated": updated}})
}

func boundedAlertQueryInt(w http.ResponseWriter, r *http.Request, key string, fallback, minimum, maximum int) (int, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minimum || value > maximum {
		writeError(w, http.StatusBadRequest, key+" is outside the allowed range")
		return 0, false
	}
	return value, true
}
