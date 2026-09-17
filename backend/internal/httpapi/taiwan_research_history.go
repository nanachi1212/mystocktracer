package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"easy-stock/backend/internal/stockanalysis"
	"easy-stock/backend/internal/taiwanwatchlist"
)

const (
	defaultTaiwanResearchHistoryLimit = 20
	maxTaiwanResearchHistoryLimit     = 100
	maxTaiwanResearchHistoryOffset    = 10000
)

var researchRunIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,80}$`)

func researchRunID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value != "" {
		if !researchRunIDPattern.MatchString(value) {
			return "", errors.New("X-Research-Run-ID must contain 16-80 letters, numbers, underscores, or hyphens")
		}
		return value, nil
	}
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate research run id: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}

func validateHistoryRunID(value string) (string, bool) {
	value = strings.TrimSpace(value)
	return value, researchRunIDPattern.MatchString(value)
}

func (s *Server) taiwanResearchHistoryListHandler(w http.ResponseWriter, r *http.Request) {
	if s.watchlistStore == nil {
		writeError(w, http.StatusServiceUnavailable, "Taiwan research history storage is unavailable")
		return
	}
	canonical := strings.ToUpper(strings.TrimSpace(r.PathValue("symbol")))
	if canonical == "" {
		writeError(w, http.StatusBadRequest, "canonical Taiwan symbol is required")
		return
	}
	limit, ok := boundedResearchHistoryQueryInt(w, r, "limit", defaultTaiwanResearchHistoryLimit, 1, maxTaiwanResearchHistoryLimit)
	if !ok {
		return
	}
	offset, ok := boundedResearchHistoryQueryInt(w, r, "offset", 0, 0, maxTaiwanResearchHistoryOffset)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	page, err := s.watchlistStore.ListResearchHistory(ctx, canonical, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load Taiwan research history")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": page})
}

func (s *Server) taiwanResearchHistoryGetHandler(w http.ResponseWriter, r *http.Request) {
	if s.watchlistStore == nil {
		writeError(w, http.StatusServiceUnavailable, "Taiwan research history storage is unavailable")
		return
	}
	canonical := strings.ToUpper(strings.TrimSpace(r.PathValue("symbol")))
	runID, ok := validateHistoryRunID(r.PathValue("runID"))
	if canonical == "" || !ok {
		writeError(w, http.StatusBadRequest, "canonical symbol and valid research run id are required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	record, err := s.watchlistStore.GetResearchHistory(ctx, canonical, runID)
	if errors.Is(err, taiwanwatchlist.ErrResearchHistoryNotFound) {
		writeError(w, http.StatusNotFound, "Taiwan research history not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load Taiwan research history")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": record})
}

func (s *Server) taiwanResearchHistoryComparisonHandler(w http.ResponseWriter, r *http.Request) {
	if s.watchlistStore == nil {
		writeError(w, http.StatusServiceUnavailable, "Taiwan research history storage is unavailable")
		return
	}
	canonical := strings.ToUpper(strings.TrimSpace(r.PathValue("symbol")))
	runID, ok := validateHistoryRunID(r.PathValue("runID"))
	if canonical == "" || !ok {
		writeError(w, http.StatusBadRequest, "canonical symbol and valid research run id are required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	current, err := s.watchlistStore.GetResearchHistory(ctx, canonical, runID)
	if errors.Is(err, taiwanwatchlist.ErrResearchHistoryNotFound) {
		writeError(w, http.StatusNotFound, "Taiwan research history not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load Taiwan research history")
		return
	}
	previous, err := s.watchlistStore.PreviousResearchHistory(ctx, canonical, runID)
	if errors.Is(err, taiwanwatchlist.ErrResearchHistoryNotFound) {
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"available": false, "reason": "no previous successful research", "current_run_id": runID}})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load previous Taiwan research history")
		return
	}
	comparison := stockanalysis.CompareTaiwanResearchHistory(current.RunID, previous.RunID, current.ResearchVersion, previous.ResearchVersion, current.PayloadVersion, previous.PayloadVersion, current.EvidenceSnapshot, previous.EvidenceSnapshot, current.ResearchResult, previous.ResearchResult)
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"available": true, "current_run_id": current.RunID, "previous_run_id": previous.RunID, "comparison": comparison}})
}

func boundedResearchHistoryQueryInt(w http.ResponseWriter, r *http.Request, key string, fallback, minimum, maximum int) (int, bool) {
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
